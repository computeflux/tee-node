package indexer

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	cgateway "github.com/computeflux/tee-provider/tee-chain/contracts/module/gateway"
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
