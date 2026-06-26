package wallet

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/ecdsa"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"

	"github.com/computeflux/tee-provider/pkg/store"
	ethcrypto "github.com/ethereum/go-ethereum/crypto"
)

// EncryptPrivateKey 用用户密码加密私钥（存储到 Pebble）
func (s *WalletService) EncryptPrivateKey(email, passwordHash string) error {
	priv, err := s.DerivePrivateKey(email)
	if err != nil {
		return err
	}
	keyBytes := ethcrypto.FromECDSA(priv)

	hash := sha256Hash([]byte(passwordHash))
	block, err := aes.NewCipher(hash)
	if err != nil {
		return err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return err
	}
	ciphertext := gcm.Seal(nonce, nonce, keyBytes, nil)

	record := &store.EncryptedKey{
		Email:    email,
		Blob:     hex.EncodeToString(ciphertext),
		BlobType: "aes-256-gcm",
	}
	if err := store.PutEncryptedKey(record); err != nil {
		return err
	}

	// 异步上链：同步 EncryptedKey 到合约，支持跨节点解密
	s.persistEncryptedKeyToChain(email, record)
	return nil
}

// DecryptPrivateKey 用用户密码解密私钥
func (s *WalletService) DecryptPrivateKey(email, passwordHash string) (*ecdsa.PrivateKey, error) {
	record, err := store.GetEncryptedKey(email)
	if err != nil || record == nil {
		return nil, fmt.Errorf("no encrypted key found — set a transaction password first")
	}

	ciphertext, _ := hex.DecodeString(record.Blob)
	hash := sha256Hash([]byte(passwordHash))
	block, err := aes.NewCipher(hash)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	nonceSize := gcm.NonceSize()
	if len(ciphertext) < nonceSize {
		return nil, fmt.Errorf("invalid ciphertext")
	}
	nonce, ciphertext := ciphertext[:nonceSize], ciphertext[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, fmt.Errorf("wrong password")
	}

	return ethcrypto.ToECDSA(plaintext)
}

func sha256Hash(data []byte) []byte {
	h := sha256.Sum256(data)
	return h[:]
}
