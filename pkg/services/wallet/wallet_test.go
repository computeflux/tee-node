package wallet

import (
	"encoding/hex"
	"fmt"
	"math/big"
	"testing"

	"github.com/computeflux/tee-provider/pkg/services/auth"
	store "github.com/computeflux/tee-provider/pkg/types"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	ethcrypto "github.com/ethereum/go-ethereum/crypto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupTestWallet(t *testing.T) *WalletService {
	t.Helper()

	db, err := store.NewDBWithPath(t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	// 重置并设置测试配置
	auth.ResetConfigCache()
	auth.SetCachedConfigForTest(&auth.AuthConfig{
		JWTSecret:  "test-jwt-secret-32bytes-long!!!",
		SeedSecret: "test-seed-secret-32bytes-long!!",
	})

	return &WalletService{tc: nil}
}

// ── Derivation tests ───────────────────────────────────────

func TestWallet_DeriveSeed_Deterministic(t *testing.T) {
	ws := setupTestWallet(t)
	email := "alice@example.com"

	seed1 := ws.DeriveSeed(email)
	seed2 := ws.DeriveSeed(email)

	assert.Equal(t, seed1, seed2, "same email must produce same seed")
	assert.Len(t, seed1, 64, "seed must be hex-encoded 32 bytes")
}

func TestWallet_DeriveSeed_DifferentEmails(t *testing.T) {
	ws := setupTestWallet(t)

	seed1 := ws.DeriveSeed("alice@example.com")
	seed2 := ws.DeriveSeed("bob@example.com")

	assert.NotEqual(t, seed1, seed2, "different emails must produce different seeds")
}

func TestWallet_DerivePrivateKey_ValidECDSA(t *testing.T) {
	ws := setupTestWallet(t)
	email := "alice@example.com"

	priv, err := ws.DerivePrivateKey(email)
	require.NoError(t, err)
	require.NotNil(t, priv)

	// 验证是有效的 secp256k1 私钥
	// 验证是有效的 secp256k1 私钥（曲线阶数范围内）
	assert.NotNil(t, priv.PublicKey, "derived key must have public key")
	assert.NotNil(t, priv.PublicKey.X, "public key X must not be nil")
	assert.NotNil(t, priv.PublicKey.Y, "public key Y must not be nil")
}

func TestWallet_DerivePrivateKey_Deterministic(t *testing.T) {
	ws := setupTestWallet(t)
	email := "alice@example.com"

	priv1, err := ws.DerivePrivateKey(email)
	require.NoError(t, err)
	priv2, err := ws.DerivePrivateKey(email)
	require.NoError(t, err)

	assert.Equal(t, priv1.D, priv2.D, "same email must produce same private key")
}

func TestWallet_DeriveAddress(t *testing.T) {
	ws := setupTestWallet(t)
	email := "alice@example.com"

	addr := ws.DeriveAddress(email)
	require.NotEqual(t, "0x", addr)
	require.True(t, common.IsHexAddress(addr), "must be valid Ethereum address")

	// 验证地址与私钥匹配
	priv, err := ws.DerivePrivateKey(email)
	require.NoError(t, err)
	expected := ethcrypto.PubkeyToAddress(priv.PublicKey).Hex()
	assert.Equal(t, expected, addr)

	// 验证自动存储了加密密钥
	assert.True(t, ws.HasEncryptedKey(email), "DeriveAddress should store encrypted key")
}

// ── Encryption/decryption tests ────────────────────────────

func TestWallet_EncryptWithServerKey_RoundTrip(t *testing.T) {
	ws := setupTestWallet(t)
	email := "alice@example.com"

	priv, err := ws.DerivePrivateKey(email)
	require.NoError(t, err)
	keyBytes := ethcrypto.FromECDSA(priv)

	err = ws.encryptWithServerKey(email, keyBytes)
	require.NoError(t, err)

	decrypted, err := ws.DecryptWithServerKey(email)
	require.NoError(t, err)
	assert.Equal(t, keyBytes, decrypted)
}

func TestWallet_EncryptPrivateKey_RoundTrip(t *testing.T) {
	ws := setupTestWallet(t)
	email := "alice@example.com"
	password := "my-secret-password"

	err := ws.EncryptPrivateKey(email, password)
	require.NoError(t, err)

	priv, err := ws.DecryptPrivateKey(email, password)
	require.NoError(t, err)
	require.NotNil(t, priv)
}

func TestWallet_DecryptPrivateKey_WrongPassword(t *testing.T) {
	ws := setupTestWallet(t)
	email := "alice@example.com"
	password := "correct-password"

	err := ws.EncryptPrivateKey(email, password)
	require.NoError(t, err)

	_, err = ws.DecryptPrivateKey(email, "wrong-password")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "wrong password")
}

func TestWallet_DecryptPrivateKey_NotFound(t *testing.T) {
	ws := setupTestWallet(t)

	_, err := ws.DecryptPrivateKey("nobody@example.com", "any")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no encrypted key found")
}

func TestWallet_DecryptWithServerKey_InvalidCiphertext(t *testing.T) {
	ws := setupTestWallet(t)
	email := "alice@example.com"

	// 存入一个过短的 blob（小于 nonce size）
	err := store.PutEncryptedKey(&store.EncryptedKey{
		Email:    email,
		Blob:     hex.EncodeToString([]byte{0x01, 0x02}),
		BlobType: "aes-256-gcm-server",
	})
	require.NoError(t, err)

	_, err = ws.DecryptWithServerKey(email)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid ciphertext")
}

// ── decryptKeyByType tests ─────────────────────────────────

func TestWallet_decryptKeyByType_UserPassword(t *testing.T) {
	ws := setupTestWallet(t)
	email := "alice@example.com"
	password := "user-password"

	err := ws.EncryptPrivateKey(email, password)
	require.NoError(t, err)

	priv, err := ws.decryptKeyByType(email, password)
	require.NoError(t, err)
	require.NotNil(t, priv)
}

func TestWallet_decryptKeyByType_ServerKey(t *testing.T) {
	ws := setupTestWallet(t)
	email := "alice@example.com"

	// 通过 DeriveAddress 触发服务端加密存储
	ws.DeriveAddress(email)

	priv, err := ws.decryptKeyByType(email, "")
	require.NoError(t, err)
	require.NotNil(t, priv)
}

func TestWallet_decryptKeyByType_PasswordRequired(t *testing.T) {
	ws := setupTestWallet(t)
	email := "alice@example.com"
	password := "user-password"

	err := ws.EncryptPrivateKey(email, password)
	require.NoError(t, err)

	// 不传密码时应当报错
	_, err = ws.decryptKeyByType(email, "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "password required")
}

func TestWallet_decryptKeyByType_UnknownType(t *testing.T) {
	ws := setupTestWallet(t)
	email := "alice@example.com"

	// 先用服务端密钥存储
	ws.DeriveAddress(email)

	// 把 blob type 改成未知的
	record, err := store.GetEncryptedKey(email)
	require.NoError(t, err)
	record.BlobType = "unknown-type"
	err = store.PutEncryptedKey(record)
	require.NoError(t, err)

	// 回退逻辑：先尝试密码（失败），再尝试服务端密钥（成功）
	priv, err := ws.decryptKeyByType(email, "")
	require.NoError(t, err)
	require.NotNil(t, priv)
}

// ── Transaction signing tests ──────────────────────────────

func TestWallet_SignTx_EIP155(t *testing.T) {
	ws := setupTestWallet(t)
	email := "alice@example.com"
	ws.DeriveAddress(email)

	tx := &TxRequest{
		From:         "0x",
		To:           "0xDeadBeef00000000000000000000000000000000",
		Value:        "0x0",
		Data:         "0x",
		Gas:          "0x5208",
		GasPrice:     "0x1DCD6500",
		Nonce:        "0x0",
		ChainID:      "0x1",
		PasswordHash: "",
	}

	raw, err := ws.SignTx(email, tx)
	require.NoError(t, err)
	require.True(t, len(raw) > 2 && raw[:2] == "0x", "must return hex with 0x prefix")

	// 解析并验证签名
	data, err := hex.DecodeString(cleanHex(raw))
	require.NoError(t, err)

	signedTx := &types.Transaction{}
	err = signedTx.UnmarshalBinary(data)
	require.NoError(t, err)

	// 验证 ChainID
	v, r, sVal := signedTx.RawSignatureValues()
	require.NotNil(t, v)
	require.NotNil(t, r)
	require.NotNil(t, sVal)

	// EIP-155: v = chainID * 2 + 35 或 chainID * 2 + 36
	chainID := big.NewInt(1)
	expectedV1 := new(big.Int).Mul(chainID, big.NewInt(2))
	expectedV1.Add(expectedV1, big.NewInt(35))
	expectedV2 := new(big.Int).Add(expectedV1, big.NewInt(1))
	assert.True(t, v.Cmp(expectedV1) == 0 || v.Cmp(expectedV2) == 0, "v must be EIP-155 compliant")

	// 验证发送地址
	signerCheck := types.NewEIP155Signer(chainID)
	fromAddr, err := signerCheck.Sender(signedTx)
	require.NoError(t, err)
	priv, _ := ws.DerivePrivateKey(email)
	expectedAddr := ethcrypto.PubkeyToAddress(priv.PublicKey)
	assert.Equal(t, expectedAddr, fromAddr)
}

func TestWallet_SignTx_WithPassword(t *testing.T) {
	ws := setupTestWallet(t)
	email := "alice@example.com"
	password := "tx-password"

	// 先用密码加密
	err := ws.EncryptPrivateKey(email, password)
	require.NoError(t, err)

	tx := &TxRequest{
		To:           "0xDeadBeef00000000000000000000000000000000",
		Value:        "0x0",
		Gas:          "0x5208",
		GasPrice:     "0x1DCD6500",
		Nonce:        "0x0",
		ChainID:      "0x1",
		PasswordHash: password,
	}

	raw, err := ws.SignTx(email, tx)
	require.NoError(t, err)
	assert.NotEmpty(t, raw)
}

func TestWallet_SignTx_WrongPassword(t *testing.T) {
	ws := setupTestWallet(t)
	email := "alice@example.com"
	password := "tx-password"

	err := ws.EncryptPrivateKey(email, password)
	require.NoError(t, err)

	tx := &TxRequest{
		To:           "0xDeadBeef00000000000000000000000000000000",
		Value:        "0x0",
		Gas:          "0x5208",
		GasPrice:     "0x1DCD6500",
		Nonce:        "0x0",
		ChainID:      "0x1",
		PasswordHash: "wrong-password",
	}

	_, err = ws.SignTx(email, tx)
	require.Error(t, err)
}

// 回归测试：GasPrice/Value 为非法 hex 时不得 panic（此前会 nil 指针解引用崩溃），
// 且应回退到默认 gas / 零 value 并成功签名。
func TestWallet_SignTx_InvalidHexGasPriceAndValue(t *testing.T) {
	ws := setupTestWallet(t)
	email := "alice@example.com"
	ws.DeriveAddress(email)

	tx := &TxRequest{
		To:           "0xDeadBeef00000000000000000000000000000000",
		Value:        "0xzz-not-hex",
		Gas:          "0x5208",
		GasPrice:     "0xzz-not-hex",
		Nonce:        "0x0",
		ChainID:      "0x1",
		PasswordHash: "",
	}

	raw, err := ws.SignTx(email, tx)
	require.NoError(t, err)
	require.NotEmpty(t, raw)

	// 解码回放，验证回退行为：GasPrice 使用默认 2e12，Value 为 0
	data, err := hex.DecodeString(cleanHex(raw))
	require.NoError(t, err)
	signedTx := &types.Transaction{}
	require.NoError(t, signedTx.UnmarshalBinary(data))

	assert.Equal(t, uint64(21000), signedTx.Gas())
	assert.Equal(t, big.NewInt(2000000000000), signedTx.GasPrice())
	assert.Equal(t, int64(0), signedTx.Value().Int64())
}

// 回归测试：Nonce/Gas 为非法 hex 时也不得 panic，应回退为 0/默认值
func TestWallet_SignTx_InvalidHexNonceAndGas(t *testing.T) {
	ws := setupTestWallet(t)
	email := "alice@example.com"
	ws.DeriveAddress(email)

	tx := &TxRequest{
		To:           "0xDeadBeef00000000000000000000000000000000",
		Value:        "0x0",
		Gas:          "0xzz-not-hex",
		GasPrice:     "0x1DCD6500",
		Nonce:        "0xzz-not-hex",
		ChainID:      "0x1",
		PasswordHash: "",
	}

	raw, err := ws.SignTx(email, tx)
	require.NoError(t, err)
	require.NotEmpty(t, raw)

	data, err := hex.DecodeString(cleanHex(raw))
	require.NoError(t, err)
	signedTx := &types.Transaction{}
	require.NoError(t, signedTx.UnmarshalBinary(data))

	assert.Equal(t, uint64(21000), signedTx.Gas(), "invalid gas hex must fall back to default gas limit")
	assert.Equal(t, uint64(0), signedTx.Nonce(), "invalid nonce hex must fall back to 0")
}

func TestWallet_SignTx_DefaultGas(t *testing.T) {
	ws := setupTestWallet(t)
	email := "alice@example.com"
	ws.DeriveAddress(email)

	// 不设置 Gas 和 GasPrice
	tx := &TxRequest{
		To:      "0xDeadBeef00000000000000000000000000000000",
		Value:   "0x0",
		Nonce:   "0x0",
		ChainID: "0x1",
	}

	raw, err := ws.SignTx(email, tx)
	require.NoError(t, err)
	assert.NotEmpty(t, raw)
}

// ── Message signing tests ──────────────────────────────────

func TestWallet_SignMessage_EIP191(t *testing.T) {
	ws := setupTestWallet(t)
	email := "alice@example.com"
	ws.DeriveAddress(email)

	message := "Hello World"
	sigHex, err := ws.SignMessage(email, message, "")
	require.NoError(t, err)
	require.True(t, len(sigHex) > 2 && sigHex[:2] == "0x", "must return hex with 0x prefix")

	sig, err := hex.DecodeString(cleanHex(sigHex))
	require.NoError(t, err)
	require.Len(t, sig, 65, "EIP-191 signature must be 65 bytes")

	// 恢复公钥并验证
	msgBytes := []byte(message)
	prefix := fmt.Sprintf("\x19Ethereum Signed Message:\n%d", len(msgBytes))
	hash := ethcrypto.Keccak256Hash(append([]byte(prefix), msgBytes...))

	pub, err := ethcrypto.SigToPub(hash.Bytes(), sig)
	require.NoError(t, err)

	priv, _ := ws.DerivePrivateKey(email)
	expectedAddr := ethcrypto.PubkeyToAddress(priv.PublicKey)
	recoveredAddr := ethcrypto.PubkeyToAddress(*pub)
	assert.Equal(t, expectedAddr, recoveredAddr)
}

func TestWallet_SignMessage_HexMessage(t *testing.T) {
	ws := setupTestWallet(t)
	email := "alice@example.com"
	ws.DeriveAddress(email)

	// hex 编码的消息
	message := "0x48656c6c6f20576f726c64" // "Hello World"
	sigHex, err := ws.SignMessage(email, message, "")
	require.NoError(t, err)
	assert.NotEmpty(t, sigHex)
}

func TestWallet_SignMessage_WithPassword(t *testing.T) {
	ws := setupTestWallet(t)
	email := "alice@example.com"
	password := "msg-password"

	err := ws.EncryptPrivateKey(email, password)
	require.NoError(t, err)

	sigHex, err := ws.SignMessage(email, "test", password)
	require.NoError(t, err)
	assert.NotEmpty(t, sigHex)
}

// ── decodeMessage tests ────────────────────────────────────

func TestWallet_decodeMessage_Hex(t *testing.T) {
	b, err := decodeMessage("0x48656c6c6f")
	require.NoError(t, err)
	assert.Equal(t, []byte("Hello"), b)
}

func TestWallet_decodeMessage_Plaintext(t *testing.T) {
	b, err := decodeMessage("Hello World")
	require.NoError(t, err)
	assert.Equal(t, []byte("Hello World"), b)
}

func TestWallet_decodeMessage_InvalidHexFallback(t *testing.T) {
	// "0xhello" 不是合法 hex，应回退为普通字符串
	b, err := decodeMessage("0xhello")
	require.NoError(t, err)
	assert.Equal(t, []byte("0xhello"), b)
}

func TestWallet_cleanHex(t *testing.T) {
	assert.Equal(t, "abc123", cleanHex("0xabc123"))
	assert.Equal(t, "abc123", cleanHex("0Xabc123"))
	assert.Equal(t, "abc123", cleanHex("abc123"))
}
