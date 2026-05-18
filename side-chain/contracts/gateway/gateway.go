package gateway

import (
	"errors"
	"fmt"

	"github.com/computeflux/tee-provider/pkg/model"
)

var (
	ErrLowBalance       = errors.New("low balance")
	ErrModelNotFound    = errors.New("model not found")
	ErrProviderNotFound = errors.New("provider not found")
	ErrTokenNotFound    = errors.New("token not found")
	ErrNotOwner         = errors.New("not owner")
)

// Gateway API 网关合约
type Gateway struct {
	api model.ContractApi

	balances         *model.StoreMapping[string, uint64]
	models           *model.IndexedList[uint64, ModelInfo]    // tag 索引 + 自增 id + 分页
	providers        *model.IndexedList[uint64, ProviderInfo] `store:"keyPfx:providers_v2"`
	providerBalances *model.StoreMapping[uint64, uint64]
	tokens           *model.StoreMapping[uint64, TokenInfo]
	usages           *model.StoreMapping[string, DailyUsage]

	nextTokenID *model.StoreValue[uint64]
}

type GatewayQuery struct{ Gateway }
type GatewayMutation struct{ Gateway }

// ── Init ─────────────────────────────────────────────────

func (g GatewayMutation) Init() error {
	return g.nextTokenID.Set(g.api.GetTxn(), 0)
}

// ── Model ────────────────────────────────────────────────

func (g GatewayMutation) RegisterModel(
	name string, pointsPer1K uint64,
	inputModalities uint8, contextLength uint64, promptPricing uint64,
	providerType ProviderType, uri string, apiKeys []string,
) error {
	owner := g.api.GetCaller()
	if _, err := g.models.Insert(g.api.GetTxn(), ModelInfo{
		Name:            name,
		Owner:           owner,
		PointsPer1K:     pointsPer1K,
		IsActive:        true,
		CreatedAt:       uint64(g.api.GetHeight()),
		InputModalities: inputModalities,
		ContextLength:   contextLength,
		PromptPricing:   promptPricing,
		ProviderType:    providerType,
		URI:             uri,
		APIKeys:         apiKeys,
	}); err != nil {
		return err
	}
	return nil
}

func (g GatewayMutation) UpdateModel(
	modelID uint64, pointsPer1K uint64,
	inputModalities uint8, contextLength uint64, promptPricing uint64,
	providerType ProviderType, uri string, apiKeys []string,
) error {
	return g.models.UpdateFn(g.api.GetTxn(), modelID, func(m *ModelInfo) {
		m.PointsPer1K = pointsPer1K
		m.InputModalities = inputModalities
		m.ContextLength = contextLength
		m.PromptPricing = promptPricing
		m.ProviderType = providerType
		m.URI = uri
		m.APIKeys = apiKeys
	})
}

func (g GatewayMutation) DeactivateModel(modelID uint64) error {
	return g.models.UpdateFn(g.api.GetTxn(), modelID, func(m *ModelInfo) {
		m.IsActive = false
	})
}

// ── Provider / Channel ───────────────────────────────────

func (g GatewayMutation) CreateProvider(
	name, logoURL, description string,
	region uint8, deposit uint64,
	providerType ProviderType, uri string, apiKeys []string,
) error {
	owner := g.api.GetCaller()
	// priority 由 deposit 自动计算
	var priority uint32
	switch {
	case deposit >= 10000:
		priority = 90
	case deposit >= 1000:
		priority = 60
	default:
		priority = 30
	}
	if _, err := g.providers.Insert(g.api.GetTxn(), ProviderInfo{
		Owner:        owner,
		Name:         name,
		LogoURL:      logoURL,
		Description:  description,
		Region:       region,
		Deposit:      deposit,
		Priority:     priority,
		ProviderType: providerType,
		URI:          uri,
		APIKeys:      apiKeys,
		IsActive:     true,
		CreatedAt:    uint64(g.api.GetHeight()),
	}); err != nil {
		return err
	}
	return nil
}

func (g GatewayMutation) UpdateProvider(
	providerID uint64,
	name, logoURL, description string,
	modelIds []uint64, region uint8, feeRate uint32,
	priority uint32, uptimeSLA uint32,
	providerType ProviderType, uri string, apiKeys []string,
) error {
	return g.providers.UpdateFn(g.api.GetTxn(), providerID, func(ch *ProviderInfo) {
		if name != "" {
			ch.Name = name
		}
		if logoURL != "" {
			ch.LogoURL = logoURL
		}
		if description != "" {
			ch.Description = description
		}
		if modelIds != nil {
			ch.ModelIds = modelIds
		}
		if region != 0 {
			ch.Region = region
		}
		if feeRate != 0 {
			ch.FeeRate = feeRate
		}
		if priority != 0 {
			ch.Priority = priority
		}
		if uptimeSLA != 0 {
			ch.UptimeSLA = uptimeSLA
		}
		if providerType != 0 {
			ch.ProviderType = providerType
		}
		if uri != "" {
			ch.URI = uri
		}
		if apiKeys != nil {
			ch.APIKeys = apiKeys
		}
	})
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
	tk, err := g.tokens.Get(g.api.GetTxn(), tokenID)
	if err != nil || tk == nil {
		return ErrTokenNotFound
	}

	m, err := g.models.Get(g.api.GetTxn(), modelID)
	if err != nil || m == nil {
		return ErrModelNotFound
	}

	cost := (tokensUsed+999)/1000*m.PointsPer1K + m.PromptPricing
	if cost < 1 {
		cost = 1
	}

	key := string(tk.Owner.V)
	bal, _ := g.balances.Get(g.api.GetTxn(), key)
	if bal == nil || *bal < cost {
		return ErrLowBalance
	}
	*bal -= cost
	g.balances.Set(g.api.GetTxn(), key, *bal)

	pb, _ := g.providerBalances.Get(g.api.GetTxn(), modelID)
	if pb == nil {
		pb = new(uint64)
	}
	*pb += cost
	g.providerBalances.Set(g.api.GetTxn(), modelID, *pb)

	dayKey := fmt.Sprintf("%d_%d_%s", tokenID, modelID, "today")
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

func (g GatewayMutation) CreateToken(owner model.UniAddr, rateLimit uint32) error {
	id, _ := g.nextTokenID.Get(g.api.GetTxn())
	if id == nil {
		id = new(uint64)
	}

	if err := g.tokens.Set(g.api.GetTxn(), *id, TokenInfo{
		Owner: owner, RateLimit: rateLimit, IsActive: true,
		CreatedAt: uint64(g.api.GetHeight()),
	}); err != nil {
		return err
	}

	g.nextTokenID.Set(g.api.GetTxn(), *id+1)
	return nil
}

func (g GatewayMutation) RevokeToken(tokenID uint64) error {
	tk, err := g.tokens.Get(g.api.GetTxn(), tokenID)
	if err != nil || tk == nil {
		return ErrTokenNotFound
	}
	tk.IsActive = false
	return g.tokens.Set(g.api.GetTxn(), tokenID, *tk)
}

// ── Query: Single ────────────────────────────────────────

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

func (g GatewayQuery) GetProvider(providerID uint64) (*ProviderInfo, error) {
	return g.providers.Get(g.api.GetTxn(), providerID)
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

// ── Query: Count ─────────────────────────────────────────

func (g GatewayQuery) GetModelCount() (uint64, error) {
	return g.models.Len(g.api.GetTxn())
}

func (g GatewayQuery) GetProviderCount() (uint64, error) {
	return g.providers.Len(g.api.GetTxn())
}

func (g GatewayQuery) GetTokenCount() (uint64, error) {
	id, err := g.nextTokenID.Get(g.api.GetTxn())
	if err != nil || id == nil {
		return 0, nil
	}
	return *id, nil
}

// ── Query: Paginated list (StoreList 原生分页) ───────────

func (g GatewayQuery) ListModels(offset uint64, limit uint32) ([]ModelInfo, error) {
	_, vals, err := g.models.List(g.api.GetTxn(), offset, limit)
	return vals, err
}

func (g GatewayQuery) ListProviders(offset uint64, limit uint32) ([]ProviderInfo, error) {
	_, vals, err := g.providers.List(g.api.GetTxn(), offset, limit)
	return vals, err
}

func (g GatewayQuery) ListTokens(offset uint64, limit uint32) ([]TokenInfo, error) {
	return g.paginateTokens(offset, limit, nil)
}

// ── Query: Active-only list ──────────────────────────────

func (g GatewayQuery) ListActiveModels(offset uint64, limit uint32) ([]ModelInfo, error) {
	return g.listModelsFiltered(offset, limit, func(m *ModelInfo) bool { return m.IsActive })
}

func (g GatewayQuery) ListActiveProviders(offset uint64, limit uint32) ([]ProviderInfo, error) {
	return g.listProvidersFiltered(offset, limit, func(ch *ProviderInfo) bool { return ch.IsActive })
}

func (g GatewayQuery) ListActiveTokens(offset uint64, limit uint32) ([]TokenInfo, error) {
	return g.paginateTokens(offset, limit, func(tk *TokenInfo) bool { return tk.IsActive })
}

// ── Query: Combined filter ───────────────────────────────

func (g GatewayQuery) QueryModels(
	modalities uint8, minContext uint64, maxContext uint64,
	maxPrice uint64, offset uint64, limit uint32,
) ([]ModelInfo, error) {
	txn := g.api.GetTxn()

	filter := map[string]any{}
	if modalities != 0 {
		filter["InputModalities"] = uint64(modalities)
	}
	if minContext > 0 || maxContext > 0 {
		filter["ContextLength"] = model.Range{Min: minContext, Max: maxContext}
	}
	if maxPrice > 0 {
		filter["PromptPricing"] = model.Range{Min: 0, Max: maxPrice}
	}

	if len(filter) == 0 {
		return g.listModelsFiltered(offset, limit, func(m *ModelInfo) bool {
			return m.IsActive
		})
	}

	ids, err := g.models.Query(txn, filter)
	if err != nil {
		return nil, err
	}

	out := make([]ModelInfo, 0, limit)
	var skipped, collected uint64
	for _, id := range ids {
		m, err := g.models.Get(txn, id)
		if err != nil || m == nil || !m.IsActive {
			continue
		}
		if minContext > 0 && m.ContextLength < minContext {
			continue
		}
		if maxContext > 0 && m.ContextLength > maxContext {
			continue
		}
		if maxPrice > 0 && m.PromptPricing > maxPrice {
			continue
		}
		if skipped < offset {
			skipped++
			continue
		}
		out = append(out, *m)
		collected++
		if collected >= uint64(limit) {
			break
		}
	}
	return out, nil
}

// ── Internal pagination helpers ──────────────────────────

func (g GatewayQuery) listModelsFiltered(
	offset uint64, limit uint32, fn func(*ModelInfo) bool,
) ([]ModelInfo, error) {
	txn := g.api.GetTxn()
	total, _ := g.models.Len(txn)
	out := make([]ModelInfo, 0, limit)
	var skipped, collected uint64
	for i := uint64(0); i < total && collected < uint64(limit); i++ {
		m, err := g.models.Get(txn, i)
		if err != nil || m == nil {
			continue
		}
		if fn != nil && !fn(m) {
			continue
		}
		if skipped < offset {
			skipped++
			continue
		}
		out = append(out, *m)
		collected++
	}
	return out, nil
}

func (g GatewayQuery) listProvidersFiltered(
	offset uint64, limit uint32, fn func(*ProviderInfo) bool,
) ([]ProviderInfo, error) {
	txn := g.api.GetTxn()
	total, _ := g.providers.Len(txn)
	out := make([]ProviderInfo, 0, limit)
	var skipped, collected uint64
	for i := uint64(0); i < total && collected < uint64(limit); i++ {
		ch, err := g.providers.Get(txn, i)
		if err != nil || ch == nil {
			continue
		}
		if fn != nil && !fn(ch) {
			continue
		}
		if skipped < offset {
			skipped++
			continue
		}
		out = append(out, *ch)
		collected++
	}
	return out, nil
}

func (g GatewayQuery) paginateTokens(
	offset uint64, limit uint32, fn func(*TokenInfo) bool,
) ([]TokenInfo, error) {
	txn := g.api.GetTxn()
	total, _ := g.GetTokenCount()
	out := make([]TokenInfo, 0, limit)
	var skipped, collected uint64
	for i := uint64(0); i < total && collected < uint64(limit); i++ {
		tk, err := g.tokens.Get(txn, i)
		if err != nil || tk == nil {
			continue
		}
		if fn != nil && !fn(tk) {
			continue
		}
		if skipped < offset {
			skipped++
			continue
		}
		out = append(out, *tk)
		collected++
	}
	return out, nil
}
