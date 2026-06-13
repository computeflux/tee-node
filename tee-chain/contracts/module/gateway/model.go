package gateway

import (
	"errors"
	"strings"

	"github.com/computeflux/tee-provider/tee-chain/contracts/store"
)

// ── Model ────────────────────────────────────────────────

// RegisterModel 注册模型
func (g GatewayMutation) RegisterModel(
	name string,
	inputModalities uint8,
	contextLength uint64,
	inPrice, cachedPrice, outPrice uint64,
	providerType uint16,
	uri string,
	apiKeys []string,
) error {
	owner := g.api.GetCaller()

	// 必须已注册 Provider
	provID, err := g.providerByOwner.Get(g.api.GetTxn(), owner)
	if err != nil || provID == nil {
		return err
	}

	// ProviderID + Name 唯一性检查
	txn := g.api.GetTxn()
	existing, err := g.models.Query(txn, store.QueryAND{
		store.QueryCond{Field: "ProviderID", Val: *provID},
		store.QueryCond{Field: "Name", Val: name},
	})
	if err != nil {
		return err
	}

	if len(existing) > 0 {
		return errors.New("model name already exists for this provider")
	}

	// 过滤空 key
	keys := make([]string, 0, len(apiKeys))
	for _, k := range apiKeys {
		if k != "" {
			keys = append(keys, k)
		}
	}

	id, err := g.models.Insert(txn, ModelInfo{
		Name:            name,
		Owner:           owner,
		ProviderID:      *provID,
		IsActive:        true,
		CreatedAt:       uint64(g.api.GetBlockTime()),
		InputModalities: inputModalities,
		ContextLength:   contextLength,
		InPrice:         inPrice,
		CachedPrice:     cachedPrice,
		OutPrice:        outPrice,
		ProviderType:    providerType,
		URI:             uri,
		APIKeys:         keys,
	})
	if err != nil {
		return err
	}

	// 将模型 ID 加入 Provider 的 ModelIds
	return g.providers.UpdateFn(txn, *provID, func(p *ProviderInfo) {
		p.ModelIds = append(p.ModelIds, id)
	})
}

func (g GatewayMutation) UpdateModel(
	ID uint64,
	inputModalities uint8,
	contextLength uint64,
	inPrice, cachedPrice, outPrice uint64,
	providerType uint16,
	uri string,
) error {
	cur, err := g.models.Get(g.api.GetTxn(), ID)
	if err != nil || cur == nil {
		return err
	}

	if err := g.models.UpdateFn(g.api.GetTxn(), ID, func(m *ModelInfo) {
		m.InputModalities = inputModalities
		m.ContextLength = contextLength
		m.InPrice = inPrice
		m.CachedPrice = cachedPrice
		m.OutPrice = outPrice
		m.ProviderType = providerType
		m.URI = uri
	}); err != nil {
		return err
	}

	return nil
}

func (g GatewayMutation) AddModelAPIKeyRef(modelID uint64, apiKey string) error {
	cur, err := g.models.Get(g.api.GetTxn(), modelID)
	if err != nil || cur == nil {
		return err
	}
	if apiKey == "" {
		return errors.New("api key cannot be empty")
	}

	return g.models.UpdateFn(g.api.GetTxn(), modelID, func(m *ModelInfo) {
		m.APIKeys = append(m.APIKeys, apiKey)
	})
}

func (g GatewayMutation) DeleteModelAPIKeyRef(modelID uint64, index int) error {
	cur, err := g.models.Get(g.api.GetTxn(), modelID)
	if err != nil || cur == nil {
		return err
	}
	if index < 0 || index >= len(cur.APIKeys) {
		return errors.New("index out of range")
	}

	return g.models.UpdateFn(g.api.GetTxn(), modelID, func(m *ModelInfo) {
		m.APIKeys = append(m.APIKeys[:index], m.APIKeys[index+1:]...)
	})
}

func (g GatewayMutation) UpdateModelAPIKeyRef(modelID uint64, index int, newApiKey string) error {
	cur, err := g.models.Get(g.api.GetTxn(), modelID)
	if err != nil || cur == nil {
		return err
	}
	if index < 0 || index >= len(cur.APIKeys) {
		return errors.New("index out of range")
	}
	if newApiKey == "" {
		return errors.New("api key cannot be empty")
	}

	return g.models.UpdateFn(g.api.GetTxn(), modelID, func(m *ModelInfo) {
		m.APIKeys[index] = newApiKey
	})
}

func (g GatewayMutation) DeactivateModel(modelID uint64) error {
	return g.models.UpdateFn(g.api.GetTxn(), modelID, func(m *ModelInfo) {
		m.IsActive = false
	})
}

func (g GatewayMutation) ActiveModel(modelID uint64) error {
	return g.models.UpdateFn(g.api.GetTxn(), modelID, func(m *ModelInfo) {
		m.IsActive = true
	})
}

func (g GatewayQuery) GetModel(modelID uint64) (*ModelInfo, error) {
	m, err := g.models.Get(g.api.GetTxn(), modelID)
	if err != nil || m == nil {
		return nil, err
	}
	m.APIKeys = maskAPIKeys(m.APIKeys)
	return m, nil
}

func (g GatewayQuery) GetModels(IDs []uint64) ([]ModelInfoWithProvider, error) {
	models := make([]ModelInfo, 0, len(IDs))
	for _, id := range IDs {
		m, err := g.models.Get(g.api.GetTxn(), id)
		if err != nil || m == nil {
			continue
		}
		m.APIKeys = maskAPIKeys(m.APIKeys)
		models = append(models, *m)
	}

	out := make([]ModelInfoWithProvider, 0, len(models))
	for i := range models {
		prov, _ := g.providers.Get(g.api.GetTxn(), models[i].ProviderID)
		if prov == nil {
			continue
		}
		out = append(out, ModelInfoWithProvider{
			ID:       IDs[i],
			Model:    models[i],
			Provider: *prov,
		})
	}
	return out, nil
}

func (g GatewayQuery) ListModels(offset uint64, limit uint32) ([]ModelInfo, error) {
	_, vals, err := g.models.List(g.api.GetTxn(), offset, limit)
	if err != nil {
		return nil, err
	}
	for i := range vals {
		vals[i].APIKeys = maskAPIKeys(vals[i].APIKeys)
	}
	return vals, nil
}

func (g GatewayQuery) listModelsByIDs(ids []uint64, offset uint64, limit uint32) ([]ModelInfoWithID, error) {
	if uint64(len(ids)) <= offset {
		return nil, nil
	}
	end := offset + uint64(limit)
	if end > uint64(len(ids)) {
		end = uint64(len(ids))
	}
	txn := g.api.GetTxn()
	out := make([]ModelInfoWithID, 0, end-offset)
	for _, id := range ids[offset:end] {
		m, err := g.models.Get(txn, id)
		if err != nil || m == nil {
			continue
		}
		out = append(out, ModelInfoWithID{
			ID:   id,
			Data: *m,
		})
	}
	return out, nil
}

func (g GatewayQuery) ListActiveModels(offset uint64, limit uint32) ([]ModelInfoWithID, error) {
	ids, err := g.models.Query(g.api.GetTxn(), store.QueryCond{Field: "IsActive", Val: uint64(1)})
	if err != nil {
		return nil, err
	}
	models, err := g.listModelsByIDs(ids, offset, limit)
	if err != nil {
		return nil, err
	}
	for i := range models {
		models[i].Data.APIKeys = maskAPIKeys(models[i].Data.APIKeys)
	}
	return models, nil
}

func (g GatewayQuery) ListProviderModels(providerID uint64) ([]ModelInfoWithID, error) {
	ids, err := g.models.Query(g.api.GetTxn(), store.QueryCond{Field: "ProviderID", Val: providerID})
	if err != nil {
		return nil, err
	}
	models, err := g.listModelsByIDs(ids, 0, 10000)
	if err != nil {
		return nil, err
	}
	for i := range models {
		models[i].Data.APIKeys = maskAPIKeys(models[i].Data.APIKeys)
	}
	return models, nil
}

func (g GatewayQuery) ListActiveModelsWithinProvider(
	minContext, maxContext, minPrice, maxPrice uint64,
	offset uint64, limit uint32,
) ([]ModelInfoWithProvider, error) {
	exprs := store.QueryAND{store.QueryCond{Field: "IsActive", Val: uint64(1)}}
	if minContext != 0 || maxContext != 0 {
		exprs = append(exprs, store.QueryCond{Field: "ContextLength", Val: store.Range{Min: minContext, Max: maxContext}})
	}
	if minPrice != 0 || maxPrice != 0 {
		exprs = append(exprs, store.QueryCond{Field: "InPrice", Val: store.Range{Min: minPrice, Max: maxPrice}})
	}

	ids, err := g.models.Query(g.api.GetTxn(), exprs)
	if err != nil {
		return nil, err
	}

	models, err := g.listModelsByIDs(ids, offset, limit)
	if err != nil {
		return nil, err
	}

	out := make([]ModelInfoWithProvider, 0, len(models))
	for i := range models {
		prov, _ := g.providers.Get(g.api.GetTxn(), models[i].Data.ProviderID)
		if prov == nil {
			continue
		}
		out = append(out, ModelInfoWithProvider{
			ID:       ids[i],
			Model:    models[i].Data,
			Provider: *prov,
		})
	}
	return out, nil
}

func (g GatewayQuery) GetModelCount() (uint64, error) {
	return g.models.Len(g.api.GetTxn())
}

func (g GatewayQuery) QueryModels(
	modalities uint8,
	minContext, maxContext,
	maxPrice uint64,
	offset uint64,
	limit uint32,
) ([]ModelInfoWithID, error) {
	txn := g.api.GetTxn()

	exprs := store.QueryAND{store.QueryCond{Field: "IsActive", Val: uint64(1)}}
	if minContext != 0 || maxContext != 0 {
		exprs = append(exprs, store.QueryCond{Field: "ContextLength", Val: store.Range{Min: minContext, Max: maxContext}})
	}
	if maxPrice != 0 {
		exprs = append(exprs, store.QueryCond{Field: "InPrice", Val: store.Range{Min: 0, Max: maxPrice}})
	}

	ids, err := g.models.Query(txn, exprs)
	if err != nil {
		return nil, err
	}

	models, err := g.listModelsByIDs(ids, 0, uint32(len(ids)))
	if err != nil {
		return nil, err
	}

	if modalities != 0 {
		filtered := make([]ModelInfoWithID, 0, len(models))
		for _, m := range models {
			if m.Data.InputModalities&modalities != 0 {
				filtered = append(filtered, m)
			}
		}
		models = filtered
	}

	if uint64(len(models)) <= offset {
		return nil, nil
	}
	end := offset + uint64(limit)
	if end > uint64(len(models)) {
		end = uint64(len(models))
	}
	models = models[offset:end]
	for i := range models {
		models[i].Data.APIKeys = maskAPIKeys(models[i].Data.APIKeys)
	}
	return models, nil
}

func (g GatewayQuery) GetModelsByProvider(providerID uint64) ([]ModelInfo, error) {
	keys, err := g.models.Query(g.api.GetTxn(), store.QueryCond{Field: "ProviderID", Val: providerID})
	if err != nil {
		return nil, err
	}
	out := make([]ModelInfo, 0, len(keys))
	for _, k := range keys {
		m, err := g.models.Get(g.api.GetTxn(), k)
		if err != nil || m == nil {
			continue
		}
		m.APIKeys = maskAPIKeys(m.APIKeys)
		out = append(out, *m)
	}
	return out, nil
}

func (g GatewayQuery) GetModelByName(pid uint64, name string) (*ModelInfoWithID, error) {
	keys, err := g.models.Query(g.api.GetTxn(), store.QueryAND{
		store.QueryCond{Field: "ProviderID", Val: pid},
		store.QueryCond{Field: "Name", Val: name},
	})
	if err != nil {
		return nil, err
	}

	if len(keys) == 0 {
		return nil, errors.New("model not found")
	}

	m, err := g.models.Get(g.api.GetTxn(), keys[0])
	if err != nil || m == nil {
		return nil, errors.New("model not found")
	}

	m.APIKeys = maskAPIKeys(m.APIKeys)
	return &ModelInfoWithID{
		ID:   keys[0],
		Data: *m,
	}, nil
}

// ── GatewaySecret ──────────────────────────────────────────
// 节点内部使用，返回未脱敏的完整数据

func (g GatewaySecret) GetModel(modelID uint64) (*ModelInfo, error) {
	return g.models.Get(g.api.GetTxn(), modelID)
}

func (g GatewaySecret) GetModelByName(pid uint64, name string) (*ModelInfoWithID, error) {
	keys, err := g.models.Query(g.api.GetTxn(), store.QueryAND{
		store.QueryCond{Field: "ProviderID", Val: pid},
		store.QueryCond{Field: "Name", Val: name},
	})
	if err != nil {
		return nil, err
	}

	if len(keys) == 0 {
		return nil, errors.New("model not found")
	}

	m, err := g.models.Get(g.api.GetTxn(), keys[0])
	if err != nil || m == nil {
		return nil, errors.New("model not found")
	}

	return &ModelInfoWithID{
		ID:   keys[0],
		Data: *m,
	}, nil
}

func (g GatewaySecret) GetModels(IDs []uint64) ([]ModelInfo, error) {
	out := make([]ModelInfo, 0, len(IDs))
	for _, id := range IDs {
		m, err := g.models.Get(g.api.GetTxn(), id)
		if err != nil || m == nil {
			continue
		}
		out = append(out, *m)
	}
	return out, nil
}

func (g GatewaySecret) ListActiveModels(offset uint64, limit uint32) ([]ModelInfo, error) {
	ids, err := g.models.Query(g.api.GetTxn(), store.QueryCond{Field: "IsActive", Val: uint64(1)})
	if err != nil {
		return nil, err
	}
	end := offset + uint64(limit)
	if uint64(len(ids)) <= offset {
		return nil, nil
	}
	if end > uint64(len(ids)) {
		end = uint64(len(ids))
	}
	txn := g.api.GetTxn()
	out := make([]ModelInfo, 0, end-offset)
	for _, id := range ids[offset:end] {
		m, err := g.models.Get(txn, id)
		if err != nil || m == nil {
			continue
		}
		out = append(out, *m)
	}
	return out, nil
}

// maskAPIKey 保留最后一个 '-' 及之前前缀，再保留其后 3 个字符明文，其余替换为 *
func maskAPIKey(key string) string {
	if len(key) <= 3 {
		return strings.Repeat("*", len(key))
	}
	idx := strings.LastIndex(key, "-")
	if idx == -1 {
		// 没有 '-' 时保留前 3 个字符
		return key[:3] + strings.Repeat("*", len(key)-3)
	}
	visibleEnd := idx + 1 + 3
	if visibleEnd > len(key) {
		visibleEnd = len(key)
	}
	return key[:visibleEnd] + strings.Repeat("*", len(key)-visibleEnd)
}

// maskAPIKeys 对所有 API Key 做脱敏处理
func maskAPIKeys(keys []string) []string {
	masked := make([]string, len(keys))
	for i, k := range keys {
		masked[i] = maskAPIKey(k)
	}
	return masked
}
