package indexer

import (
	"context"
	"testing"

	cgateway "github.com/computeflux/tee-provider/tee-chain/runtime/module/gateway"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func sampleModel(id uint64, name string, providerType uint16, contextLength, inPrice uint64) cgateway.ModelInfoWithProvider {
	return cgateway.ModelInfoWithProvider{
		ID: id,
		Model: cgateway.ModelInfo{
			Name:            name,
			ProviderID:      1,
			ProviderType:    providerType,
			ModelID:         name,
			URI:             "http://localhost",
			InputModalities: 1,
			ContextLength:   contextLength,
			InPrice:         inPrice,
			CachedPrice:     0,
			OutPrice:        0,
			IsActive:        true,
			CreatedAt:       0,
		},
		Provider: cgateway.ProviderInfo{
			Name: "test-provider",
		},
	}
}

func TestGatewayModelIndex_KeywordSearch(t *testing.T) {
	idx := NewGatewayModelIndex()
	models := []cgateway.ModelInfoWithProvider{
		sampleModel(1, "gpt-4o", 1, 128000, 1000),
		sampleModel(2, "claude-3-opus", 2, 200000, 2000),
		sampleModel(3, "gpt-4-turbo", 1, 128000, 1500),
	}
	require.NoError(t, idx.BuildFromModels(models))

	res, err := idx.Search(context.Background(), GatewayModelSearchRequest{
		Keyword: "gpt",
		Limit:   10,
	})
	require.NoError(t, err)
	assert.Len(t, res, 2)
}

func TestGatewayModelIndex_RangeFilter(t *testing.T) {
	idx := NewGatewayModelIndex()
	models := []cgateway.ModelInfoWithProvider{
		sampleModel(1, "small", 1, 4000, 100),
		sampleModel(2, "medium", 1, 128000, 500),
		sampleModel(3, "large", 1, 200000, 2000),
	}
	require.NoError(t, idx.BuildFromModels(models))

	minCtx := uint64(100000)
	maxCtx := uint64(150000)
	res, err := idx.Search(context.Background(), GatewayModelSearchRequest{
		MinContext: &minCtx,
		MaxContext: &maxCtx,
		Limit:      10,
	})
	require.NoError(t, err)
	require.Len(t, res, 1)
	assert.Equal(t, "medium", res[0].Model.Name)
}

func TestGatewayModelIndex_KeywordAndRange(t *testing.T) {
	idx := NewGatewayModelIndex()
	models := []cgateway.ModelInfoWithProvider{
		sampleModel(1, "gpt-small", 1, 4000, 100),
		sampleModel(2, "gpt-large", 1, 200000, 2000),
		sampleModel(3, "claude-large", 2, 200000, 2000),
	}
	require.NoError(t, idx.BuildFromModels(models))

	minCtx := uint64(100000)
	res, err := idx.Search(context.Background(), GatewayModelSearchRequest{
		Keyword:    "gpt",
		MinContext: &minCtx,
		Limit:      10,
	})
	require.NoError(t, err)
	require.Len(t, res, 1)
	assert.Equal(t, "gpt-large", res[0].Model.Name)
}

func TestGatewayModelIndex_Pagination(t *testing.T) {
	idx := NewGatewayModelIndex()
	models := []cgateway.ModelInfoWithProvider{
		sampleModel(1, "model-a", 1, 1000, 100),
		sampleModel(2, "model-b", 1, 1000, 100),
		sampleModel(3, "model-c", 1, 1000, 100),
	}
	require.NoError(t, idx.BuildFromModels(models))

	res, err := idx.Search(context.Background(), GatewayModelSearchRequest{
		Offset: 1,
		Limit:  2,
	})
	require.NoError(t, err)
	assert.Len(t, res, 2)
}

func TestGatewayModelIndex_Get(t *testing.T) {
	idx := NewGatewayModelIndex()
	models := []cgateway.ModelInfoWithProvider{
		sampleModel(1, "model-a", 1, 1000, 100),
		sampleModel(2, "model-b", 1, 2000, 200),
	}
	require.NoError(t, idx.BuildFromModels(models))

	m, ok := idx.Get(1)
	require.True(t, ok)
	assert.Equal(t, "model-a", m.Model.Name)

	_, ok = idx.Get(999)
	assert.False(t, ok)
}

func TestGatewayModelIndex_ModalityFilter(t *testing.T) {
	idx := NewGatewayModelIndex()
	models := []cgateway.ModelInfoWithProvider{
		{ID: 1, Model: cgateway.ModelInfo{Name: "text-only", ProviderID: 1, ProviderType: 1, ModelID: "text-only", URI: "http://localhost", InputModalities: cgateway.ModalityText, ContextLength: 1000, InPrice: 100, IsActive: true}, Provider: cgateway.ProviderInfo{Name: "p1"}},
		{ID: 2, Model: cgateway.ModelInfo{Name: "text-image", ProviderID: 1, ProviderType: 1, ModelID: "text-image", URI: "http://localhost", InputModalities: cgateway.ModalityText | cgateway.ModalityImage, ContextLength: 1000, InPrice: 100, IsActive: true}, Provider: cgateway.ProviderInfo{Name: "p1"}},
		{ID: 3, Model: cgateway.ModelInfo{Name: "audio-video", ProviderID: 1, ProviderType: 1, ModelID: "audio-video", URI: "http://localhost", InputModalities: cgateway.ModalityAudio | cgateway.ModalityVideo, ContextLength: 1000, InPrice: 100, IsActive: true}, Provider: cgateway.ProviderInfo{Name: "p1"}},
	}
	require.NoError(t, idx.BuildFromModels(models))

	// 查询支持 Text 或 Image 的模型（位掩码包含 Text|Image）
	res, err := idx.Search(context.Background(), GatewayModelSearchRequest{
		Modalities: cgateway.ModalityText | cgateway.ModalityImage,
		Limit:      10,
	})
	require.NoError(t, err)
	require.Len(t, res, 2)
	names := map[string]bool{}
	for _, m := range res {
		names[m.Model.Name] = true
	}
	assert.True(t, names["text-only"])
	assert.True(t, names["text-image"])
	assert.False(t, names["audio-video"])
}
