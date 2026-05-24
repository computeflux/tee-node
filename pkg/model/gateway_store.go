package model

import (
	"encoding/json"
	"fmt"

	"github.com/cockroachdb/pebble"
	"github.com/computeflux/tee-provider/pkg/util"
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
}

type GatewayDailyUsage struct {
	User         UniAddr `json:"user"`
	TokenID      uint64  `json:"tokenId"`
	ProviderID   uint64  `json:"providerId"`
	ModelID      uint64  `json:"modelId"`
	Day          string  `json:"day"`
	Count        uint64  `json:"count"`
	TokensUsed   uint64  `json:"tokensUsed"`
	CachedTokens uint64  `json:"cachedTokens"`
	PointsUsed   uint64  `json:"pointsUsed"`
}

// ── Token ────────────────────────────────────────────────

func SetGatewayToken(id uint64, t *GatewayTokenInfo) error {
	t.ID = id
	return SetJson("GATEWAY", fmt.Sprintf("token:%d", id), t)
}

func GetGatewayToken(id uint64) (*GatewayTokenInfo, error) {
	return GetJson[GatewayTokenInfo]("GATEWAY", fmt.Sprintf("token:%d", id))
}

// FindGatewayTokenByKey finds a token by its bearer key using prefix scan.
func FindGatewayTokenByKey(key string) *GatewayTokenInfo {
	if DBINS == nil {
		return nil
	}
	prefix := []byte(comboKey("GATEWAY", "token:"))
	iter, err := DBINS.NewIter(&pebble.IterOptions{
		LowerBound: prefix,
		UpperBound: keyUpperBound(prefix),
	})
	if err != nil {
		return nil
	}
	defer iter.Close()

	for iter.First(); iter.Valid(); iter.Next() {
		v := iter.Value()
		decrypted, err := util.Unseal(v, nil)
		if err != nil {
			continue
		}
		var tk GatewayTokenInfo
		if json.Unmarshal(decrypted, &tk) != nil {
			continue
		}
		if tk.IsActive && tk.Key == key {
			return &tk
		}
	}
	return nil
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
