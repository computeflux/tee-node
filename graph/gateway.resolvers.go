package graph

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"encoding/hex"
	"fmt"

	"github.com/computeflux/tee-provider/pkg/model"
	gateway "github.com/computeflux/tee-provider/pkg/api/gateway"
	"github.com/computeflux/tee-provider/services/gateway/relay"
)

// ── Generated types (normally from gqlgen, defined here for compilation) ─

type GatewayModel struct {
	ID          int    `json:"id"`
	Name        string `json:"name"`
	Owner       string `json:"owner"`
	PointsPer1K int    `json:"pointsPer1K"`
	IsActive    bool   `json:"isActive"`
	CreatedAt   int    `json:"createdAt"`
}

type GatewayProvider struct {
	ID       int    `json:"id"`
	Name     string `json:"name"`
	BaseURL  string `json:"baseURL"`
	ModelIds []int  `json:"modelIds"`
	Priority int    `json:"priority"`
	IsActive bool   `json:"isActive"`
	Owner    string `json:"owner"`
}

type GatewayToken struct {
	ID        int    `json:"id"`
	Owner     string `json:"owner"`
	Key       string `json:"key"`
	RateLimit int    `json:"rateLimit"`
	IsActive  bool   `json:"isActive"`
	CreatedAt int    `json:"createdAt"`
}

type PointsBalance struct {
	User    string `json:"user"`
	Balance int    `json:"balance"`
}

type DailyUsage struct {
	TokenID    int    `json:"tokenId"`
	ProviderID  int    `json:"channelId"`
	ModelID    int    `json:"modelId"`
	Day        string `json:"day"`
	Count      int    `json:"count"`
	TokensUsed int    `json:"tokensUsed"`
	PointsUsed int    `json:"pointsUsed"`
}


// ── Mutation resolvers ───────────────────────────────────

func (r *mutationResolver) RegisterModel(ctx context.Context, name string, pointsPer1K int) (*GatewayModel, error) {
	// Phase 2: 暂用 Pebble 存储，Phase 3 改为合约调用
	// 生成自增 ID
	nextID := model.GetNextID("gateway_model")
	gm := model.GatewayModelInfo{
		Name:        name,
		Owner:       "", // TODO: 从 JWT 获取 caller
		PointsPer1K: uint64(pointsPer1K),
		IsActive:    true,
		CreatedAt:   uint64(0), // TODO: block height
	}
	model.SetGatewayModel(nextID, &gm)
	model.SetNextID("gateway_model", nextID+1)

	return &GatewayModel{
		ID:          int(nextID),
		Name:        gm.Name,
		Owner:       gm.Owner,
		PointsPer1K: int(gm.PointsPer1K),
		IsActive:    gm.IsActive,
		CreatedAt:   int(gm.CreatedAt),
	}, nil
}

func (r *mutationResolver) UpdateModel(ctx context.Context, modelID int, pointsPer1K int) (*GatewayModel, error) {
	gm, err := model.GetGatewayModel(uint64(modelID))
	if err != nil || gm == nil {
		return nil, fmt.Errorf("model not found")
	}
	gm.PointsPer1K = uint64(pointsPer1K)
	model.SetGatewayModel(uint64(modelID), gm)

	return &GatewayModel{
		ID: modelID, Name: gm.Name, Owner: gm.Owner,
		PointsPer1K: int(gm.PointsPer1K), IsActive: gm.IsActive,
		CreatedAt: int(gm.CreatedAt),
	}, nil
}

func (r *mutationResolver) DeactivateModel(ctx context.Context, modelID int) (bool, error) {
	gm, err := model.GetGatewayModel(uint64(modelID))
	if err != nil || gm == nil {
		return false, fmt.Errorf("model not found")
	}
	gm.IsActive = false
	model.SetGatewayModel(uint64(modelID), gm)
	return true, nil
}

func (r *mutationResolver) CreateProvider(ctx context.Context, name, baseURL string, modelIds []int, priority int) (*GatewayProvider, error) {
	nextID := model.GetNextID("gateway_provider")
	ids := make([]uint64, len(modelIds))
	for i, v := range modelIds {
		ids[i] = uint64(v)
	}
	ch := model.GatewayProviderInfo{
		Name: name, BaseURL: baseURL, ModelIds: ids,
		Priority: uint32(priority), IsActive: true, Owner: "",
	}
	model.SetGatewayProvider(nextID, &ch)
	model.SetNextID("gateway_provider", nextID+1)

	return &GatewayProvider{
		ID: int(nextID), Name: ch.Name, BaseURL: ch.BaseURL,
		ModelIds: modelIds, Priority: int(ch.Priority),
		IsActive: ch.IsActive, Owner: ch.Owner,
	}, nil
}

func (r *mutationResolver) UpdateProvider(ctx context.Context, providerID int, modelIds []int, priority *int) (*GatewayProvider, error) {
	ch, err := model.GetGatewayProvider(uint64(providerID))
	if err != nil || ch == nil {
		return nil, fmt.Errorf("channel not found")
	}
	if modelIds != nil {
		ids := make([]uint64, len(modelIds))
		for i, v := range modelIds {
			ids[i] = uint64(v)
		}
		ch.ModelIds = ids
	}
	if priority != nil {
		ch.Priority = uint32(*priority)
	}
	model.SetGatewayProvider(uint64(providerID), ch)

	resultIds := make([]int, len(ch.ModelIds))
	for i, v := range ch.ModelIds {
		resultIds[i] = int(v)
	}
	return &GatewayProvider{
		ID: providerID, Name: ch.Name, BaseURL: ch.BaseURL,
		ModelIds: resultIds, Priority: int(ch.Priority),
		IsActive: ch.IsActive, Owner: ch.Owner,
	}, nil
}

func (r *mutationResolver) CreateToken(ctx context.Context, rateLimit int) (*GatewayToken, error) {
	nextID := model.GetNextID("gateway_token")
	key := fmt.Sprintf("gw_%d_%s", nextID, randomHex(16))
	tk := model.GatewayTokenInfo{
		Owner: "", Key: key, RateLimit: uint32(rateLimit), IsActive: true, CreatedAt: 0,
	}
	model.SetGatewayToken(nextID, &tk)
	model.SetNextID("gateway_token", nextID+1)

	return &GatewayToken{
		ID: int(nextID), Owner: tk.Owner, Key: tk.Key,
		RateLimit: int(tk.RateLimit), IsActive: tk.IsActive, CreatedAt: int(tk.CreatedAt),
	}, nil
}

func (r *mutationResolver) RevokeToken(ctx context.Context, tokenID int) (bool, error) {
	tk, err := model.GetGatewayToken(uint64(tokenID))
	if err != nil || tk == nil {
		return false, fmt.Errorf("token not found")
	}
	tk.IsActive = false
	model.SetGatewayToken(uint64(tokenID), tk)
	return true, nil
}

func (r *mutationResolver) Recharge(ctx context.Context, user string, amount int) (*PointsBalance, error) {
	bal, _ := model.GetGatewayBalance(user)
	if bal == nil {
		bal = &model.GatewayUserBalance{User: user, Balance: 0}
	}
	bal.Balance += uint64(amount)
	model.SetGatewayBalance(user, bal)

	return &PointsBalance{User: user, Balance: int(bal.Balance)}, nil
}

func (r *mutationResolver) Withdraw(ctx context.Context, modelID int, amount int) (*PointsBalance, error) {
	pb, _ := model.GetGatewayProviderBalance(uint64(modelID))
	if pb == nil {
		return nil, fmt.Errorf("no balance for model %d", modelID)
	}
	if pb.Balance < uint64(amount) {
		return nil, fmt.Errorf("insufficient balance")
	}
	pb.Balance -= uint64(amount)
	model.SetGatewayProviderBalance(uint64(modelID), pb)

	return &PointsBalance{
		User:    fmt.Sprintf("model_%d", modelID),
		Balance: int(pb.Balance),
	}, nil
}

// ── Query resolvers ──────────────────────────────────────

func (r *queryResolver) PointsBalance(ctx context.Context, user string) (*PointsBalance, error) {
	bal, _ := model.GetGatewayBalance(user)
	if bal == nil {
		return &PointsBalance{User: user, Balance: 0}, nil
	}
	return &PointsBalance{User: user, Balance: int(bal.Balance)}, nil
}

func (r *queryResolver) GatewayModel(ctx context.Context, modelID int) (*GatewayModel, error) {
	gm, err := model.GetGatewayModel(uint64(modelID))
	if err != nil || gm == nil {
		return nil, nil
	}
	return &GatewayModel{
		ID: modelID, Name: gm.Name, Owner: gm.Owner,
		PointsPer1K: int(gm.PointsPer1K), IsActive: gm.IsActive,
		CreatedAt: int(gm.CreatedAt),
	}, nil
}

func (r *queryResolver) ListModels(ctx context.Context, startKey *int, size int) ([]*GatewayModel, error) {
	sk := uint64(0)
	if startKey != nil {
		sk = uint64(*startKey)
	}
	infos, err := model.ListGatewayModels(sk, size)
	if err != nil {
		return nil, err
	}
	result := make([]*GatewayModel, len(infos))
	for i, gm := range infos {
		result[i] = &GatewayModel{
			ID: int(gm.ID),
			Name: gm.Info.Name, Owner: gm.Info.Owner,
			PointsPer1K: int(gm.Info.PointsPer1K), IsActive: gm.Info.IsActive,
			CreatedAt: int(gm.Info.CreatedAt),
		}
	}
	return result, nil
}

func (r *queryResolver) GatewayProvider(ctx context.Context, providerID int) (*GatewayProvider, error) {
	ch, err := model.GetGatewayProvider(uint64(providerID))
	if err != nil || ch == nil {
		return nil, nil
	}
	ids := make([]int, len(ch.ModelIds))
	for i, v := range ch.ModelIds {
		ids[i] = int(v)
	}
	return &GatewayProvider{
		ID: providerID, Name: ch.Name, BaseURL: ch.BaseURL,
		ModelIds: ids, Priority: int(ch.Priority),
		IsActive: ch.IsActive, Owner: ch.Owner,
	}, nil
}

func (r *queryResolver) ListActiveProviders(ctx context.Context, modelID int) ([]*GatewayProvider, error) {
	channels, err := model.ListActiveGatewayProviders(uint64(modelID))
	if err != nil {
		return nil, err
	}
	result := make([]*GatewayProvider, len(channels))
	for i, ch := range channels {
		ids := make([]int, len(ch.ModelIds))
		for j, v := range ch.ModelIds {
			ids[j] = int(v)
		}
		result[i] = &GatewayProvider{
			ID: int(ch.ID), Name: ch.Name, BaseURL: ch.BaseURL,
			ModelIds: ids, Priority: int(ch.Priority),
			IsActive: ch.IsActive, Owner: ch.Owner,
		}
	}
	return result, nil
}

func (r *queryResolver) GatewayToken(ctx context.Context, tokenID int) (*GatewayToken, error) {
	tk, err := model.GetGatewayToken(uint64(tokenID))
	if err != nil || tk == nil {
		return nil, nil
	}
	return &GatewayToken{
		ID: tokenID, Owner: tk.Owner, Key: tk.Key,
		RateLimit: int(tk.RateLimit), IsActive: tk.IsActive,
		CreatedAt: int(tk.CreatedAt),
	}, nil
}

func (r *queryResolver) ListUserTokens(ctx context.Context, user string, startKey *int, size int) ([]*GatewayToken, error) {
	sk := uint64(0)
	if startKey != nil {
		sk = uint64(*startKey)
	}
	tokens, err := model.ListUserGatewayTokens(user, sk, size)
	if err != nil {
		return nil, err
	}
	result := make([]*GatewayToken, len(tokens))
	for i, tk := range tokens {
		result[i] = &GatewayToken{
			ID: int(tk.ID), Owner: tk.Owner, Key: tk.Key,
			RateLimit: int(tk.RateLimit), IsActive: tk.IsActive,
			CreatedAt: int(tk.CreatedAt),
		}
	}
	return result, nil
}

func (r *queryResolver) DailyUsage(ctx context.Context, tokenID int, day string) (*DailyUsage, error) {
	u, err := model.GetGatewayDailyUsage(uint64(tokenID), day)
	if err != nil || u == nil {
		return nil, nil
	}
	return &DailyUsage{
		TokenID: tokenID, ProviderID: int(u.ProviderID),
		ModelID: int(u.ModelID), Day: u.Day,
		Count: int(u.Count), TokensUsed: int(u.TokensUsed),
		PointsUsed: int(u.PointsUsed),
	}, nil
}

func randomHex(n int) string {
	b := make([]byte, n)
	rand.Read(b)
	return hex.EncodeToString(b)
}

// ── Pool management ──────────────────────────────────────

type PoolStats struct {
	MaxConnsPerHost     int    `json:"maxConnsPerHost"`
	MaxIdleConns        int    `json:"maxIdleConns"`
	MaxIdleConnsPerHost int    `json:"maxIdleConnsPerHost"`
	RequestTimeout      string `json:"requestTimeout"`
}

func (r *mutationResolver) AdjustPool(ctx context.Context, maxConnsPerHost int) (*PoolStats, error) {
	cfg := relay.GetPoolConfig()
	cfg.MaxConnsPerHost = maxConnsPerHost
	relay.ApplyPoolConfig(cfg)
	return &PoolStats{
		MaxConnsPerHost:     cfg.MaxConnsPerHost,
		MaxIdleConns:        cfg.MaxIdleConns,
		MaxIdleConnsPerHost: cfg.MaxIdleConnsPerHost,
		RequestTimeout:      cfg.RequestTimeout.String(),
	}, nil
}

func (r *queryResolver) PoolStats(ctx context.Context) (*PoolStats, error) {
	cfg := relay.GetPoolConfig()
	return &PoolStats{
		MaxConnsPerHost:     cfg.MaxConnsPerHost,
		MaxIdleConns:        cfg.MaxIdleConns,
		MaxIdleConnsPerHost: cfg.MaxIdleConnsPerHost,
		RequestTimeout:      cfg.RequestTimeout.String(),
	}, nil
}

// ── Scaler ───────────────────────────────────────────────

func (r *mutationResolver) EnableScaler(ctx context.Context, enabled bool) (bool, error) {
	gateway.SetScalerEnabled(enabled)
	return enabled, nil
}

func (r *queryResolver) ScalerStats(ctx context.Context) (string, error) {
	stats := gateway.GetScalerStats()
	b, _ := json.Marshal(stats)
	return string(b), nil
}
