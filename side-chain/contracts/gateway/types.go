package gateway

import "github.com/computeflux/tee-provider/pkg/model"

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

// ProviderType 模型供应商类型
type ProviderType uint16

const (
	ProviderOpenAI   ProviderType = iota + 1 // 1
	ProviderClaude                           // 2
	ProviderGemini                           // 3
	ProviderAzure                            // 4
	ProviderDeepSeek                         // 5
	ProviderLlama                            // 6
	ProviderMistral                          // 7
	ProviderCohere                           // 8
)

// ── Model ────────────────────────────────────────────────

// ModelInfo 独立注册的模型
type ModelInfo struct {
	Name            string        `json:"name"`
	Owner           model.UniAddr `json:"owner"`
	PointsPer1K     uint64        `json:"pointsPer1K"`
	IsActive        bool          `json:"isActive"`
	CreatedAt       uint64        `json:"createdAt"`
	InputModalities uint8         `json:"inputModalities" idx:"exact"`
	ContextLength   uint64        `json:"contextLength"   idx:"bucket:4000,8000,16000,32000,64000,128000,256000,512000,1000000"`
	PromptPricing   uint64        `json:"promptPricing"   idx:"bucket:1,5,10,50,100,500,1000,5000,10000"`

	// 供应商配置
	ProviderType ProviderType `json:"providerType"`      // 供应商类型 (openai, claude, ...)
	URI          string       `json:"uri,omitempty"`     // 自定义端点，为空则用供应商官方地址
	APIKeys      []string     `json:"apiKeys,omitempty"` // 多个 API Key，轮询/故障转移
}

// ── Provider / Channel ──────────────────────────────────

// ProviderInfo 去中心化 AI 服务提供者
type ProviderInfo struct {
	// ── 身份 ──
	Owner model.UniAddr `json:"owner"`

	// ── 品牌 ──
	Name        string `json:"name"`        // 提供者名称
	LogoURL     string `json:"logoUrl"`     // logo 图片 URL (IPFS / HTTPS)
	Description string `json:"description"` // 介绍

	// ── 服务能力 ──
	BaseURL      string       `json:"baseURL"`           // API 端点
	ModelIds     []uint64     `json:"modelIds"`          // 支持的模型列表
	ProviderType ProviderType `json:"providerType"`      // 供应商类型 (openai, claude, ...)
	URI          string       `json:"uri,omitempty"`     // 自定义端点，为空则用供应商官方地址
	APIKeys      []string     `json:"apiKeys,omitempty"` // 多个 API Key，轮询/故障转移

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
	IsActive  bool   `json:"isActive"`
	CreatedAt uint64 `json:"createdAt"`
}

// ── Token ────────────────────────────────────────────────

// TokenInfo API Token
type TokenInfo struct {
	Owner     model.UniAddr `json:"owner"`
	RateLimit uint32        `json:"rateLimit"`
	IsActive  bool          `json:"isActive"`
	CreatedAt uint64        `json:"createdAt"`
}

// ── DailyUsage ───────────────────────────────────────────

// DailyUsage 每日用量
type DailyUsage struct {
	TokenID    uint64 `json:"tokenId"`
	ProviderID uint64 `json:"channelId"`
	ModelID    uint64 `json:"modelId"`
	Day        string `json:"day"`
	Count      uint64 `json:"count"`
	TokensUsed uint64 `json:"tokensUsed"`
	PointsUsed uint64 `json:"pointsUsed"`
}
