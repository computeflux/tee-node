package indexer

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"

	"github.com/blevesearch/bleve/v2"

	"github.com/computeflux/tee-provider/pkg/api/gateway"
	"github.com/computeflux/tee-provider/pkg/services/scheduler"
	teechain "github.com/computeflux/tee-provider/tee-chain"
	cgateway "github.com/computeflux/tee-provider/tee-chain/runtime/module/gateway"
)

const GatewayEndpointIndexName = "gateway_endpoints"

// GatewayEndpointIndex 缓存 scheduler 可用的全部端点（API Key 未脱敏）。
// 支持按 route 名或 provider/model 名查询；不支持裸模型名。
type GatewayEndpointIndex struct {
	mu sync.RWMutex

	byProviderModel map[string]*scheduler.ModelRoute // provider/name
	byRoute         map[string]*scheduler.ModelRoute // Edge/name 等
}

// NewGatewayEndpointIndex 创建 scheduler 端点索引。
func NewGatewayEndpointIndex() *GatewayEndpointIndex {
	return &GatewayEndpointIndex{}
}

// Name 返回索引名称。
func (g *GatewayEndpointIndex) Name() string { return GatewayEndpointIndexName }

// Bleve 返回 nil；端点索引使用内存 map，不依赖 Bleve。
func (g *GatewayEndpointIndex) Bleve() bleve.Index { return nil }

// Refresh 从链上拉取未脱敏模型与路由，重建端点缓存。
func (g *GatewayEndpointIndex) Refresh(ctx context.Context, chain *teechain.TeeChain) error {
	models, err := gateway.GatewaySecretListActiveModelsWithinProvider(chain)
	if err != nil {
		return fmt.Errorf("list secret active models: %w", err)
	}

	routes, err := gateway.GatewayListAllRoutes(chain, 0, 100000)
	if err != nil {
		return fmt.Errorf("list routes: %w", err)
	}

	return g.BuildFromData(models, routes)
}

// BuildFromData 直接从模型/路由数据重建端点缓存，便于测试和外部批量更新。
func (g *GatewayEndpointIndex) BuildFromData(models []cgateway.ModelInfoWithProvider, routes []cgateway.ModelRouteWithID) error {
	modelByID := make(map[uint64]cgateway.ModelInfoWithProvider, len(models))
	for _, m := range models {
		modelByID[m.ID] = m
	}

	byProviderModel := make(map[string]*scheduler.ModelRoute)

	for _, m := range models {
		if !m.Model.IsActive || !m.Provider.IsActive {
			continue
		}

		route := scheduler.BuildModelRoute(m)
		if len(route.Endpoints) == 0 {
			continue
		}

		provider := m.Provider.Name
		byProviderModel[providerModelKey(provider, string(route.Model))] = route
	}

	byRoute := make(map[string]*scheduler.ModelRoute)
	for _, r := range routes {
		// 跳过非活跃路由
		if !r.Data.IsActive {
			continue
		}

		// 跳过模型 ID 为空的路由
		if len(r.Data.ModelIds) == 0 {
			continue
		}

		// 允许链上存储的路由名本身带前缀（如 UI 误传 "MiningPool/foo"），
		// 索引层做一次归一化，避免请求 "MiningPool/foo" 时找不到 "MiningPool/miningpool/foo"。
		routeName := normalizeRouteName(r.Data.RouteType, r.Data.Name)
		routeKeyName := routeKey(r.Data.RouteType, routeName)
		routeModelKey := scheduler.ModelKey(routeKeyName)

		var endpoints []*scheduler.Endpoint
		maxConcurrency := 0
		for _, id := range r.Data.ModelIds {
			m, ok := modelByID[id]
			if !ok || !m.Model.IsActive || !m.Provider.IsActive {
				continue
			}
			eps := scheduler.BuildEndpointsFromModelInfo(m.ID, &m.Model, &m.Provider)
			if len(eps) == 0 {
				continue
			}
			for _, ep := range eps {
				ep.Models = append(ep.Models, routeModelKey)
				ep.RouteID = r.ID
			}
			endpoints = append(endpoints, eps...)
			if int(m.Model.MaxConcurrency) > maxConcurrency {
				maxConcurrency = int(m.Model.MaxConcurrency)
			}
		}

		if len(endpoints) == 0 {
			Logger("GatewayEndpointIndex route skipped (no usable endpoints): key=%s model_ids=%v", routeKeyName, r.Data.ModelIds)
			continue
		}

		sort.SliceStable(endpoints, func(i, j int) bool {
			return endpoints[i].Weight > endpoints[j].Weight
		})

		byRoute[routeKeyName] = &scheduler.ModelRoute{
			Model:          routeModelKey,
			MaxConcurrency: maxConcurrency,
			Endpoints:      endpoints,
		}
	}

	g.mu.Lock()
	defer g.mu.Unlock()
	g.byProviderModel = byProviderModel
	g.byRoute = byRoute

	Logger("GatewayEndpointIndex built provider_model=%d route=%d",
		len(byProviderModel), len(byRoute))
	return nil
}

// Resolve 实现 scheduler.ModelResolver。
//   - Edge/xxx、MiningPool/xxx、TEE/xxx 解析为 route（前缀大小写不敏感）；
//   - provider/model 解析为指定 provider；
//   - 裸模型名等价于 openai/model，不再跨 provider 聚合。
func (g *GatewayEndpointIndex) Resolve(ctx context.Context, model scheduler.ModelKey, provider scheduler.ProviderID) (*scheduler.ModelRoute, error) {
	name := string(model)

	if routeType, routeName, ok := parseRoutePrefix(name); ok {
		g.mu.RLock()
		route, ok := g.byRoute[routeKey(routeType, routeName)]
		routeCount := len(g.byRoute)
		g.mu.RUnlock()
		if !ok {
			Logger("GatewayEndpointIndex route not found: key=%s available_routes=%d", routeKey(routeType, routeName), routeCount)
			return nil, fmt.Errorf("%w: %s", scheduler.ErrModelNotFound, model)
		}
		return route, nil
	}

	if provider == "" {
		return nil, fmt.Errorf("%w: bare model name is not supported, use provider/model or route", scheduler.ErrModelNotFound)
	}

	key := providerModelKey(string(provider), name)
	g.mu.RLock()
	route, ok := g.byProviderModel[key]
	g.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("%w: %s/%s", scheduler.ErrModelNotFound, provider, model)
	}
	return route, nil
}

// providerModelKey 生成 provider+model 的内部索引 key（大小写不敏感）。
func providerModelKey(provider, model string) string {
	return strings.ToLower(provider) + "/" + strings.ToLower(model)
}

// routeKey 生成 route 内部索引 key。
func routeKey(routeType uint16, name string) string {
	var prefix string
	switch routeType {
	case cgateway.EdgeRoute:
		prefix = "Edge/"
	case cgateway.MiningPoolRoute:
		prefix = "MiningPool/"
	case cgateway.TEERoute:
		prefix = "TEE/"
	default:
		prefix = "route/"
	}
	return prefix + strings.ToLower(name)
}

// parseRoutePrefix 解析 route 前缀（大小写不敏感）。
func parseRoutePrefix(name string) (routeType uint16, routeName string, ok bool) {
	lower := strings.ToLower(name)
	switch {
	case strings.HasPrefix(lower, "edge/"):
		return cgateway.EdgeRoute, name[len("Edge/"):], true
	case strings.HasPrefix(lower, "miningpool/"):
		return cgateway.MiningPoolRoute, name[len("MiningPool/"):], true
	case strings.HasPrefix(lower, "tee/"):
		return cgateway.TEERoute, name[len("TEE/"):], true
	}
	return 0, "", false
}

// normalizeRouteName 去掉与 routeType 匹配的前缀，避免链上存储了带前缀的路由名。
func normalizeRouteName(routeType uint16, name string) string {
	lower := strings.ToLower(name)
	var prefix string
	switch routeType {
	case cgateway.EdgeRoute:
		prefix = "edge/"
	case cgateway.MiningPoolRoute:
		prefix = "miningpool/"
	case cgateway.TEERoute:
		prefix = "tee/"
	default:
		return name
	}
	if strings.HasPrefix(lower, prefix) {
		return name[len(prefix):]
	}
	return name
}

// cloneRoute 浅拷贝路由并替换 Model 名（端点指针共享，保证 metrics 一致）。
func cloneRoute(src *scheduler.ModelRoute, model scheduler.ModelKey) *scheduler.ModelRoute {
	cp := *src
	cp.Model = model
	return &cp
}
