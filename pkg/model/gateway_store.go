package model

import (
	"encoding/json"
	"fmt"

	"github.com/cockroachdb/pebble"
	"github.com/computeflux/tee-provider/pkg/util"
)

// ── Gateway 持久化类型 ───────────────────────────────────

type GatewayModelInfo struct {
	Name        string `json:"name"`
	Owner       string `json:"owner"`
	PointsPer1K uint64 `json:"pointsPer1K"`
	IsActive    bool   `json:"isActive"`
	CreatedAt   uint64 `json:"createdAt"`
}

type GatewayProviderInfo struct {
	Name     string   `json:"name"`
	BaseURL  string   `json:"baseURL"`
	ModelIds []uint64 `json:"modelIds"`
	Priority uint32   `json:"priority"`
	IsActive bool     `json:"isActive"`
	Owner    string   `json:"owner"`
	ID       uint64   `json:"id"`
}

type GatewayTokenInfo struct {
	Owner     string `json:"owner"`
	Key       string `json:"key"` // bearer token value
	RateLimit uint32 `json:"rateLimit"`
	IsActive  bool   `json:"isActive"`
	CreatedAt uint64 `json:"createdAt"`
	ID        uint64 `json:"id"`
}

type GatewayUserBalance struct {
	User    string `json:"user"`
	Balance uint64 `json:"balance"`
}

type GatewayProviderBalance struct {
	ModelID uint64 `json:"modelId"`
	Balance uint64 `json:"balance"`
}

type GatewayDailyUsage struct {
	TokenID    uint64 `json:"tokenId"`
	ProviderID uint64 `json:"channelId"`
	ModelID    uint64 `json:"modelId"`
	Day        string `json:"day"`
	Count      uint64 `json:"count"`
	TokensUsed uint64 `json:"tokensUsed"`
	PointsUsed uint64 `json:"pointsUsed"`
}

type gatewayModelEntry struct {
	ID   uint64
	Info *GatewayModelInfo
}

// ── Model ────────────────────────────────────────────────

func SetGatewayModel(id uint64, m *GatewayModelInfo) error {
	return SetJson("GATEWAY", fmt.Sprintf("model:%d", id), m)
}

func GetGatewayModel(id uint64) (*GatewayModelInfo, error) {
	return GetJson[GatewayModelInfo]("GATEWAY", fmt.Sprintf("model:%d", id))
}

func ListGatewayModels(startKey uint64, size int) ([]*gatewayModelEntry, error) {
	return nil, nil
}

// ── Provider ──────────────────────────────────────────────

func SetGatewayProvider(id uint64, ch *GatewayProviderInfo) error {
	ch.ID = id
	return SetJson("GATEWAY", fmt.Sprintf("channel:%d", id), ch)
}

func GetGatewayProvider(id uint64) (*GatewayProviderInfo, error) {
	return GetJson[GatewayProviderInfo]("GATEWAY", fmt.Sprintf("channel:%d", id))
}

func ListActiveGatewayProviders(modelID uint64) ([]*GatewayProviderInfo, error) {
	return nil, nil
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

// ── Balance ──────────────────────────────────────────────

func SetGatewayBalance(user string, b *GatewayUserBalance) error {
	return SetJson("GATEWAY", fmt.Sprintf("bal:%s", user), b)
}

func GetGatewayBalance(user string) (*GatewayUserBalance, error) {
	return GetJson[GatewayUserBalance]("GATEWAY", fmt.Sprintf("bal:%s", user))
}

// ── Provider Balance ─────────────────────────────────────

func SetGatewayProviderBalance(modelID uint64, b *GatewayProviderBalance) error {
	return SetJson("GATEWAY", fmt.Sprintf("pb:%d", modelID), b)
}

func GetGatewayProviderBalance(modelID uint64) (*GatewayProviderBalance, error) {
	return GetJson[GatewayProviderBalance]("GATEWAY", fmt.Sprintf("pb:%d", modelID))
}

// ── Usage ────────────────────────────────────────────────

func SetGatewayDailyUsage(tokenID uint64, day string, u *GatewayDailyUsage) error {
	return SetJson("GATEWAY", fmt.Sprintf("usage:%d:%s", tokenID, day), u)
}

func GetGatewayDailyUsage(tokenID uint64, day string) (*GatewayDailyUsage, error) {
	return GetJson[GatewayDailyUsage]("GATEWAY", fmt.Sprintf("usage:%d:%s", tokenID, day))
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
