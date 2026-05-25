package model

import (
	"fmt"
	"time"
)

// ── Gateway 持久化类型 ───────────────────────────────────

type GatewayProviderInfo struct {
	ID uint64 `json:"id"`
	// Type string `json:"name"`
}

type GatewayTokenInfo struct {
	ID        uint64  `json:"id"`
	Owner     string  `json:"owner"`
	Addr      UniAddr `json:"addr"` // 完整的链上地址（含 T+V），用于结算
	Key       string  `json:"key"`  // bearer token value
	RateLimit uint32  `json:"rateLimit"`
	IsActive  bool    `json:"isActive"`
	CreatedAt uint64  `json:"createdAt"`
	ExpiresAt uint64  `json:"expiresAt"` // 过期时间（Unix 秒），0=永不过期
}

// IsExpired 检查 token 是否已过期。
func (t *GatewayTokenInfo) IsExpired() bool {
	if t.ExpiresAt == 0 {
		return false
	}
	return time.Now().Unix() > int64(t.ExpiresAt)
}

type GatewayDailyUsage struct {
	User UniAddr `json:"user"`
	// TokenID      uint64  `json:"tokenId"`
	ProviderID   uint64 `json:"providerId"`
	ModelID      uint64 `json:"modelId"`
	Day          string `json:"day"`
	Count        uint64 `json:"count"`
	TokensUsed   uint64 `json:"tokensUsed"`
	CachedTokens uint64 `json:"cachedTokens"`
	PointsUsed   uint64 `json:"pointsUsed"`
}

// ── Token ────────────────────────────────────────────────

func SetGatewayToken(id uint64, t *GatewayTokenInfo) error {
	t.ID = id
	return SetJson("GATEWAY", fmt.Sprintf("token:%d", id), t)
}

func GetGatewayToken(id uint64) (*GatewayTokenInfo, error) {
	return GetJson[GatewayTokenInfo]("GATEWAY", fmt.Sprintf("token:%d", id))
}

// SetGatewayTokenByKey 以 API Key 为索引写入 token 信息，供 O(1) 直接查询。
func SetGatewayTokenByKey(t *GatewayTokenInfo) error {
	return SetJson("GATEWAY", fmt.Sprintf("token_by_key:%s", t.Key), t)
}

// GetGatewayTokenByKey 通过 API Key 直接查询 token（O(1)，不遍历）。
func GetGatewayTokenByKey(key string) (*GatewayTokenInfo, error) {
	return GetJson[GatewayTokenInfo]("GATEWAY", fmt.Sprintf("token_by_key:%s", key))
}

// FindGatewayTokenByKey 通过 key 索引直接查询（O(1)），同时检查是否过期。
// 不再遍历整个 token 表。
func FindGatewayTokenByKey(key string) *GatewayTokenInfo {
	tk, err := GetGatewayTokenByKey(key)
	if err != nil || tk == nil {
		return nil
	}
	if !tk.IsActive || tk.IsExpired() {
		return nil
	}
	return tk
}

func ListUserGatewayTokens(user string, startKey uint64, size int) ([]*GatewayTokenInfo, error) {
	return nil, nil
}

// ── ID Counter ───────────────────────────────────────────

func GetNextID(key string) uint64 {
	val, err := GetJson[uint64]("GATEWAY", "next:"+key)
	if err != nil || val == nil {
		return 0
	}
	return *val
}

func SetNextID(key string, id uint64) error {
	return SetJson("GATEWAY", "next:"+key, &id)
}
