package indexer

import (
	"context"
	"testing"

	cgateway "github.com/computeflux/tee-provider/tee-chain/runtime/module/gateway"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func sampleRoute(id uint64, name string, routeType uint16, isActive, isOfficial bool) cgateway.ModelRouteWithID {
	return cgateway.ModelRouteWithID{
		ID: id,
		Data: cgateway.ModelRoute{
			Name:        name,
			RouteType:   routeType,
			InPrice:     100,
			CachedPrice: 50,
			OutPrice:    200,
			ModelIds:    []uint64{},
			IsActive:    isActive,
			IsOfficial:  isOfficial,
			CreatedAt:   0,
		},
	}
}

func TestGatewayRouterIndex_Search_Keyword(t *testing.T) {
	idx := NewGatewayRouterIndex()
	routes := []cgateway.ModelRouteWithID{
		sampleRoute(1, "official-personal", cgateway.EdgeRoute, true, true),
		sampleRoute(2, "official-node", cgateway.MiningPoolRoute, true, true),
		sampleRoute(3, "custom-privacy", cgateway.TEERoute, true, false),
	}
	require.NoError(t, idx.BuildFromRoutes(routes))

	res, err := idx.Search(context.Background(), GatewayRouterSearchRequest{
		Keyword: "official",
		Limit:   10,
	})
	require.NoError(t, err)
	require.Len(t, res, 2)
	names := map[string]bool{}
	for _, r := range res {
		names[r.Data.Name] = true
	}
	assert.True(t, names["official-personal"])
	assert.True(t, names["official-node"])
	assert.False(t, names["custom-privacy"])
}

func TestGatewayRouterIndex_Search_RouteType(t *testing.T) {
	idx := NewGatewayRouterIndex()
	routes := []cgateway.ModelRouteWithID{
		sampleRoute(1, "official-personal", cgateway.EdgeRoute, true, true),
		sampleRoute(2, "official-node", cgateway.MiningPoolRoute, true, true),
		sampleRoute(3, "custom-privacy", cgateway.TEERoute, true, false),
	}
	require.NoError(t, idx.BuildFromRoutes(routes))

	nodeType := cgateway.MiningPoolRoute
	res, err := idx.Search(context.Background(), GatewayRouterSearchRequest{
		RouteType: &nodeType,
		Limit:     10,
	})
	require.NoError(t, err)
	require.Len(t, res, 1)
	assert.Equal(t, "official-node", res[0].Data.Name)
}

func TestGatewayRouterIndex_Search_IsOfficial(t *testing.T) {
	idx := NewGatewayRouterIndex()
	routes := []cgateway.ModelRouteWithID{
		sampleRoute(1, "official-personal", cgateway.EdgeRoute, true, true),
		sampleRoute(2, "official-node", cgateway.MiningPoolRoute, true, true),
		sampleRoute(3, "custom-privacy", cgateway.TEERoute, true, false),
	}
	require.NoError(t, idx.BuildFromRoutes(routes))

	official := true
	res, err := idx.Search(context.Background(), GatewayRouterSearchRequest{
		IsOfficial: &official,
		Limit:      10,
	})
	require.NoError(t, err)
	require.Len(t, res, 2)
}

func TestGatewayRouterIndex_Search_NotReady(t *testing.T) {
	idx := NewGatewayRouterIndex()
	_, err := idx.Search(context.Background(), GatewayRouterSearchRequest{Limit: 10})
	require.Error(t, err)
}
