package auth

import (
	"sync"

	teechain "github.com/computeflux/tee-provider/tee-chain"
)

// AuthConfig Web3Auth 业务配置
type AuthConfig struct {
	JWTSecret    string
	SeedSecret   string
	OTPLength    int
	OTPTTLMins   int
	MockEmail    bool
	CORSOrigins  string
	SMTPHost     string
	SMTPPort     int
	SMTPUser     string
	SMTPPassword string
	SMTPFrom     string
	SMTPFromName string

	// OAuth
	GoogleClientID      string
	GoogleClientSecret  string
	GoogleRedirectURL   string
	TwitterClientID     string
	TwitterClientSecret string
	TwitterRedirectURL  string
	GitHubClientID      string
	GitHubClientSecret  string
	GitHubRedirectURL   string
}

var (
	cacheMu   sync.RWMutex
	cachedCfg *AuthConfig
)

// LoadConfig 优先从链上合约读取配置；未设置时回退到环境变量。
// 首次从合约读取成功后缓存到内存，后续调用直接返回缓存。
func LoadConfig(tc *teechain.TeeChain) *AuthConfig {
	// 0. 命中缓存直接返回
	cacheMu.RLock()
	if cachedCfg != nil {
		cfg := cachedCfg
		cacheMu.RUnlock()
		return cfg
	}
	cacheMu.RUnlock()

	// 1. 从链上合约读取
	if tc != nil {
		cfg, err := tc.AuthGetConfig()
		if err == nil && cfg != nil {
			c := &AuthConfig{
				JWTSecret:           cfg.JWTSecret,
				SeedSecret:          cfg.SeedSecret,
				OTPLength:           6,
				OTPTTLMins:          5,
				CORSOrigins:         cfg.CORSOrigins,
				SMTPHost:            cfg.SMTPHost,
				SMTPPort:            int(cfg.SMTPPort),
				SMTPUser:            cfg.SMTPUser,
				SMTPPassword:        cfg.SMTPPassword,
				SMTPFrom:            cfg.SMTPFrom,
				SMTPFromName:        cfg.SMTPFromName,
				MockEmail:           cfg.MockEmail,
				GoogleClientID:      cfg.GoogleClientID,
				GoogleClientSecret:  cfg.GoogleClientSecret,
				GoogleRedirectURL:   cfg.GoogleRedirectURL,
				TwitterClientID:     cfg.TwitterClientID,
				TwitterClientSecret: cfg.TwitterClientSecret,
				TwitterRedirectURL:  cfg.TwitterRedirectURL,
				GitHubClientID:      cfg.GitHubClientID,
				GitHubClientSecret:  cfg.GitHubClientSecret,
				GitHubRedirectURL:   cfg.GitHubRedirectURL,
			}

			cacheMu.Lock()
			cachedCfg = c
			cacheMu.Unlock()
			return c
		}
	}

	// 2. 回退到环境变量（开发环境）
	return &AuthConfig{}
}
