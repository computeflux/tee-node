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

	// 先注册 active routes(auto/routeName 路径);直连模型(provider/model)随后由
	// buildModelRoutes 按模型名注册。两类路由统一由 scheduler 负载均衡。
	routes, err := teechain.WithRuntime(bridge, func(rt *contracts.Runtime) ([]cgateway.ModelRouteWithID, error) {
		return cgateway.GatewayQuery{Gateway: *cgateway.NewGateway(rt)}.ListRoutes(0, 0)
	})
	if err != nil {
		r.gateway.log.Error("reload failed: list routes error", "error", err)
		return fmt.Errorf("list routes: %w", err)
	}

	var activeRoutes int
	var schedRoutes []*ModelRoute
	taken := make(map[ModelKey]bool)
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
		taken[mr.Model] = true
		// 缓存 route ID → name 的映射,DistributeMiddleware 用
		SetRouteNameMapping(uint64(route.ID), route.Data.Name)
	}

	// 直连模型访问(provider/model):把每个 active 独立模型也注册成「按模型名」的路由。
	// 显式 route 同名时优先,不被覆盖(taken 集合)。
	modelRoutes := r.buildModelRoutes(bridge, taken)
	schedRoutes = append(schedRoutes, modelRoutes...)

	r.gateway.log.Info("reload completed",
		"contract_routes", len(routes),
		"active_routes", activeRoutes,
		"built_routes", len(schedRoutes),
		"model_routes", len(modelRoutes),
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
			// GatewaySecret 返回未脱敏数据,scheduler 调上游需要真实 APIKey。
			return cgateway.GatewaySecret{Gateway: *cgateway.NewGateway(rt)}.GetModel(modelID)
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
		eps := modelToEndpoints(modelID, modelInfo, providerInfo)
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

// buildModelRoutes 把所有 active 的独立模型注册成「按模型名」的单组路由,
// 恢复直连模型访问(provider/model → 去掉前缀后按模型名查)。
// 同名模型(可能跨 provider)的端点合并到一条路由,由 scheduler 在多端点间负载均衡。
// taken 里已被显式 route 占用的名字会跳过,保证 route 优先级高于独立模型。
func (r *Reloader) buildModelRoutes(bridge *teechain.TeeChain, taken map[ModelKey]bool) []*ModelRoute {
	// 先用 GatewayQuery 列出 active 模型(只为拿 ID 列表),再逐个用 GatewaySecret 取未脱敏数据。
	withIDs, err := teechain.WithRuntime(bridge, func(rt *contracts.Runtime) ([]cgateway.ModelInfoWithID, error) {
		return cgateway.GatewayQuery{Gateway: *cgateway.NewGateway(rt)}.ListActiveModels(0, 10000)
	})
	if err != nil {
		r.gateway.log.Warn("model sweep failed: list active models", "error", err)
		return nil
	}

	byName := make(map[ModelKey]*ModelRoute)
	for _, m := range withIDs {
		key := ModelKey(m.Data.Name)
		if taken[key] {
			continue // 显式 route 优先,不被独立模型覆盖
		}
		// 拿未脱敏的完整模型(含真实 APIKey),scheduler 调上游需要。
		full, err := teechain.WithRuntime(bridge, func(rt *contracts.Runtime) (*cgateway.ModelInfo, error) {
			return cgateway.GatewaySecret{Gateway: *cgateway.NewGateway(rt)}.GetModel(m.ID)
		})
		if err != nil || full == nil || !full.IsActive {
			continue
		}
		providerInfo, err := teechain.WithRuntime(bridge, func(rt *contracts.Runtime) (*cgateway.ProviderInfo, error) {
			return cgateway.GatewayQuery{Gateway: *cgateway.NewGateway(rt)}.GetProvider(full.ProviderID)
		})
		if err != nil || providerInfo == nil {
			continue
		}
		eps := modelToEndpoints(m.ID, full, providerInfo)
		if len(eps) == 0 {
			continue
		}
		route := byName[key]
		if route == nil {
			route = &ModelRoute{Model: key}
			byName[key] = route
		}
		if int(full.MaxConcurrency) > route.MaxConcurrency {
			route.MaxConcurrency = int(full.MaxConcurrency)
		}
		route.Endpoints = append(route.Endpoints, eps...)
	}

	out := make([]*ModelRoute, 0, len(byName))
	for _, route := range byName {
		out = append(out, route)
	}
	return out
}

// providerDefaultURL 返回 provider 官方默认 base URL(model.URI 为空时用)。
// 与 pkg/api/gateway.providerDefaultURL 保持一致;scheduler 不能反向 import gateway。
func providerDefaultURL(providerType uint16) string {
	switch providerType {
	case cgateway.ProviderOpenAI:
		return "https://api.openai.com"
	case cgateway.ProviderClaude:
		return "https://api.anthropic.com"
	case cgateway.ProviderGemini:
		return "https://generativelanguage.googleapis.com"
	case cgateway.ProviderPersonal:
		return "local://yamux" // 本地模型走反向隧道,不需要真实 URL
	default:
		return ""
	}
}

// modelToEndpoints 把单个 ModelInfo 转成一组 scheduler.Endpoint（每个 APIKey 一个）。
// modelID 是合约里的 uint64 model id，用于计费。
func modelToEndpoints(modelID uint64, model *cgateway.ModelInfo, provider *cgateway.ProviderInfo) []*Endpoint {
	var endpoints []*Endpoint

	// Provider 是 provider 节点名（如 "node-7f3a"），Protocol 是协议类型（如 "openai"/"anthropic"）
	providerName := provider.Name
	if providerName == "" {
		providerName = fmt.Sprintf("provider-%x", provider.Owner)
	}
	protocol := providerTypeName(model.ProviderType)

	// model.URI 为空时回退到 provider 官方地址。
	baseURL := model.URI
	if baseURL == "" {
		baseURL = providerDefaultURL(model.ProviderType)
	}

	// 本地模型(个人算力)走 Yamux 反向隧道,不需要 APIKey,单端点即可。
	if model.ProviderType == cgateway.ProviderPersonal {
		return []*Endpoint{{
			Provider:           ProviderID(providerName),
			Protocol:           ProviderID(protocol),
			KeyID:              fmt.Sprintf("%s-local", model.ModelID),
			KeyValue:           "",
			BaseURL:            baseURL,
			Weight:             1.0,
			Enabled:            model.IsActive,
			Models:             []ModelKey{ModelKey(model.Name), ModelKey(model.ModelID)},
			Concurrency:        int(model.MaxConcurrency),
			ModelID:            modelID,
			ContractProviderID: model.ProviderID,
			ModelName:          model.Name,
			ProviderType:       model.ProviderType,
		}}
	}

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
			// 仅按模型 active 启用;不用 model.Balance 作硬门槛
			// (余额/积分在 DistributeMiddleware 按用户校验,不在端点层拦截)。
			Enabled:     model.IsActive,
			Models:      []ModelKey{ModelKey(model.Name), ModelKey(model.ModelID)},
			Concurrency: int(model.MaxConcurrency),
			// 计费 / 上游调用身份
			ModelID:            modelID,
			ContractProviderID: model.ProviderID,
			ModelName:          model.Name,
			ProviderType:       model.ProviderType,
		})
	}
	return endpoints
}

// Compile-time guard
var _ sync.Mutex
