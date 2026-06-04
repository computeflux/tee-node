# 合约安全审计报告 & Nonce 设计方案

> 最后更新：2026-05-29
> **2026-05-29 更新**：TEE KMS 已移除，API Key 改为明文存储于合约。原 KMS 相关安全假设已失效。

## 一、安全等级概述

| 等级 | 数量 | 说明 |
|---|---|---|
| 🔴 Critical | 0 | 全部修复 |
| 🟠 High | 3 | 可导致权限滥用或数据篡改 |
| 🟡 Medium | 1 | 业务逻辑缺陷，已修复 |
| 🟢 Low | 1 | 建议改进，风险可控 |

---

## 二、Critical（严重）

### CR-1: `SettleUsage` 批量结算无防重放机制 ✅ 已修复

**位置**: `tee-chain/contracts/gateway/credit.go:37`

**原问题描述**（已修复）:
- ~~`SettleUsage` 接受 `[]Settlement` 批量结算，但没有任何机制防止同一批记录被重复提交~~
- ~~Provider 可以构造完全相同的 `records` 参数，反复调用 `SettleUsage`~~
- 代码中的 TODO 已移除，防重放机制已实施。

**攻击路径**（已失效）:
```
1. Provider 在 epoch N 正常服务了一批请求
2. TEE 节点调用 SettleUsage(records) 成功结算（每条记录带唯一随机 Nonce）
3. 合约将每条 Nonce 写入 settledNonces，永久标记为已使用
4. Provider 再次提交相同 records → 合约检测到重复 Nonce → 返回 ErrDuplicateNonce，拒绝执行
```

**实际修复方案**（已实施）:

1. **Settlement 结构增加 Nonce 字段** [`tee-chain/contracts/gateway/types.go:147`](tee-chain/contracts/gateway/types.go:147)
   ```go
   type Settlement struct {
       // ... 原字段 ...
       Nonce []byte `json:"nonce"` // 16 bytes 随机 nonce，防重放
   }
   ```

2. **合约增加 settledNonces 存储** [`tee-chain/contracts/gateway/gateway.go:45`](tee-chain/contracts/gateway/gateway.go:45)
   ```go
   settledNonces *model.StoreMapping[string, bool] `store:"keyPfx:v5_settled_nonces"`
   ```

3. **SettleUsage 增加 Nonce 校验** [`tee-chain/contracts/gateway/credit.go:40-51`](tee-chain/contracts/gateway/credit.go:40)
   ```go
   for _, r := range records {
       nonceHex := hex.EncodeToString(r.Nonce)
       if nonceHex == "" {
           return fmt.Errorf("settle: empty nonce")
       }
       exists, _ := g.settledNonces.Get(txn, nonceHex)
       if exists != nil && *exists {
           return fmt.Errorf("settle: %w", ErrDuplicateNonce)
       }
       // ... 结算逻辑 ...
       g.settledNonces.Set(txn, nonceHex, true) // 标记已使用
   }
   ```

4. **TEE 节点生成随机 Nonce** [`pkg/api/gateway/optimizer.go:237-248`](pkg/api/gateway/optimizer.go:237)
   ```go
   nonce := make([]byte, 16)
   io.ReadFull(rand.Reader, nonce)
   settlements = append(settlements, cgateway.Settlement{
       // ...
       Nonce: nonce,
   })
   ```

**设计选择说明**:
- 采用 **per-record nonce** 而非 batch-level nonce，最小化 ABI 改动（`SettleUsage` 签名不变），且天然支持部分失败场景
- Nonce 16 bytes = 128 bit，随机冲突概率可忽略
- Nonce 由 TEE 节点通过 `crypto/rand` 生成，Provider 无法预测或伪造
- `settledNonces` 永久存储（不清理），确保历史 nonce 终身不可复用

---

### CR-2: 全局缺少用户级 nonce/sequence 机制 ✅ 已修复

**位置**: `pkg/model/tx.proto`, `tee-chain/tx_finalize.go`, `tee-chain/tx_nonce.go`

**原问题描述**（已修复）:
- ~~当前 `Runtime` 只提供 `caller`（调用者地址），没有任何 nonce/seq 字段~~
- ~~`ProcessTx` 仅做 protobuf 解析和类型校验，不检查交易唯一性~~
- ~~`PrepareTx` 中对 `HubCall` 做了同块同 caller 去重，但对 `Contract` 类型交易不做任何去重~~

**攻击路径**（已失效）:
```
1. 用户 A 发送交易 T1: UpdateProvider(name="foo")，携带 nonce=1
2. 区块打包并执行 T1，合约更新 nonce=1
3. 用户 A 重新广播完全相同的 T1（nonce=1）
4. 新区块再次打包 T1
5. FinalizeTx 检查 nonce=1 ≠ expected=2 → 拒绝执行
```

**实际修复方案**（已实施）：

采用 **Tx 级递增 nonce** 方案，覆盖所有合约交易：

1. **Tx protobuf 增加 nonce 字段** [`pkg/model/tx.proto:11`](pkg/model/tx.proto:11)
   ```protobuf
   message Tx{
     bytes caller = 10;
     uint32 caller_type = 12;
     uint64 nonce = 14;   // 用户级严格递增 nonce
     bytes call = 13;
     bytes signature = 11;
   }
   ```

2. **FinalizeTx 中严格递增校验** [`tee-chain/tx_finalize.go:102`](tee-chain/tx_finalize.go:102)
   ```go
   case *model.SysCall_Contract:
       caller := tx.GetCaller()
       callerType := tx.GetCallerType()
       txNonce := tx.TxNonce()

       currentNonce, err := TxnGetCurrentCallerNonce(txn, caller, callerType)
       if err != nil {
           return nil, errors.Wrap(err, "get caller nonce")
       }

       expectedNonce := currentNonce + 1
       if txNonce != expectedNonce {
           return nil, errors.Errorf(
               "invalid nonce: expected %d, got %d", expectedNonce, txNonce,
           )
       }

       err = app.ContractMutation(caller, callerType, txn, height, p.Contract)
       // ... 执行合约 ...

       if err := TxnSetCallerNonce(txn, caller, callerType, txNonce); err != nil {
           return nil, errors.Wrap(err, "update caller nonce")
       }
   ```

3. **三层防御体系**
    - **Mempool 层** [`tee-chain/tx_check.go`](tee-chain/tx_check.go): `CheckTx` 阶段即拒绝 nonce 不合法的合约交易，防止无效交易进入内存池
    - **Prepare 层** [`tee-chain/tx_prepare.go`](tee-chain/tx_prepare.go): `PrepareProposal` 阶段使用临时 `model.Txn` 顺序校验 nonce，过滤掉过期或顺序错误的合约交易，同区块内多笔同 caller 交易能正确感知前面交易对 nonce 的修改
    - **Proposal 层** [`tee-chain/tx_process.go`](tee-chain/tx_process.go): `ProcessProposal` 阶段在共享 `model.Txn` 中顺序执行：① nonce 校验（感知同区块前面交易）② 写入 nonce ③ `ContractMutation` 预执行合约逻辑。任何一步失败都 `REJECT`，确保进入区块的合约交易既满足 nonce 顺序又不会因合约逻辑失败而浪费区块空间
    - **执行层** [`tee-chain/tx_finalize.go`](tee-chain/tx_finalize.go): `FinalizeBlock` 中再次严格校验 nonce，执行合约并原子更新 nonce；nonce 不匹配时标记 `ExecTxResult{Code: CodeTypeInvalidNonce}` 并 `continue`，绝不 `return error` 卡块

4. **Nonce 存储管理** [`tee-chain/tx_nonce.go`](tee-chain/tx_nonce.go)
    - `caller_nonce:<callerType>:<callerHex>` 作为 PebbleDB key
    - `GetNextCallerNonce()` 本地递增并写入 DB（供交易构造时使用）
    - `TxnGetCurrentCallerNonce()` / `TxnSetCallerNonce()` 在 block transaction 中读写

5. **所有构造 Tx 的地方传入递增 nonce**
   - `graph/contract.go:SubmitContractCall()` — 外部用户合约调用（用户自行管理 nonce）
   - `tee-chain/tx_sign.go:SubmitCallFromNode()` — 节点自签名交易
   - `tee-chain/tx_from_worker.go:signAndBuildRelayTx()` — Relay 同步交易
   - `pkg/api/gateway/optimizer.go:submitSettleUsage()` — 结算交易

6. **nonce 纳入签名数据，防止中间人篡改**
   - 签名数据格式：`caller || caller_type (uint32 BE) || nonce (uint64 BE) || call`
   - 实现：`pkg/model/sig.go:TxBytesForSig()`
   - 验证：`pkg/model/tx_verify.go:VerifyTxSigner()` 使用 `TxBytesForSig(tx)` 验证签名
   - 节点侧（`tx_sign.go`、`tx_from_worker.go`、`optimizer.go`）签名时均使用 `TxBytesForSig(tx)`
   - 用户侧（GraphQL `contractCall`）传入的 nonce 和 signature 必须匹配，服务端用同样数据验证

7. **GraphQL `contractCall` API 要求用户传入 nonce**
   ```graphql
   contractCall(
     caller: String!
     callerType: Int!
     nonce: Int!          # 用户级严格递增 nonce（从 1 开始）
     contract: String!
     method: String!
     args: [String!]!
     signature: String!   # 对 caller || caller_type || nonce || call 的签名
   ): Boolean!
   ```
   - 用户需自行查询当前 nonce，构造交易并签名后提交
   - 服务端仅验证，不替用户分配 nonce

**设计选择说明**:
- 每个 caller（公钥 + callerType）独立维护递增 nonce，天然隔离不同身份
- nonce 从 1 开始，0 表示未初始化
- nonce 写入 block transaction，与合约执行原子提交：合约失败则 nonce 不回滚（防止重放更严格）
- `ProcessProposal` 中增加 `ContractDryRun` 预执行，nonce 合法但合约逻辑会失败的交易在提案阶段即被拒绝，杜绝浪费区块空间
- nonce 纳入签名数据，杜绝中间人替换 nonce 的攻击
- 覆盖范围：所有 `SysCall_Contract` 类型交易
- 非合约交易（EpochStart/End、HubCall 等）不校验 nonce，由各自机制保证唯一性

---

## 三、High（高危）

### HI-1: `SettleUsage` 的 Cost=0 时自动计算完全依赖链下诚实 ✅ 已缓解

**位置**: `tee-chain/contracts/gateway/credit.go:62-69`, `pkg/api/gateway/relay_handler.go:97-98`

**原问题描述**（已缓解）:
- ~~合约自动计算费用的输入 `TokenIn`、`CachedToken`、`TokenOut` 全部来自链下~~
- ~~若链下数据可被 Provider 伪造，则存在少报/多报风险~~

**实际架构设计**（已实施）：

1. **结算数据由 TEE 节点真实统计**，非 Provider 提交
   - `pkg/api/gateway/relay_handler.go:97-98` 在处理 AI 请求时，由 TEE 节点内部调用 `recordUsageAsync()` 统计实际 `PromptTokens`、`CompletionTokens`、`CachedTokens`
   - 用量数据写入本地 PebbleDB（`GatewayDailyUsage`），Provider 无法篡改

2. **结算交易由 System Account 签名提交**
   - `pkg/api/gateway/optimizer.go:278-319` 中 `submitSettleUsage()` 使用 DKG signer（system call, callerType=0）签名
   - `SettleUsage` 合约方法检查 `CallbySystem()`，只有 system account 可调用
   - Provider 无法直接调用结算合约，也无法伪造结算数据

3. **防重放机制**
   - 每条结算记录带唯一随机 16-byte Nonce（`optimizer.go:242-244`）
   - 合约侧 `settledNonces` 永久记录已使用 Nonce（`credit.go:41-48`），杜绝重复结算

**结论**:
- 结算数据的可信度由 TEE 节点保证（网关处理请求时真实统计），非 Provider 提交
- `CallbySystem()` 权限控制确保只有节点可触发结算
- Provider 无法伪造用量，也无法绕过结算流程

---

### HI-2: `Recharge` / `Withdraw` 仅靠 `CallbySystem()` 权限检查 ✅ 已缓解

**位置**: `tee-chain/contracts/gateway/credit.go:12`, `credit.go:114`

**原问题描述**（已缓解）:
- ~~`Recharge` 和 `Withdraw` 只有 sudo 账户可调用，存在单点私钥泄露风险~~

**实际架构设计**（已实施）：

1. **System Account = DKG 分布式密钥**
   - 合约侧 `CallbySystem()` 检查 caller 是否为 system account（`callerType=0` 且地址匹配 DKG 公钥）
   - `system account` 不是单一私钥，而是 DKG（Distributed Key Generation）生成的阈值签名密钥
   - 需要至少 `Threshold` 个 TEE 节点共同签名才能构造有效的 system call

2. **充值/提现交易由多节点共识驱动**
   - `relay_worker.go` 和 `optimizer.go` 中的 system call 均通过 DKG signer 签名
   - 单个节点无法独自构造有效签名，必须达成阈值共识
   - 这天然实现了"多签/阈值"机制，无需额外引入

3. **操作历史链上永久记录**
   - `Recharge` 写入 `rechargeHistory`（`credit.go:27-31`）
   - `Withdraw` 写入 `withdrawHistory`（`credit.go:139-144`）
   - 链上数据可审计，任何异常操作都可追溯

**结论**:
- `CallbySystem()` 的权限控制由 DKG 阈值签名保障，不存在单一私钥泄露风险
- 充值/提现操作需要多节点共识，单点无法作恶
- 链上历史记录提供完整审计能力

**未来增强**（可选）:
- 引入时间锁：大额提现需延迟 N 个区块才可执行
- 增加链下实时审计告警，监控异常大额操作

---

### HI-3: `SettleUsage` 结算频率无链上限制 ✅ 已缓解

**位置**: `tee-chain/contracts/gateway/credit.go:38`, `pkg/api/gateway/optimizer.go:278`

**原问题描述**（已缓解）:
- ~~注释说"每 5 分钟执行一次"，但链上无限制，Provider 可连续调用~~

**实际架构设计**（已实施）：

1. **调用权限由 `CallbySystem()` 控制**
   - `SettleUsage` 只能由 system account（TEE 节点）调用，Provider 无法直接触发结算
   - `submitSettleUsage()` 使用 DKG signer 签名，callerType=0

2. **结算频率由 TEE 节点定时器控制**
   - `pkg/api/gateway/optimizer.go` 中 `settlePendingUsage()` 由节点后台定时触发
   - 结算 goroutine 周期由节点配置决定，Provider 无法加速或绕过

3. **防重放机制杜绝重复结算**
   - 每条记录带唯一随机 Nonce，合约永久记录已使用 Nonce
   - 即使高频调用，同一批记录也无法被重复结算

**结论**:
- Provider 无法自行触发结算，结算节奏完全由 TEE 节点控制
- 链上虽无显式 `lastSettleBlock` 限制，但 `CallbySystem()` 权限已从根本上消除了 Provider 滥用频率的可能

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

### LOW-1: `UpdateModel` 不允许改 name，但 `CreatedAt` 被覆盖 ✅ 已修复

**位置**: `tee-chain/contracts/gateway/model.go:147`

**修复状态**: 已移除 `UpdateModel` 中对 `CreatedAt` 的覆盖。`CreatedAt` 仅在 `CreateModel` 时设置，更新操作不再修改该字段，确保模型首次注册时间永久保留。

**如需增强**（可选）:
- 后续可增加 `UpdatedAt` 字段记录最后更新时间

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
    TokenIn     uint64        `json:"TokenIn"`
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
| P0 | 立即在 `SettleUsage` 增加 `batchNonce` + 合约维护 `usedBatchNonces` | Critical | ✅ 已实施（per-record nonce 方案） |
| P1 | 为所有合约增加用户级 `nonce` 机制（方案 A） | High | ✅ 已实施（Tx 级严格递增 nonce） |
| P2 | `SettleUsage` 增加 TEE 签名验证，确保用量数据真实 | High | ✅ 已实施（CallbySystem + DKG 阈值签名） |
| P3 | 增加链上结算频率限制（`lastSettleBlock`） | Medium | ✅ 已缓解（CallbySystem 控制，TEE 节点定时驱动） |
| P4 | 增加 `RateLimit` 网关层执行 | Medium | ⚠️ 待实施 |
| P5 | 修复 `UpdateModel` 不应覆盖 `CreatedAt` | Low | ✅ 已修复 |

---

## 七、总结

当前合约最严重的安全问题是 **SettleUsage 无防重放** 和 **全局缺少 nonce 机制**。前者可被 Provider 直接利用套利，后者是所有合约交易的底层安全缺陷。建议按 P0→P2 的顺序优先实施，P3→P5 可逐步完善。
