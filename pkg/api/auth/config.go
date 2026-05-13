package auth

import (
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

func LoadConfig() *AuthConfig {
	return &AuthConfig{
		JWTSecret:    util.GetEnv("JWT_SECRET", "dev-secret-change-in-production"),
		SeedSecret:   util.GetEnv("SEED_SECRET", "dev-seed-secret-change-in-production"),
		OTPLength:    6,
		OTPTTLMins:   5,
		MockEmail:    util.GetEnv("MOCK_EMAIL", "true") == "true",
		CORSOrigins:  util.GetEnv("CORS_ORIGINS", "*"),
		SMTPHost:     util.GetEnv("SMTP_HOST", ""),
		SMTPPort:     util.GetEnvInt("SMTP_PORT", 587),
		SMTPUser:     util.GetEnv("SMTP_USER", ""),
		SMTPPassword: util.GetEnv("SMTP_PASSWORD", ""),
		SMTPFrom:     util.GetEnv("SMTP_FROM", ""),
		SMTPFromName: util.GetEnv("SMTP_FROM_NAME", "WeTEE Auth"),

		GoogleClientID:      util.GetEnv("GOOGLE_CLIENT_ID", ""),
		GoogleClientSecret:  util.GetEnv("GOOGLE_CLIENT_SECRET", ""),
		GoogleRedirectURL:   util.GetEnv("GOOGLE_REDIRECT_URL", ""),
		TwitterClientID:     util.GetEnv("TWITTER_CLIENT_ID", ""),
		TwitterClientSecret: util.GetEnv("TWITTER_CLIENT_SECRET", ""),
		TwitterRedirectURL:  util.GetEnv("TWITTER_REDIRECT_URL", ""),
		GitHubClientID:      util.GetEnv("GITHUB_CLIENT_ID", ""),
		GitHubClientSecret:  util.GetEnv("GITHUB_CLIENT_SECRET", ""),
		GitHubRedirectURL:   util.GetEnv("GITHUB_REDIRECT_URL", ""),
	}
}
