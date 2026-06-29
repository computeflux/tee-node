# Scheduler 架构文档

## 概述

`scheduler` 是一个为 TEE 节点设计的 LLM 路由调度器，负责将客户端请求（按**逻辑模型名**）路由到最优的**供应商端点**（Provider + API Key）。

核心能力：
- 支持几百个逻辑模型、几千个 (provider, apikey) 端点
- 加权 + 实时评分的端点选择
- per-model 并发上限 + per-endpoint 熔断
- 跨端点 / 跨模型回退（Fallback）
- 流式（SSE）一致性保证
- OpenTelemetry 原生 tracing + metrics
- 从链上合约无锁热更新路由表

设计参考 [maximhq/bifrost](https://github.com/maximhq/bifrost) 的调度思想，适配"同一模型多供应商"的去中心化路由场景。

---

## 文件职责

| 文件 | 职责 |
|------|------|
| **`types.go`** | 核心数据结构：`ModelKey`、`ProviderID`、`Endpoint`、`ModelRoute`、`EndpointMetrics`、`Registry`。`Registry` 用 `atomic.Pointer` 实现无锁热更新。 |
| **`scheduler.go`** | 调度器主引擎：`Scheduler` 结构体、`Schedule`/`ScheduleStream` 入口、`tryOneModel` 重试逻辑、指数退避、错误分类处理。 |
| **`service.go`** | 服务封装层：`GatewayScheduler` 把 `Scheduler` 包装成 TEE 节点服务，提供 `ReloadRegistry`（批量刷新 + 自动清理旧缓存）、`RegisterProvider`/`ReplaceClients`。 |
| **`selector.go`** | 端点选择算法：`Selector` 按加权随机选择端点，`computeScore` 综合评分（成功率 × 负载 × 延迟）。含 `sync.Pool` 复用候选 slice。 |
| **`concurrency.go`** | 并发控制 + 熔断：`ModelSemaphore`（per-model FIFO 信号量）、`Breaker`/`BreakerSet`（三态熔断器：Closed→Open→HalfOpen）、`SemaphoreSet`（信号量容器）。 |
| **`errors.go`** | 错误分类体系：`ErrorClass`（Transient/RateLimit/Auth/Client/Cancel）、`Classify` 函数、`SchedError`、`UpstreamError`。零依赖 `strings` 包。 |
| **`observability.go`** | OpenTelemetry 集成：`Observability` 管理 tracer + meter，提供 span 创建、metrics 记录（请求数、延迟、TTFT、fallback、熔断触发）。 |
| **`streaming.go`** | 流式抽象：`Stream` 接口、`StreamChunk`、`instrumentedStream`（包装层，记录 TTFT）。 |
| **`adaptor_bridge.go`** | 与现有 `gateway_relay` 的桥接：`AdaptorClient` 把 `relay.Adaptor` 包装成 `ProviderClient`，`sseStream` 解析 SSE 响应。 |
| **`reloader.go`** | 链上合约同步：`Reloader` 定期从 `teechain` 合约读取 `ModelRoute`/`ModelInfo`/`ProviderInfo`，构建 `scheduler.ModelRoute`，调用 `ReloadRegistry` 批量刷新。 |
| **`route_cache.go`** | 辅助缓存：`routeIDToName`（`sync.Map`），用于 `DistributeMiddleware` 把 route ID 映射到 scheduler 键。 |
| **`global.go`** | 全局单例：`SetGlobal`/`GetGlobal`/`ResetGlobal`，供 `DistributeMiddleware`/`HandleRelay` 在不注入依赖时获取 scheduler 实例。 |
| **`time.go`** | 时间源抽象：`timeNowUnixNano` 变量，方便测试时替换。 |
| **`openai_client_example.go`** | 示例代码（`//go:build ignore`）：展示如何自己实现 `ProviderClient` 和 `Stream`。 |
| **`scheduler_test.go`** | 测试覆盖：happy path、429 重试、401 dead key、fallback model、stream、并发信号量、熔断器、registry 热替换、评分算法、错误分类、大规模压测。 |

---

## 整体架构

```
┌──────────────────────────────────────────────────────────────────────┐
│                         Client Request                                │
│  POST /v1/chat/completions  model=gpt-4o  stream=true/false          │
└──────────────────────────────────────────────────────────────────────┘
                                    │
                                    ▼
┌──────────────────────────────────────────────────────────────────────┐
│                    GatewayScheduler (service.go)                      │
│  ┌──────────────┐  ┌──────────────┐  ┌────────────────────────────┐  │
│  │  Schedule()  │  │ScheduleStream│  │      ReloadRegistry()      │  │
│  │  (非流式)    │  │   (流式)     │  │  从合约批量刷新 + 清理缓存   │  │
│  └──────┬───────┘  └──────┬───────┘  └────────────────────────────┘  │
│         │                 │                                           │
│         └────────┬────────┘                                           │
│                  ▼                                                    │
│  ┌────────────────────────────────────────────────────────────────┐  │
│  │              Scheduler (scheduler.go) — 调度引擎                │  │
│  │  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐              │  │
│  │  │   Registry  │  │  Selector   │  │ BreakerSet  │              │  │
│  │  │ (atomic.Ptr)│  │ (加权随机)  │  │ (per-ep熔断)│              │  │
│  │  └─────────────┘  └─────────────┘  └─────────────┘              │  │
│  │  ┌─────────────┐  ┌─────────────┐  ┌─────────────────────────┐  │  │
│  │  │SemaphoreSet │  │  clients    │  │     Observability       │  │  │
│  │  │(per-model)  │  │(Protocol→   │  │  (OTel tracing/metrics) │  │  │
│  │  └─────────────┘  │ ProviderClient)│  └─────────────────────────┘  │  │
│  │                   └─────────────┘                               │  │
│  └────────────────────────────────────────────────────────────────┘  │
└──────────────────────────────────────────────────────────────────────┘
                                    │
         ┌──────────────────────────┼──────────────────────────┐
         │                          │                          │
         ▼                          ▼                          ▼
┌──────────────┐           ┌──────────────┐           ┌──────────────┐
│   openai     │           │  anthropic   │           │    gemini    │
│  Adaptor     │           │   Adaptor    │           │   Adaptor    │
│  (protocol)  │           │  (protocol)  │           │  (protocol)  │
└──────────────┘           └──────────────┘           └──────────────┘
```

---

## 核心数据流

### 1. 请求调度流

```
Client ──► GatewayScheduler.Schedule/ScheduleStream
              │
              ▼
          Scheduler.schedule
              │
              ├──► Registry.LookupModel(model) → ModelRoute
              │         │
              │         ▼
              │    SemaphoreSet.GetOrCreate(model) → Acquire
              │         │
              │         ▼
              │    遍历 models = [主模型] + [FallbackModels]
              │         │
              │         ▼
              │    tryOneModel
              │         │
              │         ├──► Selector.Select → 加权随机选 Endpoint
              │         │         │
              │         │         ├──► modelAllowed(Models白名单)
              │         │         ├──► ep.Concurrency 检查
              │         │         └──► computeScore(Weight × SuccessRate × LoadFactor / LatencyFactor)
              │         │
              │         ├──► BreakerSet.GetOrCreate(EndpointKey).Allow()
              │         │
              │         ├──► getClient(ep.Protocol) → ProviderClient
              │         │         │
              │         │         ▼
              │         │    client.Do/Stream(ctx, ep, body) → HTTP 请求
              │         │         │
              │         │         ▼
              │         │    成功: updateEMA, breaker.RecordSuccess, RecordRequest
              │         │    失败: Classify(err) → 错误策略
              │         │              │
              │         │              ├──► ClassAuth: deadKeys[ep.KeyID]=true, 继续
              │         │              ├──► ClassRateLimit: usedKeys[ep.KeyID]=true, backoff, 继续
              │         │              ├──► ClassTransient: backoff, 继续
              │         │              └──► ClassClient/ClassCancel: 直接返回
              │         │
              │         └──► 重试直到 MaxRetries 或成功
              │
              └──► 返回 Result{Body/Stream, Meta}
```

### 2. 路由表热更新流

```
Reloader.Run (后台 ticker, 每 30s)
    │
    ├──► getBridge() → TeeChain
    │
    ├──► GatewayQuery.ListRoutes() → []ModelRouteWithID
    │
    ├──► 遍历 routes:
    │       ├──► GatewayQuery.GetModel(modelID) → ModelInfo
    │       ├──► GatewayQuery.GetProvider(providerID) → ProviderInfo
    │       └──► modelToEndpoints(modelInfo, providerInfo) → []Endpoint
    │               │
    │               └──► Endpoint{Provider: providerName, Protocol: protocolType, ...}
    │
    ├──► GatewayScheduler.ReloadRegistry(schedRoutes)
    │         │
    │         ├──► Registry.Replace(routes) ──atomic.Pointer──► 新 snapshot
    │         │         │
    │         │         └──► 旧 snapshot 仍被老请求引用 → GC 安全回收
    │         │
    │         ├──► BreakerSet.Cleanup(keepEndpoints) → 删除旧端点的 breaker
    │         └──► SemaphoreSet.Cleanup(keepModels) → 删除旧 model 的信号量
    │
    └──► SetRouteNameMapping(routeID, routeName) → 供 Middleware 用
```

**关键：ReloadRegistry 全程不阻塞正在执行的请求。**

---

## 关键设计决策

### 1. Provider vs Protocol 解耦

```
Endpoint.Provider = provider 节点名（如 "node-7f3a"）→ 用于统计/计费
Endpoint.Protocol = 协议类型（如 "openai"/"anthropic"/"gemini"）→ 用于选 adaptor
```

同一个 provider 节点可以同时提供多种协议的 API。Adaptor 只按协议注册（固定几种），不随 provider 节点增减而变化。

### 2. 无锁热更新（Copy-on-Write）

`Registry` 内部用 `atomic.Pointer[routesSnapshot]`：
- **读取**：`atomic.Load()` 一个 CPU 指令，无锁
- **写入**：创建全新的 `routesSnapshot`，原子替换指针
- **旧数据**：被老请求持有的指针保护，GC 自动回收

### 3. 三层并发控制

| 层级 | 机制 | 说明 |
|------|------|------|
| **模型级** | `ModelSemaphore` | 限制单个模型的全局并发数（如 gpt-4o 同时最多 100 个请求） |
| **端点级** | `Endpoint.Concurrency` | 限制单个 (provider, key) 的并发数（如某 key 同时最多 50 个请求） |
| **熔断器** | `Breaker` | 错误率超过阈值时自动熔断，半开后探测恢复 |

### 4. 端点评分算法

```
score = weight × success_rate × load_factor / latency_factor

- weight:        静态配置权重
- success_rate:  最近窗口成功率（冷启动期只 weight）
- load_factor:   1 / (1 + inFlight × 0.1)  → 已用并发惩罚
- latency_factor: 1 + EMA_latency / 1s      → 延迟惩罚
```

熔断时 `score = 0`，被 selector 自动跳过。

### 5. 错误分类策略

| 错误类型 | 示例 | 策略 |
|----------|------|------|
| **ClassAuth** | 401/402/403 | 标记 dead key，永不回访，换端点 |
| **ClassRateLimit** | 429 | 标记 used key，退避后换端点（窗口滑过可复用） |
| **ClassTransient** | 5xx/网络错误 | 同端点退避后重试 |
| **ClassClient** | 400/422 | 直接返回，不重试 |
| **ClassCancel** | context.Canceled | 直接返回 |

### 6. 流式一致性

`instrumentedStream` 包装原始 `Stream`：
- 记录首个 chunk 的 **TTFT**（Time To First Token）
- 提供可靠的 `ctx` 取消传播
- `Close()` 幂等，确保资源释放

---

## 典型使用场景

### 场景 A：启动时注册固定协议

```go
gw, _ := scheduler.NewGatewayScheduler()

// 注册所有协议 adaptor（代码实现，固定不变）
gw.RegisterProviders(map[string]relay.Adaptor{
    "openai":    relay.GetAdaptorForProviderTypeCode(cgateway.ProviderOpenAI),
    "anthropic": relay.GetAdaptorForProviderTypeCode(cgateway.ProviderClaude),
    "gemini":    relay.GetAdaptorForProviderTypeCode(cgateway.ProviderGemini),
})

gw.Start(ctx)
```

### 场景 B：从合约批量刷新（自动执行）

```go
scheduler.SetBridgeGetter(func() *teechain.TeeChain { return myChain })

reloader := scheduler.NewReloader(gw, 30*time.Second)
go reloader.Run(ctx)  // 每 30 秒自动从合约读取并全量替换
```

### 场景 C：HTTP Handler 中调用

```go
func HandleRequest(w http.ResponseWriter, r *http.Request) {
    gw := scheduler.GetGlobal()
    body, _ := io.ReadAll(r.Body)
    
    if isStream {
        res, _ := gw.ScheduleStream(r.Context(), model, body)
        for {
            chunk, _ := res.Stream.Next(r.Context())
            if chunk.IsFinal { break }
            fmt.Fprintf(w, "data: %s\n\n", chunk.Data)
        }
    } else {
        res, _ := gw.Schedule(r.Context(), model, body)
        w.Write(res.Body)
    }
}
```

---

## 性能基准

测试环境：`go test -run TestScale`（100 模型 × 50 端点 = 5000 端点）

| 指标 | 结果 |
|------|------|
| 路由表替换 | ~5ms（5000 端点） |
| 单次调度平均耗时 | ~11µs |
| 内存 | 旧 snapshot 等 GC 回收，峰值 ~2× |
| 并发安全 | 零锁竞争（atomic.Pointer + 独立锁分区） |

---

## 扩展点

| 扩展 | 方式 |
|------|------|
| 新增协议 | 实现 `ProviderClient` + `Stream` 接口，注册到 `clients` |
| 自定义评分 | 修改 `selector.go` 的 `computeScore` 函数 |
| 自定义熔断策略 | 修改 `BreakerConfig` 参数或重写 `Breaker.Tick` |
| 自定义观测 | 替换 `Observability` 实现或修改 `NewObservability` |
| 合约字段扩展 | 修改 `reloader.go` 的 `modelToEndpoints` 映射逻辑 |

---

## 文件依赖图

```
types.go ───┬──► scheduler.go ──► service.go
            │         │              │
            │         ├──► selector.go│
            │         ├──► concurrency.go│
            │         ├──► errors.go   │
            │         ├──► observability.go│
            │         ├──► streaming.go│
            │         └──► adaptor_bridge.go
            │                        │
            └──► reloader.go ◄───────┘
                     │
                     ├──► route_cache.go
                     └──► global.go

(辅助)
time.go ──► selector.go (int64Fast)
```
