package store

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/cockroachdb/pebble"
	"github.com/computeflux/tee-provider/pkg/util"
)

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
	return SetKey("WEB3AUTH", "u:"+u.Email, b)
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
	return SetKey("WEB3AUTH", "sig:"+s.Token, b)
}

func GetSignSession(token string) (*SignSession, error) {
	return GetJson[SignSession]("WEB3AUTH", "sig:"+token)
}

func DeleteSignSession(token string) error {
	return DeleteKey("WEB3AUTH", "sig:"+token)
}

// ── Challenge ─────────────────────────────────────────────

// ChallengeRecord 挑战记录（用于 challenge-response 验证）
type ChallengeRecord struct {
	Challenge string `json:"challenge"`
	Email     string `json:"email"`
	CreatedAt int64  `json:"createdAt"`
	Used      bool   `json:"used"`
}

func PutChallenge(email, challenge string, ttlSeconds int) error {
	rec := &ChallengeRecord{
		Challenge: challenge,
		Email:     email,
		CreatedAt: time.Now().Unix(),
		Used:      false,
	}
	b, err := json.Marshal(rec)
	if err != nil {
		return err
	}
	key := fmt.Sprintf("c:%s:%s", email, challenge)
	return SetKey("WEB3AUTH", key, b)
}

func GetChallenge(email, challenge string) (*ChallengeRecord, error) {
	key := fmt.Sprintf("c:%s:%s", email, challenge)
	return GetJson[ChallengeRecord]("WEB3AUTH", key)
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
	return SetKey("WEB3AUTH", key, b)
}

func DeleteChallenge(email, challenge string) error {
	return DeleteKey("WEB3AUTH", fmt.Sprintf("c:%s:%s", email, challenge))
}

// FindPendingChallenge 查找用户最新的未使用 challenge
func FindPendingChallenge(email string) (string, error) {
	prefix := []byte(comboKey("WEB3AUTH", fmt.Sprintf("c:%s:", email)))
	iter, err := DBINS.NewIter(&pebble.IterOptions{
		LowerBound: prefix,
	})
	if err != nil {
		return "", err
	}
	defer iter.Close()

	for iter.First(); iter.Valid() && strings.HasPrefix(string(iter.Key()), string(prefix)); iter.Next() {
		raw, err := util.Unseal(iter.Value(), nil)
		if err != nil {
			continue
		}
		var rec ChallengeRecord
		if json.Unmarshal(raw, &rec) == nil && !rec.Used {
			return rec.Challenge, nil
		}
	}
	return "", nil
}

// ClearAllChallenges 清除用户所有未使用的 challenge（避免多 challenge 残留导致验证错位）
func ClearAllChallenges(email string) error {
	prefix := []byte(comboKey("WEB3AUTH", fmt.Sprintf("c:%s:", email)))
	iter, err := DBINS.NewIter(&pebble.IterOptions{
		LowerBound: prefix,
	})
	if err != nil {
		return err
	}
	defer iter.Close()

	// 收集所有需要删除的 key
	var toDelete []string
	for iter.First(); iter.Valid() && strings.HasPrefix(string(iter.Key()), string(prefix)); iter.Next() {
		raw, err := util.Unseal(iter.Value(), nil)
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
	b, err := GetKey("WEB3AUTH", key)
	var count int
	if err == nil && len(b) > 0 {
		_ = json.Unmarshal(b, &count)
	}
	count++
	nb, _ := json.Marshal(count)
	if err := SetKey("WEB3AUTH", key, nb); err != nil {
		return count, err
	}
	// TTL 1 hour: tee-node Pebble 无原生 TTL，需外部清理或接受长期存储
	// 实际生产中可用定时任务清理，或存创建时间戳
	return count, nil
}

func ResetFailedChallenge(email string) error {
	return DeleteKey("WEB3AUTH", fmt.Sprintf("fl:%s", email))
}

func IsLocked(email string) bool {
	// // 简化实现：检查失败次数 >= 5
	// key := fmt.Sprintf("fl:%s", email)
	// b, err := GetKey("WEB3AUTH", key)
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

func PutRecoveryOTP(userEmail, otp string, ttlSeconds int) error {
	data := fmt.Sprintf("%d:%s", time.Now().Add(time.Duration(ttlSeconds)*time.Second).Unix(), otp)
	return SetKey("WEB3AUTH", "ro:"+userEmail, []byte(data))
}

func GetRecoveryOTP(userEmail string) (string, error) {
	b, err := GetKey("WEB3AUTH", "ro:"+userEmail)
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
		return "", fmt.Errorf("recovery otp expired")
	}
	return parts[1], nil
}

func DeleteRecoveryOTP(userEmail string) error {
	return DeleteKey("WEB3AUTH", "ro:"+userEmail)
}

// ── Wallet Login Nonce ────────────────────────────────────

// WalletNonceRecord 钱包登录一次性 nonce 记录
type WalletNonceRecord struct {
	Address   string `json:"address"`
	Nonce     string `json:"nonce"`
	CreatedAt int64  `json:"createdAt"`
	Used      bool   `json:"used"`
}

func PutWalletNonce(address, nonce string, ttlSeconds int) error {
	rec := &WalletNonceRecord{
		Address:   address,
		Nonce:     nonce,
		CreatedAt: time.Now().Unix(),
		Used:      false,
	}
	b, err := json.Marshal(rec)
	if err != nil {
		return err
	}
	return SetKey("WEB3AUTH", "wn:"+address+":"+nonce, b)
}

func GetWalletNonce(address, nonce string) (*WalletNonceRecord, error) {
	return GetJson[WalletNonceRecord]("WEB3AUTH", "wn:"+address+":"+nonce)
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
	return SetKey("WEB3AUTH", "wn:"+address+":"+nonce, b)
}

// CleanupWalletNonces 清理某地址下已过期的 nonce（由外部定时任务调用）
func CleanupWalletNonces(address string, maxAgeSeconds int) error {
	prefix := []byte(comboKey("WEB3AUTH", "wn:"+address+":"))
	iter, err := DBINS.NewIter(&pebble.IterOptions{})
	if err != nil {
		return err
	}
	defer iter.Close()

	cutoff := time.Now().Unix() - int64(maxAgeSeconds)
	for iter.SeekGE(prefix); iter.Valid() && strings.HasPrefix(string(iter.Key()), string(prefix)); iter.Next() {
		raw, err := util.Unseal(iter.Value(), nil)
		if err != nil {
			continue
		}
		var rec WalletNonceRecord
		if json.Unmarshal(raw, &rec) == nil && rec.CreatedAt < cutoff {
			_ = DeleteKey("WEB3AUTH", "wn:"+address+":"+rec.Nonce)
		}
	}
	return nil
}
