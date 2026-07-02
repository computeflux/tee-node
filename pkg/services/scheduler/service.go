package scheduler

import (
	"context"
	"fmt"
)

// GatewayScheduler 把 Scheduler 封装成 TEE 节点的一个服务。
// 提供:
//   - ProviderClient 注册(由 gateway_relay 直接实现)
//   - 路由表热更新
//   - Schedule / ScheduleStream 给 HandleRelay 调用
type GatewayScheduler struct {
	scheduler *Scheduler
	registry  *Registry
	log       Logger
}

// NewGatewayScheduler 创建服务。
func NewGatewayScheduler() (*GatewayScheduler, error) {
	obs, err := NewObservability("tee-gateway")
	if err != nil {
		return nil, fmt.Errorf("create observability: %w", err)
	}
	reg := NewRegistry()
	sch := New(DefaultConfig(), reg, obs)
	logger := sch.log
	gw := &GatewayScheduler{
		scheduler: sch,
		registry:  reg,
		log:       logger,
	}
	logger.Info("gateway scheduler initialized")
	return gw, nil
}

// SetLog 设置日志实现（可覆盖默认的 noop）。
func (g *GatewayScheduler) SetLog(l Logger) {
	if l == nil {
		l = noopLogger{}
	}
	g.log = l
	g.scheduler.cfg.Log = l
}

// RegisterProvider 把一个 ProviderClient 注册成 scheduler 的 provider。
// 调用示例:
//
//	gw.RegisterProvider("openai", relay.GetProviderClient(cgateway.ProviderOpenAI))
func (g *GatewayScheduler) RegisterProvider(name string, c ProviderClient) {
	g.log.Info("registering provider", "name", name)
	g.scheduler.RegisterProvider(ProviderID(name), c)
}

// RegisterProviders 批量注册 provider。
func (g *GatewayScheduler) RegisterProviders(providers map[string]ProviderClient) {
	g.log.Info("batch registering providers", "count", len(providers))
	batch := make(map[ProviderID]ProviderClient, len(providers))
	for name, c := range providers {
		batch[ProviderID(name)] = c
	}
	g.scheduler.RegisterClients(batch)
}

// ReplaceClients 全量替换 provider clients。
// 当合约新增了 provider 协议类型（或删除了旧类型）时调用。
func (g *GatewayScheduler) ReplaceClients(providers map[string]ProviderClient) {
	g.log.Info("replacing all provider clients", "count", len(providers))
	batch := make(map[ProviderID]ProviderClient, len(providers))
	for name, c := range providers {
		batch[ProviderID(name)] = c
	}
	g.scheduler.ReplaceClients(batch)
}

// Start 启动后台熔断评估。
func (g *GatewayScheduler) Start(ctx context.Context) {
	g.log.Info("starting gateway scheduler")
	g.scheduler.Start(ctx)
}

// ReloadRegistry 替换整个路由表，并清理不再需要的 breaker、semaphore。
// 这是从合约批量刷新时的统一入口，一次性完成全量替换。
func (g *GatewayScheduler) ReloadRegistry(routes []*ModelRoute) {
	// 1. 预计算新表中的有效 key 集合
	keepEndpoints := make(map[EndpointKey]bool, 64)
	keepModels := make(map[ModelKey]bool, len(routes))
	for _, route := range routes {
		keepModels[route.Model] = true
		for _, ep := range route.Endpoints {
			keepEndpoints[EndpointKey{Provider: ep.Provider, KeyID: ep.KeyID}] = true
		}
	}

	g.log.Info("reloading registry",
		"route_count", len(routes),
		"endpoint_count", len(keepEndpoints),
	)

	// 2. 原子替换路由表（无锁，不阻塞请求）
	g.registry.Replace(routes)
	g.log.Info("registry replaced atomically")

	// 3. 更新信号量容量（ ReloadRegistry 后 MaxConcurrency 可能变化）
	for _, route := range routes {
		g.scheduler.sems.UpdateCapacity(route.Model, route.MaxConcurrency)
	}

	// 4. 清理旧缓存（加锁，但操作很快，因为只是遍历 map）
	oldBreakerCount := len(g.scheduler.breakers.set)
	oldSemCount := len(g.scheduler.sems.cache)
	g.scheduler.breakers.Cleanup(keepEndpoints)
	g.scheduler.sems.Cleanup(keepModels)
	g.log.Info("cleanup completed",
		"breakers_before", oldBreakerCount,
		"breakers_after", len(g.scheduler.breakers.set),
		"sems_before", oldSemCount,
		"sems_after", len(g.scheduler.sems.cache),
	)
}

// Schedule 非流式入口。
func (g *GatewayScheduler) Schedule(ctx context.Context, model string, body []byte, sessionID string) (*Result, error) {
	g.log.Debug("schedule request", "model", model, "body_len", len(body), "stream", false, "session", sessionID)
	return g.scheduler.Schedule(
		ctx,
		Invoke{
			Model:     ModelKey(model),
			SessionID: sessionID,
			Body:      body,
			Stream:    false,
		},
	)
}

// ScheduleStream 流式入口。
func (g *GatewayScheduler) ScheduleStream(ctx context.Context, model string, body []byte, sessionID string) (*Result, error) {
	g.log.Debug("schedule request", "model", model, "body_len", len(body), "stream", true, "session", sessionID)
	return g.scheduler.ScheduleStream(
		ctx,
		Invoke{
			Model:     ModelKey(model),
			SessionID: sessionID,
			Body:      body,
			Stream:    true,
		},
	)
}

// Registry 暴露 Registry 用于 reloader 写、metrics 读。
func (g *GatewayScheduler) Registry() *Registry { return g.registry }
