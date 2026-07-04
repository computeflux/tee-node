package indexer

import (
	"context"
	"testing"

	"github.com/computeflux/tee-provider/pkg/services/scheduler"
	cgateway "github.com/computeflux/tee-provider/tee-chain/runtime/module/gateway"
	"github.com/stretchr/testify/require"
)

func makeProvider(name string, active bool) cgateway.ProviderInfo {
	return cgateway.ProviderInfo{
		Name:     name,
		IsActive: active,
	}
}

func TestGatewayEndpointIndex_ResolveProviderModel(t *testing.T) {
	idx := NewGatewayEndpointIndex()
	models := []cgateway.ModelInfoWithProvider{
		{
			ID: 1,
			Model: cgateway.ModelInfo{
				Name:         "MiniMax-M2.7",
				ModelID:      "mini-max-m27",
				ProviderID:   100,
				ProviderType: cgateway.ProviderOpenAI,
				APIKeys:      []string{"key1"},
				IsActive:     true,
				MaxConcurrency: 5,
			},
			Provider: cgateway.ProviderInfo{Name: "Local", IsActive: true},
		},
		{
			ID: 2,
			Model: cgateway.ModelInfo{
				Name:         "MiniMax-M2.7",
				ModelID:      "mini-max-m27",
				ProviderID:   200,
				ProviderType: cgateway.ProviderOpenAI,
				APIKeys:      []string{"key2", "key3"},
				IsActive:     true,
				MaxConcurrency: 8,
			},
			Provider: cgateway.ProviderInfo{Name: "OpenAI", IsActive: true},
		},
	}
	require.NoError(t, idx.BuildFromData(models, nil))

	r, err := idx.Resolve(context.Background(), scheduler.ModelKey("MiniMax-M2.7"), scheduler.ProviderID("Local"))
	require.NoError(t, err)
	require.Len(t, r.Endpoints, 1)
	require.Equal(t, "key1", r.Endpoints[0].KeyValue)

	// modelID lookup
	r2, err := idx.Resolve(context.Background(), scheduler.ModelKey("mini-max-m27"), scheduler.ProviderID("OpenAI"))
	require.NoError(t, err)
	require.Len(t, r2.Endpoints, 2)
}

func TestGatewayEndpointIndex_ResolveBareModel_NotSupported(t *testing.T) {
	idx := NewGatewayEndpointIndex()
	models := []cgateway.ModelInfoWithProvider{
		{
			ID: 1,
			Model: cgateway.ModelInfo{
				Name:         "gpt-4",
				ProviderID:   100,
				ProviderType: cgateway.ProviderOpenAI,
				APIKeys:      []string{"a"},
				IsActive:     true,
				MaxConcurrency: 1,
			},
			Provider: cgateway.ProviderInfo{Name: "OpenAI", IsActive: true},
		},
	}
	require.NoError(t, idx.BuildFromData(models, nil))

	_, err := idx.Resolve(context.Background(), scheduler.ModelKey("gpt-4"), "")
	require.ErrorIs(t, err, scheduler.ErrModelNotFound)
}

func TestGatewayEndpointIndex_ResolveRoute(t *testing.T) {
	idx := NewGatewayEndpointIndex()
	models := []cgateway.ModelInfoWithProvider{
		{
			ID: 10,
			Model: cgateway.ModelInfo{
				Name:         "route-member",
				ProviderID:   1,
				ProviderType: cgateway.ProviderOpenAI,
				APIKeys:      []string{"k1"},
				IsActive:     true,
				MaxConcurrency: 3,
			},
			Provider: cgateway.ProviderInfo{Name: "p1", IsActive: true},
		},
	}
	routes := []cgateway.ModelRouteWithID{
		{
			ID: 1,
			Data: cgateway.ModelRoute{
				Name:      "myroute",
				RouteType: cgateway.EdgeRoute,
				IsActive:  true,
				ModelIds:  []uint64{10},
			},
		},
	}
	require.NoError(t, idx.BuildFromData(models, routes))

	r, err := idx.Resolve(context.Background(), scheduler.ModelKey("Edge/myroute"), "")
	require.NoError(t, err)
	require.Len(t, r.Endpoints, 1)
	require.Contains(t, r.Endpoints[0].Models, scheduler.ModelKey("Edge/myroute"))
}

func TestGatewayEndpointIndex_ResolveRoute_CaseInsensitive(t *testing.T) {
	idx := NewGatewayEndpointIndex()
	models := []cgateway.ModelInfoWithProvider{
		{
			ID: 10,
			Model: cgateway.ModelInfo{
				Name:           "route-member",
				ProviderID:     1,
				ProviderType:   cgateway.ProviderOpenAI,
				APIKeys:        []string{"k1"},
				IsActive:       true,
				MaxConcurrency: 3,
			},
			Provider: cgateway.ProviderInfo{Name: "p1", IsActive: true},
		},
	}
	routes := []cgateway.ModelRouteWithID{
		{
			ID: 1,
			Data: cgateway.ModelRoute{
				Name:      "myroute",
				RouteType: cgateway.MiningPoolRoute,
				IsActive:  true,
				ModelIds:  []uint64{10},
			},
		},
	}
	require.NoError(t, idx.BuildFromData(models, routes))

	for _, input := range []string{"MiningPool/myroute", "miningpool/myroute", "MININGPOOL/myroute"} {
		r, err := idx.Resolve(context.Background(), scheduler.ModelKey(input), "")
		require.NoError(t, err, "input=%s", input)
		require.Len(t, r.Endpoints, 1, "input=%s", input)
	}
}

func TestGatewayEndpointIndex_ResolveRoute_NameWithPrefix(t *testing.T) {
	idx := NewGatewayEndpointIndex()
	models := []cgateway.ModelInfoWithProvider{
		{
			ID: 10,
			Model: cgateway.ModelInfo{
				Name:           "route-member",
				ProviderID:     1,
				ProviderType:   cgateway.ProviderOpenAI,
				APIKeys:        []string{"k1"},
				IsActive:       true,
				MaxConcurrency: 3,
			},
			Provider: cgateway.ProviderInfo{Name: "p1", IsActive: true},
		},
	}
	// 模拟链上路由名被误写成带前缀的情况。
	routes := []cgateway.ModelRouteWithID{
		{
			ID: 1,
			Data: cgateway.ModelRoute{
				Name:      "MiningPool/myroute",
				RouteType: cgateway.MiningPoolRoute,
				IsActive:  true,
				ModelIds:  []uint64{10},
			},
		},
	}
	require.NoError(t, idx.BuildFromData(models, routes))

	r, err := idx.Resolve(context.Background(), scheduler.ModelKey("MiningPool/myroute"), "")
	require.NoError(t, err)
	require.Len(t, r.Endpoints, 1)
}

func TestGatewayEndpointIndex_RouteEndpointHasRouteID(t *testing.T) {
	idx := NewGatewayEndpointIndex()
	models := []cgateway.ModelInfoWithProvider{
		{
			ID: 10,
			Model: cgateway.ModelInfo{
				Name:           "route-member",
				ProviderID:     1,
				ProviderType:   cgateway.ProviderOpenAI,
				APIKeys:        []string{"k1"},
				IsActive:       true,
				MaxConcurrency: 3,
			},
			Provider: cgateway.ProviderInfo{Name: "p1", IsActive: true},
		},
	}
	routes := []cgateway.ModelRouteWithID{
		{
			ID: 42,
			Data: cgateway.ModelRoute{
				Name:      "myroute",
				RouteType: cgateway.MiningPoolRoute,
				IsActive:  true,
				ModelIds:  []uint64{10},
			},
		},
	}
	require.NoError(t, idx.BuildFromData(models, routes))

	r, err := idx.Resolve(context.Background(), scheduler.ModelKey("MiningPool/myroute"), "")
	require.NoError(t, err)
	require.Len(t, r.Endpoints, 1)
	require.Equal(t, uint64(42), r.Endpoints[0].RouteID)
}

func TestGatewayEndpointIndex_InactiveSkipped(t *testing.T) {
	idx := NewGatewayEndpointIndex()
	models := []cgateway.ModelInfoWithProvider{
		{
			ID: 1,
			Model: cgateway.ModelInfo{
				Name:         "off",
				ProviderID:   1,
				ProviderType: cgateway.ProviderOpenAI,
				APIKeys:      []string{"k"},
				IsActive:     false,
			},
			Provider: cgateway.ProviderInfo{Name: "p1", IsActive: true},
		},
	}
	require.NoError(t, idx.BuildFromData(models, nil))

	_, err := idx.Resolve(context.Background(), scheduler.ModelKey("off"), scheduler.ProviderID("p1"))
	require.ErrorIs(t, err, scheduler.ErrModelNotFound)
}
