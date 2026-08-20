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
	"context"
	"fmt"
	"sort"
	"sync/atomic"
)

// ModelKey 是逻辑模型名(如 "gpt-4o"、"claude-3-5-sonnet-20241022")。
// 它是客户端面向的标识,与供应商的"实际 model id"解耦。
type ModelKey string

// ProviderID 是供应商标识(如 "openai"、"azure-eastus"、"openrouter")。
// 它只是路由分类,不影响 HTTP 调用细节。
type ProviderID string

// EdgeProviderType 本地/个人算力模型的协议哨兵值(历史值 9999)。
// 新模型用 ModelType==EdgeRoute 标识本地模型,但旧的按协议分发的客户端注册/过滤仍需要该哨兵。
const EdgeProviderType uint16 = 9999

// EndpointKey 在 registry 内全局唯一。
type EndpointKey struct {
	Provider ProviderID
	KeyID    string
}

func (k EndpointKey) String() string { return string(k.Provider) + "/" + k.KeyID }

// Endpoint = 一个 (供应商, API key) 对。
// 同一供应商可以有多个 Endpoint(多个 key),分别参与负载均衡。
type Endpoint struct {
	Provider ProviderID `json:"provider"` // provider 节点名（如 "node-7f3a"）
	KeyID    string     `json:"key_id"`
	// KeyValue 是上游 API Key 明文，绝不应被默认 JSON 序列化带出。
	KeyValue string  `json:"-"`
	Weight   float64 `json:"weight"`
	Enabled  bool    `json:"enabled"`

	// Models 限定该端点能服务的模型列表(白名单)。
	// 空切片 = deny-all;"*" 通配 = allow-all。
	Models []ModelKey `json:"models"`

	// Concurrency 是端点级并发上限(0 表示不限)。
	// 与 ModelRoute.MaxConcurrency 是两层独立的信号量。
	Concurrency int `json:"concurrency,omitempty"`

	// ExtraHeaders 是每次请求都要带的 header(如 OpenAI 的 Organization)。
	ExtraHeaders map[string]string `json:"extra_headers,omitempty"`

	// ── 计费 / 上游调用身份（由 reloader 从合约盖章）──
	// 这些字段让 HandleRelay 在 scheduler 选完端点后,无需回查合约即可:
	//   - provider client 发上游请求(模型名 / provider id / provider type)
	//   - 上报 provider 指标(EmitProviderMetric)
	//   - 记录用量计费(recordUsageAsync 需要 ModelID + ContractProviderID)
	ModelID            uint64            `json:"model_id,omitempty"`             // 合约 model id(计费 key)
	ContractProviderID uint64            `json:"contract_provider_id,omitempty"` // 合约 provider id
	ModelName          string            `json:"model_name,omitempty"`           // 上游真实 model name
	ModelUrls          map[uint16]string `json:"provider_type,omitempty"`        // 合约 ProviderType(选 client / 转换用)

	// Route identity for route-based settlement.
	RouteID uint64 `json:"route_id,omitempty"`

	// metrics 是端点的实时指标(由 Scheduler 初始化时注入)。
	// 不导出 JSON,因为它是运行时状态。
	metrics *EndpointMetrics
}

// EffectiveScore 返回端点的当前评分,供 selector 使用。
func (e *Endpoint) EffectiveScore(coldStartThreshold int) float64 {
	return computeScore(e.Weight, e.metrics, coldStartThreshold)
}

// NativeProtocol 返回端点的原生协议(用于适配器选择/转换目标)。
// 端点支持的协议见 ModelUrls 的 key；多协议时按确定性的最小 key 选取。
func (e *Endpoint) NativeProtocol() uint16 {
	if len(e.ModelUrls) == 0 {
		return 0
	}
	// 确定性地选择最小的协议 key
	min := uint16(^uint16(0))
	for pt := range e.ModelUrls {
		if pt < min {
			min = pt
		}
	}
	return min
}

// SupportsProtocol 判断端点是否原生支持指定协议。
func (e *Endpoint) SupportsProtocol(pt uint16) bool {
	if pt == 0 {
		return false
	}
	_, ok := e.ModelUrls[pt]
	return ok
}

// ProtocolForClient 根据入口协议名(调度器 client 名,如 "openai"/"anthropic"/"openai-responses-api")，
// 返回该端点用于该入口请求的协议类型。
// 若端点原生支持该入口协议则返回对应类型(直接透传,无需转换)；否则回退到 NativeProtocol(最小 key)。
func (e *Endpoint) ProtocolForClient(inbound string) uint16 {
	for pt := range e.ModelUrls {
		if ClientTypeName(pt) == inbound {
			return pt
		}
	}
	return e.NativeProtocol()
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

// ModelResolver 把请求中的模型/路由名解析为可调度的 ModelRoute。
type ModelResolver interface {
	Resolve(ctx context.Context, model ModelKey, provider ProviderID) (*ModelRoute, error)
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

// EndpointMetricsSource 为 breaker ticker 提供运行时指标。
type EndpointMetricsSource interface {
	GetOrCreate(k EndpointKey) *EndpointMetrics
}

// routesSnapshot 是 registry 内部不可变快照。
type routesSnapshot struct {
	byModel map[ModelKey]*ModelRoute
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
		byModel: make(map[ModelKey]*ModelRoute, len(routes)),
	}
	for _, route := range routes {
		// 按 weight 降序排序,优先尝试高分端点(短路)
		sort.SliceStable(route.Endpoints, func(i, j int) bool {
			return route.Endpoints[i].Weight > route.Endpoints[j].Weight
		})
		snap.byModel[route.Model] = route
		for _, ep := range route.Endpoints {
			if ep.metrics == nil {
				ep.metrics = &EndpointMetrics{}
			}
		}
	}
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

// Resolve 实现 ModelResolver，供静态测试注入。
func (r *Registry) Resolve(ctx context.Context, model ModelKey, provider ProviderID) (*ModelRoute, error) {
	route := r.LookupModel(model)
	if route == nil {
		return nil, fmt.Errorf("%w: %s", ErrModelNotFound, model)
	}
	if provider == "" {
		return route, nil
	}
	// 显式指定 provider 时只保留该 provider 的端点。
	var filtered []*Endpoint
	for _, ep := range route.Endpoints {
		if ep.Provider == provider {
			filtered = append(filtered, ep)
		}
	}
	if len(filtered) == 0 {
		return nil, fmt.Errorf("%w: %s/%s", ErrModelNotFound, provider, model)
	}
	return &ModelRoute{
		Model:          route.Model,
		MaxConcurrency: route.MaxConcurrency,
		Endpoints:      filtered,
		FallbackModels: route.FallbackModels,
	}, nil
}
