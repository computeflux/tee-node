package auth

import (
	"crypto/rand"
	"encoding/hex"
	"errors"

	"github.com/computeflux/tee-provider/pkg/model"
	"github.com/computeflux/tee-provider/pkg/util"
	"github.com/computeflux/tee-provider/tee-chain/contracts/store"
)

var (
	ErrUserNotFound      = errors.New("user not found")
	ErrUserAlreadyExists = errors.New("user already exists")
	ErrInvalidThreshold  = errors.New("invalid threshold")
)

// Auth 认证合约 — 使用 []byte 作为 key
type Auth struct {
	api model.ContractApi

	config        *store.StoreValue[AuthConfig] `store:"keyPfx:v1_config"`
	users         *store.StoreMapping[[]byte, UserInfo]
	shareRoots    *store.StoreMapping[[]byte, []byte]
	recoveries    *store.StoreMapping[[]byte, RecoveryConfig]
	recoveryReqs  *store.StoreMapping[[]byte, RecoveryRequest]
	encryptedAuth *store.StoreMapping[string, AuthUserRecord]   // email → AuthUser（跨节点同步）
	encryptedKey  *store.StoreMapping[string, PrivateKeyRecord] // email → 私钥（跨节点同步）
}

type AuthQuery struct{ Auth }
type AuthMutation struct{ Auth }
type AuthSecret struct{ Auth }

// ── RegisterUser ─────────────────────────────────────────

func (a AuthMutation) RegisterUser(emailHash []byte, shareRoot []byte, address []byte) error {
	existing, _ := a.users.Get(a.api.GetTxn(), emailHash)
	if existing != nil {
		return ErrUserAlreadyExists
	}
	return a.users.Set(a.api.GetTxn(), emailHash, UserInfo{
		EmailHash:      emailHash,
		ShareRoot:      shareRoot,
		DerivedAddress: address,
		RegisteredAt:   uint64(a.api.GetBlockTime()),
	})
}

func (a AuthMutation) UpdateShareRoot(emailHash []byte, newRoot []byte) error {
	user, err := a.users.Get(a.api.GetTxn(), emailHash)
	if err != nil || user == nil {
		return ErrUserNotFound
	}
	user.ShareRoot = newRoot
	return a.users.Set(a.api.GetTxn(), emailHash, *user)
}

// ── Recovery ─────────────────────────────────────────────

func (a AuthMutation) SetRecovery(emailHash []byte, cfg RecoveryConfig) error {
	return a.recoveries.Set(a.api.GetTxn(), emailHash, cfg)
}

func (a AuthMutation) RequestRecovery(emailHash []byte, newRoot []byte) error {
	a.recoveries.Get(a.api.GetTxn(), emailHash) // validate exists
	return a.recoveryReqs.Set(a.api.GetTxn(), emailHash, RecoveryRequest{
		EmailHash:    emailHash,
		NewShareRoot: newRoot,
		Approvals:    [][]byte{},
		ExpiresAt:    uint64(a.api.GetBlockTime()) + 100,
	})
}

func (a AuthMutation) ApproveRecovery(emailHash []byte, guardian []byte) error {
	req, err := a.recoveryReqs.Get(a.api.GetTxn(), emailHash)
	if err != nil || req == nil {
		return ErrUserNotFound
	}
	for _, g := range req.Approvals {
		if bytesEq(g, guardian) {
			return nil
		}
	}
	req.Approvals = append(req.Approvals, guardian)
	return a.recoveryReqs.Set(a.api.GetTxn(), emailHash, *req)
}

func (a AuthMutation) CompleteRecovery(emailHash []byte, newShareRoot []byte) error {
	req, err := a.recoveryReqs.Get(a.api.GetTxn(), emailHash)
	if err != nil || req == nil {
		return ErrUserNotFound
	}
	cfg, err := a.recoveries.Get(a.api.GetTxn(), emailHash)
	if err != nil || cfg == nil {
		return ErrUserNotFound
	}
	if uint32(len(req.Approvals)) < cfg.Threshold {
		return errors.New("insufficient approvals")
	}
	return a.UpdateShareRoot(emailHash, newShareRoot)
}

// ── Query ────────────────────────────────────────────────

// ── Config ─────────────────────────────────────────────────

func (a AuthMutation) SetConfig(
	smtpHost string, smtpPort uint32,
	smtpUser, smtpPwd, smtpFrom, smtpFromName string, mockEmail bool,
	googleID, googleSecret, googleRedirect string,
	githubID, githubSecret, githubRedirect string,
	twitterID, twitterSecret, twitterRedirect string,
) error {
	existing, err := a.config.Get(a.api.GetTxn())
	if err == nil && existing != nil {
		if !a.api.CallbySystem() {
			util.LogWithYellow("AUTH", "config already set, only system can update")
			return errors.New("auth config already set, only system can update")
		}

		util.LogWithYellow("AUTH", "set config with system")

		// system 调用：保留原有的 JWTSecret 和 SeedSecret，更新其他字段
		cfg := AuthConfig{
			JWTSecret:           existing.JWTSecret,
			SeedSecret:          existing.SeedSecret,
			SMTPHost:            smtpHost,
			SMTPPort:            smtpPort,
			SMTPUser:            smtpUser,
			SMTPPassword:        smtpPwd,
			SMTPFrom:            smtpFrom,
			SMTPFromName:        smtpFromName,
			MockEmail:           mockEmail,
			OTPLength:           existing.OTPLength,
			OTPTTLMins:          existing.OTPTTLMins,
			GoogleClientID:      googleID,
			GoogleClientSecret:  googleSecret,
			GoogleRedirectURL:   googleRedirect,
			GitHubClientID:      githubID,
			GitHubClientSecret:  githubSecret,
			GitHubRedirectURL:   githubRedirect,
			TwitterClientID:     twitterID,
			TwitterClientSecret: twitterSecret,
			TwitterRedirectURL:  twitterRedirect,
		}
		return a.config.Set(a.api.GetTxn(), cfg)
	}

	util.LogWithYellow("AUTH", "config init")

	jwtSecret := make([]byte, 32)
	seedSecret := make([]byte, 32)
	if _, err := rand.Read(jwtSecret); err != nil {
		return err
	}
	if _, err := rand.Read(seedSecret); err != nil {
		return err
	}

	cfg := AuthConfig{
		JWTSecret:           hex.EncodeToString(jwtSecret),
		SeedSecret:          hex.EncodeToString(seedSecret),
		SMTPHost:            smtpHost,
		SMTPPort:            smtpPort,
		SMTPUser:            smtpUser,
		SMTPPassword:        smtpPwd,
		SMTPFrom:            smtpFrom,
		SMTPFromName:        smtpFromName,
		MockEmail:           mockEmail,
		OTPLength:           6,
		OTPTTLMins:          5,
		GoogleClientID:      googleID,
		GoogleClientSecret:  googleSecret,
		GoogleRedirectURL:   googleRedirect,
		GitHubClientID:      githubID,
		GitHubClientSecret:  githubSecret,
		GitHubRedirectURL:   githubRedirect,
		TwitterClientID:     twitterID,
		TwitterClientSecret: twitterSecret,
		TwitterRedirectURL:  twitterRedirect,
	}
	return a.config.Set(a.api.GetTxn(), cfg)
}

func (a AuthQuery) GetConfig() (*AuthConfig, error) {
	cfg, err := a.config.Get(a.api.GetTxn())
	if err != nil {
		return nil, err
	}
	if cfg == nil {
		return nil, nil
	}

	// 脱敏：隐藏敏感字段
	masked := &AuthConfig{
		JWTSecret:           maskIfNonEmpty(cfg.JWTSecret),
		SeedSecret:          maskIfNonEmpty(cfg.SeedSecret),
		SMTPHost:            cfg.SMTPHost,
		SMTPPort:            cfg.SMTPPort,
		SMTPUser:            maskIfNonEmpty(cfg.SMTPUser),
		SMTPPassword:        maskIfNonEmpty(cfg.SMTPPassword),
		SMTPFrom:            cfg.SMTPFrom,
		SMTPFromName:        cfg.SMTPFromName,
		MockEmail:           cfg.MockEmail,
		OTPLength:           cfg.OTPLength,
		OTPTTLMins:          cfg.OTPTTLMins,
		GoogleClientID:      cfg.GoogleClientID,
		GoogleClientSecret:  maskIfNonEmpty(cfg.GoogleClientSecret),
		GoogleRedirectURL:   cfg.GoogleRedirectURL,
		GitHubClientID:      cfg.GitHubClientID,
		GitHubClientSecret:  maskIfNonEmpty(cfg.GitHubClientSecret),
		GitHubRedirectURL:   cfg.GitHubRedirectURL,
		TwitterClientID:     cfg.TwitterClientID,
		TwitterClientSecret: maskIfNonEmpty(cfg.TwitterClientSecret),
		TwitterRedirectURL:  cfg.TwitterRedirectURL,
	}
	return masked, nil
}

func maskIfNonEmpty(s string) string {
	if s == "" {
		return ""
	}
	if len(s) <= 4 {
		return "****"
	}
	return s[:2] + "****" + s[len(s)-2:]
}

func (a AuthSecret) GetConfig() (*AuthConfig, error) {
	return a.config.Get(a.api.GetTxn())
}

// ── Encrypted AuthUser / EncryptedKey ─────────────────────

func (a AuthMutation) StoreAuthUser(ref string, ciphertext []byte) error {
	return a.encryptedAuth.Set(a.api.GetTxn(), ref, AuthUserRecord{
		Ref:        ref,
		Ciphertext: ciphertext,
	})
}

func (a AuthMutation) StorePrivateKey(ref string, ciphertext []byte) error {
	return a.encryptedKey.Set(a.api.GetTxn(), ref, PrivateKeyRecord{
		Ref:        ref,
		Ciphertext: ciphertext,
	})
}

func (a AuthQuery) GetAuthUser(ref string) (*AuthUserRecord, error) {
	return a.encryptedAuth.Get(a.api.GetTxn(), ref)
}

func (a AuthQuery) GetPrivateKey(ref string) (*PrivateKeyRecord, error) {
	return a.encryptedKey.Get(a.api.GetTxn(), ref)
}

// ── Query ────────────────────────────────────────────────

func (a AuthQuery) GetUser(emailHash []byte) (*UserInfo, error) {
	return a.users.Get(a.api.GetTxn(), emailHash)
}

func (a AuthQuery) HasKey(emailHash []byte) (bool, error) {
	u, err := a.users.Get(a.api.GetTxn(), emailHash)
	return u != nil && err == nil, err
}

func (a AuthQuery) GetRecovery(emailHash []byte) (*RecoveryConfig, error) {
	return a.recoveries.Get(a.api.GetTxn(), emailHash)
}

func bytesEq(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
