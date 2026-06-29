package scheduler

import (
	"context"
	"fmt"
	"sync"
	"time"

	teechain "github.com/computeflux/tee-provider/tee-chain"
	contracts "github.com/computeflux/tee-provider/tee-chain/runtime"
	cgateway "github.com/computeflux/tee-provider/tee-chain/runtime/module/gateway"
)

// BridgeGetter 函数式注入,避免在 scheduler 包里直接依赖 teechain 包。
// main.go 启动时调用 SetBridgeGetter 注入。
type BridgeGetter func() *teechain.TeeChain

var (
	bridgeGetter   BridgeGetter
	bridgeGetterMu sync.RWMutex
)

// SetBridgeGetter 注入 bridge getter。
func SetBridgeGetter(g BridgeGetter) {
	bridgeGetterMu.Lock()
	defer bridgeGetterMu.Unlock()
	bridgeGetter = g
}

func getBridge() *teechain.TeeChain {
	bridgeGetterMu.RLock()
	defer bridgeGetterMu.RUnlock()
	if bridgeGetter == nil {
		return nil
	}
	return bridgeGetter()
}

// providerTypeName 把合约的 ProviderType 转成调度器用的字符串名。
func providerTypeName(pt uint16) string {
	switch pt {
	case cgateway.ProviderOpenAI:
		return "openai"
	case cgateway.ProviderClaude:
		return "anthropic"
	case cgateway.ProviderGemini:
		return "gemini"
	case cgateway.ProviderPersonal:
		return "personal"
	default:
		return fmt.Sprintf("provider-%d", pt)
	}
}

// Reloader 从 on-chain contracts 定期同步路由表到 scheduler.Registry。
type Reloader struct {
	gateway  *GatewayScheduler
	interval time.Duration
}

// NewReloader 构造 reloader。
func NewReloader(g *GatewayScheduler, interval time.Duration) *Reloader {
	if interval == 0 {
		interval = time.Minute
	}
	return &Reloader{gateway: g, interval: interval}
}

// Run 启动后台 ticker。ctx 取消时停止。
func (r *Reloader) Run(ctx context.Context) {
	r.gateway.log.Info("reloader started", "interval", r.interval)
	defer r.gateway.log.Info("reloader stopped")
	t := time.NewTicker(r.interval)
	defer t.Stop()

	// 立即跑一次,不等第一个 tick
	if err := r.reloadOnce(ctx); err != nil {
		r.gateway.log.Warn("initial reload failed", "error", err)
	}

	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			if err := r.reloadOnce(ctx); err != nil {
				r.gateway.log.Warn("scheduled reload failed", "error", err)
			}
		}
	}
}

// ReloadNow 立即重载一次(给 GraphQL mutation 用)。
func (r *Reloader) ReloadNow(ctx context.Context) error {
	r.gateway.log.Info("manual reload triggered")
	return r.reloadOnce(ctx)
}

func (r *Reloader) reloadOnce(ctx context.Context) error {
	bridge := getBridge()
	if bridge == nil {
		r.gateway.log.Error("reload failed: bridge not initialized")
		return fmt.Errorf("bridge not initialized")
	}

	// 只注册 active routes。直连请求(provider/model)走 EndpointBalancer,
	// scheduler 只服务 router 路径(auto/routeName)。
	routes, err := teechain.WithRuntime(bridge, func(rt *contracts.Runtime) ([]cgateway.ModelRouteWithID, error) {
		return cgateway.GatewayQuery{Gateway: *cgateway.NewGateway(rt)}.ListRoutes(0, 0)
	})
	if err != nil {
		r.gateway.log.Error("reload failed: list routes error", "error", err)
		return fmt.Errorf("list routes: %w", err)
	}

	var activeRoutes int
	var schedRoutes []*ModelRoute
	for _, route := range routes {
		if !route.Data.IsActive {
			continue
		}
		activeRoutes++
		mr, err := r.buildRoute(bridge, route)
		if err != nil {
			r.gateway.log.Warn("route build failed", "route_name", route.Data.Name, "route_id", route.ID, "error", err)
			continue
		}
		schedRoutes = append(schedRoutes, mr)
		// 缓存 route ID → name 的映射,DistributeMiddleware 用
		SetRouteNameMapping(uint64(route.ID), route.Data.Name)
	}

	r.gateway.log.Info("reload completed",
		"contract_routes", len(routes),
		"active_routes", activeRoutes,
		"built_routes", len(schedRoutes),
	)

	r.gateway.ReloadRegistry(schedRoutes)
	return nil
}

// buildRoute 把合约的 ModelRoute 转成 scheduler.ModelRoute。
func (r *Reloader) buildRoute(bridge *teechain.TeeChain, route cgateway.ModelRouteWithID) (*ModelRoute, error) {
	var endpoints []*Endpoint
	maxConcurrency := 0
	for _, modelID := range route.Data.ModelIds {
		modelInfo, err := teechain.WithRuntime(bridge, func(rt *contracts.Runtime) (*cgateway.ModelInfo, error) {
			return cgateway.GatewayQuery{Gateway: *cgateway.NewGateway(rt)}.GetModel(modelID)
		})
		if err != nil || modelInfo == nil || !modelInfo.IsActive {
			continue
		}
		if int(modelInfo.MaxConcurrency) > maxConcurrency {
			maxConcurrency = int(modelInfo.MaxConcurrency)
		}
		providerInfo, err := teechain.WithRuntime(bridge, func(rt *contracts.Runtime) (*cgateway.ProviderInfo, error) {
			return cgateway.GatewayQuery{Gateway: *cgateway.NewGateway(rt)}.GetProvider(modelInfo.ProviderID)
		})
		if err != nil || providerInfo == nil {
			continue
		}
		eps := modelToEndpoints(modelInfo, providerInfo)
		endpoints = append(endpoints, eps...)
	}
	if len(endpoints) == 0 {
		return nil, fmt.Errorf("no endpoints for route %s", route.Data.Name)
	}
	return &ModelRoute{
		Model:          ModelKey(route.Data.Name),
		MaxConcurrency: maxConcurrency,
		Endpoints:      endpoints,
		FallbackModels: nil,
	}, nil
}

// modelToEndpoints 把单个 ModelInfo 转成一组 scheduler.Endpoint（每个 APIKey 一个）。
func modelToEndpoints(model *cgateway.ModelInfo, provider *cgateway.ProviderInfo) []*Endpoint {
	var endpoints []*Endpoint

	// Provider 是 provider 节点名（如 "node-7f3a"），Protocol 是协议类型（如 "openai"/"anthropic"）
	providerName := provider.Name
	if providerName == "" {
		providerName = fmt.Sprintf("provider-%x", provider.Owner)
	}
	protocol := providerTypeName(model.ProviderType)
	baseURL := model.URI

	for i, key := range model.APIKeys {
		if key == "" {
			continue
		}
		endpoints = append(endpoints, &Endpoint{
			Provider:    ProviderID(providerName),
			Protocol:    ProviderID(protocol),
			KeyID:       fmt.Sprintf("%s-%d", model.ModelID, i),
			KeyValue:    key,
			BaseURL:     baseURL,
			Weight:      1.0, // 合约暂无权重，均分
			Enabled:     model.IsActive && model.Balance > 0,
			Models:      []ModelKey{ModelKey(model.Name), ModelKey(model.ModelID)},
			Concurrency: int(model.MaxConcurrency),
		})
	}
	return endpoints
}

// Compile-time guard
var _ sync.Mutex
