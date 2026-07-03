package scheduler

import (
	"context"
	"fmt"
)

// GatewayScheduler 把 Scheduler 封装成 TEE 节点的一个服务。
// 提供:
//   - ProviderClient 注册(由 gateway_relay 直接实现)
//   - Schedule / ScheduleStream 给 HandleRelay 调用
type GatewayScheduler struct {
	scheduler *Scheduler
	resolver  ModelResolver
	log       Logger
}

// NewGatewayScheduler 创建服务。
func NewGatewayScheduler(resolver ModelResolver) (*GatewayScheduler, error) {
	obs, err := NewObservability("tee-gateway")
	if err != nil {
		return nil, fmt.Errorf("create observability: %w", err)
	}
	sch := New(DefaultConfig(), resolver, obs)
	logger := sch.log
	gw := &GatewayScheduler{
		scheduler: sch,
		resolver:  resolver,
		log:       logger,
	}
	logger.Info("gateway scheduler initialized")
	return gw, nil
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

// Start 启动后台熔断评估。
func (g *GatewayScheduler) Start(ctx context.Context) {
	g.log.Info("starting gateway scheduler")
	g.scheduler.Start(ctx)
}

// OnIndexRefreshed 在 indexer 完成一次全量刷新后调用，清空路由缓存。
func (g *GatewayScheduler) OnIndexRefreshed() {
	g.scheduler.routeCache.Clear()
	g.log.Info("route cache cleared after index refresh")
}

// Schedule 非流式入口。
func (g *GatewayScheduler) Schedule(ctx context.Context, provider string, model string, body []byte, sessionID string) (*Result, error) {
	g.log.Debug("schedule request", "provider", provider, "model", model, "body_len", len(body), "stream", false, "session", sessionID)
	return g.scheduler.Schedule(
		ctx,
		Invoke{
			Model:     ModelKey(model),
			Provider:  ProviderID(provider),
			SessionID: sessionID,
			Body:      body,
			Stream:    false,
		},
	)
}

// ScheduleStream 流式入口。
func (g *GatewayScheduler) ScheduleStream(ctx context.Context, provider string, model string, body []byte, sessionID string) (*Result, error) {
	g.log.Debug("schedule request", "provider", provider, "model", model, "body_len", len(body), "stream", true, "session", sessionID)
	return g.scheduler.ScheduleStream(
		ctx,
		Invoke{
			Model:     ModelKey(model),
			Provider:  ProviderID(provider),
			SessionID: sessionID,
			Body:      body,
			Stream:    true,
		},
	)
}
