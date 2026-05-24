# 索引查询条件重构方案

## 1. 背景与问题

当前 `Query` 使用 `map[string]any` 表示查询条件：

```go
ids, _ := store.Query(txn, map[string]any{
    "Category": uint64(1),
    "Price":    Range{Min: 0, Max: 50},
})
```

### 存在的问题

1. **多条件 AND 存在空交集 bug**：
   - 第一个条件无匹配时返回 `[]ID{}`
   - 与第二个条件求交时，`IntersectIDs([]ID{}, ids2)` 返回 `nil`
   - 第三个条件判断 `candidates == nil`，误将 nil 当成"尚无过滤"，返回第三个条件的全部结果
   - **根因**：`IntersectIDs` 在任一输入为空时返回 `nil`，`mergeIDs` 无法区分"第一次查询"与"空交集"

2. **无法表达 OR 逻辑**：`map` 天然是 AND 关系，无法做"A 或 B"的组合查询

3. **迭代顺序不确定**：Go map 遍历顺序随机，多个条件时无法按业务语义排序优化

## 2. 新设计：表达式树

引入 `QueryExpr` 接口，用类型化的表达式树替代 `map[string]any`：

```go
// 单个条件
type QueryCond struct {
    Field string
    Val   any // uint64 | string | Range
}

// 逻辑与（交集）
type QueryAND []QueryExpr

// 逻辑或（并集）
type QueryOR []QueryExpr
```

### 语义

| 原写法 | 新写法 |
|---|---|
| `map{"Name": "foo"}` | `QueryCond{Field: "Name", Val: "foo"}` |
| `map{"A": 1, "B": 2}` | `QueryAND{QueryCond{A,1}, QueryCond{B,2}}` |
| 无法表达 | `QueryOR{QueryCond{A,1}, QueryCond{B,2}}` |
| 无法表达 | `QueryAND{QueryCond{A,1}, QueryOR{QueryCond{B,2}, QueryCond{C,3}}}` |

### Bug 修复

- `IntersectIDs` 在任一输入为空时返回 `[]ID{}` 而非 `nil`
- `mergeIDs` 中 `candidates == nil` 仅表示"首次条件"，`len(candidates) == 0` 正确保持空集
- `QueryAND` 中每个子表达式先求值再逐次交，空结果不会被后续条件覆盖

## 3. 影响范围

### 需修改的文件

| 文件 | 修改内容 |
|---|---|
| `pkg/model/contract_index.go` | `IntersectIDs` 返回空切片；添加 `UnionIDs` |
| `pkg/model/contract_indexer.go` | 添加 `QueryExpr/QueryCond/QueryAND/QueryOR` 类型；重写 `query` / `Query` 签名 |
| `tee-chain/contracts/gateway/api_key.go` | `Query` 调用改用 `QueryCond` |
| `tee-chain/contracts/gateway/provider.go` | `Query` 调用改用 `QueryCond` |
| `tee-chain/contracts/gateway/model.go` | `Query` 多条件调用改用 `QueryAND` |
| `pkg/model/contract_indexer_test.go` | 所有测试用例改用新 API |

### 调用方迁移示例

#### 单条件（最小改动）

```go
// 原
ids, _ := models.Query(txn, map[string]any{"ProviderID": pid})

// 新
ids, _ := models.Query(txn, model.QueryCond{Field: "ProviderID", Val: pid})
```

#### 多条件 AND

```go
// 原
filter := map[string]any{
    "IsActive": uint64(1),
    "ContextLength": model.Range{Min: 100, Max: 200},
}
ids, _ := models.Query(txn, filter)

// 新
ids, _ := models.Query(txn, model.QueryAND{
    model.QueryCond{Field: "IsActive", Val: uint64(1)},
    model.QueryCond{Field: "ContextLength", Val: model.Range{Min: 100, Max: 200}},
})
```

#### 动态构建（推荐写法）

```go
exprs := model.QueryAND{model.QueryCond{Field: "IsActive", Val: uint64(1)}}
if minContext != 0 || maxContext != 0 {
    exprs = append(exprs, model.QueryCond{
        Field: "ContextLength",
        Val:   model.Range{Min: minContext, Max: maxContext},
    })
}
if maxPrice != 0 {
    exprs = append(exprs, model.QueryCond{
        Field: "PromptPricing",
        Val:   model.Range{Min: 0, Max: maxPrice},
    })
}
ids, _ := models.Query(txn, exprs)
```

## 4. 兼容性

- `QueryCond` 和 `QueryAND`/`QueryOR` 实现了同一接口 `QueryExpr`，调用方只需导入 `model` 包
- `Query` 方法签名从 `Query(txn, filter map[string]any)` 改为 `Query(txn, expr QueryExpr)`
- 旧 `map[string]any` 写法编译期就会报错，迁移安全

## 5. 执行步骤

1. [x] 修改 `contract_index.go` — 修复 `IntersectIDs` 空交集 bug
2. [x] 修改 `contract_indexer.go` — 定义新类型，重写 `query`
3. [ ] 修改调用方：`api_key.go`、`provider.go`、`model.go`
4. [ ] 更新测试：`contract_indexer_test.go`
5. [ ] `go build ./...` 验证编译
6. [ ] `go test ./pkg/model/...` 跑测试
