package graph

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	ethcrypto "github.com/ethereum/go-ethereum/crypto"

	"github.com/computeflux/tee-provider/pkg/api/gateway"
	"github.com/computeflux/tee-provider/pkg/model"
)

// generateAPIKey generates a new API key in the format tee_<64 hex chars>
func generateAPIKey() string {
	b := make([]byte, 32)
	rand.Read(b)
	return "auth_tee_" + hex.EncodeToString(b)
}

// createUserAPIKey creates and stores a new API Key for the given user.
// addrHex 为 Ethereum 地址 hex 字符串（如 "0xabc..."），用于结算时构造 UniAddr。
func createUserAPIKey(owner string, addrHex string) string {
	apiKey := generateAPIKey()
	nextID := model.GetNextID("gateway_token")

	// 构造 UniAddr：以太坊地址 callerType=2（EVM），去掉 0x 前缀
	addr := model.UniAddr{T: 2, V: []byte{}}
	if addrHex != "" && addrHex != "0x" {
		addrHex = strings.TrimPrefix(addrHex, "0x")
		if b, err := hex.DecodeString(addrHex); err == nil && len(b) == 20 {
			addr.V = b
		}
	}

	token := &model.GatewayTokenInfo{
		Owner:     owner,
		Addr:      addr,
		Key:       apiKey,
		RateLimit: 60,
		IsActive:  true,
		CreatedAt: uint64(time.Now().Unix()),
		ExpiresAt: uint64(time.Now().Add(24 * time.Hour).Unix()), // 默认1天过期
	}
	model.SetGatewayToken(nextID, token)
	model.SetGatewayTokenByKey(token) // 同时写入 key 索引，供 O(1) 查询
	model.SetNextID("gateway_token", nextID+1)
	gateway.InvalidateCache()
	return apiKey
}

// resolveEmailFromToken resolves user email from an API Key or sig_ token.
func resolveEmailFromToken(token string) (string, error) {
	if len(token) > 4 && token[:4] == "sig_" {
		session, err := model.GetSignSession(token)
		if err != nil || session == nil {
			return "", fmt.Errorf("invalid sign token")
		}
		if time.Now().Unix() > session.ExpiresAt {
			model.DeleteSignSession(token)
			return "", fmt.Errorf("sign token expired")
		}
		return session.Email, nil
	}

	// 优先从合约查询 API Key
	if tk := gateway.LookupAPIKeyViaContract(token); tk != nil && tk.IsActive {
		return tk.Owner, nil
	}

	// Fallback: PebbleDB（Web3Auth 登录生成的临时 key）
	tk, _ := model.GetGatewayTokenByKey(token)
	if tk != nil {
		if tk.IsExpired() {
			return "", fmt.Errorf("token expired")
		}
		if tk.IsActive {
			return tk.Owner, nil
		}
	}

	return "", fmt.Errorf("unauthorized")
}

// strPtr 返回字符串指针
func strPtr(s string) *string { return &s }

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
