# ModelInfo 字段迁移 + 唯一性约束方案

> 最后更新：2026-05-26
> 状态：已落地 — 字段迁移完成，唯一性约束已实施

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

## 2. 已修改的内容

### 2.1 Model 合约方法字段迁移 ✅

**`RegisterModel`** 参数（当前代码）：
```go
func (g GatewayMutation) RegisterModel(
    name string,
    inputModalities uint8, contextLength uint64,
    inPrice, cachedPrice, outPrice uint64,
    providerType uint16,
    uri string,
    apiKeys []string,
) error
```

**`UpdateModel`** 参数（当前代码）：
```go
func (g GatewayMutation) UpdateModel(
    ID uint64,
    inputModalities uint8, contextLength uint64,
    inPrice, cachedPrice, outPrice uint64,
    name string,
    providerType uint16,
    uri string,
    apiKeys []string,
) error
```

> **已确认**：`UpdateModel` **允许修改 `name`**（`model.go:146` 有 `m.Name = name`）。
> 修改 name 时的 ProviderID+Name 唯一性防御性检查：当前代码中 `UpdateModel` 获取当前模型后直接更新，**尚未在 UpdateModel 中显式排除自身 ID 后做冲突检查**。如需严格保证 name 唯一性，应补充此检查。

**查询方法字段更新**：
- `ListActiveModelsWithinProvider` / `QueryModels` 中 `PromptPricing` → `InPrice` ✅ 已完成

### 2.2 Provider Name 唯一性约束 ✅

在 `CreateProvider` 和 `UpdateProvider` 中已检查：

```go
// CreateProvider
cur, _ := g.providers.Query(txn, model.QueryCond{Field: "Name", Val: name})
if len(cur) > 0 {
    return errors.New("provider name already exists")
}

// UpdateProvider（name 变更时检查）
if cur.Name != name && name != "" {
    existing, _ := g.providers.Query(txn, model.QueryCond{Field: "Name", Val: name})
    if len(existing) > 0 {
        return errors.New("provider name already exists")
    }
}
```

### 2.3 Model ProviderID+Name 唯一性约束 ✅

在 `RegisterModel` 中已检查：

```go
existing, err := g.models.Query(txn, model.QueryAND{
    model.QueryCond{Field: "ProviderID", Val: *provID},
    model.QueryCond{Field: "Name", Val: name},
})
if err != nil {
    return err
}
if len(existing) > 0 {
    return errors.New("model name already exists for this provider")
}
```

## 3. 已确认的问题

1. **`UpdateModel` 允许修改 `name`** ✅
   - 代码中已包含 `m.Name = name`
   - 建议：补充 ProviderID+Name 冲突检查（排除自身 ID）

2. **`UpdateModel` 的新参数签名** ✅
   - 保留 `name` 参数（第7个参数）
   - 新价格字段 `inPrice`, `cachedPrice`, `outPrice` 均为 `uint64`

3. **GraphQL / API 调用方**
   - `graph/contract.resolvers.go` 调用 `RegisterModel` / `UpdateModel` 的地方需要确认已同步更新参数顺序

---

*文档更新：2026-05-26*
