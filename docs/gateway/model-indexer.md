# Gateway 模型索引器（Bleve 内存索引）

## 概述

`pkg/indexer` 是基于 [Bleve](https://github.com/blevesearch/bleve) 实现的合约数据内存索引框架。当前首要服务于 GraphQL `Models` 查询，解决以下问题：

- **高并发查询**：直接读链在 5000 并发下会成为瓶颈；内存索引将查询降到微秒/毫秒级。
- **模糊搜索**：链上合约方法不支持按模型名模糊匹配；Bleve 提供全文检索能力。
- **范围筛选**：支持 `ContextLength`、`InPrice` 等数值区间过滤。
- **可扩展性**：框架设计为通用，未来 `gov`、`batch` 等合约模块可复用同一套索引机制。

## 索引数据流

```
+---------+     每分钟/启动时      +---------------------+
|  合约   | -------------------> | GatewayModelIndex   |
| (chain) |   GatewayListActive  | - Bleve 内存索引      |
+---------+   ModelsWithinProvider | - models map 缓存原始 |
         |                        +---------------------+
         |                                  ^
         |                                  | Search(ctx, req)
         v                                  |
   +-------------+                   +--------------+
   | Indexer     |                   | GraphQL      |
   | 管理器      |                   | Models resolver
   | - 周期刷新  |                   +--------------+
   | - 生命周期  |
   +-------------+
```

1. `main.go` 创建 `indexer.Indexer` 并注册 `GatewayModelIndex`。
2. `Indexer.Start(1m)` 立即从合约拉取全量活跃模型，构建 Bleve 索引和 `map[docID]ModelInfoWithProvider`。
3. 后台 goroutine 每分钟重新拉取并重建索引。
4. GraphQL `Models` resolver 在索引中搜索，然后对 `ProviderType == 9999` 的模型执行 `pool.HasEntry` 在线过滤。

## 核心组件

### 1. `pkg/indexer/indexer.go`

通用索引管理器。

```go
type Index interface {
    Name() string
    Refresh(ctx context.Context, chain *teechain.TeeChain) error
    Bleve() bleve.Index
}

type Indexer struct {
    chain   *teechain.TeeChain
    mu      sync.RWMutex
    indexes map[string]Index
    ctx     context.Context
    cancel  context.CancelFunc
}
```

关键方法：

| 方法 | 说明 |
|------|------|
| `NewIndexer(chain)` | 创建管理器 |
| `Register(index)` | 注册一个 `Index` 实现 |
| `Start(interval)` | 立即全量刷新，并启动周期刷新 |
| `Stop()` | 停止刷新并关闭所有 Bleve 索引 |
| `Get(name)` | 按名称获取索引 |
| `Refresh(ctx, name)` | 立即刷新指定索引 |
| `RefreshAll(ctx)` | 立即刷新所有索引 |

### 2. `pkg/indexer/gateway.go`

Gateway 模型索引实现。

```go
type GatewayModelIndex struct {
    mu     sync.RWMutex
    index  bleve.Index
    models map[string]cgateway.ModelInfoWithProvider
}
```

索引字段：

| 字段 | 类型 | 用途 |
|------|------|------|
| `name` | text | 模型名模糊搜索 |
| `modelId` | text | 模型 ID 文本 |
| `providerType` | numeric | 供应商类型过滤 |
| `contextLength` | numeric | 上下文长度范围 |
| `inPrice` | numeric | 输入价格范围 |
| `cachedPrice` | numeric | 命中缓存价格 |
| `outPrice` | numeric | 输出价格 |
| `providerID` | numeric | 供应商 ID |

查询请求：

```go
type GatewayModelSearchRequest struct {
    Keyword    string
    MinContext *uint64
    MaxContext *uint64
    MinPrice   *uint64
    MaxPrice   *uint64
    Offset     uint64
    Limit      uint32
}
```

## GraphQL 查询

### Schema

```graphql
type ModelInfoWithProvider {
  id: Int!
  model: ModelInfo!
  provider: ProviderInfo!
}

type ProviderInfo {
  owner: String!
  name: String!
  logoUrl: String!
  description: String!
  modelIds: [Int!]!
  region: Int!
  deposit: Int!
  feeRate: Int!
  priority: Int!
  totalRequests: Int!
  successRate: Int!
  avgLatency: Int!
  uptimeSLA: Int!
  isActive: Boolean!
  createdAt: Int!
}

extend type Query {
  Models(
    keyword: String
    minContext: Int
    maxContext: Int
    minPrice: Int
    maxPrice: Int
    offset: Int
    limit: Int
  ): [ModelInfoWithProvider!]!
}
```

### 查询示例

```graphql
query {
  Models(keyword: "gpt", minContext: 100000, maxContext: 200000, limit: 20) {
    id
    model {
      name
      providerType
      contextLength
      inPrice
    }
    provider {
      name
      logoUrl
      totalRequests
      successRate
    }
  }
}
```

## 集成方式

### 启动注册

在 `main.go` 中：

```go
import "github.com/computeflux/tee-provider/pkg/indexer"

idx := indexer.NewIndexer(teeChain)
if err := idx.Register(indexer.NewGatewayModelIndex()); err != nil {
    log.Fatalf("failed to register gateway model index: %v", err)
}
if err := idx.Start(1 * time.Minute); err != nil {
    log.Fatalf("failed to start indexer: %v", err)
}
defer idx.Stop()

resolver := &graph.Resolver{
    TeeChain: teeChain,
    Indexer:  idx,
    // ...
}
```

### 在 Resolver 中使用

```go
idx, err := r.Indexer.Get(indexer.GatewayModelIndexName)
if err != nil {
    return nil, err
}
gatewayIdx := idx.(*indexer.GatewayModelIndex)

models, err := gatewayIdx.Search(ctx, indexer.GatewayModelSearchRequest{
    Keyword: "gpt",
    Limit:   100,
})
```

## 扩展新的合约索引

为其他合约模块添加索引只需三步：

### 1. 实现 `Index` 接口

```go
package indexer

type GovProposalIndex struct {
    mu     sync.RWMutex
    index  bleve.Index
    items  map[string]cgov.ProposalWithID
}

func (g *GovProposalIndex) Name() string { return "gov_proposals" }

func (g *GovProposalIndex) Refresh(ctx context.Context, chain *teechain.TeeChain) error {
    proposals, err := gateway.GatewayListProposals(chain, 0, 10000)
    if err != nil { return err }
    // 构建 Bleve 索引
    return g.BuildFromProposals(proposals)
}

func (g *GovProposalIndex) Bleve() bleve.Index {
    g.mu.RLock()
    defer g.mu.RUnlock()
    return g.index
}
```

### 2. 注册到 `Indexer`

```go
idx := indexer.NewIndexer(teeChain)
idx.Register(indexer.NewGovProposalIndex())
idx.Start(1 * time.Minute)
```

### 3. 在 Resolver 中查询

```go
idx, _ := r.Indexer.Get("gov_proposals")
govIdx := idx.(*indexer.GovProposalIndex)
results, _ := govIdx.Search(ctx, ...)
```

## 配置与调优

### 刷新间隔

```go
idx.Start(1 * time.Minute) // 默认 1 分钟
```

数据滞后 = 刷新间隔。链上模型变更后，最长一个间隔才能在查询结果中反映。

### 日志

```go
indexer.Logger = func(format string, args ...interface{}) {
    log.Printf("[Indexer] "+format, args...)
}
```

### 索引上限

`GatewayModelIndex.Refresh` 默认拉取最多 10 万条活跃模型：

```go
gateway.GatewayListActiveModelsWithinProvider(chain, 0, 0, 0, 0, 0, 100000)
```

若未来模型数量超过此上限，需调整该值。

## 性能

- **内存占用**：10 万条约 50~100 MB（Bleve 索引 + 原始模型 map）。
- **查询耗时**：
  - 关键词搜索 + 范围筛选：亚毫秒级。
  - 5000 并发读同一份只读索引：无锁竞争，水平扩展良好。
- **刷新耗时**：
  - 从 pebble 拉取 10 万条模型：几十到几百毫秒。
  - 重建 Bleve 索引：几毫秒到几十毫秒。

## 限制与注意事项

1. **数据滞后**：内存索引与链上数据存在刷新间隔的延迟。
2. **启动依赖**：`Indexer.Start` 会阻塞到首次刷新完成；若链未就绪，启动会失败。
3. **模糊匹配范围**：默认开启 `SetFuzziness(1)`，允许编辑距离 1 的拼写错误；极短关键词（1~2 字符）可能匹配较多结果。
4. **Owner 格式**：`ProviderInfo.owner` 使用 `UniAddr.String()`，输出 `T:hex` 格式。
5. **热切换**：刷新失败时保留旧索引，不会导致查询中断。

## 测试

```bash
go test ./pkg/indexer/... -v
```

测试覆盖：关键词搜索、范围筛选、组合条件、分页。
