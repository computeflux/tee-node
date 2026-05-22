package model

import (
	"encoding/json"
	"errors"
	"strings"

	"github.com/cockroachdb/pebble"
	"github.com/computeflux/tee-provider/pkg/util"
)

// ── Auth 持久化类型 ───────────────────────────────────────

// AuthUser 认证用户（最小化存储）
type AuthUser struct {
	Email        string `json:"email"`
	Address      string `json:"address"`
	PasswordHash string `json:"passwordHash,omitempty"`
}

// AuthSession 认证会话
type AuthSession struct {
	Token     string `json:"token"`
	Email     string `json:"email"`
	Address   string `json:"address"`
	CreatedAt int64  `json:"createdAt"`
}

// EncryptedKey 加密私钥
type EncryptedKey struct {
	Email    string `json:"email"`
	Blob     string `json:"blob"`
	BlobType string `json:"blobType"`
}

// RecoveryConfig 社交恢复配置
type RecoveryConfig struct {
	UserEmail     string `json:"userEmail"`
	RecoveryEmail string `json:"recoveryEmail"`
}

// ── User ──────────────────────────────────────────────────

func PutAuthUser(u *AuthUser) error {
	b, err := json.Marshal(u)
	if err != nil {
		return err
	}
	return SetKey("WEB3AUTH", "u:"+u.Email, b)
}

func GetAuthUser(email string) (*AuthUser, error) {
	return GetJson[AuthUser]("WEB3AUTH", "u:"+email)
}

// ── Session ───────────────────────────────────────────────

func PutAuthSession(s *AuthSession) error {
	tok := s.Token
	if len(tok) > 32 {
		tok = tok[:32]
	}
	b, err := json.Marshal(s)
	if err != nil {
		return err
	}
	return SetKey("WEB3AUTH", "s:"+tok, b)
}

func GetAuthSession(token string) (*AuthSession, error) {
	if len(token) > 32 {
		token = token[:32]
	}
	return GetJson[AuthSession]("WEB3AUTH", "s:"+token)
}

func DeleteAuthSession(token string) error {
	if len(token) > 32 {
		token = token[:32]
	}
	return DeleteKey("WEB3AUTH", "s:"+token)
}

// SignSession 签名会话（密码验证后的短期授权）
type SignSession struct {
	Token     string `json:"token"`
	Email     string `json:"email"`
	ExpiresAt int64  `json:"expiresAt"`
}

// ── SignSession ───────────────────────────────────────────

func PutSignSession(s *SignSession) error {
	b, err := json.Marshal(s)
	if err != nil {
		return err
	}
	return SetKey("WEB3AUTH", "sig:"+s.Token, b)
}

func GetSignSession(token string) (*SignSession, error) {
	return GetJson[SignSession]("WEB3AUTH", "sig:"+token)
}

func DeleteSignSession(token string) error {
	return DeleteKey("WEB3AUTH", "sig:"+token)
}

// ── EncryptedKey ──────────────────────────────────────────

func PutEncryptedKey(k *EncryptedKey) error {
	b, err := json.Marshal(k)
	if err != nil {
		return err
	}
	return SetKey("WEB3AUTH", "k:"+k.Email, b)
}

func GetEncryptedKey(email string) (*EncryptedKey, error) {
	return GetJson[EncryptedKey]("WEB3AUTH", "k:"+email)
}

func DeleteEncryptedKey(email string) error {
	return DeleteKey("WEB3AUTH", "k:"+email)
}

// ── OTP ───────────────────────────────────────────────────

func PutOTP(email, otp string) error {
	return SetKey("WEB3AUTH", "o:"+email, []byte(otp))
}

func GetOTP(email string) (string, error) {
	b, err := GetKey("WEB3AUTH", "o:"+email)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func DeleteOTP(email string) error {
	return DeleteKey("WEB3AUTH", "o:"+email)
}

// ── Recovery ──────────────────────────────────────────────

func PutRecoveryConfig(c *RecoveryConfig) error {
	b, err := json.Marshal(c)
	if err != nil {
		return err
	}
	return SetKey("WEB3AUTH", "r:"+c.UserEmail, b)
}

func GetRecoveryConfig(userEmail string) (*RecoveryConfig, error) {
	return GetJson[RecoveryConfig]("WEB3AUTH", "r:"+userEmail)
}

func FindRecoveryByEmail(recoveryEmail string) (*RecoveryConfig, error) {
	iter, err := DBINS.NewIter(&pebble.IterOptions{})
	if err != nil {
		return nil, err
	}
	defer iter.Close()

	prefix := []byte(comboKey("WEB3AUTH", "r:"))
	for iter.SeekGE(prefix); iter.Valid() && strings.HasPrefix(string(iter.Key()), string(prefix)); iter.Next() {
		raw, err := util.Unseal(iter.Value(), nil)
		if err != nil {
			continue
		}
		var c RecoveryConfig
		if json.Unmarshal(raw, &c) == nil && strings.EqualFold(c.RecoveryEmail, recoveryEmail) {
			return &c, nil
		}
	}
	return nil, errors.New("not found")
}

func PutRecoveryOTP(userEmail, otp string) error {
	return SetKey("WEB3AUTH", "ro:"+userEmail, []byte(otp))
}

func GetRecoveryOTP(userEmail string) (string, error) {
	b, err := GetKey("WEB3AUTH", "ro:"+userEmail)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func DeleteRecoveryOTP(userEmail string) error {
	return DeleteKey("WEB3AUTH", "ro:"+userEmail)
}
