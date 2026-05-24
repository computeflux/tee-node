# 用量上链结算架构设计与实现

## 问题背景

之前的用量统计系统存在**核心架构缺陷**：用量只记录在本地 PebbleDB（通过 `model.SetJson`），**从未调用侧链合约 `SettleUsage` 进行链上结算**。这导致：
- 用户积分从未被扣除
- Provider 从未获得收益
- 本地用量记录持续增长，永不清除

## 设计目标

1. **最小改动**：尽量复用现有数据结构（`GatewayDailyUsage`、`Settlement`）
2. **无重复结算**：每个节点只结算自己记录的流量数据
3. **可容错**：单 Provider 结算失败不影响其他 Provider
4. **可测试**：结算周期 `SettleInterval` 可外部修改

## 整体链路

```
用户请求 → relay_handler 记录用量 → usageCh → 异步写入 PebbleDB
                                                ↓（每5分钟）
                                    settle goroutine 扫描 usage: 前缀
                                                ↓
                                    按 ProviderID 聚合为 []Settlement
                                                ↓
                                    DKG signer 签名 → SubmitTx → 侧链
                                                ↓
                                    ContractMutation → SettleUsage
                                                ↓
                                    扣用户余额 + 加 Provider 余额
                                                ↓
                                    删除本地已结算记录
```

## 关键修改

### 1. 数据模型扩展

**[`GatewayTokenInfo`](pkg/model/gateway_store.go:18) 增加 `Addr` 字段**
- 原：`Owner string`（hex 字符串，无 callerType）
- 新：增加 `Addr model.UniAddr`（含 `T` + `V`，用于链上结算）
- 修改点：`graph/auth_helpers.go` 的 `createUserAPIKey` 在创建 token 时从 `address`（Ethereum hex）构造 `UniAddr{T: 1, V: 20bytes}`

**[`GatewayDailyUsage`](pkg/model/gateway_store.go:28) 增加 `User` 字段**
- 新增 `User model.UniAddr` —— 结算时直接作为 `Settlement.User` 使用

### 2. 用量记录时传入完整用户信息

**[`relay_handler.go`](pkg/api/gateway/relay_handler.go:100)**
- 从 token 取出 `token.Addr` 作为 `userAddr`
- `recordUsageAsync(userAddr, tokenID, ...)` 传入完整 `UniAddr`

### 3. 结算 goroutine（新增）

**[`optimizer.go`](pkg/api/gateway/optimizer.go:188)**

```go
func initUsageSettler()  // 每 SettleInterval（默认5分钟）触发
func settlePendingUsage() // 核心结算逻辑
func submitSettleUsage()  // 构造交易 + DKG 签名 + SubmitTx
```

结算流程：
1. `GetJsonList("GATEWAY", "usage:")` 扫描所有本地用量记录
2. 按 `ProviderID` 聚合为 `[]cgateway.Settlement`
3. 逐 Provider 提交链上交易（DKG signer 签名，`callerType=0`）
4. 成功后 `DeleteByteKey` 删除本地记录

### 4. 合约权限修改

**[`credit.go`](tee-chain/contracts/gateway/credit.go:41)**

`SettleUsage` 原要求调用者必须是 Provider Owner。修改后：
- `system account`（`CallbySystem()`）也可调用
- 普通节点调用仍需验证 Provider 归属
- 为后续多节点各自结算自己记录的流量做准备

```go
isSystem := g.api.CallbySystem()
if !isSystem {
    // 验证 caller == prov.Owner
}
```

## 关键代码

### 提交结算交易

```go
// SCALE 编码 Settlement 数组
argBytes, _ := codec.Encode(records)

method := model.MethodToSelector("SettleUsage")
call := &model.ContractCall{
    Name:   []byte("gateway"),
    Method: method[:],
    Args:   [][]byte{argBytes},
}

// DKG signer 签名（system call，callerType=0）
sigBytes, _ := sysCall.BytesForSig()
signer := dkg.Signer.ToSigner(model.SuiteEd25519)
signature, _ := signer.Sign(sigBytes)

// 提交到侧链 mempool
teechain.SubmitTx(tx)
```

### 合约侧执行

```go
func (g GatewayMutation) SettleUsage(records []Settlement) error {
    // 扣用户余额 → 加 Provider 余额 → 记录历史
    for _, r := range records {
        bal, _ := g.credits.Get(txn, r.User)
        *bal -= cost
        g.credits.Set(txn, r.User, *bal)
        
        pb, _ := g.providerCredits.Get(txn, r.ProviderID)
        *pb += cost
        g.providerCredits.Set(txn, r.ProviderID, *pb)
    }
}
```

## 与已有系统的兼容性

| 组件 | 影响 | 说明 |
|------|------|------|
| `usageCh` 写入 | 无变化 | 只增加了 `userAddr` 参数 |
| `FlushUsage` / `StopUsageWriter` | 无变化 | 仍用于 flush 内存到 PebbleDB |
| `InitOptimizations` | 新增 `initUsageSettler()` | 启动时自动启动结算循环 |
| 合约查询接口 | 无变化 | 所有 `GatewayQuery` 方法不变 |
| GraphQL 登录 | 无变化 | `createUserAPIKey` 增加参数，调用方已同步更新 |

## 待办安全项（P0→P2）

1. **P0 — 防重放**：当前 `SettleUsage` 无 nonce，同一批 settlement 可被重复提交。需要增加 `batchNonce` 或 `merkleRoot` + TEE 签名验证。
2. **P1 — 费用校验**：本地 `PointsUsed` 预计算与链上 `cost` 公式应保持一致（已在 `optimizer.go` 和 `credit.go` 中统一公式）。
3. **P2 — 失败重试**：`submitSettleUsage` 失败后，当前仅打印日志，未做重试或死信队列。
4. **P2 — 并发控制**：多 Provider 同时结算时，各自独立提交交易，存在 mempool 竞争。当前由侧链顺序执行保证原子性。

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
| `pkg/api/gateway/optimizer.go` | 重大修改 | 新增 `initUsageSettler`、`settlePendingUsage`、`submitSettleUsage` |
| `tee-chain/contracts/gateway/credit.go` | 修改 | `SettleUsage` 允许 `system account` 调用 |
| `docs/usage-settle-design.md` | 新增 | 本文档 |
