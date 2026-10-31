# Gateway 历史查询 API 接口总结

> 最后更新：2026-10-30
> 重要变更：DailyUsage 已废弃，不再写入；结算记录双写（Provider + User 维度）
> 2026-10-30：`RechargeRecord` 增加 `ChainID`/`Nonce` 跨链凭据（运行时 V33 迁移）；`Recharge` 参数变为 `(user, amount, chainID, nonce)`

## 1. 设计目标

将充值、结算等关键业务数据通过侧链合约存储，确保所有节点查询结果一致、不可篡改、可审计。

## 2. 合约层变更

### 2.1 存储字段（`Gateway` struct）

```go
rechargeHistory       *model.StoreList2D[model.UniAddr, uint32, RechargeRecord]       // user → []RechargeRecord
settlementHistory     *model.StoreList2D[uint64, uint32, ProviderSettlementRecord]    // providerID → []ProviderSettlementRecord
userSettlementHistory *model.StoreList2D[model.UniAddr, uint32, UserSettlementRecord] // user → []UserSettlementRecord
```

### 2.2 数据类型（`types.go`）

```go
// RechargeRecord 单笔充值记录（V33 起含跨链凭据）
type RechargeRecord struct {
    User    model.UniAddr `json:"user"`
    Amount  uint64        `json:"amount"`
    Time    int64         `json:"time"`
    ChainID uint32        `json:"chainId"` // 充值来源链 ID（0 = 非跨链/旧记录）
    Nonce   uint64        `json:"nonce"`   // 来源链事件序号
}

// ProviderSettlementRecord Provider 视角的结算记录
type ProviderSettlementRecord struct {
    ProviderID  uint64        `json:"providerId"`
    User        model.UniAddr `json:"user"`
    ModelID     uint64        `json:"modelId"`
    TokenIn    uint64        `json:"TokenIn"`
    TokenOut    uint64        `json:"tokenOut"`
    CachedToken uint64        `json:"cachedToken"`
    Cost        uint64        `json:"cost"`
    Count       uint16        `json:"count"`
    Time        int64         `json:"time"`
}

// UserSettlementRecord User 视角的结算记录
type UserSettlementRecord struct {
    ProviderID  uint64        `json:"providerId"`
    ModelID     uint64        `json:"modelId"`
    TokenIn    uint64        `json:"TokenIn"`
    TokenOut    uint64        `json:"tokenOut"`
    CachedToken uint64        `json:"cachedToken"`
    Cost        uint64        `json:"cost"`
    Count       uint16        `json:"count"`
    Time        int64         `json:"time"`
}
```

> **注意**：`DailyUsage` 类型已废弃，合约中不再写入每日用量记录。用量数据通过 `SettleUsage` 结算后直接转为 `SettlementRecord` 存储。

### 2.3 修改的 Mutation 方法

| 方法 | 变更说明 |
|------|----------|
| `Recharge(user, amount, chainID, nonce)` | 充值成功后向 `rechargeHistory.Insert()` 写入记录（含 `Time` 字段与跨链凭据 `ChainID`/`Nonce`）；`(chainID, nonce)` 同时是防重放 key |
| `SettleUsage(records)` | 每笔结算成功后双写：`settlementHistory.Insert()` + `userSettlementHistory.Insert()` |

### 2.4 新增 Query 方法

| 方法 | 参数 | 返回 | 说明 |
|------|------|------|------|
| `GetRechargeHistory` | `user`, `offset`, `limit` | `[]RechargeRecord` | 按时间倒序返回用户充值记录 |
| `GetSettlementHistory` | `providerID`, `offset`, `limit` | `[]ProviderSettlementRecord` | 按时间倒序返回 Provider 结算记录 |
| `GetUserSettlementHistory` | `user`, `offset`, `limit` | `[]UserSettlementRecord` | 按时间倒序返回用户消费记录 |

> **分页语义**：`offset` = 从最新记录起跳过的条数（0 = 从最新一条开始），`limit` = 单页最多返回条数。
> 早期实现起始游标多跳一条（`startKey = 总数 - offset`，越界一条），每页实际只返回 `limit-1` 条；
> 客户端不能用“返回条数 < limit”判断拉完，必须按实际返回条数推进 offset、以空页为结束。
> 四个历史查询（含充值/转账）已在合约侧统一修正为上述语义。

> **已移除**：`GetUsageHistory` / `GetAllUsageByUser` — `DailyUsage` 不再写入，查询无意义。

### 2.5 合约路由（`gateway_gen.go`）

新增 method selector（实际值以 `gateway_gen.go` 生成为准）：

| 方法名 | ABI 中的 label |
|----------|--------|
| `get_recharge_history` | `0xc89f2f8f` |
| `get_settlement_history` | `0x74d599fb` |
| `get_user_settlement_history` | `0x56b906c0` |

## 3. 链上层桥接（`tee-chain/contract.go`）

```go
func (app *TeeChain) GatewayGetRechargeHistory(user model.UniAddr, offset uint64, limit uint32) ([]RechargeRecord, error)
func (app *TeeChain) GatewayGetSettlementHistory(providerID uint64, offset uint64, limit uint32) ([]ProviderSettlementRecord, error)
func (app *TeeChain) GatewayGetUserSettlementHistory(user model.UniAddr, offset uint64, limit uint32) ([]UserSettlementRecord, error)
```

## 4. REST API 接口

> **当前状态**：历史查询接口已全部从 REST 层移除。
>
> 之前的 `/v1/logs` 和 `/v1/data/export` 端点**未在代码中实现**，不存在于当前路由注册中。
>
> 所有历史数据查询应直接通过合约 Query 方法或 GraphQL 合约查询接口进行。

## 5. 数据一致性说明

- 所有历史数据均通过侧链合约的 `StoreList2D` 存储，写入后随区块同步到全部共识节点。
- REST 层通过 `gatewayRuntime()` 创建只读事务查询本地已同步的合约状态，**无需跨节点 RPC**，查询延迟极低。
- `DailyUsage` 已废弃：本地 PebbleDB 中的 `usage:` 前缀记录仅作为结算前的临时缓冲，结算成功后即删除。

## 6. 双存储设计说明

结算记录采用**双写策略**（Provider 维度 + User 维度），详见 [`settlement-dual-storage-design.md`](settlement-dual-storage-design.md)。

| 维度 | 存储 | 查询 |
|------|------|------|
| Provider | `settlementHistory` (StoreList2D[providerID]) | `GetSettlementHistory` |
| User | `userSettlementHistory` (StoreList2D[user]) | `GetUserSettlementHistory` |

---

*文档更新：2026-05-26*
