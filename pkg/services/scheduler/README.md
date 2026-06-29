# Scheduler — LLM 路由调度器

> 高性能 LLM API 网关调度器。几百个逻辑模型 × 几千个 `(provider, apikey)` 端点。
> 设计参考 [maximhq/bifrost](https://github.com/maximhq/bifrost) 的调度思想,
> 适配**同一模型多供应商**的路由场景。

## 核心特性

| 特性 | 说明 |
|---|---|
| **多模型 × 多供应商** | 同一个 `gpt-4o` 可以注册到 OpenAI、Azure、OpenRouter,自动加权路由 |
| **加权 + 实时评分** | `score = weight × success_rate × load_factor / latency_factor` |
| **per-model 并发上限** | 用 FIFO 等待队列的信号量,不是粗暴的 buffered channel |
| **per-endpoint 熔断** | 滑动窗口错误率 + 半开探测(Closed / Open / HalfOpen) |
| **跨模型回退** | `FallbackModels` 链,主模型全部端点失败后试备用模型 |
| **错误分类 + 死 key** | 401/402/403 → deadKeyIDs(永不复用);429 → usedKeyIDs(周期复用) |
| **Streaming 友好** | TTFT 自动埋点;握手期失败可重试,握手后只能透传 |
| **OpenTelemetry 原生** | 复用 `gen_ai.*` 语义约定 + 自定义 `sched.*` 命名空间 |
| **路由表热更新** | `atomic.Pointer` 无锁快照替换,几千端点的查表 O(1) |

## 文件结构

```
pkg/scheduler/
├── types.go           # ModelKey / ProviderID / Endpoint / ModelRoute / Registry
├── errors.go          # ErrorClass + UpstreamError + Classify()
├── concurrency.go     # ModelSemaphore + CircuitBreaker + BreakerSet
├── selector.go        # computeScore() + 加权随机选端点
├── observability.go   # OTel Tracer + Meter 封装
├── streaming.go       # Stream / StreamChunk + instrumentedStream
├── scheduler.go       # Scheduler 主调度 + 重试编排
├── time.go            # 包级时间源(便于测试替换)
└── scheduler_test.go  # 单元测试 + 大规模压测
```

## 使用方法

### 1. 定义 ProviderClient

实现 `ProviderClient` 接口,封装对真实 LLM API 的 HTTP 调用:

```go
type ProviderClient interface {
    Do(ctx context.Context, ep *Endpoint, body []byte) ([]byte, error)
    Stream(ctx context.Context, ep *Endpoint, body []byte) (Stream, error)
}
```

错误必须用 `UpstreamError{Status, Code, Message}` 类型返回,这样 Scheduler 才能分类。

### 2. 构建 Registry

```go
reg := scheduler.NewRegistry()
reg.Replace([]*scheduler.ModelRoute{
    {
        Model:          "gpt-4o",
        MaxConcurrency: 100,    // 模型级总并发上限
        Endpoints: []*scheduler.Endpoint{
            {Provider: "openai",  KeyID: "team-a-1", KeyValue: "sk-...", Weight: 1.0, Enabled: true, Models: []scheduler.ModelKey{"*"}, Concurrency: 50},
            {Provider: "openai",  KeyID: "team-a-2", KeyValue: "sk-...", Weight: 1.0, Enabled: true, Models: []scheduler.ModelKey{"*"}, Concurrency: 50},
            {Provider: "azure",   KeyID: "azure-1",   KeyValue: "...",    Weight: 1.0, Enabled: true, Models: []scheduler.ModelKey{"*"}, Concurrency: 30, BaseURL: "https://my.openai.azure.com"},
            {Provider: "openrouter", KeyID: "or-1",   KeyValue: "sk-...", Weight: 1.0, Enabled: true, Models: []scheduler.ModelKey{"*"}, Concurrency: 20},
        },
        FallbackModels: []scheduler.ModelKey{"claude-3-5-sonnet"},  // 跨模型回退
    },
    {
        Model: "claude-3-5-sonnet",
        Endpoints: []*scheduler.Endpoint{
            {Provider: "anthropic", KeyID: "ant-1", KeyValue: "sk-ant-...", Weight: 1.0, Enabled: true, Models: []scheduler.ModelKey{"*"}, Concurrency: 50},
        },
    },
})
```

### 3. 启动 Scheduler

```go
obs, _ := scheduler.NewObservability("my-ai-gateway")  // OTel 已通过 otel.SetTracerProvider 全局设置
sch := scheduler.New(scheduler.DefaultConfig(), reg, obs)
sch.AttachMetrics()                                     // 给每个 Endpoint 注入 *EndpointMetrics
sch.RegisterProvider("openai", &myOpenAIClient{})
sch.RegisterProvider("azure", &myAzureClient{})
sch.RegisterProvider("openrouter", &myOpenRouterClient{})
sch.Start(ctx)                                          // 启动熔断 ticker
```

### 4. 调用

```go
// 非流式
res, err := sch.Schedule(ctx, scheduler.Invoke{
    Model: "gpt-4o",
    Body:  []byte(`{"messages":[{"role":"user","content":"hi"}]}`),
})
fmt.Printf("via %s/%s, attempts=%d, score=%.2f\n",
    res.Meta.Provider, res.Meta.KeyID, res.Meta.Attempts, res.Meta.Score)

// 流式
res, err := sch.ScheduleStream(ctx, scheduler.Invoke{
    Model:  "gpt-4o",
    Body:   []byte(`{"stream":true,...}`),
    Stream: true,
})
for {
    chunk, err := res.Stream.Next(ctx)
    if chunk == nil || chunk.IsFinal { break }
    fmt.Printf("chunk %d: %s\n", chunk.Index, chunk.Data)
}
```

## 错误处理

所有错误都是 `*SchedError`,可用 `errors.Is(err, ...)` 判断:

```go
switch {
case errors.Is(err, scheduler.ErrNoHealthyEndpoint):
    // 全部端点被熔断 / 打满
case errors.Is(err, scheduler.ErrAllEndpointsDead):
    // 所有端点的凭证都死了(401/402/403)
case errors.Is(err, scheduler.ErrModelNotFound):
    // 模型未配置
case errors.Is(err, scheduler.ErrCanceled):
    // ctx 取消
default:
    var se *scheduler.SchedError
    if errors.As(err, &se) {
        switch se.Class {
        case scheduler.ClassAuth:
            // 凭证死(单个 key)
        case scheduler.ClassRateLimit:
            // 限速
        case scheduler.ClassTransient:
            // 上游 5xx
        case scheduler.ClassClient:
            // 4xx(请求本身错)
        }
    }
}
```

## OpenTelemetry

`NewObservability("service-name")` 依赖全局 OTel providers:

```go
// 启动时
tp := otel.GetTracerProvider()        // 或自定义: sdktrace.NewTracerProvider(...)
mp := otel.MeterProvider()             // 或自定义: sdkmetric.NewMeterProvider(...)
otel.SetTracerProvider(tp)
otel.SetMeterProvider(mp)

obs, _ := scheduler.NewObservability("my-ai-gateway")
```

### 自动埋点的指标

| 指标 | 类型 | 标签 |
|---|---|---|
| `sched.requests.total` | Counter | `gen_ai.provider.name`, `gen_ai.request.model`, `sched.endpoint.key_id`, `sched.error.class` |
| `sched.requests.success` | Counter | 同上 |
| `sched.requests.error` | Counter | 同上 |
| `sched.requests.duration` | Histogram (ms) | 同上 |
| `sched.retries.total` | Counter | `gen_ai.provider.name` |
| `sched.fallbacks.total` | Counter | `gen_ai.request.model`, `sched.attempt` |
| `sched.circuit.opens.total` | Counter | `gen_ai.provider.name`, `sched.endpoint.key_id` |
| `sched.stream.ttft` | Histogram (ms) | `gen_ai.provider.name`, `gen_ai.request.model` |

### 自动埋点的 Span

- `sched.endpoint.call` (Client kind) — 每个端点调用,含 `gen_ai.*` 和 `sched.*` 属性

## 性能预期

基于 `TestScale_ThousandsOfEndpoints`(100 模型 × 50 端点,共 5000 端点):

- 单次调度延迟 **< 100 µs**(走完评分 + 选择 + 信号量)
- 1000 次调度总耗时 **< 100 ms**
- 端点查询 O(1) 哈希,无锁读
- atomic 计数器无 contention(数千端点时)

## 测试

```bash
# 跑所有测试(包括大规模压测)
go test ./pkg/scheduler/ -v

# 跑短测试(跳过压测)
go test ./pkg/scheduler/ -short

# 只跑特定测试
go test ./pkg/scheduler/ -v -run TestSchedule_RetryOn429

# 看性能
go test ./pkg/scheduler/ -v -run TestScale
```