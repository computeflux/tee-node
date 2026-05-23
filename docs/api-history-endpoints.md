# Gateway 历史查询 API 接口总结

## 1. 设计目标

将充值、结算、用量等关键业务数据通过侧链合约存储，确保所有节点查询结果一致、不可篡改、可审计。

## 2. 合约层变更

### 2.1 新增存储字段（`Gateway` struct）

```go
rechargeHistory   *model.StoreList2D[model.UniAddr, uint32, RechargeRecord] // user → []RechargeRecord
settlementHistory *model.StoreList2D[uint64, uint32, SettlementRecord]      // providerID → []SettlementRecord
```

### 2.2 新增数据类型（`types.go`）

```go
type RechargeRecord struct {
    User   model.UniAddr `json:"user"`
    Amount uint64        `json:"amount"`
    Day    string        `json:"day"`
    Block  int64         `json:"block"`
}

type SettlementRecord struct {
    ProviderID uint64        `json:"providerId"`
    User       model.UniAddr `json:"user"`
    ModelID    uint64        `json:"modelId"`
    TokensUsed uint64        `json:"tokensUsed"`
    Cost       uint64        `json:"cost"`
    Day        string        `json:"day"`
    Block      int64         `json:"block"`
}
```

### 2.3 修改的 Mutation 方法

| 方法 | 变更说明 |
|------|----------|
| `Recharge(user, amount)` | 充值成功后向 `rechargeHistory.Insert()` 写入记录 |
| `SettleUsage(records)` | 每笔结算成功后向 `settlementHistory.Insert()` 写入记录 |

### 2.4 新增 Query 方法

| 方法 | 参数 | 返回 | 说明 |
|------|------|------|------|
| `GetRechargeHistory` | `user`, `offset`, `limit` | `[]RechargeRecord` | 按时间倒序返回用户充值记录 |
| `GetSettlementHistory` | `providerID`, `offset`, `limit` | `[]SettlementRecord` | 按时间倒序返回 Provider 结算记录 |
| `GetUsageHistory` | `user`, `providerID`, `modelID`, `startDay`, `endDay` | `[]DailyUsage` | 返回指定范围的每日用量聚合；`providerID=0 && modelID=0` 时返回该用户全部用量 |
| `GetAllUsageByUser` | `user`, `startDay`, `endDay` | `[]DailyUsage` | `GetUsageHistory` 的便捷封装 |

### 2.5 合约路由（`gateway_gen.go`）

新增 method selector：

| Selector | 方法名 |
|----------|--------|
| `0xe1 0x2a 0x3b 0x4c` | `get_recharge_history` |
| `0xe2 0x3b 0x4c 0x5d` | `get_settlement_history` |
| `0xe3 0x4c 0x5d 0x6e` | `get_usage_history` |

## 3. 链上层桥接（`tee-chain/contract.go`）

```go
func (app *TeeChain) GatewayGetRechargeHistory(user, offset, limit)
func (app *TeeChain) GatewayGetSettlementHistory(providerID, offset, limit)
func (app *TeeChain) GatewayGetUsageHistory(user, providerID, modelID, startDay, endDay)
```

## 4. REST API 接口

### 4.1 路由注册（`relay_handler.go`）

将 AI 调用端点与历史查询端点分离，避免历史查询误走 `DistributeMiddleware` 模型路由：

```go
/v1/models                          // 公开
/v1/chat/completions                // TokenAuth + DistributeMiddleware
/v1/logs                            // TokenAuth
/v1/data/export                     // TokenAuth
```

> `GET /v1/usage` 和 `GET /v1/recharge/history` 已从 REST 层移除，改为直接通过合约查询（见第 2 节）。

### 4.2 接口详情

> **注意：** `GET /v1/usage` 和 `GET /v1/recharge/history` 已移除，改为直接通过合约查询（见第 2 节）。

#### `GET /v1/logs`

查询用户调用日志（复用 `DailyUsage` 聚合数据）。

**Query 参数：** 同 `GET /v1/usage`

**Response：** 同 `GET /v1/usage`

#### `POST /v1/data/export`

数据导出（基于合约数据同步返回）。支持导出 usage、recharge、settlement、logs 类型数据。

**Request Body：**

```json
{
  "type": "usage",        // usage | recharge | settlement | logs
  "start_day": "2026-04-01",
  "end_day": "2026-05-01",
  "format": "json",       // json | csv（预留）
  "provider_id": 0        // settlement 类型时必填
}
```

**Response：**

```json
{
  "type": "usage",
  "format": "json",
  "data": [ ... ]
}
```

## 5. 数据一致性说明

- 所有历史数据均通过侧链合约的 `StoreMapping` / `StoreList2D` 存储，写入后随区块同步到全部共识节点。
- REST 层通过 `gatewayRuntime()` 创建只读事务查询本地已同步的合约状态，**无需跨节点 RPC**，查询延迟极低。
- `DailyUsage` 在 `SettleUsage` 批量结算时由合约自动聚合更新，保证用量数据与扣费记录原子一致。
