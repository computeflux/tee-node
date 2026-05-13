package wallet

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/ecdsa"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"

	ethcrypto "github.com/ethereum/go-ethereum/crypto"

	"github.com/computeflux/tee-provider/pkg/api/auth"
	"github.com/computeflux/tee-provider/pkg/model"
)

// WalletService 密钥派生、签名、加密
type WalletService struct {
	seedSecret string
	jwtSecret  string
}

func NewWalletService(cfg *auth.AuthConfig) *WalletService {
	return &WalletService{
		seedSecret: cfg.SeedSecret,
		jwtSecret:  cfg.JWTSecret,
	}
}

// DeriveSeed email → 确定性种子 (HMAC-SHA256)
func (s *WalletService) DeriveSeed(email string) string {
	mac := hmac.New(sha256.New, []byte(s.seedSecret))
	mac.Write([]byte(email))
	return hex.EncodeToString(mac.Sum(nil))
}

// DerivePrivateKey email → ECDSA 私钥
func (s *WalletService) DerivePrivateKey(email string) (*ecdsa.PrivateKey, error) {
	seed := s.DeriveSeed(email)
	mac := hmac.New(sha256.New, []byte(s.seedSecret))
	mac.Write([]byte("eth-key:" + seed))
	keyBytes := mac.Sum(nil)
	return ethcrypto.ToECDSA(keyBytes)
}

// DeriveAddress email → Ethereum 地址
func (s *WalletService) DeriveAddress(email string) string {
	priv, err := s.DerivePrivateKey(email)
	if err != nil {
		return "0x"
	}
	addr := ethcrypto.PubkeyToAddress(priv.PublicKey).Hex()

	// 如果还没有加密存储，用服务端密钥加密保存
	if !s.HasEncryptedKey(email) {
		s.encryptWithServerKey(email, ethcrypto.FromECDSA(priv))
	}
	return addr
}

// ── Server-side key wrapping ─────────────────────────────

func (s *WalletService) serverKey(email string) []byte {
	h := sha256.Sum256([]byte(s.jwtSecret + ":keywrap:" + email))
	return h[:]
}

// encryptWithServerKey 用服务端密钥加密私钥（便利模式，用户未设密码时使用）
func (s *WalletService) encryptWithServerKey(email string, keyBytes []byte) error {
	block, _ := aes.NewCipher(s.serverKey(email))
	gcm, _ := cipher.NewGCM(block)
	nonce := make([]byte, gcm.NonceSize())
	io.ReadFull(rand.Reader, nonce)
	ciphertext := gcm.Seal(nonce, nonce, keyBytes, nil)

	return model.PutEncryptedKey(&model.EncryptedKey{
		Email:    email,
		Blob:     hex.EncodeToString(ciphertext),
		BlobType: "aes-256-gcm-server",
	})
}

// DecryptWithServerKey 用服务端密钥解密私钥
func (s *WalletService) DecryptWithServerKey(email string) ([]byte, error) {
	record, err := model.GetEncryptedKey(email)
	if err != nil {
		return nil, fmt.Errorf("no stored key for %s", email)
	}
	ciphertext, _ := hex.DecodeString(record.Blob)
	block, _ := aes.NewCipher(s.serverKey(email))
	gcm, _ := cipher.NewGCM(block)
	nonceSize := gcm.NonceSize()
	if len(ciphertext) < nonceSize {
		return nil, fmt.Errorf("invalid ciphertext")
	}
	return gcm.Open(nil, ciphertext[:nonceSize], ciphertext[nonceSize:], nil)
}

// HasEncryptedKey 检查用户是否已有加密存储的密钥
func (s *WalletService) HasEncryptedKey(email string) bool {
	_, err := model.GetEncryptedKey(email)
	return err == nil
}
