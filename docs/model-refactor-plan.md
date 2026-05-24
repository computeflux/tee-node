# ModelInfo 字段迁移 + 唯一性约束方案

## 1. 背景

`ModelInfo` 结构已更新（`types.go`），字段变化：

| 旧字段 | 新字段 | 说明 |
|---|---|---|
| `PointsPer1K` | — | 删除 |
| `PromptPricing` | `InPrice` | 重命名，仍保持 `idx:"range"` |
| — | `CachedPrice` | 新增 |
| — | `OutPrice` | 新增 |

当前 `ModelInfo`（`types.go`）:
```go
type ModelInfo struct {
    Name            string        `json:"name" idx:"exact"`
    Owner           model.UniAddr `json:"owner"`
    ProviderID      uint64        `json:"providerId" idx:"exact"`
    InputModalities uint8         `json:"inputModalities" idx:"exact"`
    ContextLength   uint64        `json:"contextLength"   idx:"range"`
    InPrice         uint64        `json:"inPrice"   idx:"range"`
    CachedPrice     uint64        `json:"cachedPrice"`
    OutPrice        uint64        `json:"outPrice"`
    ProviderType    uint16        `json:"providerType"`
    ModelID         string        `json:"modelId"`
    URI             string        `json:"uri,omitempty"`
    APIKeyRefs      []string      `json:"apiKeyRefs,omitempty"`
    IsActive        bool          `json:"isActive" idx:"exact"`
    CreatedAt       uint64        `json:"createdAt"`
}
```

## 2. 需要修改的内容

### 2.1 Model 合约方法字段迁移

**`RegisterModel`** 参数变更：
```go
// 旧
func (g GatewayMutation) RegisterModel(
    name string, pointsPer1K uint64,
    inputModalities uint8, contextLength uint64, promptPricing uint64,
    modelID string, providerType uint16, uri string, apiKeys []string,
)

// 新
func (g GatewayMutation) RegisterModel(
    name string,
    inputModalities uint8, contextLength uint64,
    inPrice, cachedPrice, outPrice uint64,
    modelID string, providerType uint16, uri string, apiKeys []string,
)
```

**`UpdateModel`** 参数变更：
```go
// 旧
func (g GatewayMutation) UpdateModel(
    modelID uint64, pointsPer1K uint64,
    inputModalities uint8, contextLength uint64, promptPricing uint64,
    modelIDStr string, providerType uint16, uri string, apiKeys []string,
)

// 新（需要确认是否允许改 name）
func (g GatewayMutation) UpdateModel(
    modelID uint64,
    inputModalities uint8, contextLength uint64,
    inPrice, cachedPrice, outPrice uint64,
    modelIDStr string, providerType uint16, uri string, apiKeys []string,
)
```

> 问题：`UpdateModel` 是否需要支持修改 `name`？如果不改 name，则 ProviderID+Name 唯一性只在注册时检查即可。

**查询方法字段更新**：
- `ListActiveModelsWithinProvider` / `QueryModels` 中 `PromptPricing` → `InPrice`

### 2.2 Provider Name 唯一性约束

在 `CreateProvider` 和 `UpdateProvider` 中检查：

```go
// CreateProvider 新增
existing, _ := g.providers.Query(txn, model.QueryCond{Field: "Name", Val: name})
if len(existing) > 0 {
    return errors.New("provider name already exists")
}
```

```go
// UpdateProvider 新增（仅在 name 变更时检查）
cur, _ := g.providers.Get(txn, providerID)
if cur != nil && cur.Name != name && name != "" {
    existing, _ := g.providers.Query(txn, model.QueryCond{Field: "Name", Val: name})
    if len(existing) > 0 {
        return errors.New("provider name already exists")
    }
}
```

### 2.3 Model ProviderID+Name 唯一性约束

在 `RegisterModel` 中检查：

```go
existing, _ := g.models.Query(txn, model.QueryAND{
    model.QueryCond{Field: "ProviderID", Val: *provID},
    model.QueryCond{Field: "Name", Val: name},
})
if len(existing) > 0 {
    return errors.New("model name already exists for this provider")
}
```

## 3. 需要确认的问题

1. **`UpdateModel` 是否允许修改 `name`？**
   - 如果允许：需要添加 ProviderID+Name 冲突检查（排除自身）
   - 如果不允许：无需额外检查

2. **`UpdateModel` 的新参数签名确认**：
   - 是否保留 `name` 参数？
   - 新价格字段是否都为 `uint64`？

3. **现有 GraphQL / API 调用方**：
   - `graph/contract.resolvers.go` 等调用 `RegisterModel` / `UpdateModel` 的地方需要同步更新
