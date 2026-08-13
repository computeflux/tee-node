package indexer

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"sync"

	"github.com/blevesearch/bleve/v2"
	"github.com/blevesearch/bleve/v2/mapping"

	"github.com/computeflux/tee-provider/pkg/contractquery"
	teechain "github.com/computeflux/tee-provider/tee-chain"
	cgateway "github.com/computeflux/tee-provider/tee-chain/runtime/module/gateway"
)

const GatewayRouterIndexName = "gateway_routers"

// GatewayRouterSearchRequest 定义 gateway 路由索引的查询参数。
type GatewayRouterSearchRequest struct {
	Keyword    string
	RouteType  *uint16
	IsActive   *bool
	IsOfficial *bool
	Offset     uint64
	Limit      uint32
}

// GatewayRouterIndex 为 gateway 合约中的路由建立内存索引，支持名称模糊搜索和状态筛选。
type GatewayRouterIndex struct {
	mu     sync.RWMutex
	index  bleve.Index
	routes map[string]cgateway.ModelRouteWithID
}

// NewGatewayRouterIndex 创建 gateway 路由索引实例。
func NewGatewayRouterIndex() *GatewayRouterIndex {
	return &GatewayRouterIndex{}
}

// Name 返回索引名称。
func (g *GatewayRouterIndex) Name() string {
	return GatewayRouterIndexName
}

// Bleve 返回当前底层 Bleve 索引。
func (g *GatewayRouterIndex) Bleve() bleve.Index {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.index
}

// Refresh 从链上拉取全部路由并重建索引。
func (g *GatewayRouterIndex) Refresh(ctx context.Context, chain *teechain.TeeChain) error {
	routes, err := contractquery.GatewayListAllRoutes(chain, 0, 100000)
	if err != nil {
		return fmt.Errorf("list all routes: %w", err)
	}
	return g.BuildFromRoutes(routes)
}

// BuildFromRoutes 直接从路由数据重建索引，便于测试和外部批量更新。
func (g *GatewayRouterIndex) BuildFromRoutes(routes []cgateway.ModelRouteWithID) error {
	indexMapping := bleve.NewIndexMapping()
	indexMapping.DefaultMapping = g.buildDocumentMapping()

	newIndex, err := bleve.NewMemOnly(indexMapping)
	if err != nil {
		return fmt.Errorf("create bleve index: %w", err)
	}

	newRoutes := make(map[string]cgateway.ModelRouteWithID, len(routes))
	for _, r := range routes {
		docID := strconv.FormatUint(r.ID, 10)
		newRoutes[docID] = r

		doc := map[string]any{
			"name":        r.Data.Name,
			"routeType":   r.Data.RouteType,
			"inPrice":     r.Data.InPrice,
			"cachedPrice": r.Data.CachedPrice,
			"outPrice":    r.Data.OutPrice,
			"isActive":    boolToNum(r.Data.IsActive),
			"isOfficial":  boolToNum(r.Data.IsOfficial),
			"createdAt":   r.Data.CreatedAt,
		}
		if err := newIndex.Index(docID, doc); err != nil {
			_ = newIndex.Close()
			return fmt.Errorf("index route %s: %w", docID, err)
		}
	}

	g.mu.Lock()
	defer g.mu.Unlock()
	if g.index != nil {
		_ = g.index.Close()
	}
	g.index = newIndex
	g.routes = newRoutes
	Logger("GatewayRouterIndex built routes=%d", len(routes))
	return nil
}

// Search 在索引中查询路由。
// 名称使用 Bleve 模糊匹配；routeType / isActive / isOfficial 在内存中精确过滤。
func (g *GatewayRouterIndex) Search(ctx context.Context, req GatewayRouterSearchRequest) ([]cgateway.ModelRouteWithID, error) {
	g.mu.RLock()
	index := g.index
	routes := g.routes
	g.mu.RUnlock()

	if routes == nil || index == nil {
		return nil, fmt.Errorf("gateway router index not ready")
	}

	candidateIDs := make(map[string]struct{}, len(routes))
	if req.Keyword != "" {
		mq := bleve.NewMatchQuery(req.Keyword)
		mq.SetField("name")
		mq.SetFuzziness(1)
		searchReq := bleve.NewSearchRequest(mq)
		searchReq.Size = len(routes)
		searchReq.Fields = []string{"*"}
		res, err := index.Search(searchReq)
		if err != nil {
			return nil, fmt.Errorf("search index: %w", err)
		}
		for _, hit := range res.Hits {
			candidateIDs[hit.ID] = struct{}{}
		}
	} else {
		for id := range routes {
			candidateIDs[id] = struct{}{}
		}
	}

	out := make([]cgateway.ModelRouteWithID, 0, len(candidateIDs))
	for id := range candidateIDs {
		r, ok := routes[id]
		if !ok {
			continue
		}
		if req.RouteType != nil && r.Data.RouteType != *req.RouteType {
			continue
		}
		if req.IsActive != nil && r.Data.IsActive != *req.IsActive {
			continue
		}
		if req.IsOfficial != nil && r.Data.IsOfficial != *req.IsOfficial {
			continue
		}
		out = append(out, r)
	}

	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })

	if req.Offset >= uint64(len(out)) {
		return []cgateway.ModelRouteWithID{}, nil
	}
	end := req.Offset + uint64(req.Limit)
	if end == 0 || end > uint64(len(out)) {
		end = uint64(len(out))
	}
	return out[req.Offset:end], nil
}

func (g *GatewayRouterIndex) buildDocumentMapping() *mapping.DocumentMapping {
	dm := bleve.NewDocumentMapping()

	nameFM := bleve.NewTextFieldMapping()
	nameFM.Analyzer = "standard"
	dm.AddFieldMappingsAt("name", nameFM)

	dm.AddFieldMappingsAt("routeType", bleve.NewNumericFieldMapping())
	dm.AddFieldMappingsAt("inPrice", bleve.NewNumericFieldMapping())
	dm.AddFieldMappingsAt("cachedPrice", bleve.NewNumericFieldMapping())
	dm.AddFieldMappingsAt("outPrice", bleve.NewNumericFieldMapping())
	dm.AddFieldMappingsAt("isActive", bleve.NewNumericFieldMapping())
	dm.AddFieldMappingsAt("isOfficial", bleve.NewNumericFieldMapping())
	dm.AddFieldMappingsAt("createdAt", bleve.NewNumericFieldMapping())

	return dm
}

func boolToNum(b bool) uint64 {
	if b {
		return 1
	}
	return 0
}
