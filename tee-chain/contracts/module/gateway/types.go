package gateway

import "github.com/computeflux/tee-provider/tee-chain/contracts/store"

// ── Modality bitmask ────────────────────────────────────

const (
	ModalityText  uint8 = 1 << iota // 0x01
	ModalityImage                   // 0x02
	ModalityFile                    // 0x04
	ModalityAudio                   // 0x08
	ModalityVideo                   // 0x10
)

// ── Region bitmask ──────────────────────────────────────

// 用户可选择就近的提供者；一个 Provider 可服务多区域。
const (
	RegionAsiaEast     uint8 = 1 << iota // 0x01 亚太
	RegionAsiaSouth                      // 0x02 南亚
	RegionEurope                         // 0x04 欧洲
	RegionNorthAmerica                   // 0x08 北美
	RegionSouthAmerica                   // 0x10 南美
	RegionMiddleEast                     // 0x20 中东
	RegionAfrica                         // 0x40 非洲
	RegionOceania                        // 0x80 大洋洲
)

// ── Provider Type ───────────────────────────────────────
const (
	ProviderOpenAI uint16 = iota + 1 // 1
	ProviderClaude                   // 2
	ProviderGemini                   // 3
	// ProviderPersonal 是个人算力（本地隧道）的 ProviderType 取值，
	ProviderPersonal uint16 = 9999
)

// ── Model ────────────────────────────────────────────────

// ModelInfo 独立注册的模型
// scale-codec-gen:generate
type ModelInfo struct {
	Name            string        `json:"name" idx:"exact"`
	Owner           store.UniAddr `json:"owner"`
	ProviderID      uint64        `json:"providerId" idx:"exact"`
	InputModalities uint8         `json:"inputModalities" idx:"exact"`
	ContextLength   uint64        `json:"contextLength"   idx:"range"`
	InPrice         uint64        `json:"inPrice"   idx:"range"`
	CachedPrice     uint64        `json:"cachedPrice"`
	OutPrice        uint64        `json:"outPrice"`

	// 供应商配置
	ProviderType uint16   `json:"providerType"`      // 供应商类型 (openai, claude, ...)
	ModelID      string   `json:"modelId"`           // 模型 ID
	URI          string   `json:"uri,omitempty"`     // 自定义端点，为空则用供应商官方地址
	APIKeys      []string `json:"apiKeys,omitempty"` // API Key 列表

	IsActive  bool   `json:"isActive" idx:"exact"`
	CreatedAt uint64 `json:"createdAt"`
}

// ModelInfoWithID 是 DataWithID[ModelInfo] 的非泛型版本，用于合约 metadata 兼容。
// scale-codec-gen:generate
type ModelInfoWithID struct {
	ID   uint64    `json:"id"`
	Data ModelInfo `json:"data"`
}

// ProviderInfoWithID 是 DataWithID[ProviderInfo] 的非泛型版本，用于合约 metadata 兼容。
// scale-codec-gen:generate
type ProviderInfoWithID struct {
	ID   uint64       `json:"id"`
	Data ProviderInfo `json:"data"`
}

// scale-codec-gen:generate
type ModelInfoWithProvider struct {
	ID       uint64 `json:"id"`
	Model    ModelInfo
	Provider ProviderInfo
}

// ── Provider / Provider ──────────────────────────────────

// ProviderInfo 去中心化 AI 服务提供者
// scale-codec-gen:generate
type ProviderInfo struct {
	// ── 身份 ──
	Owner store.UniAddr `json:"owner"`

	// ── 品牌 ──
	Name        string `json:"name" idx:"exact"` // 提供者名称
	LogoURL     string `json:"logoUrl"`          // logo 图片 URL (IPFS / HTTPS)
	Description string `json:"description"`      // 介绍

	// ── 服务能力 ──
	ModelIds []uint64 `json:"modelIds"` // 支持的模型列表

	// ── 地域（用户就近选择） ──
	Region uint8 `json:"region" idx:"exact"` // 服务区域位掩码

	// ── 经济安全 ──
	Deposit uint64 `json:"deposit"` // 质押金（可被罚没）
	FeeRate uint32 `json:"feeRate"` // 额外费率（基点，如 100=1%）

	// ── 信誉 ──
	Priority      uint32 `json:"priority"`      // 调度优先级
	TotalRequests uint64 `json:"totalRequests"` // 累计服务次数
	SuccessRate   uint32 `json:"successRate"`   // 成功率（基点，9900=99%）
	AvgLatency    uint32 `json:"avgLatency"`    // 平均延迟（ms）
	UptimeSLA     uint32 `json:"uptimeSLA"`     // 承诺可用率（基点，9990=99.9%）

	// ── 状态 ──
	IsActive  bool   `json:"isActive" idx:"exact"`
	CreatedAt uint64 `json:"createdAt"`
}

// ── API Key ──────────────────────────────────────────────

// APIKeyInfo 用户 API Key（仅存哈希，原始 key 绝不上链）
// TEE 提供方收到明文 key 后 blake2b 哈希，再以此查询链上记录
// scale-codec-gen:generate
type APIKeyInfo struct {
	OwnerKey  uint64        `json:"ownerKey" idx:"exact"` // blake2b(owner.V)[:8] 转 uint64，用于索引
	Owner     store.UniAddr `json:"owner"`
	KeyHash   []byte        `json:"keyHash"`   // blake2b(apiKey), 32 bytes
	RateLimit uint32        `json:"rateLimit"` // 每分钟最大请求数, 0=无限制
	IsActive  bool          `json:"isActive"`
	CreatedAt uint64        `json:"createdAt"`
}

// Settlement 单笔结算记录
// scale-codec-gen:generate
type Settlement struct {
	User        store.UniAddr `json:"user"`
	ModelID     uint64        `json:"modelId"`
	ProviderID  uint64        `json:"providerId"`
	TokenIn     uint64        `json:"TokenIn"`
	TokenOut    uint64        `json:"tokensOut"`
	CachedToken uint64        `json:"cachedToken"`
	Count       uint16        `json:"count"`
	Nonce       []byte        `json:"nonce"` // 16 bytes 随机 nonce，防重放
}

// ── DailyUsage ───────────────────────────────────────────

// DailyUsage 每日用量（已废弃，不再写入，保留类型用于兼容查询）
// scale-codec-gen:generate
type DailyUsage struct {
	User        store.UniAddr `json:"user"`
	ProviderID  uint64        `json:"providerId"`
	ModelID     uint64        `json:"modelId"`
	Count       uint64        `json:"count"`
	TokenIn     uint64        `json:"TokenIn"`
	TokenOut    uint64        `json:"tokensOut"`
	CachedToken uint64        `json:"cachedToken"`
	PointsUsed  uint64        `json:"pointsUsed"`
}

// ── RechargeHistory ──────────────────────────────────────

// RechargeRecord 单笔充值记录
// scale-codec-gen:generate
type RechargeRecord struct {
	User   store.UniAddr `json:"user" idx:"exact"`
	Amount uint64        `json:"amount"`
	Time   int64         `json:"time"`
}

// ── SettlementHistory ────────────────────────────────────

// SettlementRecord 单笔结算记录（Provider 视角）
// scale-codec-gen:generate
type UserSettlementRecord struct {
	ProviderID  uint64 `json:"providerId"`
	ModelID     uint64 `json:"modelId"`
	TokenIn     uint64 `json:"TokenIn"`
	TokenOut    uint64 `json:"tokensOut"`
	CachedToken uint64 `json:"cachedToken"`
	Cost        uint64 `json:"cost"`
	Count       uint16 `json:"count"`
	Time        int64  `json:"time"`
}

// SettlementRecord 单笔结算记录（Provider 视角）
// scale-codec-gen:generate
type ProviderSettlementRecord struct {
	User        store.UniAddr `json:"user" idx:"exact"`
	ModelID     uint64        `json:"modelId"`
	TokenIn     uint64        `json:"TokenIn"`
	TokenOut    uint64        `json:"tokensOut"`
	CachedToken uint64        `json:"cachedToken"`
	Cost        uint64        `json:"cost"`
	Count       uint16        `json:"count"`
	Time        int64         `json:"time"`
}

type WithdrawStatus uint8

const (
	WithdrawPending   WithdrawStatus = 0
	WithdrawCancelled WithdrawStatus = 1
	WithdrawDone      WithdrawStatus = 2
)

// scale-codec-gen:generate
type WithdrawApply struct {
	ChainID  uint32
	Contract []byte
	User     store.UniAddr
	Credit   store.U256
	Nonce    uint64
	Status   WithdrawStatus
}
