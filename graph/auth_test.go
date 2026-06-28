package graph

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/computeflux/tee-provider/pkg/services/auth"
	"github.com/computeflux/tee-provider/pkg/services/wallet"
	"github.com/computeflux/tee-provider/pkg/types"
	teechain "github.com/computeflux/tee-provider/tee-chain"
	ethcrypto "github.com/ethereum/go-ethereum/crypto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// setupAuthTest 初始化内存数据库和认证服务，返回 resolver 和 cleanup 函数
func setupAuthTest(t *testing.T, seedSecret string) (*Resolver, func()) {
	db, err := types.NewDBWithPath(t.TempDir())
	require.NoError(t, err)

	auth.SetCachedConfigForTest(&auth.AuthConfig{
		SeedSecret: seedSecret,
		JWTSecret:  "test-jwt-secret",
	})

	walletSvc := wallet.NewWalletService(&teechain.TeeChain{})
	otpCtx, otpCancel := context.WithCancel(context.Background())

	resolver := &Resolver{
		TeeChain:  nil, // nil 时 getAuthUserFromContract 回退到本地存储
		WalletSvc: walletSvc,
		OTPSvc:    auth.NewOTPService(otpCtx, 5),
		EmailSvc:  auth.NewMockEmailService("test"),
	}

	cleanup := func() {
		otpCancel()
		auth.ResetConfigCache()
		_ = db.Close()
	}

	return resolver, cleanup
}

func TestVerifyOtp_NewUser_DerivesAddress(t *testing.T) {
	resolver, cleanup := setupAuthTest(t, "test-seed-secret-node-a")
	defer cleanup()

	email := "user@example.com"
	clientID := "test-client"

	// 生成 OTP 并写入存储
	otp, err := resolver.OTPSvc.Generate(email, clientID)
	require.NoError(t, err)

	m := &mutationResolver{Resolver: resolver}
	resp, err := m.VerifyOtp(context.Background(), email, otp, clientID)
	require.NoError(t, err)
	require.True(t, resp.Success, "新用户 OTP 验证应成功")
	require.NotEmpty(t, resp.Address, "新用户应派生钱包地址")
	require.NotEmpty(t, resp.Token, "新用户应获得 API Key")
	require.NotEmpty(t, resp.RefreshToken, "新用户应获得 Refresh Token")
	assert.True(t, strings.HasPrefix(resp.RefreshToken, "refresh_tee_"))

	// 验证本地存储已写入
	user, err := getAuthUserFromContract(nil, email)
	require.NoError(t, err)
	// Email 字段在 Ciphertext 中存放 hash
	assert.Equal(t, types.HashEmailKey(email), user.Email)
	assert.Equal(t, resp.Address, user.Address)
}

func TestVerifyOtp_ExistingUser_KeepsAddress(t *testing.T) {
	resolver, cleanup := setupAuthTest(t, "test-seed-secret-node-a")
	defer cleanup()

	email := "user@example.com"
	clientID := "test-client"

	// 第一次登录：派生地址
	otp1, _ := resolver.OTPSvc.Generate(email, clientID)
	m := &mutationResolver{Resolver: resolver}
	resp1, _ := m.VerifyOtp(context.Background(), email, otp1, clientID)
	addr1 := resp1.Address
	require.NotEmpty(t, addr1)

	// 等待异步上链 goroutine 完成（使用小睡眠确保本地写入完成）
	// 实际上本地写入是同步的，无需等待

	// 第二次登录：应使用已有地址
	otp2, _ := resolver.OTPSvc.Generate(email, clientID)
	resp2, err := m.VerifyOtp(context.Background(), email, otp2, clientID)
	require.NoError(t, err)
	require.True(t, resp2.Success)
	assert.Equal(t, addr1, resp2.Address, "已有用户再次登录时地址必须保持一致")
	require.NotEmpty(t, resp2.RefreshToken, "已有用户再次登录也应获得 Refresh Token")
}

func TestVerifyOtp_CrossNodeConsistency(t *testing.T) {
	// 节点 A：固定 SeedSecret
	resolverA, cleanupA := setupAuthTest(t, "test-seed-secret-node-a")
	defer cleanupA()

	email := "user@example.com"
	clientID := "test-client"

	// 在节点 A 注册
	otp1, _ := resolverA.OTPSvc.Generate(email, clientID)
	mA := &mutationResolver{Resolver: resolverA}
	respA, _ := mA.VerifyOtp(context.Background(), email, otp1, clientID)
	addrA := respA.Address
	require.NotEmpty(t, addrA)

	// 模拟节点 B：不同的 SeedSecret（模拟不同节点配置）
	// 但共享同一个本地存储（模拟链上同步后回读到本地）
	auth.SetCachedConfigForTest(&auth.AuthConfig{
		SeedSecret: "test-seed-secret-node-b",
		JWTSecret:  "test-jwt-secret",
	})
	otpCtxB, otpCancelB := context.WithCancel(context.Background())
	defer otpCancelB()
	resolverB := &Resolver{
		TeeChain:  nil,
		WalletSvc: wallet.NewWalletService(&teechain.TeeChain{}),
		OTPSvc:    auth.NewOTPService(otpCtxB, 5),
	}

	// 在节点 B 登录：由于本地存储已有记录，应使用已有地址，
	// 而不是用 node-b 的 SeedSecret 重新派生
	otp2, _ := resolverB.OTPSvc.Generate(email, clientID)
	mB := &mutationResolver{Resolver: resolverB}
	respB, err := mB.VerifyOtp(context.Background(), email, otp2, clientID)
	require.NoError(t, err)
	require.True(t, respB.Success)
	assert.Equal(t, addrA, respB.Address,
		"跨节点登录时，若存储中已有地址记录，应保持一致，不因 SeedSecret 不同而变更")

	// 验证：如果不用已有地址，node-b 的 SeedSecret 会派生不同地址
	addrIfReDerived := resolverB.WalletSvc.DeriveAddress(email)
	assert.NotEqual(t, addrA, addrIfReDerived,
		"不同 SeedSecret 确实会派生不同地址，证明修复是必要的")
}

func TestSocialLogin_AddressConsistency(t *testing.T) {
	resolver, cleanup := setupAuthTest(t, "test-seed-secret")
	defer cleanup()

	email := "oauth@example.com"

	// 由于 OAuthSvc 未配置真实客户端，SocialLogin 的完整 resolver 测试较难模拟。
	// 这里改为直接验证核心逻辑：getAuthUserFromContract 读取已有地址，
	// 以及 DeriveAddress 会生成不同值。
	// Email 字段现在存 hash
	_ = types.PutAuthUser(&types.AuthUser{Email: types.HashEmailKey(email), Address: "0xOAuthAddress123"})

	user, err := getAuthUserFromContract(nil, email)
	require.NoError(t, err)
	assert.Equal(t, "0xOAuthAddress123", user.Address)

	// 若尝试重新派生，地址会不同（因为 SeedSecret 派生逻辑不同）
	derivedAddr := resolver.WalletSvc.DeriveAddress(email)
	assert.NotEqual(t, "0xOAuthAddress123", derivedAddr,
		"证明直接派生会生成不同地址，从而强调存储已有地址的重要性")
}

func TestCompleteRecovery_KeepsAddress(t *testing.T) {
	resolver, cleanup := setupAuthTest(t, "test-seed-secret")
	defer cleanup()

	email := "recover@example.com"

	// 先注册一个用户并固定地址
	m := &mutationResolver{Resolver: resolver}
	otp, _ := resolver.OTPSvc.Generate(email, "client")
	resp, _ := m.VerifyOtp(context.Background(), email, otp, "client")
	originalAddr := resp.Address
	require.NotEmpty(t, originalAddr)

	// 模拟恢复流程：创建一个恢复 token
	recoveryToken := fmt.Sprintf("sig_%d_%s", time.Now().Unix(), email)
	types.PutSignSession(&types.SignSession{
		Token:     recoveryToken,
		Email:     email,
		ExpiresAt: time.Now().Add(1 * time.Hour).Unix(),
	})

	// 完成恢复：应保持原地址
	result, err := m.CompleteRecovery(context.Background(), recoveryToken, "new-password-hash")
	require.NoError(t, err)
	assert.Equal(t, "ok", result.Status)
	assert.Equal(t, originalAddr, result.Address, "恢复密码后地址必须保持不变")
}

func TestRequestRecovery_RandomOTPAndDelivery(t *testing.T) {
	resolver, cleanup := setupAuthTest(t, "test-seed")
	defer cleanup()

	userEmail := "user@example.com"
	recoveryEmail := "recovery@example.com"

	// 先设置恢复配置
	_ = types.PutRecoveryConfig(&types.RecoveryConfig{
		UserEmail:     userEmail,
		RecoveryEmail: recoveryEmail,
	})

	m := &mutationResolver{Resolver: resolver}
	resp, err := m.RequestRecovery(context.Background(), recoveryEmail)
	require.NoError(t, err)
	require.Equal(t, "ok", resp.Status)

	// 验证 OTP 已存储且非空
	storedOTP, err := types.GetRecoveryOTP(userEmail)
	require.NoError(t, err)
	require.NotEmpty(t, storedOTP)
	require.Len(t, storedOTP, 6)

	// 验证 OTP 是纯数字
	for _, c := range storedOTP {
		assert.True(t, c >= '0' && c <= '9')
	}

	// 再次请求，OTP 应该不同（随机性验证）
	resp2, err := m.RequestRecovery(context.Background(), recoveryEmail)
	require.NoError(t, err)
	require.Equal(t, "ok", resp2.Status)

	storedOTP2, err := types.GetRecoveryOTP(userEmail)
	require.NoError(t, err)
	assert.NotEqual(t, storedOTP, storedOTP2, "两次请求的 OTP 应该不同")
}

func TestVerifyRecovery_ExpiredOTP(t *testing.T) {
	resolver, cleanup := setupAuthTest(t, "test-seed")
	defer cleanup()

	userEmail := "user@example.com"
	recoveryEmail := "recovery@example.com"

	_ = types.PutRecoveryConfig(&types.RecoveryConfig{
		UserEmail:     userEmail,
		RecoveryEmail: recoveryEmail,
	})

	// 手动写入一个已过期的 OTP
	_ = types.PutRecoveryOTP(userEmail, "123456", -600)

	m := &mutationResolver{Resolver: resolver}
	resp, err := m.VerifyRecovery(context.Background(), recoveryEmail, "123456")
	require.NoError(t, err)
	require.Equal(t, "error", resp.Status)
	assert.Equal(t, "invalid OTP", resp.Error)
}

func TestResolveSignerEmail_ExpiredAPIKey(t *testing.T) {
	resolver, cleanup := setupAuthTest(t, "test-seed")
	defer cleanup()

	email := "expired@example.com"
	apiKey := "auth_tee_" + strings.Repeat("a", 64)

	// 创建一个已过期的 API Key
	types.SetGatewayTokenByKey(&types.GatewayTokenInfo{
		ID:        1,
		Owner:     email,
		Key:       apiKey,
		IsActive:  true,
		ExpiresAt: uint64(time.Now().Add(-1 * time.Hour).Unix()), // 1小时前过期
	})

	_, err := resolver.ResolveSignerEmail(apiKey)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unauthorized", "过期 API Key 应被拒绝")

	// 创建一个未过期的 API Key
	apiKey2 := "auth_tee_" + strings.Repeat("b", 64)
	types.SetGatewayTokenByKey(&types.GatewayTokenInfo{
		ID:        2,
		Owner:     email,
		Key:       apiKey2,
		IsActive:  true,
		ExpiresAt: uint64(time.Now().Add(1 * time.Hour).Unix()), // 1小时后过期
	})

	resolved, err := resolver.ResolveSignerEmail(apiKey2)
	require.NoError(t, err)
	assert.Equal(t, email, resolved)
}

func TestResolveEmailFromToken_ExpiredAPIKey(t *testing.T) {
	_, cleanup := setupAuthTest(t, "test-seed")
	defer cleanup()

	email := "expired@example.com"
	apiKey := "auth_tee_" + strings.Repeat("c", 64)

	// 创建一个已过期的 API Key（本地存储，合约查询路径在 TeeChain=nil 时跳过）
	types.SetGatewayTokenByKey(&types.GatewayTokenInfo{
		ID:        3,
		Owner:     email,
		Key:       apiKey,
		IsActive:  true,
		ExpiresAt: uint64(time.Now().Add(-1 * time.Hour).Unix()),
	})

	_, err := resolveEmailFromToken(apiKey)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "expired", "resolveEmailFromToken 应检测本地 API Key 过期")

	// 未过期
	apiKey2 := "auth_tee_" + strings.Repeat("d", 64)
	types.SetGatewayTokenByKey(&types.GatewayTokenInfo{
		ID:        4,
		Owner:     email,
		Key:       apiKey2,
		IsActive:  true,
		ExpiresAt: uint64(time.Now().Add(1 * time.Hour).Unix()),
	})

	resolved, err := resolveEmailFromToken(apiKey2)
	require.NoError(t, err)
	assert.Equal(t, email, resolved)
}

func TestGetAuthUserFromContract_LocalFallback(t *testing.T) {
	_, cleanup := setupAuthTest(t, "test-seed")
	defer cleanup()

	email := "local@example.com"
	expectedAddr := "0x1234567890abcdef1234567890abcdef12345678"

	// 直接写入本地存储（Email 字段存放 hash）
	err := types.PutAuthUser(&types.AuthUser{
		Email:   types.HashEmailKey(email),
		Address: expectedAddr,
	})
	require.NoError(t, err)

	// TeeChain=nil 时应回退到本地存储
	user, err := getAuthUserFromContract(nil, email)
	require.NoError(t, err)
	assert.Equal(t, expectedAddr, user.Address)
	// Email 字段在 Ciphertext 中存放 hash
	assert.Equal(t, types.HashEmailKey(email), user.Email)
}

// ── Wallet Login Nonce Tests ──────────────────────────────

// signEIP191 使用私钥对 message 做 EIP-191 personal_sign 签名（测试辅助）
func signEIP191(privKeyHex string, message string) (string, error) {
	privKey, err := ethcrypto.HexToECDSA(strings.TrimPrefix(privKeyHex, "0x"))
	if err != nil {
		return "", err
	}
	msg := fmt.Sprintf("\x19Ethereum Signed Message:\n%d%s", len(message), message)
	hash := ethcrypto.Keccak256Hash([]byte(msg))
	sig, err := ethcrypto.Sign(hash.Bytes(), privKey)
	if err != nil {
		return "", err
	}
	sig[64] += 27 // EIP-191 V value
	return hex.EncodeToString(sig), nil
}

func TestRequestWalletNonce(t *testing.T) {
	resolver, cleanup := setupAuthTest(t, "test-seed")
	defer cleanup()

	m := &mutationResolver{Resolver: resolver}
	address := "0x90F8bf6A479f320ead074411a4B0e7944Ea8c9C1"

	nonce, err := m.RequestWalletNonce(context.Background(), address)
	require.NoError(t, err)
	require.NotEmpty(t, nonce)

	// 验证 nonce 已写入存储
	rec, err := types.GetWalletNonce(address, nonce)
	require.NoError(t, err)
	require.NotNil(t, rec)
	assert.Equal(t, address, rec.Address)
	assert.False(t, rec.Used)
}

func TestLoginWithWallet_Success(t *testing.T) {
	resolver, cleanup := setupAuthTest(t, "test-seed")
	defer cleanup()

	m := &mutationResolver{Resolver: resolver}
	address := "0x90F8bf6A479f320ead074411a4B0e7944Ea8c9C1"

	// 1. 请求 nonce
	nonce, err := m.RequestWalletNonce(context.Background(), address)
	require.NoError(t, err)

	// 2. 用测试私钥签名 message + nonce
	// 该私钥对应的地址恰好是 0xAb5801a7D398351b8bE11C439e05C5B3259aeC9B
	privKeyHex := "4f3edf983ac636a65a842ce7c78d9aa706d3b113bce9c46f30d7d21715b23b1d"
	message := "Sign in to TEE Node"
	sig, err := signEIP191(privKeyHex, message+nonce)
	require.NoError(t, err)

	// 3. 登录
	resp, err := m.LoginWithWallet(context.Background(), address, sig, message, nonce)
	require.NoError(t, err)
	require.True(t, resp.Success, "登录应成功: %s", resp.Error)
	require.NotEmpty(t, resp.Token)
	assert.Equal(t, address, resp.Address)

	// 4. 验证 nonce 已被标记为已使用
	rec, _ := types.GetWalletNonce(address, nonce)
	require.NotNil(t, rec)
	assert.True(t, rec.Used)
}

func TestLoginWithWallet_ReplayRejected(t *testing.T) {
	resolver, cleanup := setupAuthTest(t, "test-seed")
	defer cleanup()

	m := &mutationResolver{Resolver: resolver}
	address := "0x90F8bf6A479f320ead074411a4B0e7944Ea8c9C1"
	privKeyHex := "4f3edf983ac636a65a842ce7c78d9aa706d3b113bce9c46f30d7d21715b23b1d"

	// 第一次登录
	nonce, _ := m.RequestWalletNonce(context.Background(), address)
	message := "Sign in to TEE Node"
	sig, _ := signEIP191(privKeyHex, message+nonce)
	resp1, _ := m.LoginWithWallet(context.Background(), address, sig, message, nonce)
	require.True(t, resp1.Success)

	// 重放同一签名和 nonce
	resp2, err := m.LoginWithWallet(context.Background(), address, sig, message, nonce)
	require.NoError(t, err)
	require.False(t, resp2.Success)
	assert.Equal(t, "nonce already used", resp2.Error)
}

func TestLoginWithWallet_ExpiredNonce(t *testing.T) {
	resolver, cleanup := setupAuthTest(t, "test-seed")
	defer cleanup()

	m := &mutationResolver{Resolver: resolver}
	address := "0x90F8bf6A479f320ead074411a4B0e7944Ea8c9C1"
	privKeyHex := "4f3edf983ac636a65a842ce7c78d9aa706d3b113bce9c46f30d7d21715b23b1d"

	// 手动写入一个已过期的 nonce
	nonce := "expired_nonce_123"
	err := types.PutWalletNonce(address, nonce, 300)
	require.NoError(t, err)
	// 修改记录为 10 分钟前创建并已过期
	rec, _ := types.GetWalletNonce(address, nonce)
	rec.CreatedAt = time.Now().Unix() - 600
	rec.ExpiresAt = rec.CreatedAt + 300
	b, _ := json.Marshal(rec)
	_ = types.SECRET_DB.SetKey(types.Web3AuthNamespace, "wn:"+address+":"+nonce, b)

	message := "Sign in to TEE Node"
	sig, _ := signEIP191(privKeyHex, message+nonce)
	resp, err := m.LoginWithWallet(context.Background(), address, sig, message, nonce)
	require.NoError(t, err)
	require.False(t, resp.Success)
	// 过期 nonce 会在 GetWalletNonce 时被懒删除
	assert.Equal(t, "invalid or expired nonce", resp.Error)
}

func TestLoginWithWallet_InvalidSignature(t *testing.T) {
	resolver, cleanup := setupAuthTest(t, "test-seed")
	defer cleanup()

	m := &mutationResolver{Resolver: resolver}
	address := "0x90F8bf6A479f320ead074411a4B0e7944Ea8c9C1"

	nonce, _ := m.RequestWalletNonce(context.Background(), address)

	// 使用错误私钥签名
	wrongPrivKey := "6cbed15c793ce57650b9877cf6fa156fbef513c4e6134f022a85b1ffdd59b2a1"
	message := "Sign in to TEE Node"
	sig, _ := signEIP191(wrongPrivKey, message+nonce)

	resp, err := m.LoginWithWallet(context.Background(), address, sig, message, nonce)
	require.NoError(t, err)
	require.False(t, resp.Success)
	assert.Equal(t, "invalid signature", resp.Error)
}

func TestLoginWithWallet_MissingNonce(t *testing.T) {
	resolver, cleanup := setupAuthTest(t, "test-seed")
	defer cleanup()

	m := &mutationResolver{Resolver: resolver}
	address := "0x90F8bf6A479f320ead074411a4B0e7944Ea8c9C1"
	privKeyHex := "4f3edf983ac636a65a842ce7c78d9aa706d3b113bce9c46f30d7d21715b23b1d"

	// 未请求 nonce 直接登录
	nonce := "i_did_not_request_this"
	message := "Sign in to TEE Node"
	sig, _ := signEIP191(privKeyHex, message+nonce)

	resp, err := m.LoginWithWallet(context.Background(), address, sig, message, nonce)
	require.NoError(t, err)
	require.False(t, resp.Success)
	assert.Equal(t, "invalid or expired nonce", resp.Error)
}

// TestPersistAuthUserToChain_RetryAndObservability verifies that the async
// chain persistence helper retries on failure and eventually logs the error
// instead of silently dropping it (fire-and-forget).
func TestPersistAuthUserToChain_RetryAndObservability(t *testing.T) {
	_, cleanup := setupAuthTest(t, "test-seed-secret")
	defer cleanup()

	calls := 0
	originalFn := authStoreAuthUserFn
	defer func() { authStoreAuthUserFn = originalFn }()

	authStoreAuthUserFn = func(chain *teechain.TeeChain, ref string, ciphertext []byte) error {
		calls++
		return fmt.Errorf("simulated chain failure")
	}

	user := &types.AuthUser{Email: "retry-test@example.com", Address: "0x1234"}
	persistAuthUserToChain(&teechain.TeeChain{}, user)

	// Wait for goroutine + retries: 3 retries with 0ms, 500ms, 1000ms backoff.
	time.Sleep(2 * time.Second)

	assert.GreaterOrEqual(t, calls, 3, "expected at least 3 retry attempts")
}

// TestPersistAuthUserToChain_NilChain_NoPanic ensures nil chain/user is safe.
func TestPersistAuthUserToChain_NilChain_NoPanic(t *testing.T) {
	persistAuthUserToChain(nil, &types.AuthUser{Email: "x@example.com"})
	persistAuthUserToChain(&teechain.TeeChain{}, nil)
	// No panic = pass
}

// TestGenerateAPIKey_ReturnsErrorOnRandFailure verifies that generateAPIKey
// propagates rand.Read errors instead of returning weak/zero entropy keys.
func TestGenerateAPIKey_ReturnsErrorOnRandFailure(t *testing.T) {
	originalFn := randReadFn
	defer func() { randReadFn = originalFn }()

	randReadFn = func(b []byte) (int, error) {
		return 0, fmt.Errorf("simulated rand failure")
	}

	key, err := generateAPIKey()
	assert.Error(t, err)
	assert.Empty(t, key)
}

// TestGenerateWalletNonce_ReturnsErrorOnRandFailure verifies that
// generateWalletNonce propagates rand.Read errors.
func TestGenerateWalletNonce_ReturnsErrorOnRandFailure(t *testing.T) {
	originalFn := randReadFn
	defer func() { randReadFn = originalFn }()

	randReadFn = func(b []byte) (int, error) {
		return 0, fmt.Errorf("simulated rand failure")
	}

	nonce, err := generateWalletNonce()
	assert.Error(t, err)
	assert.Empty(t, nonce)
}

// TestGenerateAPIKey_Success verifies the happy path returns a well-formed key.
func TestGenerateAPIKey_Success(t *testing.T) {
	key, err := generateAPIKey()
	require.NoError(t, err)
	assert.True(t, strings.HasPrefix(key, "auth_tee_"))
	assert.Equal(t, 64+len("auth_tee_"), len(key))
}

// TestGenerateWalletNonce_Success verifies the happy path returns a 32-char hex nonce.
func TestGenerateWalletNonce_Success(t *testing.T) {
	nonce, err := generateWalletNonce()
	require.NoError(t, err)
	assert.Equal(t, 32, len(nonce))
}

// TestLoginWithWallet_ReturnsRefreshToken verifies that wallet login issues a
// refresh token alongside the API token.
func TestLoginWithWallet_ReturnsRefreshToken(t *testing.T) {
	resolver, cleanup := setupAuthTest(t, "test-seed")
	defer cleanup()

	m := &mutationResolver{Resolver: resolver}
	address := "0x90F8bf6A479f320ead074411a4B0e7944Ea8c9C1"
	privKeyHex := "4f3edf983ac636a65a842ce7c78d9aa706d3b113bce9c46f30d7d21715b23b1d"
	message := "Sign in to TEE Node"

	nonce, err := m.RequestWalletNonce(context.Background(), address)
	require.NoError(t, err)

	sig, err := signEIP191(privKeyHex, message+nonce)
	require.NoError(t, err)

	resp, err := m.LoginWithWallet(context.Background(), address, sig, message, nonce)
	require.NoError(t, err)
	require.True(t, resp.Success)
	require.NotEmpty(t, resp.Token)
	require.NotEmpty(t, resp.RefreshToken)
	assert.True(t, strings.HasPrefix(resp.RefreshToken, "refresh_tee_"))

	// Verify refresh token is stored and active
	rt, err := types.GetRefreshToken(resp.RefreshToken)
	require.NoError(t, err)
	require.NotNil(t, rt)
	assert.Equal(t, address, rt.Owner)
	assert.Equal(t, address, rt.Address)
	assert.True(t, rt.IsActive)
	assert.False(t, rt.IsExpired())
}

// TestRefreshToken_Success verifies that a valid refresh token returns a new API key.
func TestRefreshToken_Success(t *testing.T) {
	resolver, cleanup := setupAuthTest(t, "test-seed")
	defer cleanup()

	m := &mutationResolver{Resolver: resolver}
	address := "0x90F8bf6A479f320ead074411a4B0e7944Ea8c9C1"
	privKeyHex := "4f3edf983ac636a65a842ce7c78d9aa706d3b113bce9c46f30d7d21715b23b1d"
	message := "Sign in to TEE Node"

	nonce, _ := m.RequestWalletNonce(context.Background(), address)
	sig, _ := signEIP191(privKeyHex, message+nonce)
	loginResp, _ := m.LoginWithWallet(context.Background(), address, sig, message, nonce)
	require.True(t, loginResp.Success)

	refreshResp, err := m.RefreshToken(context.Background(), loginResp.RefreshToken)
	require.NoError(t, err)
	require.True(t, refreshResp.Success)
	require.NotEmpty(t, refreshResp.Token)
	assert.NotEqual(t, loginResp.Token, refreshResp.Token, "refresh should issue a new API key")
	assert.NotEqual(t, loginResp.RefreshToken, refreshResp.RefreshToken, "refresh should rotate refresh token")
	assert.Equal(t, address, refreshResp.Address)

	// 旧的 refresh token 应已失效，不能再次使用。
	reusedResp, err := m.RefreshToken(context.Background(), loginResp.RefreshToken)
	require.NoError(t, err)
	require.False(t, reusedResp.Success)
	assert.Equal(t, "invalid or expired refresh token", reusedResp.Error)
}

// TestRefreshToken_InvalidToken verifies that an invalid refresh token is rejected.
func TestRefreshToken_InvalidToken(t *testing.T) {
	resolver, cleanup := setupAuthTest(t, "test-seed")
	defer cleanup()

	m := &mutationResolver{Resolver: resolver}
	resp, err := m.RefreshToken(context.Background(), "refresh_tee_nonexistent")
	require.NoError(t, err)
	require.False(t, resp.Success)
	assert.Equal(t, "invalid or expired refresh token", resp.Error)
}

// TestRefreshToken_ExpiredToken verifies that an expired refresh token is rejected.
func TestRefreshToken_ExpiredToken(t *testing.T) {
	resolver, cleanup := setupAuthTest(t, "test-seed")
	defer cleanup()

	refreshToken := "refresh_tee_expiredtoken"
	err := types.SetRefreshToken(refreshToken, &types.RefreshTokenInfo{
		Owner:     "user@example.com",
		Address:   "0x1234",
		IsActive:  true,
		CreatedAt: uint64(time.Now().Add(-31 * 24 * time.Hour).Unix()),
		ExpiresAt: uint64(time.Now().Add(-24 * time.Hour).Unix()),
	})
	require.NoError(t, err)

	m := &mutationResolver{Resolver: resolver}
	resp, err := m.RefreshToken(context.Background(), refreshToken)
	require.NoError(t, err)
	require.False(t, resp.Success)
	assert.Equal(t, "invalid or expired refresh token", resp.Error)
}
