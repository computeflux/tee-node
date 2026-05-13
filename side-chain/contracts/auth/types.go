package auth

// UserInfo 用户信息（链上存储）
type UserInfo struct {
	EmailHash      []byte  // email 的哈希 (H160)
	ShareRoot      []byte  // 分片承诺 Merkle 根 (H256)
	DerivedAddress []byte  // 派生地址
	RegisteredAt   uint64  // 注册区块高度
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
	Approvals    [][]byte   // 已批准的守护者
	ExpiresAt    uint64
}
