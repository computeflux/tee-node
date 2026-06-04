# 结算记录双存储设计说明

## 背景

`StoreList2D[K1, Ix, V]` 是"单 key 多维列表"结构：一个 K1 对应一个按 Ix 递增的列表。它的特点是：
- **查询快**：给定 K1，O(1) 定位列表，然后 DescList 分页返回
- **限制**：只能按 K1 这一个维度索引

结算记录 `SettlementRecord` 有两个核心查询维度：
1. **Provider 视角**：我（Provider）赚了多少钱？
2. **User 视角**：我（User）花了多少钱？

## 为什么存两份

### 方案对比

| 方案 | Provider 查询 | User 查询 | 评价 |
|------|-------------|----------|------|
| **只按 Provider 存** | O(1) DescList | O(N) 全表扫描 + 过滤 | User 查询极慢 |
| **只按 User 存** | O(N) 全表扫描 + 过滤 | O(1) DescList | Provider 查询极慢 |
| **双写（当前方案）** | O(1) DescList | O(1) DescList | 两次 O(1)，空间换时间 |
| **IndexedList + idx tag** | 理论上可行 | 理论上可行 | StoreList2D 不支持复合索引，改动大 |

### 双写的本质

不是"重复存储浪费空间"，而是**按不同维度建立索引**。同一份结算记录：

```
Provider 维度：providerID=7 → [record3, record2, record1]  //  Provider 看自己的收益
User 维度：   user=0xabc  → [record3, record5, record1]  //  User 看自己的消费
```

## 存储成本分析

`SettlementRecord` 结构：
```go
type SettlementRecord struct {
    ProviderID   uint64        // 8 bytes
    User         model.UniAddr // 4 + len(V) bytes
    ModelID      uint64        // 8 bytes
    TokensUsed   uint64        // 8 bytes
    CachedTokens uint64        // 8 bytes
    Cost         uint64        // 8 bytes
    Day          string        // ~8 bytes
    Block        int64         // 8 bytes
}
```

单条约 60-80 bytes。假设一天 10000 次调用，双写增加约 1.6 MB/天，可忽略不计。

## 查询性能

```go
// Provider 查收益 — O(1) + 分页
records, _ := g.settlementHistory.DescList(txn, providerID, nil, 50)

// User 查消费 — O(1) + 分页
records, _ := g.userSettlementHistory.DescList(txn, user, nil, 50)
```

两者都是直接定位到列表，不需要遍历、不需要过滤。

## 结论

双写是**在 StoreList2D 单 key 限制下的最优解**：
- 空间成本极低（结算记录本身很小）
- 查询性能最优（双方都是 O(1) + 分页）
- 代码改动最小（复用现有 StoreList2D，不需要引入复合索引）
