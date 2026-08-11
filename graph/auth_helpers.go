package graph

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	ethcrypto "github.com/ethereum/go-ethereum/crypto"

	"github.com/computeflux/tee-provider/pkg/api/gateway"
	"github.com/computeflux/tee-provider/pkg/types"
	"github.com/computeflux/tee-provider/pkg/types/chaintypes"
	teechain "github.com/computeflux/tee-provider/tee-chain"
)

// randReadFn 是 crypto/rand.Read 的可测试替换点。
var randReadFn = rand.Read

// generateAPIKey generates a new API key in the format auth_tee_<64 hex chars>
func generateAPIKey() (string, error) {
	b := make([]byte, 32)
	if _, err := randReadFn(b); err != nil {
		return "", fmt.Errorf("failed to generate API key entropy: %w", err)
	}
	return "auth_tee_" + hex.EncodeToString(b), nil
}

// generateWalletNonce generates a random nonce for wallet login.
func generateWalletNonce() (string, error) {
	b := make([]byte, 16)
	if _, err := randReadFn(b); err != nil {
		return "", fmt.Errorf("failed to generate wallet nonce entropy: %w", err)
	}
	return hex.EncodeToString(b), nil
}

// generateRefreshToken generates a new refresh token in the format refresh_tee_<64 hex chars>
func generateRefreshToken() (string, error) {
	b := make([]byte, 32)
	if _, err := randReadFn(b); err != nil {
		return "", fmt.Errorf("failed to generate refresh token entropy: %w", err)
	}
	return "refresh_tee_" + hex.EncodeToString(b), nil
}

// createRefreshToken creates and stores a new refresh token for the given user.
// Refresh token 有效期较长（默认 30 天），用于在 API Key 过期后换取新的 API Key。
func createRefreshToken(owner string, address string) (string, error) {
	refreshToken, err := generateRefreshToken()
	if err != nil {
		return "", err
	}
	token := &types.RefreshTokenInfo{
		Owner:     owner,
		Address:   address,
		IsActive:  true,
		CreatedAt: uint64(time.Now().Unix()),
		ExpiresAt: uint64(time.Now().Add(30 * 24 * time.Hour).Unix()),
	}
	if err := types.SetRefreshToken(refreshToken, token); err != nil {
		return "", fmt.Errorf("failed to store refresh token: %w", err)
	}
	return refreshToken, nil
}

// createUserAPIKey creates and stores a new API Key for the given user.
// addrHex 为 Ethereum 地址 hex 字符串（如 "0xabc..."），用于结算时构造 UniAddr。
func createUserAPIKey(owner string, addrHex string) (string, error) {
	apiKey, err := generateAPIKey()
	if err != nil {
		return "", err
	}
	nextID, err := types.AllocNextID("gateway_token")
	if err != nil {
		return "", fmt.Errorf("allocate gateway token id: %w", err)
	}

	// 构造 UniAddr：以太坊地址 callerType=2（EVM），去掉 0x 前缀
	addr := chaintypes.UniAddr{T: 2, V: []byte{}}
	if addrHex != "" && addrHex != "0x" {
		addrHex = strings.TrimPrefix(addrHex, "0x")
		if b, err := hex.DecodeString(addrHex); err == nil && len(b) == 20 {
			addr.V = b
		}
	}

	token := &types.GatewayTokenInfo{
		ID:        nextID,
		Owner:     owner,
		Addr:      addr,
		Key:       apiKey,
		RateLimit: 60,
		IsActive:  true,
		CreatedAt: uint64(time.Now().Unix()),
		ExpiresAt: uint64(time.Now().Add(24 * time.Hour).Unix()), // 默认1天过期
	}
	if err := types.SetGatewayTokenByKey(token); err != nil {
		return "", fmt.Errorf("failed to store gateway token: %w", err)
	}

	return apiKey, nil
}

// resolveEmailFromToken resolves user email from an API Key or sig_ token.
func resolveEmailFromToken(token string) (string, error) {
	if len(token) > 4 && token[:4] == "sig_" {
		session, err := types.GetSignSession(token)
		if err != nil || session == nil {
			return "", fmt.Errorf("invalid sign token")
		}
		if time.Now().Unix() > session.ExpiresAt {
			types.DeleteSignSession(token)
			return "", fmt.Errorf("sign token expired")
		}
		return session.Email, nil
	}

	// 优先从合约查询 API Key
	if tk := gateway.LookupAPIKeyViaContract(token); tk != nil && tk.IsActive {
		// 合约查询不携带过期时间，补充本地过期检查确保一致性
		if localTk, _ := types.GetGatewayTokenByKey(token); localTk != nil && localTk.IsExpired() {
			_ = types.DeleteGatewayTokenByKey(token)
			return "", fmt.Errorf("token expired")
		}
		return tk.Owner, nil
	}

	// Fallback: PebbleDB（Web3Auth 登录生成的临时 key）
	tk, _ := types.GetGatewayTokenByKey(token)
	if tk != nil {
		if tk.IsExpired() {
			_ = types.DeleteGatewayTokenByKey(token)
			return "", fmt.Errorf("token expired")
		}
		if tk.IsActive {
			return tk.Owner, nil
		}
	}

	return "", fmt.Errorf("unauthorized")
}

// computeHMAC 计算 HMAC-SHA256，key 和 msg 都是 hex 字符串
func computeHMAC(keyHex, msgHex string) string {
	key, _ := hex.DecodeString(keyHex)
	msg, _ := hex.DecodeString(msgHex)
	mac := hmac.New(sha256.New, key)
	mac.Write(msg)
	return hex.EncodeToString(mac.Sum(nil))
}

// verifyEIP191Signature 验证 MetaMask 等钱包的 EIP-191 personal_sign 签名。
func verifyEIP191Signature(address, signatureHex, message string) bool {
	sigBytes, err := hex.DecodeString(strings.TrimPrefix(signatureHex, "0x"))
	if err != nil || len(sigBytes) != 65 {
		return false
	}

	// EIP-191: "\x19Ethereum Signed Message:\n" + len(message) + message
	msg := fmt.Sprintf("\x19Ethereum Signed Message:\n%d%s", len(message), message)
	hash := ethcrypto.Keccak256Hash([]byte(msg))

	// Transform V from 27/28 to 0/1 for recovery
	if sigBytes[64] >= 27 {
		sigBytes[64] -= 27
	}

	pubKey, err := ethcrypto.SigToPub(hash.Bytes(), sigBytes)
	if err != nil {
		return false
	}

	recoveredAddr := ethcrypto.PubkeyToAddress(*pubKey).Hex()
	return strings.EqualFold(recoveredAddr, address)
}

// getAuthUserFromContract 读取 AuthUser。
// 链上合约为数据源，本地缓存仅在异步上链窗口期内回退。
func getAuthUserFromContract(tc *teechain.TeeChain, email string) (*types.AuthUser, error) {
	// 1. 链上合约为数据源（ref 内部按 HashEmailKey 哈希，与 persistAuthUserToChain 写入侧一致）
	if tc != nil {
		ref := "auth_user:" + types.HashEmailKey(email)
		rec, err := teechain.AuthGetAuthUser(tc, ref)
		if err == nil && rec != nil && len(rec.Ciphertext) > 0 {
			var chainUser types.AuthUser
			if json.Unmarshal(rec.Ciphertext, &chainUser) == nil {
				return &chainUser, nil
			}
		}
	}

	// 2. 回退到本地缓存（异步上链尚未完成的短暂窗口；key 按 HashEmailKey 哈希）
	localUser, _ := types.DBGetJson[types.AuthUser](types.SECRET_DB, types.Web3AuthNamespace, "u:"+types.HashEmailKey(email))
	if localUser != nil {
		return localUser, nil
	}
	return nil, fmt.Errorf("auth user not found")
}

// ── GraphQL 返回类型 ──────────────────────────────────────────

// AuthResponse 认证操作返回
type AuthResponse struct {
	Success      bool      `json:"success"`
	UserInfo     *UserInfo `json:"userInfo,omitempty"`
	Token        string    `json:"token,omitempty"`
	RefreshToken string    `json:"refreshToken,omitempty"`
	Address      string    `json:"address,omitempty"`
	Error        string    `json:"error,omitempty"`
}

// UserInfo 用户信息
type UserInfo struct {
	Email                   string `json:"email"`
	Name                    string `json:"name,omitempty"`
	ProfileImage            string `json:"profileImage,omitempty"`
	Verifier                string `json:"verifier,omitempty"`
	VerifierID              string `json:"verifierId,omitempty"`
	TypeOfLogin             string `json:"typeOfLogin,omitempty"`
	AggregateVerifier       string `json:"aggregateVerifier,omitempty"`
	AuthConnection          string `json:"authConnection,omitempty"`
	AuthConnectionID        string `json:"authConnectionId,omitempty"`
	GroupedAuthConnectionID string `json:"groupedAuthConnectionId,omitempty"`
	IDToken                 string `json:"idToken,omitempty"`
}

// OAuthURL OAuth 授权 URL
type OAuthURL struct {
	AuthUrl string `json:"authUrl"`
}

// WalletResult 钱包操作结果
type WalletResult struct {
	Status string `json:"status"`
	Result string `json:"result,omitempty"`
	Error  string `json:"error,omitempty"`
}

// RecoveryResult 恢复操作结果
type RecoveryResult struct {
	Status        string `json:"status"`
	RecoveryToken string `json:"recoveryToken,omitempty"`
	Token         string `json:"token,omitempty"`
	RefreshToken  string `json:"refreshToken,omitempty"`
	Address       string `json:"address,omitempty"`
	Email         string `json:"email,omitempty"`
	Error         string `json:"error,omitempty"`
}

// ── GraphQL 输入类型 ──────────────────────────────────────────

// TxInput 交易签名输入
type TxInput struct {
	From         string `json:"from"`
	To           string `json:"to"`
	Value        string `json:"value,omitempty"`
	Data         string `json:"data,omitempty"`
	Gas          string `json:"gas,omitempty"`
	GasPrice     string `json:"gasPrice,omitempty"`
	Nonce        string `json:"nonce,omitempty"`
	ChainID      string `json:"chainId,omitempty"`
	PasswordHash string `json:"passwordHash,omitempty"`
}
