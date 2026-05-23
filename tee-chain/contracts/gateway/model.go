package gateway

import (
	"fmt"

	"github.com/computeflux/tee-provider/pkg/model"
)

// ── Model ────────────────────────────────────────────────

func (g GatewayMutation) RegisterModel(
	name string, pointsPer1K uint64,
	inputModalities uint8, contextLength uint64, promptPricing uint64,
	modelID string, providerType uint16, uri string, apiKeys []string,
) error {
	owner := g.api.GetCaller()

	// 必须已注册 Provider
	provID, err := g.providerByOwner.Get(g.api.GetTxn(), owner)
	if err != nil || provID == nil {
		return fmt.Errorf("must create a provider before registering models")
	}

	// 1. 注册每个 apiKey（明文 → TEE KMS 加密 → 链上存储 ref）
	refs := make([]string, 0, len(apiKeys))
	for i, key := range apiKeys {
		if key == "" {
			continue
		}
		ref := fmt.Sprintf("%s_%d_%d", modelID, g.api.GetHeight(), i)
		gk, err := model.GetGlobalKey()
		if err != nil {
			return fmt.Errorf("kms not ready: %w", err)
		}
		ek, err := model.EncryptWithGK(gk, ref, key)
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
		refs = append(refs, ref)
	}

	// 2. 注册模型（只存 ref 列表，明文永不上链）
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
		APIKeyRefs:      refs,
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
	// 获取当前模型，保存旧 APIKeyRefs 用于清理
	cur, err := g.models.Get(g.api.GetTxn(), modelID)
	if err != nil || cur == nil {
		return fmt.Errorf("model not found")
	}
	oldRefs := cur.APIKeyRefs

	// 加密新的 apiKeys 并存储
	refs := make([]string, 0, len(apiKeys))
	for i, key := range apiKeys {
		if key == "" {
			continue
		}
		ref := fmt.Sprintf("%s_%d_%d", modelIDStr, g.api.GetHeight(), i)
		gk, err := model.GetGlobalKey()
		if err != nil {
			return fmt.Errorf("kms not ready: %w", err)
		}
		ek, err := model.EncryptWithGK(gk, ref, key)
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
		refs = append(refs, ref)
	}

	// 更新模型字段
	if err := g.models.UpdateFn(g.api.GetTxn(), modelID, func(m *ModelInfo) {
		m.PointsPer1K = pointsPer1K
		m.InputModalities = inputModalities
		m.ContextLength = contextLength
		m.PromptPricing = promptPricing
		m.ModelID = modelIDStr
		m.ProviderType = providerType
		m.URI = uri
		m.APIKeyRefs = refs
	}); err != nil {
		return err
	}

	// 清理旧的 encryptedKeys
	for _, ref := range oldRefs {
		_ = g.encryptedKeys.Delete(g.api.GetTxn(), ref)
	}
	return nil
}

func (g GatewayMutation) DeactivateModel(modelID uint64) error {
	return g.models.UpdateFn(g.api.GetTxn(), modelID, func(m *ModelInfo) {
		m.IsActive = false
	})
}

func (g GatewayQuery) GetModel(modelID uint64) (*ModelInfo, error) {
	return g.models.Get(g.api.GetTxn(), modelID)
}

func (g GatewayQuery) ListModels(offset uint64, limit uint32) ([]ModelInfo, error) {
	_, vals, err := g.models.List(g.api.GetTxn(), offset, limit)
	return vals, err
}

func (g GatewayQuery) ListActiveModels(offset uint64, limit uint32) ([]ModelInfo, error) {
	return g.listModelsFiltered(offset, limit, func(m *ModelInfo) bool { return m.IsActive })
}

func (g GatewayQuery) ListActiveModelsWithinProvider(providerID uint64, offset uint64, limit uint32) ([]ModelInfoWithProvider, error) {
	models, err := g.listModelsFiltered(offset, limit, func(m *ModelInfo) bool {
		return m.IsActive && m.ProviderID == providerID
	})
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

func (g GatewayQuery) listModelsFiltered(
	offset uint64, limit uint32, fn func(*ModelInfo) bool,
) ([]ModelInfo, error) {
	txn := g.api.GetTxn()
	total, _ := g.models.Len(txn)
	out := make([]ModelInfo, 0, limit)
	var skipped, collected uint64
	for i := uint64(0); i < uint64(total) && collected < uint64(limit); i++ {
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

func (g GatewayQuery) GetModelCount() (uint64, error) {
	return g.models.Len(g.api.GetTxn())
}

func (g GatewayQuery) QueryModels(
	modalities uint8,
	minContext, maxContext, maxPrice uint64,
	offset uint64, limit uint32,
) ([]ModelInfo, error) {
	return g.listModelsFiltered(offset, limit, func(m *ModelInfo) bool {
		if !m.IsActive {
			return false
		}
		if modalities != 0 && m.InputModalities&modalities == 0 {
			return false
		}
		if minContext != 0 && m.ContextLength < minContext {
			return false
		}
		if maxContext != 0 && m.ContextLength > maxContext {
			return false
		}
		if maxPrice != 0 && m.PromptPricing > maxPrice {
			return false
		}
		return true
	})
}

// GetModelsByProvider 返回某个 Provider 注册的所有模型（使用 ProviderID exact 索引）
func (g GatewayQuery) GetModelsByProvider(providerID uint64) ([]ModelInfo, error) {
	keys, err := g.models.Query(g.api.GetTxn(), map[string]any{"ProviderID": providerID})
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
func (g GatewayQuery) GetModelByName(name string) ([]ModelInfo, error) {
	keys, err := g.models.Query(g.api.GetTxn(), map[string]any{"Name": name})
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
