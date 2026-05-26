# 合约安全审计报告 & Nonce 设计方案

> 最后更新：2026-05-26
> 状态：MED-1 已修复，P0/P1 待实施

## 一、安全等级概述

| 等级 | 数量 | 说明 |
|---|---|---|
| 🔴 Critical | 2 | 可导致资金直接损失或无限重放 |
| 🟠 High | 3 | 可导致权限滥用或数据篡改 |
| 🟡 Medium | 1 | 业务逻辑缺陷，已修复 |
| 🟢 Low | 1 | 建议改进，风险可控 |

---

## 二、Critical（严重）

### CR-1: `SettleUsage` 批量结算无防重放机制

**位置**: `tee-chain/contracts/gateway/credit.go:37`

**问题描述**:
- `SettleUsage` 接受 `[]Settlement` 批量结算，但没有任何机制防止同一批记录被重复提交
- Provider 可以构造完全相同的 `records` 参数，反复调用 `SettleUsage`，每次都会重新扣用户余额、加 provider 余额、写用量历史
- 代码中的 TODO 明确标注了此问题：
  ```go
  // TODO: 防重放 — 增加 batchNonce/merkle root + TEE 签名校验，防止 Provider 重复提交同一批结算
  ```

**攻击路径**:
```
1. Provider 在 epoch N 正常服务了一批请求
2. Provider 调用 SettleUsage(records) 成功结算
3. Provider 在 epoch N+1 再次提交完全相同的 records
4. 合约无去重检查，用户被二次扣款，Provider 二次获利
```

**影响**: 用户资金可被无限次扣除，Provider 可无限套利。

**修复建议**:
- 增加 `batchNonce` 字段到 `Settlement` 或作为单独参数
- 合约维护 `usedNonces map[string]bool`，已使用的 nonce 永久拒绝
- nonce 由 TEE 生成（基于时间窗口 + 随机数），Provider 无法伪造
- 或者使用 Merkle Root + TEE 签名，每批次唯一

---

### CR-2: 全局缺少用户级 nonce/sequence 机制

**位置**: `tee-chain/contracts/runtime.go`, `tee-chain/tx_process.go`, `tee-chain/tx_prepare.go`

**问题描述**:
- 当前 `Runtime` 只提供 `caller`（调用者地址），没有任何 nonce/seq 字段
- `ProcessTx` 仅做 protobuf 解析和类型校验，不检查交易唯一性
- `PrepareTx` 中对 `HubCall` 做了同块同 caller 去重，但对 `Contract` 类型交易不做任何去重
- 这意味着：
  1. 同一用户可以构造完全相同的 `Contract` 交易，在同一块或不同块中重复执行
  2. 例如 `RegisterModel` 被重复调用会创建多个同名模型（虽然加了唯一性检查，但像 `UpdateProvider` 等操作可重复执行）
  3. `SettleUsage` 的重放只是此问题的最严重后果

**攻击路径**:
```
1. 用户 A 发送交易 T1: UpdateProvider(name="foo")
2. 区块打包并执行 T1
3. 用户 A 重新广播完全相同的 T1（或 mempool 中的旧交易）
4. 新区块再次打包 T1，合约再次执行 UpdateProvider
5. 如果 UpdateProvider 有副作用（如扣费、发奖），用户可被多次扣费/多次获利
```

**影响**: 所有状态变更方法都存在重放风险，只是大多数方法重复执行不产生额外资金流动（幂等）。但任何未来增加"每次调用有副作用"的功能都会暴露此漏洞。

**修复建议**:
- 在 `Runtime` 或 `ContractCall` 中增加 `nonce uint64`
- 合约层维护每个用户的 `nextNonce`，严格递增验证
- 或使用基于时间的 TTL 机制：交易中包含 `expiresAtBlock`，过期拒绝
- 详见下方「Nonce 设计方案」

---

## 三、High（高危）

### HI-1: `SettleUsage` 的 Cost=0 时自动计算完全依赖链下诚实

**位置**: `tee-chain/contracts/gateway/credit.go:57-64`

**问题描述**:
- 合约自动计算费用：
  ```go
  uncached := r.TokensIn - r.CachedToken
  cost = (uncached*m.InPrice + r.CachedToken*m.CachedPrice + r.TokenOut*m.OutPrice) / 1_000_000
  ```
- 所有输入 `TokensIn`、`CachedToken`、`TokenOut` 都来自 Provider 提交的 `records`
- Provider 可以伪造 `TokensIn=0`、`CachedToken=0`、`TokenOut=0`，则 `cost=0`
- 或者伪造巨大的 `TokensIn`，让用户承担超额费用
- 虽然 `cost == 0` 时 `continue` 跳过，但这只是最小值保护

**攻击路径**:
```
1. Provider 真实服务消耗了用户 1000 tokens
2. Provider 提交 Settlement{TokensIn: 0, CachedToken: 0, TokenOut: 0}
3. 合约计算 cost = 0 → continue 跳过
4. 用户不被扣费，Provider 无收益（但服务了请求）
```

**影响**: Provider 可以少报用量少扣费，或者多报用量多扣费。结算数据不可信。

**修复建议**:
- `SettleUsage` 必须由 TEE 签名，TEE 在网关侧真实统计用量后签名
- 合约验证 TEE 签名后才接受结算
- 或者用户侧先授权（如预签名用量上限），超出部分拒绝

---

### HI-2: `Recharge` / `Withdraw` 仅靠 `CallbySystem()` 权限检查

**位置**: `tee-chain/contracts/gateway/credit.go:12`, `credit.go:114`

**问题描述**:
- `Recharge` 和 `Withdraw` 都检查 `!g.api.CallbySystem()`，只有 sudo 账户可调用
- 这是正确的权限控制，但存在两个问题：
  1. **单点风险**: sudo 账户私钥泄露 = 全部资金可被任意转移
  2. **无多签/阈值**: 没有多签机制，sudo 可直接任意操作
  3. **无操作日志/审计**: 充值和提现操作只记录历史，没有实时审计告警

**影响**: sudo 账户是单点故障，被攻破则整个经济系统崩溃。

**修复建议**:
- 增加多签/阈值机制：充值/提现需要多个 TEE 节点共识签名
- 或引入时间锁：大额提现需延迟 N 个区块才可执行
- 增加实时审计日志，异常大额操作触发告警

---

### HI-3: `SettleUsage` 结算频率无链上限制

**位置**: `tee-chain/contracts/gateway/credit.go:38`

**问题描述**:
- 注释说"每 5 分钟执行一次"，但这只是业务约定，链上没有任何限制
- Provider 可以连续调用 `SettleUsage`，每次结算不同批次
- 结合 CR-1 的重放问题，Provider 可以在短时间内榨干用户余额

**攻击路径**:
```
1. Provider 拥有多个用户的待结算记录
2. Provider 在 1 分钟内连续调用 10 次 SettleUsage
3. 每次结算都扣费成功
4. 用户余额可能在一次服务周期内被超额扣除
```

**影响**: 用户无法预期扣费节奏，可能被快速清空余额。

**修复建议**:
- 链上维护每个 Provider 的 `lastSettleBlock`，限制最小结算间隔（如 100 个 block ≈ 5 分钟）
- 或者维护每个用户的 `lastSettleBlock`，限制单个用户被结算的频率

---

## 四、Medium（中危）

### MED-1: 每日用量 key 使用固定字符串 `"today"` 而非实际日期 ✅ 已修复

**位置**: ~~`tee-chain/contracts/gateway/credit.go`~~ — `DailyUsage` 已废弃，不再写入

**修复状态**: 已移除 `DailyUsage` 写入逻辑。本地用量记录通过 `usage:tokenID_modelID` key 聚合，结算后删除。不再需要按天隔离。

---

### MED-2: `APIKeyInfo.RateLimit` 只在链上存储，不主动限流

**位置**: `tee-chain/contracts/gateway/types.go:133`

**问题描述**:
- `APIKeyInfo` 有 `RateLimit uint32` 字段，表示每分钟最大请求数
- 但在 `auth_middleware.go` 和 `relay_handler.go` 中没有看到任何基于 `RateLimit` 的限流逻辑
- 链上存储了限流配置，但网关层没有读取和执行

**影响**: RateLimit 形同虚设，API Key 可被无限速率滥用。

**修复建议**:
- 在 `relay_handler.go` 或 `auth_middleware.go` 中读取 `token.RateLimit`
- 使用内存计数器或 Redis 做滑动窗口限流
- 或考虑在合约层维护用量计数（成本高，不推荐）

---

## 五、Low（低危）

### LOW-1: `UpdateModel` 不允许改 name，但 `CreatedAt` 被覆盖

**位置**: `tee-chain/contracts/gateway/model.go:147`

**问题描述**:
- `UpdateModel` 中 `CreatedAt` 被设为当前区块时间：`m.CreatedAt = uint64(g.api.GetBlockTime())`
- 这导致每次更新模型，创建时间都被刷新
- 无法准确追踪模型首次注册时间

**修复建议**:
- `UpdateModel` 不应修改 `CreatedAt`
- 如需记录更新时间，增加 `UpdatedAt` 字段

---

## 六、Nonce 设计方案

### 6.1 问题背景

当前合约运行时 (`Runtime`) 和交易处理流程 (`tx_process.go`, `tx_prepare.go`) 中，没有任何机制确保同一笔交易不会被重复执行。这对于 `Contract` 类型的交易尤为危险。

### 6.2 设计目标

1. **防重放**: 同一笔交易不能被重复执行（同一块或跨块）
2. **有序性**: 用户交易按顺序执行，跳过 nonce 的交易被拒绝
3. **最小改动**: 尽量复用现有 `ContractCall` 结构
4. **不依赖主链**: nonce 在侧链合约层维护

### 6.3 方案 A: 合约层严格递增 Nonce（推荐）

#### 6.3.1 数据模型

增加 `ContractCall` 字段（或作为 args 第一个参数）：

```go
// ContractCall 增加 Nonce 字段
type ContractCall struct {
    Name   string   // 合约名，如 "gateway"
    Method [4]byte  // 方法 selector
    Nonce  uint64   // NEW: 调用者 nonce，严格递增
    Args   [][]byte // 参数列表
}
```

增加合约状态存储：

```go
// gateway.go 中增加
userNonces *model.StoreMapping[model.UniAddr, uint64]  // caller → nextNonce
```

#### 6.3.2 验证逻辑

在 `ExecCall` / `ExecQuery` 入口统一验证：

```go
func (d GatewayMutation) ExecCall(call *model.ContractCall) error {
    // 1. 验证 nonce
    caller := d.api.GetCaller()
    expectedNonce, _ := d.userNonces.Get(d.api.GetTxn(), caller)
    if expectedNonce == nil {
        var zero uint64
        expectedNonce = &zero
    }
    if call.Nonce != *expectedNonce {
        return fmt.Errorf("invalid nonce: expected %d, got %d", *expectedNonce, call.Nonce)
    }
    
    // 2. 执行原逻辑
    err := d.dispatch(call)
    
    // 3. nonce +1（仅在成功时递增）
    if err == nil {
        *expectedNonce++
        d.userNonces.Set(d.api.GetTxn(), caller, *expectedNonce)
    }
    return err
}
```

#### 6.3.3 客户端流程

```
1. 用户查询当前 nonce: Query("gateway", "get_nonce", [caller])
2. 用户构造交易: ContractCall{Nonce: currentNonce, ...}
3. 用户签名并提交
4. 交易被打包执行，nonce 验证通过并递增
5. 下一笔交易必须使用 nonce = currentNonce + 1
```

#### 6.3.4 优势

- 严格有序：nonce 必须连续，跳过则拒绝
- 天然防重放：已使用的 nonce 永远不能再使用
- 失败交易不递增 nonce：避免用户因一笔失败交易导致后续全部交易卡住

#### 6.3.5 劣势 & 解决

| 问题 | 解决 |
|---|---|
| 用户需要预先查询 nonce | 增加 `get_nonce` 查询方法 |
| 并发提交可能乱序 | 客户端按顺序发送，或增加 `nonce = expected + N` 的批量预留机制 |
| 旧交易在 mempool 中滞留 | 增加 `expiresAtBlock` 字段，超时自动失效 |

### 6.4 方案 B: 交易哈希去重（轻量替代）

如果不希望修改 `ContractCall` 结构，可以在侧链交易层做去重：

```go
// TeeChain 状态增加
seenTxs map[string]int64  // txHash → blockHeight，定期清理

// ProcessTx 中
func (app *TeeChain) ProcessTx(txs [][]byte, _time time.Time) ... {
    for _, txbt := range txs {
        txHash := blake2b.Sum256(txbt)
        if app.seenTxs[string(txHash)] > 0 {
            return REJECT  // 重复交易
        }
        // ... 正常处理
    }
}

// FinalizeTx 中记录
func (app *TeeChain) FinalizeTx(txs [][]byte, height int64) {
    for _, txbt := range txs {
        txHash := blake2b.Sum256(txbt)
        app.seenTxs[string(txHash)] = height
    }
    // 清理超过 N 个 block 的旧记录
    app.gcOldTxs(height - 10000)
}
```

**优势**: 无需修改合约，对现有代码无侵入
**劣势**: 只能防交易级重放，不能防"用户故意构造两笔不同但语义相同的交易"

### 6.5 方案 C: 时间窗口 + TEE 签名（针对 SettleUsage）

专门解决 `SettleUsage` 的结算重放问题：

```go
type Settlement struct {
    User         model.UniAddr `json:"user"`
    ModelID      uint64        `json:"modelId"`
    ProviderID   uint64        `json:"providerId"`
    TokensIn     uint64        `json:"tokensIn"`
    TokenOut     uint64        `json:"tokenOut"`
    CachedToken  uint64        `json:"cachedToken"`
    Count        uint16        `json:"count"`
    WindowID     uint64        `json:"windowId"`  // NEW: 结算窗口 ID（如 height/720）
}

// SettleUsage 增加 batchNonce 参数
func (g GatewayMutation) SettleUsage(batchNonce uint64, records []Settlement) error {
    // 检查 batchNonce 是否已使用
    if g.isBatchNonceUsed(batchNonce) {
        return errors.New("batch nonce already used")
    }
    g.markBatchNonceUsed(batchNonce)
    
    // 检查所有记录的 WindowID 是否一致且合法
    expectedWindow := g.api.GetHeight() / 720
    for _, r := range records {
        if r.WindowID != expectedWindow && r.WindowID != expectedWindow-1 {
            return errors.New("invalid settlement window")
        }
    }
    
    // ... 原结算逻辑
}
```

**优势**: 针对性强，解决 Provider 结算重放问题
**劣势**: 只解决 SettleUsage，不解决全局重放问题

### 6.6 推荐实施路径

| 阶段 | 内容 | 优先级 | 状态 |
|---|---|---|---|
| P0 | 立即在 `SettleUsage` 增加 `batchNonce` + 合约维护 `usedBatchNonces` | Critical | 🔲 待实施 |
| P1 | 为所有合约增加用户级 `nonce` 机制（方案 A） | High | 🔲 待实施 |
| P2 | `SettleUsage` 增加 TEE 签名验证，确保用量数据真实 | High | 🔲 待实施 |
| P3 | 增加链上结算频率限制（`lastSettleBlock`） | Medium | 🔲 待实施 |
| P4 | 增加 `RateLimit` 网关层执行 | Medium | 🔲 待实施 |
| P5 | 修复 `UpdateModel` 不应覆盖 `CreatedAt` | Low | 🔲 待实施 |

---

## 七、总结

当前合约最严重的安全问题是 **SettleUsage 无防重放** 和 **全局缺少 nonce 机制**。前者可被 Provider 直接利用套利，后者是所有合约交易的底层安全缺陷。建议按 P0→P2 的顺序优先实施，P3→P5 可逐步完善。
