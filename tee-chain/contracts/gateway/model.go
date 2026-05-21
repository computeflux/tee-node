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

func (g GatewayQuery) GetModel(modelID uint64) (*ModelInfo, error) {
	return g.models.Get(g.api.GetTxn(), modelID)
}

func (g GatewayQuery) GetModelCount() (uint64, error) {
	return g.models.Len(g.api.GetTxn())
}

func (g GatewayQuery) ListModels(offset uint64, limit uint32) ([]ModelInfo, error) {
	_, vals, err := g.models.List(g.api.GetTxn(), offset, limit)
	return vals, err
}

func (g GatewayQuery) ListActiveModels(offset uint64, limit uint32) ([]ModelInfo, error) {
	return g.listModelsFiltered(offset, limit, func(m *ModelInfo) bool { return m.IsActive })
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
