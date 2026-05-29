package gateway

import (
	"errors"
	"fmt"

	"github.com/computeflux/tee-provider/pkg/model"
)

// ── Model ────────────────────────────────────────────────

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
		return fmt.Errorf("must create a provider before registering models")
	}

	// ProviderID + Name 唯一性检查
	txn := g.api.GetTxn()
	existing, err := g.models.Query(txn, model.QueryAND{
		model.QueryCond{Field: "ProviderID", Val: *provID},
		model.QueryCond{Field: "Name", Val: name},
	})
	if err != nil {
		return err
	}

	if len(existing) > 0 {
		return errors.New("model name already exists for this provider")
	}

	// 1. 先注册模型（不存 ref），获取 modelID 用于生成唯一 ref
	modelID, err := g.models.Insert(txn, ModelInfo{
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
		APIKeyRefs:      nil,
	})
	if err != nil {
		return err
	}

	// 2. 注册每个 apiKey（明文 → TEE KMS 加密 → 链上存储 ref）
	refs := make([]string, 0, len(apiKeys))
	for i, key := range apiKeys {
		if key == "" {
			continue
		}

		prefix := key
		if len(prefix) > 0 {
			prefix = prefix[:(len(prefix)+1)/2]
		}
		ref := fmt.Sprintf("%s_%d_%d_%d", prefix, modelID, g.api.GetHeight(), i)
		gk, err := model.GetGlobalKey()
		if err != nil {
			return fmt.Errorf("kms not ready: %w", err)
		}

		ek, err := model.EncryptWithGK(gk, ref, key)
		if err != nil {
			return fmt.Errorf("encrypt api key failed: %w", err)
		}

		if err := g.encryptedKeys.Set(txn, ref, EncryptedAPIKey{
			Ref:        ref,
			Ciphertext: ek.Ciphertext,
			Nonce:      ek.Nonce,
		}); err != nil {
			return fmt.Errorf("store encrypted key failed: %w", err)
		}
		refs = append(refs, ref)
	}

	// 3. 更新模型写入 APIKeyRefs
	if err := g.models.UpdateFn(txn, modelID, func(m *ModelInfo) {
		m.APIKeyRefs = refs
	}); err != nil {
		return err
	}
	return nil
}

func (g GatewayMutation) UpdateModel(
	ID uint64,
	inputModalities uint8,
	contextLength uint64,
	inPrice, cachedPrice, outPrice uint64,
	providerType uint16,
	uri string,
) error {
	// 获取当前模型
	cur, err := g.models.Get(g.api.GetTxn(), ID)
	if err != nil || cur == nil {
		return fmt.Errorf("model not found")
	}

	// ProviderID + Name 唯一性检查（name 不允许修改，但防御性检查）
	// 如果未来允许改 name，需要在此排除自身 ID 后做冲突检查

	// 更新模型字段（CreatedAt 保持不变，不覆盖首次注册时间；APIKeyRefs 保持不变）
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
		return fmt.Errorf("model not found")
	}
	if apiKey == "" {
		return fmt.Errorf("api key cannot be empty")
	}

	prefix := apiKey
	if len(prefix) > 0 {
		prefix = prefix[:(len(prefix)+1)/2]
	}
	ref := fmt.Sprintf("%s_%d_%d_%d", prefix, modelID, g.api.GetHeight(), len(cur.APIKeyRefs))
	gk, err := model.GetGlobalKey()
	if err != nil {
		return fmt.Errorf("kms not ready: %w", err)
	}

	ek, err := model.EncryptWithGK(gk, ref, apiKey)
	if err != nil {
		return fmt.Errorf("encrypt api key failed: %w", err)
	}

	if err := g.encryptedKeys.Set(g.api.GetTxn(), ref, EncryptedAPIKey{
		Ref:        ref,
		Ciphertext: ek.Ciphertext,
		Nonce:      ek.Nonce,
	}); err != nil {
		return fmt.Errorf("store encrypted key failed: %w", err)
	}

	return g.models.UpdateFn(g.api.GetTxn(), modelID, func(m *ModelInfo) {
		m.APIKeyRefs = append(m.APIKeyRefs, ref)
	})
}

func (g GatewayMutation) DeleteModelAPIKeyRef(modelID uint64, ref string) error {
	cur, err := g.models.Get(g.api.GetTxn(), modelID)
	if err != nil || cur == nil {
		return fmt.Errorf("model not found")
	}

	newRefs := make([]string, 0, len(cur.APIKeyRefs))
	found := false
	for _, r := range cur.APIKeyRefs {
		if r == ref {
			found = true
			continue
		}
		newRefs = append(newRefs, r)
	}
	if !found {
		return fmt.Errorf("ref not found in model")
	}

	if err := g.models.UpdateFn(g.api.GetTxn(), modelID, func(m *ModelInfo) {
		m.APIKeyRefs = newRefs
	}); err != nil {
		return err
	}

	_ = g.encryptedKeys.Delete(g.api.GetTxn(), ref)
	return nil
}

func (g GatewayMutation) UpdateModelAPIKeyRef(modelID uint64, ref, newApiKey string) error {
	cur, err := g.models.Get(g.api.GetTxn(), modelID)
	if err != nil || cur == nil {
		return fmt.Errorf("model not found")
	}

	found := false
	for _, r := range cur.APIKeyRefs {
		if r == ref {
			found = true
			break
		}
	}
	if !found {
		return fmt.Errorf("ref not found in model")
	}
	if newApiKey == "" {
		return fmt.Errorf("api key cannot be empty")
	}

	gk, err := model.GetGlobalKey()
	if err != nil {
		return fmt.Errorf("kms not ready: %w", err)
	}

	ek, err := model.EncryptWithGK(gk, ref, newApiKey)
	if err != nil {
		return fmt.Errorf("encrypt api key failed: %w", err)
	}

	return g.encryptedKeys.Set(g.api.GetTxn(), ref, EncryptedAPIKey{
		Ref:        ref,
		Ciphertext: ek.Ciphertext,
		Nonce:      ek.Nonce,
	})
}

func (g GatewayMutation) DeactivateModel(modelID uint64) error {
	return g.models.UpdateFn(g.api.GetTxn(), modelID, func(m *ModelInfo) {
		m.IsActive = false
	})
}

func (g GatewayQuery) GetModel(modelID uint64) (*ModelInfo, error) {
	return g.models.Get(g.api.GetTxn(), modelID)
}

func (g GatewayQuery) GetModels(IDs []uint64) ([]ModelInfoWithProvider, error) {
	models := make([]ModelInfo, 0, len(IDs))
	for _, id := range IDs {
		m, err := g.models.Get(g.api.GetTxn(), id)
		if err != nil || m == nil {
			continue
		}
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
	return vals, err
}

func (g GatewayQuery) listModelsByIDs(ids []uint64, offset uint64, limit uint32) ([]ModelInfo, error) {
	if uint64(len(ids)) <= offset {
		return nil, nil
	}
	end := offset + uint64(limit)
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

func (g GatewayQuery) ListActiveModels(offset uint64, limit uint32) ([]ModelInfo, error) {
	ids, err := g.models.Query(g.api.GetTxn(), model.QueryCond{Field: "IsActive", Val: uint64(1)})
	if err != nil {
		return nil, err
	}
	return g.listModelsByIDs(ids, offset, limit)
}

func (g GatewayQuery) ListProviderModels(providerID uint64) ([]ModelInfo, error) {
	ids, err := g.models.Query(g.api.GetTxn(), model.QueryCond{Field: "ProviderID", Val: providerID})
	if err != nil {
		return nil, err
	}
	return g.listModelsByIDs(ids, 0, 10000)
}

func (g GatewayQuery) ListActiveModelsWithinProvider(
	minContext, maxContext, minPrice, maxPrice uint64,
	offset uint64, limit uint32,
) ([]ModelInfoWithProvider, error) {
	exprs := model.QueryAND{model.QueryCond{Field: "IsActive", Val: uint64(1)}}
	if minContext != 0 || maxContext != 0 {
		exprs = append(exprs, model.QueryCond{Field: "ContextLength", Val: model.Range{Min: minContext, Max: maxContext}})
	}
	if minPrice != 0 || maxPrice != 0 {
		exprs = append(exprs, model.QueryCond{Field: "InPrice", Val: model.Range{Min: minPrice, Max: maxPrice}})
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
		prov, _ := g.providers.Get(g.api.GetTxn(), models[i].ProviderID)
		if prov == nil {
			continue
		}
		out = append(out, ModelInfoWithProvider{
			Model:    models[i],
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
) ([]ModelInfo, error) {
	txn := g.api.GetTxn()

	exprs := model.QueryAND{model.QueryCond{Field: "IsActive", Val: uint64(1)}}
	if minContext != 0 || maxContext != 0 {
		exprs = append(exprs, model.QueryCond{Field: "ContextLength", Val: model.Range{Min: minContext, Max: maxContext}})
	}
	if maxPrice != 0 {
		exprs = append(exprs, model.QueryCond{Field: "InPrice", Val: model.Range{Min: 0, Max: maxPrice}})
	}

	ids, err := g.models.Query(txn, exprs)
	if err != nil {
		return nil, err
	}

	models, err := g.listModelsByIDs(ids, 0, uint32(len(ids)))
	if err != nil {
		return nil, err
	}

	// 按位查询无法用 exact 索引，需二次过滤
	if modalities != 0 {
		filtered := make([]ModelInfo, 0, len(models))
		for _, m := range models {
			if m.InputModalities&modalities != 0 {
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
	return models[offset:end], nil
}

// GetModelsByProvider 返回某个 Provider 注册的所有模型（使用 ProviderID exact 索引）
func (g GatewayQuery) GetModelsByProvider(providerID uint64) ([]ModelInfo, error) {
	keys, err := g.models.Query(g.api.GetTxn(), model.QueryCond{Field: "ProviderID", Val: providerID})
	if err != nil {
		return nil, err
	}
	out := make([]ModelInfo, 0, len(keys))
	for _, k := range keys {
		m, err := g.models.Get(g.api.GetTxn(), k)
		if err != nil || m == nil {
			continue
		}
		out = append(out, *m)
	}
	return out, nil
}

// GetModelByName 返回指定名称的所有模型（使用 Name exact 索引）
func (g GatewayQuery) GetModelByName(pid uint64, name string) (*ModelInfoWithID, error) {
	keys, err := g.models.Query(g.api.GetTxn(), model.QueryAND{
		model.QueryCond{Field: "ProviderID", Val: pid},
		model.QueryCond{Field: "Name", Val: name},
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

// ── Encrypted API Key ────────────────────────────────────

func (g GatewayMutation) RegisterEncryptedAPIKey(ref string, ciphertext, nonce []byte) error {
	if len(ciphertext) == 0 || len(nonce) == 0 {
		return fmt.Errorf("ciphertext and nonce required")
	}
	if existing, _ := g.encryptedKeys.Get(g.api.GetTxn(), ref); existing != nil {
		return fmt.Errorf("ref %s already exists", ref)
	}
	return g.encryptedKeys.Set(g.api.GetTxn(), ref, EncryptedAPIKey{
		Ref:        ref,
		Ciphertext: ciphertext,
		Nonce:      nonce,
	})
}

func (g GatewayQuery) GetEncryptedAPIKey(ref string) (*EncryptedAPIKey, error) {
	return g.encryptedKeys.Get(g.api.GetTxn(), ref)
}
