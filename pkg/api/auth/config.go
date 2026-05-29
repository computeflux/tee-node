package auth

import (
	"strconv"

	"github.com/computeflux/tee-provider/pkg/model"
	"github.com/computeflux/tee-provider/pkg/util"
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

const authNamespace = "auth"

func LoadConfig() *AuthConfig {
	cfg := &AuthConfig{OTPLength: 6, OTPTTLMins: 5}

	// 1. 优先从 TEE 密封存储加载（生产环境已部署后）
	if model.HasConfigNamespace(authNamespace) {
		readSealedString := func(key string) string {
			v, err := model.GetConfigValue(authNamespace, key)
			if err != nil {
				util.LogError("TEE CFG", "read sealed", key, err)
				return ""
			}
			return v
		}
		cfg.JWTSecret = readSealedString("JWT_SECRET")
		cfg.SeedSecret = readSealedString("SEED_SECRET")
		cfg.CORSOrigins = readSealedString("CORS_ORIGINS")
		cfg.GoogleClientID = readSealedString("GOOGLE_CLIENT_ID")
		cfg.GoogleClientSecret = readSealedString("GOOGLE_CLIENT_SECRET")
		cfg.GoogleRedirectURL = readSealedString("GOOGLE_REDIRECT_URL")
		cfg.GitHubClientID = readSealedString("GITHUB_CLIENT_ID")
		cfg.GitHubClientSecret = readSealedString("GITHUB_CLIENT_SECRET")
		cfg.GitHubRedirectURL = readSealedString("GITHUB_REDIRECT_URL")
		cfg.TwitterClientID = readSealedString("TWITTER_CLIENT_ID")
		cfg.TwitterClientSecret = readSealedString("TWITTER_CLIENT_SECRET")
		cfg.TwitterRedirectURL = readSealedString("TWITTER_REDIRECT_URL")
		cfg.SMTPHost = readSealedString("SMTP_HOST")
		cfg.SMTPUser = readSealedString("SMTP_USER")
		cfg.SMTPPassword = readSealedString("SMTP_PASSWORD")
		cfg.SMTPFrom = readSealedString("SMTP_FROM")
		cfg.SMTPFromName = readSealedString("SMTP_FROM_NAME")
		if v := readSealedString("SMTP_PORT"); v != "" {
			cfg.SMTPPort, _ = strconv.Atoi(v)
		} else {
			cfg.SMTPPort = 587
		}
		if v := readSealedString("MOCK_EMAIL"); v != "" {
			cfg.MockEmail = v == "true"
		} else {
			cfg.MockEmail = false
		}
		return cfg
	}

	// 2. 回退到环境变量（首次启动 / 开发环境）
	cfg.JWTSecret = util.GetEnv("JWT_SECRET", "dev-secret-change-in-production")
	cfg.SeedSecret = util.GetEnv("SEED_SECRET", "dev-seed-secret-change-in-production")
	cfg.MockEmail = util.GetEnv("MOCK_EMAIL", "true") == "true"
	cfg.CORSOrigins = util.GetEnv("CORS_ORIGINS", "*")
	cfg.SMTPHost = util.GetEnv("SMTP_HOST", "")
	cfg.SMTPPort = util.GetEnvInt("SMTP_PORT", 587)
	cfg.SMTPUser = util.GetEnv("SMTP_USER", "")
	cfg.SMTPPassword = util.GetEnv("SMTP_PASSWORD", "")
	cfg.SMTPFrom = util.GetEnv("SMTP_FROM", "")
	cfg.SMTPFromName = util.GetEnv("SMTP_FROM_NAME", "TEE Auth")
	cfg.GoogleClientID = util.GetEnv("GOOGLE_CLIENT_ID", "")
	cfg.GoogleClientSecret = util.GetEnv("GOOGLE_CLIENT_SECRET", "")
	cfg.GoogleRedirectURL = util.GetEnv("GOOGLE_REDIRECT_URL", "")
	cfg.TwitterClientID = util.GetEnv("TWITTER_CLIENT_ID", "")
	cfg.TwitterClientSecret = util.GetEnv("TWITTER_CLIENT_SECRET", "")
	cfg.TwitterRedirectURL = util.GetEnv("TWITTER_REDIRECT_URL", "")
	cfg.GitHubClientID = util.GetEnv("GITHUB_CLIENT_ID", "")
	cfg.GitHubClientSecret = util.GetEnv("GITHUB_CLIENT_SECRET", "")
	cfg.GitHubRedirectURL = util.GetEnv("GITHUB_REDIRECT_URL", "")
	return cfg
}
