package auth

// UserInfo 用户信息（链上存储）
type UserInfo struct {
	EmailHash      []byte // email 的哈希 (H160)
	ShareRoot      []byte // 分片承诺 Merkle 根 (H256)
	DerivedAddress []byte // 派生地址
	RegisteredAt   uint64 // 注册区块高度
}

// AuthUserRecord Web3Auth 用户信息（跨节点同步）
type AuthUserRecord struct {
	Ref        string `json:"ref"`        // email（作为引用标识）
	Ciphertext []byte `json:"ciphertext"` // json(AuthUser)
}

// PrivateKeyRecord 用户私钥（跨节点同步）
type PrivateKeyRecord struct {
	Ref        string `json:"ref"`        // email
	Ciphertext []byte `json:"ciphertext"` // json(EncryptedKey)
}

// RecoveryConfig 社交恢复配置
type RecoveryConfig struct {
	Guardians     [][]byte // 守护者 email_hash 列表
	Threshold     uint32   // 恢复所需最小守护者数
	RecoveryEmail []byte   // 恢复邮箱（加密存储）
}

// RecoveryRequest 进行中的恢复请求
type RecoveryRequest struct {
	EmailHash    []byte
	NewShareRoot []byte
	Approvals    [][]byte // 已批准的守护者
	ExpiresAt    uint64
}

// AuthConfig 认证服务配置（链上存储，仅 AuthSecret 可查询）
type AuthConfig struct {
	JWTSecret           string `json:"jwt_secret"`
	SeedSecret          string `json:"seed_secret"`
	SMTPHost            string `json:"smtp_host"`
	SMTPPort            uint32 `json:"smtp_port"`
	SMTPUser            string `json:"smtp_user"`
	SMTPPassword        string `json:"smtp_password"`
	SMTPFrom            string `json:"smtp_from"`
	SMTPFromName        string `json:"smtp_from_name"`
	MockEmail           bool   `json:"mock_email"`
	OTPLength           uint32 `json:"otp_length"`
	OTPTTLMins          uint32 `json:"otp_ttl_mins"`
	GoogleClientID      string `json:"google_client_id"`
	GoogleClientSecret  string `json:"google_client_secret"`
	GoogleRedirectURL   string `json:"google_redirect_url"`
	GitHubClientID      string `json:"github_client_id"`
	GitHubClientSecret  string `json:"github_client_secret"`
	GitHubRedirectURL   string `json:"github_redirect_url"`
	TwitterClientID     string `json:"twitter_client_id"`
	TwitterClientSecret string `json:"twitter_client_secret"`
	TwitterRedirectURL  string `json:"twitter_redirect_url"`
}
