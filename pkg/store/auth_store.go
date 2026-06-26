package store

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/cockroachdb/pebble"
)

// Web3AuthNamespace 是 Web3Auth 相关数据在 KV 存储中的命名空间。
const Web3AuthNamespace = "WEB3AUTH"

// ── Auth 持久化类型 ───────────────────────────────────────

// AuthUser 认证用户（最小化存储）
type AuthUser struct {
	Email        string `json:"email"`
	Address      string `json:"address"`
	PasswordHash string `json:"passwordHash,omitempty"` // 用于解密私钥（不再作为网络凭证传输）
	Salt         string `json:"salt,omitempty"`         // challenge salt（hex）
	DKHash       string `json:"dkHash,omitempty"`       // PBKDF2 derived key hash（hex，用于 challenge-response 验证）
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
	return SECRET_DB.SetKey(Web3AuthNamespace, "u:"+u.Email, b)
}

// SignSession 签名会话（密码验证后的短期授权）
type SignSession struct {
	Token        string `json:"token"`
	Email        string `json:"email"`
	ExpiresAt    int64  `json:"expiresAt"`
	PasswordHash string `json:"passwordHash,omitempty"` // 关联的密码 hash，用于后续签名时解密私钥
}

// ── SignSession ───────────────────────────────────────────

func PutSignSession(s *SignSession) error {
	b, err := json.Marshal(s)
	if err != nil {
		return err
	}
	key := "sig:" + s.Token
	return SECRET_DB.CacheKey(Web3AuthNamespace, key, b, s.ExpiresAt)
}

func GetSignSession(token string) (*SignSession, error) {
	s, err := DBGetJson[SignSession](SECRET_DB, Web3AuthNamespace, "sig:"+token)
	if err != nil {
		return nil, err
	}
	if s != nil && time.Now().Unix() > s.ExpiresAt {
		_ = DeleteSignSession(token)
		_ = cacheCancelExpiration(Web3AuthNamespace, "sig:"+token, s.ExpiresAt)
		return nil, nil
	}
	return s, nil
}

func DeleteSignSession(token string) error {
	key := "sig:" + token
	if s, _ := DBGetJson[SignSession](SECRET_DB, Web3AuthNamespace, key); s != nil {
		_ = cacheCancelExpiration(Web3AuthNamespace, key, s.ExpiresAt)
	}
	return SECRET_DB.DeleteKey(Web3AuthNamespace, key)
}

// ── Challenge ─────────────────────────────────────────────

// ChallengeRecord 挑战记录（用于 challenge-response 验证）
type ChallengeRecord struct {
	Challenge string `json:"challenge"`
	Email     string `json:"email"`
	CreatedAt int64  `json:"createdAt"`
	ExpiresAt int64  `json:"expiresAt"`
	Used      bool   `json:"used"`
}

func PutChallenge(email, challenge string, ttlSeconds int) error {
	now := time.Now().Unix()
	expiresAt := now + int64(ttlSeconds)
	rec := &ChallengeRecord{
		Challenge: challenge,
		Email:     email,
		CreatedAt: now,
		ExpiresAt: expiresAt,
		Used:      false,
	}
	b, err := json.Marshal(rec)
	if err != nil {
		return err
	}
	key := fmt.Sprintf("c:%s:%s", email, challenge)
	return SECRET_DB.CacheKey(Web3AuthNamespace, key, b, expiresAt)
}

func GetChallenge(email, challenge string) (*ChallengeRecord, error) {
	key := fmt.Sprintf("c:%s:%s", email, challenge)
	rec, err := DBGetJson[ChallengeRecord](SECRET_DB, Web3AuthNamespace, key)
	if err != nil {
		return nil, err
	}
	if rec != nil && time.Now().Unix() > rec.ExpiresAt {
		_ = DeleteChallenge(email, challenge)
		_ = cacheCancelExpiration(Web3AuthNamespace, key, rec.ExpiresAt)
		return nil, nil
	}
	return rec, nil
}

func MarkChallengeUsed(email, challenge string) error {
	key := fmt.Sprintf("c:%s:%s", email, challenge)
	rec, err := GetChallenge(email, challenge)
	if err != nil {
		return err
	}
	if rec == nil {
		// challenge 已被删除（如被 ClearAllChallenges 清理），忽略
		return nil
	}
	rec.Used = true
	b, _ := json.Marshal(rec)
	return SECRET_DB.SetKey(Web3AuthNamespace, key, b)
}

func DeleteChallenge(email, challenge string) error {
	key := fmt.Sprintf("c:%s:%s", email, challenge)
	if rec, _ := DBGetJson[ChallengeRecord](SECRET_DB, Web3AuthNamespace, key); rec != nil {
		_ = cacheCancelExpiration(Web3AuthNamespace, key, rec.ExpiresAt)
	}
	return SECRET_DB.DeleteKey(Web3AuthNamespace, key)
}

// ClearAllChallenges 清除用户所有未使用的 challenge（避免多 challenge 残留导致验证错位）
func ClearAllChallenges(email string) error {
	prefix := []byte(comboKey(Web3AuthNamespace, fmt.Sprintf("c:%s:", email)))
	iter, err := SECRET_DB.NewIter(&pebble.IterOptions{
		LowerBound: prefix,
	})
	if err != nil {
		return err
	}
	defer iter.Close()

	// 收集所有需要删除的 key
	var toDelete []string
	for iter.First(); iter.Valid() && strings.HasPrefix(string(iter.Key()), string(prefix)); iter.Next() {
		raw, err := SECRET_DB.Unseal(iter.Value())
		if err != nil {
			continue
		}
		var rec ChallengeRecord
		if json.Unmarshal(raw, &rec) == nil && !rec.Used {
			toDelete = append(toDelete, rec.Challenge)
		}
	}

	for _, c := range toDelete {
		DeleteChallenge(email, c)
	}
	return nil
}

// ── Rate Limit ────────────────────────────────────────────

func IncrFailedChallenge(email string) (int, error) {
	key := fmt.Sprintf("fl:%s", email)
	b, err := SECRET_DB.GetKey(Web3AuthNamespace, key)
	var count int
	if err == nil && len(b) > 0 {
		_ = json.Unmarshal(b, &count)
	}
	count++
	nb, _ := json.Marshal(count)
	if err := SECRET_DB.SetKey(Web3AuthNamespace, key, nb); err != nil {
		return count, err
	}
	// TTL 1 hour: tee-node Pebble 无原生 TTL，需外部清理或接受长期存储
	// 实际生产中可用定时任务清理，或存创建时间戳
	return count, nil
}

func ResetFailedChallenge(email string) error {
	return SECRET_DB.DeleteKey(Web3AuthNamespace, fmt.Sprintf("fl:%s", email))
}

func IsLocked(email string) bool {
	// // 简化实现：检查失败次数 >= 5
	// key := fmt.Sprintf("fl:%s", email)
	// b, err := SECRET_DB.GetKey(Web3AuthNamespace, key)
	// if err != nil || len(b) == 0 {
	// 	return false
	// }
	// var count int
	// _ = json.Unmarshal(b, &count)
	// return count >= 5
	return false // 目前不锁定账户，避免误伤
}

// ── EncryptedKey ──────────────────────────────────────────

func PutEncryptedKey(k *EncryptedKey) error {
	b, err := json.Marshal(k)
	if err != nil {
		return err
	}
	return SECRET_DB.SetKey(Web3AuthNamespace, "k:"+k.Email, b)
}

func GetEncryptedKey(email string) (*EncryptedKey, error) {
	return DBGetJson[EncryptedKey](SECRET_DB, Web3AuthNamespace, "k:"+email)
}

// ── OTP ───────────────────────────────────────────────────

func PutOTP(email, otp string) error {
	return SECRET_DB.SetKey(Web3AuthNamespace, "o:"+email, []byte(otp))
}

func GetOTP(email string) (string, error) {
	b, err := SECRET_DB.GetKey(Web3AuthNamespace, "o:"+email)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func DeleteOTP(email string) error {
	return SECRET_DB.DeleteKey(Web3AuthNamespace, "o:"+email)
}

// ── Recovery ──────────────────────────────────────────────

func PutRecoveryConfig(c *RecoveryConfig) error {
	b, err := json.Marshal(c)
	if err != nil {
		return err
	}
	return SECRET_DB.SetKey(Web3AuthNamespace, "r:"+c.UserEmail, b)
}

func FindRecoveryByEmail(recoveryEmail string) (*RecoveryConfig, error) {
	iter, err := SECRET_DB.NewIter(&pebble.IterOptions{})
	if err != nil {
		return nil, err
	}
	defer iter.Close()

	prefix := []byte(comboKey(Web3AuthNamespace, "r:"))
	for iter.SeekGE(prefix); iter.Valid() && strings.HasPrefix(string(iter.Key()), string(prefix)); iter.Next() {
		raw, err := SECRET_DB.Unseal(iter.Value())
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

func PutRecoveryOTP(userEmail, otp string, ttlSeconds int) error {
	expiresAt := time.Now().Add(time.Duration(ttlSeconds) * time.Second).Unix()
	data := fmt.Sprintf("%d:%s", expiresAt, otp)
	key := "ro:" + userEmail
	return SECRET_DB.CacheKey(Web3AuthNamespace, key, []byte(data), expiresAt)
}

func GetRecoveryOTP(userEmail string) (string, error) {
	key := "ro:" + userEmail
	b, err := SECRET_DB.GetKey(Web3AuthNamespace, key)
	if err != nil {
		return "", err
	}
	// 解析: expiresAt:otp
	parts := strings.SplitN(string(b), ":", 2)
	if len(parts) != 2 {
		return "", fmt.Errorf("invalid recovery otp format")
	}
	expiresAt, _ := strconv.ParseInt(parts[0], 10, 64)
	if time.Now().Unix() > expiresAt {
		_ = DeleteRecoveryOTP(userEmail)
		_ = cacheCancelExpiration(Web3AuthNamespace, key, expiresAt)
		return "", fmt.Errorf("recovery otp expired")
	}
	return parts[1], nil
}

func DeleteRecoveryOTP(userEmail string) error {
	key := "ro:" + userEmail
	if b, _ := SECRET_DB.GetKey(Web3AuthNamespace, key); len(b) > 0 {
		parts := strings.SplitN(string(b), ":", 2)
		if len(parts) == 2 {
			if expiresAt, err := strconv.ParseInt(parts[0], 10, 64); err == nil {
				_ = cacheCancelExpiration(Web3AuthNamespace, key, expiresAt)
			}
		}
	}
	return SECRET_DB.DeleteKey(Web3AuthNamespace, key)
}

// ── Wallet Login Nonce ────────────────────────────────────

// WalletNonceRecord 钱包登录一次性 nonce 记录
type WalletNonceRecord struct {
	Address   string `json:"address"`
	Nonce     string `json:"nonce"`
	CreatedAt int64  `json:"createdAt"`
	ExpiresAt int64  `json:"expiresAt"`
	Used      bool   `json:"used"`
}

func PutWalletNonce(address, nonce string, ttlSeconds int) error {
	now := time.Now().Unix()
	expiresAt := now + int64(ttlSeconds)
	rec := &WalletNonceRecord{
		Address:   address,
		Nonce:     nonce,
		CreatedAt: now,
		ExpiresAt: expiresAt,
		Used:      false,
	}
	b, err := json.Marshal(rec)
	if err != nil {
		return err
	}
	key := "wn:" + address + ":" + nonce
	return SECRET_DB.CacheKey(Web3AuthNamespace, key, b, expiresAt)
}

func GetWalletNonce(address, nonce string) (*WalletNonceRecord, error) {
	key := "wn:" + address + ":" + nonce
	rec, err := DBGetJson[WalletNonceRecord](SECRET_DB, Web3AuthNamespace, key)
	if err != nil {
		return nil, err
	}
	if rec != nil && time.Now().Unix() > rec.ExpiresAt {
		_ = SECRET_DB.DeleteKey(Web3AuthNamespace, key)
		_ = cacheCancelExpiration(Web3AuthNamespace, key, rec.ExpiresAt)
		return nil, nil
	}
	return rec, nil
}

func MarkWalletNonceUsed(address, nonce string) error {
	rec, err := GetWalletNonce(address, nonce)
	if err != nil {
		return err
	}
	if rec == nil {
		return fmt.Errorf("nonce not found")
	}
	rec.Used = true
	b, _ := json.Marshal(rec)
	return SECRET_DB.SetKey(Web3AuthNamespace, "wn:"+address+":"+nonce, b)
}
