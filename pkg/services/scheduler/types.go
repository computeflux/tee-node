// Package scheduler 实现一个 LLM 路由调度器:
//
//   - 几百个逻辑模型,几千个 (provider, apikey) 端点
//   - 加权 + 实时评分的端点选择
//   - per-model 并发上限 + per-endpoint 熔断
//   - 跨端点 / 跨模型回退
//   - streaming 一致性保证
//   - OpenTelemetry 原生 tracing + metrics
//
// 设计参考 maximhq/bifrost 的调度思想,适配"同一模型多供应商"的路由场景。
package scheduler

import (
	"sort"
	"sync/atomic"
)

// ModelKey 是逻辑模型名(如 "gpt-4o"、"claude-3-5-sonnet-20241022")。
// 它是客户端面向的标识,与供应商的"实际 model id"解耦。
type ModelKey string

// ProviderID 是供应商标识(如 "openai"、"azure-eastus"、"openrouter")。
// 它只是路由分类,不影响 HTTP 调用细节。
type ProviderID string

// EndpointKey 在 registry 内全局唯一。
type EndpointKey struct {
	Provider ProviderID
	KeyID    string
}

func (k EndpointKey) String() string { return string(k.Provider) + "/" + k.KeyID }

// Endpoint = 一个 (供应商, API key) 对。
// 同一供应商可以有多个 Endpoint(多个 key),分别参与负载均衡。
type Endpoint struct {
	Provider ProviderID `json:"provider"`  // provider 节点名（如 "node-7f3a"）
	Protocol ProviderID `json:"protocol"` // 协议类型（如 "openai"/"anthropic"/"gemini"）
	KeyID    string     `json:"key_id"`
	KeyValue string     `json:"key_value"`
	BaseURL  string     `json:"base_url,omitempty"`
	Weight   float64    `json:"weight"`
	Enabled  bool       `json:"enabled"`

	// Models 限定该端点能服务的模型列表(白名单)。
	// 空切片 = deny-all;"*" 通配 = allow-all。
	Models []ModelKey `json:"models"`

	// Concurrency 是端点级并发上限(0 表示不限)。
	// 与 ModelRoute.MaxConcurrency 是两层独立的信号量。
	Concurrency int `json:"concurrency,omitempty"`

	// ExtraHeaders 是每次请求都要带的 header(如 OpenAI 的 Organization)。
	ExtraHeaders map[string]string `json:"extra_headers,omitempty"`

	// metrics 是端点的实时指标(由 Scheduler 初始化时注入)。
	// 不导出 JSON,因为它是运行时状态。
	metrics *EndpointMetrics
}

// EffectiveScore 返回端点的当前评分,供 selector 使用。
func (e *Endpoint) EffectiveScore(coldStartThreshold int) float64 {
	return computeScore(e.Weight, e.metrics, coldStartThreshold)
}

// InFlight 当前正在处理的请求数。
func (e *Endpoint) InFlight() int64 { return e.metrics.InFlight.Load() }

// ModelRoute 是某个逻辑模型的完整路由配置。
type ModelRoute struct {
	Model          ModelKey
	MaxConcurrency int         // 模型级总并发上限(0 = 不限)
	Endpoints      []*Endpoint // 候选端点
	FallbackModels []ModelKey  // 跨模型回退链(本模型所有端点失败时尝试)
}

// EndpointMetrics 是单个端点的实时运行时指标。
// 所有字段都是原子的,scheduler 在请求进入/退出时更新。
type EndpointMetrics struct {
	InFlight          atomic.Int64
	TotalRequests     atomic.Int64
	SuccessCount      atomic.Int64
	ErrorCount429     atomic.Int64 // 限速
	ErrorCount5xx     atomic.Int64 // 上游故障
	ErrorCountAuth    atomic.Int64 // 401/402/403(凭证死)
	ErrorCountClient  atomic.Int64 // 其他 4xx
	EMALatencyMicros  atomic.Int64 // 指数移动平均(微秒)
	LastErrorAtUnix   atomic.Int64
	LastSuccessAtUnix atomic.Int64
	CircuitOpen       atomic.Bool
}

// MetricsSnapshot 取一份一致性快照(用于 metrics 导出 / 调试)。
func (m *EndpointMetrics) Snapshot() MetricsSnapshot {
	return MetricsSnapshot{
		InFlight:       m.InFlight.Load(),
		TotalRequests:  m.TotalRequests.Load(),
		SuccessCount:   m.SuccessCount.Load(),
		ErrorCount429:  m.ErrorCount429.Load(),
		ErrorCount5xx:  m.ErrorCount5xx.Load(),
		ErrorCountAuth: m.ErrorCountAuth.Load(),
	}
}

// MetricsSnapshot 是 EndpointMetrics 的非原子快照。
type MetricsSnapshot struct {
	InFlight       int64
	TotalRequests  int64
	SuccessCount   int64
	ErrorCount429  int64
	ErrorCount5xx  int64
	ErrorCountAuth int64
}

// routesSnapshot 是 registry 内部不可变快照。
type routesSnapshot struct {
	byModel    map[ModelKey]*ModelRoute
	byEndpoint map[EndpointKey]*Endpoint
	allModels  []ModelKey
}

// Registry 维护一张"模型 → 端点池"的路由表。
// 使用 atomic.Pointer 做无锁热更新:Replace 后所有新请求立即看到新表,
// 老请求仍持有旧表指针,直到它们完成。
type Registry struct {
	snap atomic.Pointer[routesSnapshot]
}

// NewRegistry 构造一个空 Registry。
func NewRegistry() *Registry { return &Registry{} }

// Replace 原子地替换整个路由表。
// 内部会按 Weight 降序排序端点(让选择结果更稳定)。
func (r *Registry) Replace(routes []*ModelRoute) {
	snap := &routesSnapshot{
		byModel:    make(map[ModelKey]*ModelRoute, len(routes)),
		byEndpoint: make(map[EndpointKey]*Endpoint),
	}
	for _, route := range routes {
		// 按 weight 降序排序,优先尝试高分端点(短路)
		sort.SliceStable(route.Endpoints, func(i, j int) bool {
			return route.Endpoints[i].Weight > route.Endpoints[j].Weight
		})
		snap.byModel[route.Model] = route
		snap.allModels = append(snap.allModels, route.Model)
		for _, ep := range route.Endpoints {
			if ep.metrics == nil {
				ep.metrics = &EndpointMetrics{}
			}
			snap.byEndpoint[EndpointKey{Provider: ep.Provider, KeyID: ep.KeyID}] = ep
		}
	}
	sort.Slice(snap.allModels, func(i, j int) bool {
		return string(snap.allModels[i]) < string(snap.allModels[j])
	})
	r.snap.Store(snap)
}

// LookupModel 取出某模型的路由配置(只读快照)。
// 返回 nil 表示该模型未配置。
func (r *Registry) LookupModel(m ModelKey) *ModelRoute {
	snap := r.snap.Load()
	if snap == nil {
		return nil
	}
	return snap.byModel[m]
}

// LookupEndpoint 通过 (provider, keyID) 找端点(用于 metrics 导出)。
func (r *Registry) LookupEndpoint(k EndpointKey) *Endpoint {
	snap := r.snap.Load()
	if snap == nil {
		return nil
	}
	return snap.byEndpoint[k]
}

// AllEndpoints 返回所有端点的快照(用于 metrics 导出 / 后台任务)。
func (r *Registry) AllEndpoints() []*Endpoint {
	snap := r.snap.Load()
	if snap == nil {
		return nil
	}
	out := make([]*Endpoint, 0, len(snap.byEndpoint))
	for _, ep := range snap.byEndpoint {
		out = append(out, ep)
	}
	return out
}

// AllModels 返回已注册的全部模型(已排序)。
func (r *Registry) AllModels() []ModelKey {
	snap := r.snap.Load()
	if snap == nil {
		return nil
	}
	out := make([]ModelKey, len(snap.allModels))
	copy(out, snap.allModels)
	return out
}
