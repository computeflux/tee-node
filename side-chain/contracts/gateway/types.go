package gateway

import "github.com/computeflux/tee-provider/pkg/model"

// ModelInfo 独立注册的模型
type ModelInfo struct {
	Name        string       `json:"name"`
	Owner       model.UniAddr `json:"owner"`
	PointsPer1K uint64       `json:"pointsPer1K"`
	IsActive    bool         `json:"isActive"`
	CreatedAt   uint64       `json:"createdAt"`
}

// ChannelInfo Provider 信息
type ChannelInfo struct {
	Name     string       `json:"name"`
	BaseURL  string       `json:"baseURL"`
	ModelIds []uint64     `json:"modelIds"`
	Priority uint32       `json:"priority"`
	IsActive bool         `json:"isActive"`
	Owner    model.UniAddr `json:"owner"`
}

// TokenInfo API Token
type TokenInfo struct {
	Owner     model.UniAddr `json:"owner"`
	RateLimit uint32       `json:"rateLimit"`
	IsActive  bool         `json:"isActive"`
	CreatedAt uint64       `json:"createdAt"`
}

// DailyUsage 每日用量
type DailyUsage struct {
	TokenID    uint64 `json:"tokenId"`
	ProviderID  uint64 `json:"channelId"`
	ModelID    uint64 `json:"modelId"`
	Day        string `json:"day"`
	Count      uint64 `json:"count"`
	TokensUsed uint64 `json:"tokensUsed"`
	PointsUsed uint64 `json:"pointsUsed"`
}
