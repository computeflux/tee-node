package model

import (
	"encoding/json"
	"errors"
	"strings"

	"github.com/cockroachdb/pebble"
	"github.com/computeflux/tee-provider/pkg/util"
)

// ── Auth 持久化类型 ───────────────────────────────────────

// AuthUser 认证用户
type AuthUser struct {
	Email        string `json:"email"`
	Name         string `json:"name"`
	Address      string `json:"address"`
	AuthProvider string `json:"authProvider"`
	LastLogin    int64  `json:"lastLogin"`
}

// AuthSession 认证会话
type AuthSession struct {
	Token        string `json:"token"`
	Email        string `json:"email"`
	Address      string `json:"address"`
	AuthProvider string `json:"authProvider"`
	CreatedAt    int64  `json:"createdAt"`
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
	return SetKey("AUTH", "u:"+u.Email, b)
}

func GetAuthUser(email string) (*AuthUser, error) {
	return GetJson[AuthUser]("AUTH", "u:"+email)
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
	return SetKey("AUTH", "s:"+tok, b)
}

func GetAuthSession(token string) (*AuthSession, error) {
	if len(token) > 32 {
		token = token[:32]
	}
	return GetJson[AuthSession]("AUTH", "s:"+token)
}

func DeleteAuthSession(token string) error {
	if len(token) > 32 {
		token = token[:32]
	}
	return DeleteKey("AUTH", "s:"+token)
}

// ── EncryptedKey ──────────────────────────────────────────

func PutEncryptedKey(k *EncryptedKey) error {
	b, err := json.Marshal(k)
	if err != nil {
		return err
	}
	return SetKey("AUTH", "k:"+k.Email, b)
}

func GetEncryptedKey(email string) (*EncryptedKey, error) {
	return GetJson[EncryptedKey]("AUTH", "k:"+email)
}

func DeleteEncryptedKey(email string) error {
	return DeleteKey("AUTH", "k:"+email)
}

// ── OTP ───────────────────────────────────────────────────

func PutOTP(email, otp string) error {
	return SetKey("AUTH", "o:"+email, []byte(otp))
}

func GetOTP(email string) (string, error) {
	b, err := GetKey("AUTH", "o:"+email)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func DeleteOTP(email string) error {
	return DeleteKey("AUTH", "o:"+email)
}

// ── Recovery ──────────────────────────────────────────────

func PutRecoveryConfig(c *RecoveryConfig) error {
	b, err := json.Marshal(c)
	if err != nil {
		return err
	}
	return SetKey("AUTH", "r:"+c.UserEmail, b)
}

func GetRecoveryConfig(userEmail string) (*RecoveryConfig, error) {
	return GetJson[RecoveryConfig]("AUTH", "r:"+userEmail)
}

func FindRecoveryByEmail(recoveryEmail string) (*RecoveryConfig, error) {
	iter, err := DBINS.NewIter(&pebble.IterOptions{})
	if err != nil {
		return nil, err
	}
	defer iter.Close()

	prefix := []byte(comboKey("AUTH", "r:"))
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
	return SetKey("AUTH", "ro:"+userEmail, []byte(otp))
}

func GetRecoveryOTP(userEmail string) (string, error) {
	b, err := GetKey("AUTH", "ro:"+userEmail)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func DeleteRecoveryOTP(userEmail string) error {
	return DeleteKey("AUTH", "ro:"+userEmail)
}
