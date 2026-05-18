package auth

// ── GraphQL 返回类型 (与 graph/auth.graphqls 对应) ──────────

// AuthResponse 认证操作返回
type AuthResponse struct {
	Success  bool      `json:"success"`
	UserInfo *UserInfo `json:"userInfo,omitempty"`
	Token    string    `json:"token,omitempty"`
	Address  string    `json:"address,omitempty"`
	Error    string    `json:"error,omitempty"`
}

// UserInfo 用户信息
type UserInfo struct {
	Email                   string `json:"email"`
	Name                    string `json:"name,omitempty"`
	ProfileImage            string `json:"profileImage,omitempty"`
	Verifier                string `json:"verifier,omitempty"`
	VerifierID              string `json:"verifierId,omitempty"`
	TypeOfLogin             string `json:"typeOfLogin,omitempty"`
	AggregateVerifier       string `json:"aggregateVerifier,omitempty"`
	AuthConnection          string `json:"authConnection,omitempty"`
	AuthConnectionID        string `json:"authConnectionId,omitempty"`
	GroupedAuthConnectionID string `json:"groupedAuthConnectionId,omitempty"`
	IDToken                 string `json:"idToken,omitempty"`
}

// OAuthURL OAuth 授权 URL
type OAuthURL struct {
	AuthUrl string `json:"authUrl"`
}

// WalletResult 钱包操作结果
type WalletResult struct {
	Status string `json:"status"`
	Result string `json:"result,omitempty"`
	Error  string `json:"error,omitempty"`
}

// RecoveryResult 恢复操作结果
type RecoveryResult struct {
	Status        string `json:"status"`
	RecoveryToken string `json:"recoveryToken,omitempty"`
	Token         string `json:"token,omitempty"`
	Address       string `json:"address,omitempty"`
	Email         string `json:"email,omitempty"`
	Error         string `json:"error,omitempty"`
}



// ── GraphQL 输入类型 ─────────────────────────────────────

// TxInput 交易签名输入
type TxInput struct {
	From         string `json:"from"`
	To           string `json:"to"`
	Value        string `json:"value,omitempty"`
	Data         string `json:"data,omitempty"`
	Gas          string `json:"gas,omitempty"`
	GasPrice     string `json:"gasPrice,omitempty"`
	Nonce        string `json:"nonce,omitempty"`
	ChainID      string `json:"chainId,omitempty"`
	PasswordHash string `json:"passwordHash,omitempty"`
}

// ── 内部类型 ──────────────────────────────────────────────

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
