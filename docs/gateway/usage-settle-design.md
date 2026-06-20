# 用量上链结算架构设计与实现

## 问题背景

之前的用量统计系统存在**核心架构缺陷**：用量只记录在本地 PebbleDB（通过 `model.SetJson`），**从未调用侧链合约 `SettleUsage` 进行链上结算**。这导致：
- 用户积分从未被扣除
- Provider 从未获得收益
- 本地用量记录持续增长，永不清除

## 设计目标

1. **最小改动**：尽量复用现有数据结构（`GatewayDailyUsage` 仅作为本地 PebbleDB 缓冲，`Settlement` 作为链上结算参数）
2. **统一批量提交**：所有用量合并为一个 `[]Settlement` 数组，一笔交易上链
3. **无重复结算**：每个节点只结算自己记录的流量数据
4. **可测试**：结算周期 `SettleInterval` 可外部修改

## 整体链路

```
用户请求 → relay_handler 记录用量 → usageCh → 异步写入 PebbleDB
                                                ↓（每5分钟）
                                    settle goroutine 扫描 usage: 前缀
                                                ↓
                                    统一构造为 []Settlement（大数组）
                                                ↓
                                    DKG signer 签名 → SubmitTx → 侧链
                                                ↓
                                    ContractMutation → SettleUsage
                                                ↓
                                    遍历 records：扣用户余额 + 加 Provider 余额
                                                ↓
                                    删除本地已结算记录
```

## 关键修改

### 1. 数据模型扩展

**[`GatewayTokenInfo`](pkg/model/gateway_store.go:18) 增加 `Addr` 字段**
- 原：`Owner string`（hex 字符串，无 callerType）
- 新：增加 `Addr model.UniAddr`（含 `T` + `V`，用于链上结算）
- 修改点：`graph/auth_helpers.go` 的 `createUserAPIKey` 在创建 token 时从 `address`（Ethereum hex）构造 `UniAddr{T: 1, V: 20bytes}`

**[`GatewayDailyUsage`](pkg/model/gateway_store.go:34) 字段说明**
- `User model.UniAddr` —— 结算时直接作为 `Settlement.User` 使用
- `TokenIn`、`TokenOut`、`CachedToken` —— 三维用量指标（替代旧版的单一 `TokensUsed`）
- 仅作为本地 PebbleDB 缓冲，**不再写入合约 `DailyUsage`**（该存储已废弃）

### 2. 用量记录时传入完整用户信息

**[`relay_handler.go`](pkg/api/gateway/relay_handler.go:100)**
- 从 token 取出 `token.Addr` 作为 `userAddr`
- `recordUsageAsync(userAddr, tokenID, ...)` 传入完整 `UniAddr`

### 3. 结算 goroutine（新增）

**[`optimizer.go`](pkg/api/gateway/optimizer.go:188)**

```go
func initUsageSettler()  // 每 SettleInterval（默认5分钟）触发
func settlePendingUsage() // 核心结算逻辑（统一批量提交）
func submitSettleUsage()  // 构造交易 + DKG 签名 + SubmitTx
```

结算流程：
1. `GetJsonList("GATEWAY", "usage:")` 扫描所有本地用量记录
2. 统一构造为一个大的 `[]cgateway.Settlement` 数组（不再按 Provider 拆分）
3. 一次性提交链上交易（DKG signer 签名，`callerType=0`）
4. 成功后统一 `DeleteByteKey` 删除本地记录
5. 交易失败时不删除记录，下次结算重试

### 4. 合约权限修改

**[`credit.go`](tee-chain/contracts/gateway/credit.go:41)**

`SettleUsage` 原要求调用者必须是 Provider Owner。修改后：
- `system account`（`CallbySystem()`）可调用（统一批量提交场景）
- 普通节点调用仍需验证 Provider 归属
- 合约内部遍历 `[]Settlement`，逐条扣用户余额、加 Provider 余额

```go
isSystem := g.api.CallbySystem()
if !isSystem {
    // 验证 caller == prov.Owner
}
```

## 关键代码

### 统一批量提交

```go
func settlePendingUsage() error {
    records, keys, err := model.GetJsonList[model.GatewayDailyUsage]("GATEWAY", "usage:")
    // ...
    settlements := make([]cgateway.Settlement, 0, len(records))
    for _, r := range records {
        settlements = append(settlements, cgateway.Settlement{...})
    }
    // 一次性提交所有记录
    if err := submitSettleUsage(settlements); err != nil {
        return err  // 失败不删除，下次重试
    }
    // 成功后再删除本地记录
    for _, k := range keys {
        model.DeleteByteKey(k)
    }
}
```

### 提交结算交易

```go
func submitSettleUsage(records []cgateway.Settlement) error {
    argBytes, _ := codec.Encode(records)
    method := model.MethodToSelector("SettleUsage")
    call := &model.ContractCall{...}
    sysCall := &model.SysCall{Payload: &model.SysCall_Contract{Contract: call}}
    sigBytes, _ := sysCall.BytesForSig()
    signer := dkg.Signer.ToSigner(model.SuiteEd25519)
    signature, _ := signer.Sign(sigBytes)
    return teechain.SubmitTx(tx)
}
```

### 合约侧执行

```go
func (g GatewayMutation) SettleUsage(records []Settlement) error {
    for _, r := range records {
        // 校验 provider 归属
        prov, _ := g.providers.Get(txn, r.ProviderID)
        // 校验模型存在
        m, _ := g.models.Get(txn, r.ModelID)

        // 计算费用（三维定价）
        uncached := r.TokenIn - r.CachedToken
        cost := (uncached*m.InPrice + r.CachedToken*m.CachedPrice + r.TokenOut*m.OutPrice) / 1_000_000
        if cost == 0 {
            continue
        }

        // 累加用户已用额度（credit_useds），不直接扣除 credits
        c, _ := g.credit_useds.Get(txn, r.User)
        *c += cost
        g.credit_useds.Set(txn, r.User, *c)

        // 加 provider 余额
        pb, _ := g.providerCredits.Get(txn, r.ProviderID)
        *pb += cost
        g.providerCredits.Set(txn, r.ProviderID, *pb)

        // 记录结算历史（双写：Provider 维度 + User 维度）
        g.settlementHistory.Insert(txn, r.ProviderID, ProviderSettlementRecord{...})
        g.userSettlementHistory.Insert(txn, r.User, UserSettlementRecord{...})
    }
}
```

> **关键变更**：
> - 结算不再直接扣除 `credits`，而是累加到 `credit_useds`，用户实际余额 = `credits - credit_useds`
> - 用量字段从单一 `TokensUsed` 改为 `TokenIn` / `TokenOut` / `CachedToken` 三维指标
> - 计费公式：`cost = (uncached*InPrice + cached*CachedPrice + out*OutPrice) / 1_000_000`
> - 结算历史双写：`settlementHistory`（Provider 维度）+ `userSettlementHistory`（User 维度）

## 与已有系统的兼容性

| 组件 | 影响 | 说明 |
|------|------|------|
| `usageCh` 写入 | 无变化 | 只增加了 `userAddr` 参数 |
| `FlushUsage` / `StopUsageWriter` | 已删除 | 死代码清理中移除，当前依赖 100ms ticker 落盘 |
| `InitOptimizations` | 新增 `initUsageSettler()` | 启动时自动启动结算循环 |
| 合约查询接口 | 无变化 | 所有 `GatewayQuery` 方法不变 |
| GraphQL 登录 | 无变化 | `createUserAPIKey` 增加参数，调用方已同步更新 |

## 待办安全项（P0→P2）

1. **P0 — 防重放**：当前 `SettleUsage` 无 nonce，同一批 settlement 可被重复提交。需要增加 `batchNonce` 或 `merkleRoot` + TEE 签名验证。
2. **P1 — 费用校验**：本地 `PointsUsed` 预计算与链上 `cost` 公式应保持一致（已在 `optimizer.go` 和 `credit.go` 中统一公式）。
3. **P2 — 失败重试**：`submitSettleUsage` 失败后，当前仅打印日志，未做重试或死信队列。

## 测试验证

- `go build ./...` —— 编译通过
- `go test ./pkg/model/...` —— 全部通过

## 文件变更清单

| 文件 | 变更类型 | 说明 |
|------|---------|------|
| `pkg/model/gateway_store.go` | 修改 | `GatewayTokenInfo` + `Addr`，`GatewayDailyUsage` + `User` |
| `graph/auth_helpers.go` | 修改 | `createUserAPIKey` 增加 `addrHex` 参数，构造 `UniAddr` |
| `graph/auth.resolvers.go` | 修改 | 3 处调用 `createUserAPIKey(email, address)` |
| `pkg/api/gateway/contract.go` | 修改 | `LookupAPIKeyViaContract` 返回 `Addr` |
| `pkg/api/gateway/relay_handler.go` | 修改 | `HandleRelay` 传入 `userAddr` 到 `recordUsageAsync` |
| `pkg/api/gateway/optimizer.go` | 重大修改 | 新增统一批量结算：`initUsageSettler`、`settlePendingUsage`、`submitSettleUsage`（DKG 签名 + `callerType=0` system call） |
| `tee-chain/contracts/gateway/credit.go` | 修改 | `SettleUsage` 允许 `system account` 调用 |
| `docs/usage-settle-design.md` | 新增 | 本文档 |
