package gateway

import (
	"errors"
	"fmt"

	"github.com/computeflux/tee-provider/pkg/model"
)

var (
	ErrLowBalance        = errors.New("low balance")
	ErrModelNotFound     = errors.New("model not found")
	ErrChannelNotFound   = errors.New("channel not found")
	ErrTokenNotFound     = errors.New("token not found")
	ErrNotOwner          = errors.New("not owner")
)

// Gateway API 网关合约
type Gateway struct {
	api model.ContractApi

	balances         *model.StoreMapping[string, uint64]       // user -> balance
	models           *model.StoreMapping[uint64, ModelInfo]    // modelId -> ModelInfo
	providerBalances *model.StoreMapping[uint64, uint64]       // modelId -> providerBalance
	channels         *model.StoreMapping[uint64, ChannelInfo]  // channelId -> ChannelInfo
	tokens           *model.StoreMapping[uint64, TokenInfo]    // tokenId -> TokenInfo
	usages           *model.StoreMapping[string, DailyUsage]   // "{tokenId}_{day}" -> DailyUsage

	nextModelID   *model.StoreValue[uint64]
	nextProviderID *model.StoreValue[uint64]
	nextTokenID   *model.StoreValue[uint64]
}

type GatewayQuery struct{ Gateway }
type GatewayMutation struct{ Gateway }

// ── Init ─────────────────────────────────────────────────

func (g GatewayMutation) Init() error {
	g.nextModelID.Set(g.api.GetTxn(), 0)
	g.nextProviderID.Set(g.api.GetTxn(), 0)
	return g.nextTokenID.Set(g.api.GetTxn(), 0)
}

// ── Model ────────────────────────────────────────────────

func (g GatewayMutation) RegisterModel(owner model.UniAddr, name string, pointsPer1K uint64) (uint64, error) {
	id, _ := g.nextModelID.Get(g.api.GetTxn())
	if id == nil {
		id = new(uint64)
	}

	if err := g.models.Set(g.api.GetTxn(), *id, ModelInfo{
		Name: name, Owner: owner, PointsPer1K: pointsPer1K,
		IsActive: true, CreatedAt: uint64(g.api.GetHeight()),
	}); err != nil {
		return 0, err
	}

	g.nextModelID.Set(g.api.GetTxn(), *id+1)
	return *id, nil
}

func (g GatewayMutation) UpdateModel(modelID uint64, pointsPer1K uint64) error {
	m, err := g.models.Get(g.api.GetTxn(), modelID)
	if err != nil || m == nil {
		return ErrModelNotFound
	}
	m.PointsPer1K = pointsPer1K
	return g.models.Set(g.api.GetTxn(), modelID, *m)
}

func (g GatewayMutation) DeactivateModel(modelID uint64) error {
	m, err := g.models.Get(g.api.GetTxn(), modelID)
	if err != nil || m == nil {
		return ErrModelNotFound
	}
	m.IsActive = false
	return g.models.Set(g.api.GetTxn(), modelID, *m)
}

// ── Provider ──────────────────────────────────────────────

func (g GatewayMutation) CreateProvider(owner model.UniAddr, name, baseURL string, modelIds []uint64, priority uint32) (uint64, error) {
	id, _ := g.nextProviderID.Get(g.api.GetTxn())
	if id == nil {
		id = new(uint64)
	}

	if err := g.channels.Set(g.api.GetTxn(), *id, ChannelInfo{
		Name: name, BaseURL: baseURL, ModelIds: modelIds,
		Priority: priority, IsActive: true, Owner: owner,
	}); err != nil {
		return 0, err
	}

	g.nextProviderID.Set(g.api.GetTxn(), *id+1)
	return *id, nil
}

func (g GatewayMutation) UpdateProvider(providerID uint64, modelIds []uint64, priority uint32) error {
	ch, err := g.channels.Get(g.api.GetTxn(), providerID)
	if err != nil || ch == nil {
		return ErrChannelNotFound
	}
	if modelIds != nil {
		ch.ModelIds = modelIds
	}
	ch.Priority = priority
	return g.channels.Set(g.api.GetTxn(), providerID, *ch)
}

// ── Points ───────────────────────────────────────────────

func (g GatewayMutation) Recharge(user model.UniAddr, amount uint64) error {
	key := string(user.V)
	bal, _ := g.balances.Get(g.api.GetTxn(), key)
	if bal == nil {
		bal = new(uint64)
	}
	*bal += amount
	return g.balances.Set(g.api.GetTxn(), key, *bal)
}

func (g GatewayMutation) Consume(tokenID, providerID, modelID uint64, tokensUsed uint64) error {
	// 查 token owner
	tk, err := g.tokens.Get(g.api.GetTxn(), tokenID)
	if err != nil || tk == nil {
		return ErrTokenNotFound
	}

	// 查模型价格
	m, err := g.models.Get(g.api.GetTxn(), modelID)
	if err != nil || m == nil {
		return ErrModelNotFound
	}

	// 计算费用
	cost := (tokensUsed + 999) / 1000 * m.PointsPer1K
	if cost < 1 {
		cost = 1
	}

	// 扣用户余额
	key := string(tk.Owner.V)
	bal, _ := g.balances.Get(g.api.GetTxn(), key)
	if bal == nil || *bal < cost {
		return ErrLowBalance
	}
	*bal -= cost
	g.balances.Set(g.api.GetTxn(), key, *bal)

	// 加供应商收益
	pb, _ := g.providerBalances.Get(g.api.GetTxn(), modelID)
	if pb == nil {
		pb = new(uint64)
	}
	*pb += cost
	g.providerBalances.Set(g.api.GetTxn(), modelID, *pb)

	// 记录用量
	dayKey := fmt.Sprintf("%d_%d_%s", tokenID, modelID, "today") // simplified
	usage, _ := g.usages.Get(g.api.GetTxn(), dayKey)
	if usage == nil {
		usage = &DailyUsage{TokenID: tokenID, ProviderID: providerID, ModelID: modelID}
	}
	usage.Count++
	usage.TokensUsed += tokensUsed
	usage.PointsUsed += cost
	return g.usages.Set(g.api.GetTxn(), dayKey, *usage)
}

func (g GatewayMutation) Withdraw(modelID uint64, amount uint64) error {
	pb, err := g.providerBalances.Get(g.api.GetTxn(), modelID)
	if err != nil || pb == nil {
		return ErrModelNotFound
	}
	if *pb < amount {
		return ErrLowBalance
	}
	*pb -= amount
	return g.providerBalances.Set(g.api.GetTxn(), modelID, *pb)
}

// ── Token ────────────────────────────────────────────────

func (g GatewayMutation) CreateToken(owner model.UniAddr, rateLimit uint32) (uint64, error) {
	id, _ := g.nextTokenID.Get(g.api.GetTxn())
	if id == nil {
		id = new(uint64)
	}

	if err := g.tokens.Set(g.api.GetTxn(), *id, TokenInfo{
		Owner: owner, RateLimit: rateLimit, IsActive: true,
		CreatedAt: uint64(g.api.GetHeight()),
	}); err != nil {
		return 0, err
	}

	g.nextTokenID.Set(g.api.GetTxn(), *id+1)
	return *id, nil
}

func (g GatewayMutation) RevokeToken(tokenID uint64) error {
	tk, err := g.tokens.Get(g.api.GetTxn(), tokenID)
	if err != nil || tk == nil {
		return ErrTokenNotFound
	}
	tk.IsActive = false
	return g.tokens.Set(g.api.GetTxn(), tokenID, *tk)
}

// ── Query ────────────────────────────────────────────────

func (g GatewayQuery) GetBalance(user model.UniAddr) (uint64, error) {
	bal, err := g.balances.Get(g.api.GetTxn(), string(user.V))
	if err != nil || bal == nil {
		return 0, nil
	}
	return *bal, nil
}

func (g GatewayQuery) GetModel(modelID uint64) (*ModelInfo, error) {
	return g.models.Get(g.api.GetTxn(), modelID)
}

func (g GatewayQuery) GetChannel(providerID uint64) (*ChannelInfo, error) {
	return g.channels.Get(g.api.GetTxn(), providerID)
}

func (g GatewayQuery) GetToken(tokenID uint64) (*TokenInfo, error) {
	return g.tokens.Get(g.api.GetTxn(), tokenID)
}

func (g GatewayQuery) GetProviderBalance(modelID uint64) (uint64, error) {
	bal, err := g.providerBalances.Get(g.api.GetTxn(), modelID)
	if err != nil || bal == nil {
		return 0, nil
	}
	return *bal, nil
}
