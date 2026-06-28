package auth

// ── Service 层内部类型 ─────────────────────────────────────

// OAuthUserInfo OAuth 认证返回的用户信息
type OAuthUserInfo struct {
	Email      string
	Name       string
	AvatarURL  string
	ProviderID string
}

// JWTClaims JWT 载荷
type JWTClaims struct {
	Email        string `json:"email"`
	Address      string `json:"address"`
	AuthProvider string `json:"authProvider"`
}
