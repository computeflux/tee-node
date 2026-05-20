package gateway

import (
	"bytes"
	"encoding/hex"
	"errors"
	"fmt"

	"github.com/computeflux/tee-provider/pkg/model"
)

var (
	ErrLowBalance       = errors.New("low balance")
	ErrModelNotFound    = errors.New("model not found")
	ErrProviderNotFound = errors.New("provider not found")
	ErrAPIKeyNotFound   = errors.New("api key not found")
	ErrNotOwner         = errors.New("not owner")
)

// Gateway API 网关合约
type Gateway struct {
	api model.ContractApi

	balances         *model.StoreMapping[string, uint64]
	models           *model.IndexedList[uint64, ModelInfo]    // tag 索引 + 自增 id + 分页
	providers        *model.IndexedList[uint64, ProviderInfo] `store:"keyPfx:providers_v3"`
	providerByOwner  *model.StoreMapping[string, uint64]      // owner → providerID（一人一 provider）
	providerBalances *model.StoreMapping[uint64, uint64]
	apiKeys          *model.StoreMapping[string, APIKeyInfo] // hex(keyHash) → APIKeyInfo
	usages           *model.StoreMapping[string, DailyUsage]
}

type GatewayQuery struct{ Gateway }
type GatewayMutation struct{ Gateway }

// ── Init ─────────────────────────────────────────────────

func (g GatewayMutation) Init() error {
	return nil
}

// ── Model ────────────────────────────────────────────────

func (g GatewayMutation) RegisterModel(
	name string, pointsPer1K uint64,
	inputModalities uint8, contextLength uint64, promptPricing uint64,
	modelID string, providerType uint16, uri string, apiKeys []string,
) error {
	owner := g.api.GetCaller()

	// 必须已注册 Provider
	ownerKey := string(owner.V)
	provID, err := g.providerByOwner.Get(g.api.GetTxn(), ownerKey)
	if err != nil || provID == nil {
		return fmt.Errorf("must create a provider before registering models")
	}

	if _, err := g.models.Insert(g.api.GetTxn(), ModelInfo{
		Name:            name,
		Owner:           owner,
		ProviderID:      *provID,
		PointsPer1K:     pointsPer1K,
		IsActive:        true,
		CreatedAt:       uint64(g.api.GetHeight()),
		InputModalities: inputModalities,
		ContextLength:   contextLength,
		PromptPricing:   promptPricing,
		ModelID:         modelID,
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
	modelIDStr string, providerType uint16, uri string, apiKeys []string,
) error {
	return g.models.UpdateFn(g.api.GetTxn(), modelID, func(m *ModelInfo) {
		m.PointsPer1K = pointsPer1K
		m.InputModalities = inputModalities
		m.ContextLength = contextLength
		m.PromptPricing = promptPricing
		m.ModelID = modelIDStr
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
) error {
	owner := g.api.GetCaller()

	// 一人只能注册一个 provider
	ownerKey := string(owner.V)
	if existing, _ := g.providerByOwner.Get(g.api.GetTxn(), ownerKey); existing != nil {
		return fmt.Errorf("provider already exists for this account (id=%d)", *existing)
	}

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
	id, err := g.providers.Insert(g.api.GetTxn(), ProviderInfo{
		Owner:       owner,
		Name:        name,
		LogoURL:     logoURL,
		Description: description,
		Region:      region,
		Deposit:     deposit,
		Priority:    priority,
		IsActive:    true,
		CreatedAt:   uint64(g.api.GetHeight()),
	})
	if err != nil {
		return err
	}

	// 记录 owner → providerID 映射
	if err := g.providerByOwner.Set(g.api.GetTxn(), ownerKey, id); err != nil {
		return err
	}
	return nil
}

func (g GatewayMutation) UpdateProvider(
	providerID uint64,
	name, logoURL, description string,
	modelIds []uint64, region uint8, feeRate uint32,
	priority uint32, uptimeSLA uint32,
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
	})
}

// ── Points ───────────────────────────────────────────────

func (g GatewayMutation) Recharge(user model.UniAddr, amount uint64) error {
	if !g.api.CallbySystem() {
		return fmt.Errorf("Recharge: caller is not sudo account")
	}

	key := string(user.V)
	bal, _ := g.balances.Get(g.api.GetTxn(), key)
	if bal == nil {
		bal = new(uint64)
	}
	*bal += amount
	return g.balances.Set(g.api.GetTxn(), key, *bal)
}

// ── Billing ──────────────────────────────────────────────

// SettleUsage 批量结算（仅模型所属 Provider 可调用，每 5 分钟执行一次）
// TODO: 防重放 — 增加 batchNonce/merkle root + TEE 签名校验，防止 Provider 重复提交同一批结算
func (g GatewayMutation) SettleUsage(records []Settlement) error {
	caller := g.api.GetCaller()

	for _, r := range records {
		// 校验 provider 归属
		prov, err := g.providers.Get(g.api.GetTxn(), r.ProviderID)
		if err != nil || prov == nil {
			return fmt.Errorf("settle: %w", ErrProviderNotFound)
		}
		if !bytes.Equal(prov.Owner.V, caller.V) || prov.Owner.T != caller.T {
			return fmt.Errorf("settle: provider %d not owned by caller", r.ProviderID)
		}

		// 校验模型存在
		m, err := g.models.Get(g.api.GetTxn(), r.ModelID)
		if err != nil || m == nil {
			return fmt.Errorf("settle: %w", ErrModelNotFound)
		}

		cost := r.Cost
		if cost == 0 {
			cost = (r.TokensUsed+999)/1000*m.PointsPer1K + m.PromptPricing
		}
		if cost < 1 {
			cost = 1
		}

		// 扣用户余额
		userKey := string(r.User.V)
		bal, _ := g.balances.Get(g.api.GetTxn(), userKey)
		if bal == nil || *bal < cost {
			return fmt.Errorf("settle user %x: %w", r.User.V, ErrLowBalance)
		}
		*bal -= cost
		g.balances.Set(g.api.GetTxn(), userKey, *bal)

		// 加 provider 余额
		pb, _ := g.providerBalances.Get(g.api.GetTxn(), r.ProviderID)
		if pb == nil {
			pb = new(uint64)
		}
		*pb += cost
		g.providerBalances.Set(g.api.GetTxn(), r.ProviderID, *pb)

		// 记录每日用量
		dayKey := fmt.Sprintf("%x_%d_%d_%s", r.User.V, r.ProviderID, r.ModelID, "today")
		usage, _ := g.usages.Get(g.api.GetTxn(), dayKey)
		if usage == nil {
			usage = &DailyUsage{User: r.User, ProviderID: r.ProviderID, ModelID: r.ModelID}
		}
		usage.Count++
		usage.TokensUsed += r.TokensUsed
		usage.PointsUsed += cost
		g.usages.Set(g.api.GetTxn(), dayKey, *usage)
	}
	return nil
}

func (g GatewayMutation) Withdraw(providerID uint64, amount uint64) error {
	if !g.api.CallbySystem() {
		return fmt.Errorf("Withdraw: caller is not sudo account")
	}

	pb, err := g.providerBalances.Get(g.api.GetTxn(), providerID)
	if err != nil || pb == nil {
		return ErrProviderNotFound
	}

	if *pb < amount {
		return ErrLowBalance
	}
	*pb -= amount
	return g.providerBalances.Set(g.api.GetTxn(), providerID, *pb)
}

// ── API Key ──────────────────────────────────────────────

// RegisterAPIKey 注册 API Key 哈希（原始 key 绝不上链）
func (g GatewayMutation) RegisterAPIKey(keyHash []byte, rateLimit uint32) error {
	if len(keyHash) != 32 {
		return fmt.Errorf("keyHash must be 32 bytes")
	}
	owner := g.api.GetCaller()
	hexKey := hex.EncodeToString(keyHash)

	if existing, _ := g.apiKeys.Get(g.api.GetTxn(), hexKey); existing != nil {
		return fmt.Errorf("api key already registered")
	}

	return g.apiKeys.Set(g.api.GetTxn(), hexKey, APIKeyInfo{
		Owner:     owner,
		KeyHash:   keyHash,
		RateLimit: rateLimit,
		IsActive:  true,
		CreatedAt: uint64(g.api.GetHeight()),
	})
}

// RevokeAPIKey 吊销 API Key
func (g GatewayMutation) RevokeAPIKey(keyHash []byte) error {
	if len(keyHash) != 32 {
		return fmt.Errorf("keyHash must be 32 bytes")
	}
	owner := g.api.GetCaller()
	hexKey := hex.EncodeToString(keyHash)

	info, err := g.apiKeys.Get(g.api.GetTxn(), hexKey)
	if err != nil || info == nil {
		return ErrAPIKeyNotFound
	}
	if !bytes.Equal(info.Owner.V, owner.V) || info.Owner.T != owner.T {
		return ErrNotOwner
	}
	info.IsActive = false
	return g.apiKeys.Set(g.api.GetTxn(), hexKey, *info)
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

// GetProviderByOwner 查询当前用户的 provider
func (g GatewayQuery) GetProviderByOwner() (uint64, error) {
	owner := g.api.GetCaller()
	id, err := g.providerByOwner.Get(g.api.GetTxn(), string(owner.V))
	if err != nil || id == nil {
		return 0, nil
	}
	return *id, nil
}

// LookupAPIKey TEE 提供方根据 hash 查找 API Key 对应的用户
func (g GatewayQuery) LookupAPIKey(keyHash []byte) (*APIKeyInfo, error) {
	if len(keyHash) != 32 {
		return nil, ErrAPIKeyNotFound
	}
	hexKey := hex.EncodeToString(keyHash)
	return g.apiKeys.Get(g.api.GetTxn(), hexKey)
}

func (g GatewayQuery) GetProviderBalance(providerID uint64) (uint64, error) {
	bal, err := g.providerBalances.Get(g.api.GetTxn(), providerID)
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

// ── Query: Paginated list (StoreList 原生分页) ───────────

func (g GatewayQuery) ListModels(offset uint64, limit uint32) ([]ModelInfo, error) {
	_, vals, err := g.models.List(g.api.GetTxn(), offset, limit)
	return vals, err
}

func (g GatewayQuery) ListProviders(offset uint64, limit uint32) ([]ProviderInfo, error) {
	_, vals, err := g.providers.List(g.api.GetTxn(), offset, limit)
	return vals, err
}

// ── Query: Active-only list ──────────────────────────────

func (g GatewayQuery) ListActiveModels(offset uint64, limit uint32) ([]ModelInfo, error) {
	return g.listModelsFiltered(offset, limit, func(m *ModelInfo) bool { return m.IsActive })
}

func (g GatewayQuery) ListActiveProviders(offset uint64, limit uint32) ([]ProviderInfo, error) {
	return g.listProvidersFiltered(offset, limit, func(ch *ProviderInfo) bool { return ch.IsActive })
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
