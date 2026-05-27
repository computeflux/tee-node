# 节点合约交易管理器设计

> 最后更新：2026-05-27

---

## 一、设计定位

TxManager **仅服务节点自身的合约调用**（SystemContractInit、Faucet、结算）。
系统交易（Epoch、Sync、HubCall）**不经此管理器**，走 `SubmitCallFromNode` 直接签名提交。

```
合约交易（需要 nonce，通过 TxManager）：
  SystemContractInit / Faucet / SettleUsage
    → TxManager.SubmitContract(contract, method, args, opts)  [入队]
    → 后台 flushLoop 每秒批量处理
    → buildContractTx → GetNodeNextNonce（连续） → signTx → submitBytes
    → FinalizeTx Notify → Executed 删除 / Failed 保留待重试

系统交易（nonce=0，不经 TxManager）：
  Epoch / Sync / HubCall / Empty
    → SubmitCallFromNode → signTx → SubmitTx

> 合约交易失败（nonce 错误、合约逻辑错误）不会回滚整个区块。FinalizeTx 将该笔交易标记为
> Code=1 后 continue，区块正常提交。这使得其它交易不受单笔合约失败影响。
```

---

## 二、核心组件

**文件**：[`tee-chain/tx_manager.go`](../tee-chain/tx_manager.go)

```go
// 将合约调用请求加入队列，返回临时 txHash
func (m *TxManager) SubmitContract(ctx context.Context,
    contract string, method []byte, args [][]byte,
    opts SubmitOpts) (txHash string, err error)

// 通知交易状态变更（由 FinalizeTx 调用）
// Executed → 从 pending 删除
// Failed   → 标记为 failed，下次 flush 自动重试
func (m *TxManager) Notify(event TxEvent)

// 启动后台 flush goroutine
func (m *TxManager) StartFlush(interval time.Duration)
func (m *TxManager) StopFlush()

// 交易哈希（blake2b）
func TxHash(tx *model.Tx) string
```

内部公共工具（`tx_sign.go` 也使用）：

```go
// 签名并构造 Tx
func signTx(call *model.SysCall, signer ink.SignerType) (*model.Tx, error)

// 提交序列化后的交易 bytes 到 mempool
func submitBytes(txBytes []byte) error
```

---

## 三、队列 + 批量 Flush 模式

### 为什么不用"来一笔发一笔"

旧模式的问题：
1. **并发重试竞争 nonce**：每笔交易独立 `retryLoop`，同时 `ResetPendingNonce` + `GetNodeNextNonce`，nonce 分配混乱
2. **CheckTx 级联拒绝**：nonce=1 通过，nonce=2 被 CheckTx 拒绝（DB nonce 仍为 0），每笔启动独立重试 goroutine
3. **效率极低**：5 笔交易产生 4 个 retryLoop，各自 30s 超时 + 退避

### 新模式

```
SubmitContract          flushLoop (单线程)
    ↓                        ↓
  [queue]  ──────────→  合并 pending(failed) + queue
                             ↓
                         逐个 buildContractTx（连续 nonce）
                             ↓
                         submitBytes → mempool
                             ↓
                         移入 pending 等待 Notify
```

- `SubmitContract`：只加锁写入 `queue`，**不构建、不提交**
- `flushLoop`：单线程定时（默认 1s）执行，将 `queue` 和 `pending` 中 `Failed` 的交易合并，按顺序逐个构建连续 nonce 并提交
- `Notify(Executed)`：从 `pending` 删除
- `Notify(Failed)`：保留在 `pending` 中标记为 `Failed`，下次 `flush` 自动重试

### 优势

1. **单线程构建**：只有一个 `flushLoop` 调用 `GetNodeNextNonce`，无并发竞争
2. **连续 nonce**：一批次内 `nonce = DB+1, DB+2, DB+3...`
3. **CheckTx 自然处理**：第一笔通过，后面几笔暂时被拒；但不会丢失——仍在 `pending` 中，下轮 flush 用更新后的 DB nonce 重新构建
4. **无 retryLoop goroutine**：不需要 `watch`、`subscriptions`、timer，复杂度大幅降低

---

## 四、Nonce 管理

**文件**：[`tee-chain/tx_nonce.go`](../tee-chain/tx_nonce.go)

### 分配：纯内存，不写 DB

`GetNodeNextNonce` 通过 `pendingNonces` map 在内存中递增分配 nonce，**不调用 `model.SetKey`**。
DB 中仅保存已提交区块的 nonce 值。Key 前缀为 `call_nonce`。

> 注：`call_nonce` 前缀为重构后的新命名，与旧前缀 `caller_nonce` 不兼容，启动时会自动从 nonce=0 开始重新计数。

```go
// 第一次调用：读 DB 已提交值 (如 0)，返回 1，pendingNonces 记录下次从 2 开始
// 第二次调用：pendingNonces 已有 2，返回 2，记录下次从 3 开始
```

### 校验与提交：走 PebbleDB 事务

`FinalizeTx` 中通过 `TxnGetCurrentCallerNonce` / `TxnSetCallerNonce` 在区块事务（txn）内校验和更新 nonce。
txn 快照不受未提交 nonce 污染，同块多笔同 caller 合约交易可正确通过校验。

```
buildContractTx（内存）                FinalizeTx（txn）                  Commit（DB）
  GetNodeNextNonce → 返回 1              TxnGetCurrentCallerNonce → 0
  GetNodeNextNonce → 返回 2              expected=1, tx1.Nonce=1 ✓         TxnSetCallerNonce → 1
                                          TxnSetCallerNonce → 1
                                          expected=2, tx2.Nonce=2 ✓
                                          TxnSetCallerNonce → 2
```

---

## 五、使用示例

### 5.1 合约交易

```go
method := model.MethodToSelectorBytes("Init")
txHash, err := chain.TxMgr.SubmitContract(ctx, "gov", method, args,
    teechain.DefaultSubmitOpts())
```

### 5.2 结算交易（更多重试）

```go
txHash, err := chain.TxMgr.SubmitContract(ctx, "gateway",
    model.MethodToSelector("SettleUsage")[:], args,
    teechain.SubmitOpts{MaxRetries: 5, BaseDelay: 3 * time.Second})
```

### 5.3 系统交易

```go
call := &model.SysCall{Payload: &model.SysCall_EpochStart{...}}
err := chain.SubmitCallFromNode(call)
```

---

## 六、调用点

| 函数 | 路径 | 类型 |
|------|------|------|
| `SystemContractInit` | `TxManager.SubmitContract(ctx, contract, "Init", args, opts)` | 合约 |
| `Faucet` | `TxManager.SubmitContract(ctx, "gov", "Mint", args, opts)` | 合约 |
| `submitSettleUsage` | `TxManager.SubmitContract(ctx, "gateway", "SettleUsage", args, opts)` | 合约 |
| `SubmitCallFromNode` | `signTx` + `SubmitTx` | 系统 |
| `GetCallBytesFromNode` | `signTx`（DKG）+ `GetTxBytes` | 系统 |
| `SignAndBuildRelayTx` | `signTx`（DKG）+ `GetTxBytes` | 系统 |

---

## 七、初始化与回调

```go
c.TxMgr = NewTxManager(c)
c.TxMgr.StartFlush(1 * time.Second) // 每秒批量 flush 队列中的交易
```

```go
// 成功
app.TxMgr.Notify(TxEvent{TxHash: txHash, Status: TxStatusExecuted, ...})
// 失败
app.TxMgr.Notify(TxEvent{TxHash: txHash, Status: TxStatusFailed, ...})
```

---

## 八、重试策略

```
SubmitContract → 加入 queue
flushLoop 每秒执行：
  1. 收集 pending(Failed) + queue
  2. 逐个 buildContractTx（连续 nonce）
  3. submitBytes
  4. 成功 → 移入 pending 等待 Notify
  5. 失败 → 移入 pending 标记 Failed，下次 flush 重试

Notify(Executed) → 从 pending 删除
Notify(Failed)   → 保留在 pending，RetryCount++，下次 flush 重试
MaxRetries 耗尽  → 丢弃，打印日志
```

- `MaxRetries` 控制总尝试次数（含初始提交）
- 结算交易建议 `MaxRetries=5`
- 无退避延迟：flush 固定间隔（1s），失败后下轮立即重试

---

## 九、文件清单

| 文件 | 职责 |
|------|------|
| `tx_manager.go` | 队列 + 批量 flush + 单线程提交 + `signTx`/`TxHash` 工具 |
| `tx_nonce.go` | Nonce 内存分配 + DB 读写 |
| `tx_sign.go` | 系统交易提交 + `GetTxBytes`/`submitBytes`/`SubmitTx` |
| `tx_finalize.go` | FinalizeTx nonce 校验 + `TxManager.Notify` 回调 |

---

## 十、修改记录

### 2026-05-27 重写 TxManager 为队列 + 批量 Flush 模式

| # | 问题 | 文件 | 修复内容 |
|---|------|------|----------|
| 1 | 并发重试竞争 nonce | `tx_manager.go` | 删除 retryLoop/subscriptions/GC，改为 `queue` + 单线程 `flushLoop` |
| 2 | CheckTx 级联拒绝 | `tx_manager.go` | 单线程顺序构建，nonce 连续；CheckTx 暂时拒绝的交易在 pending 中保留，下轮 flush 重试 |
| 3 | timer goroutine 泄漏 | `tx_manager.go` | 删除 `time.After`/`watch`/`subscriptions`，只剩一个 `time.Ticker` |
| 4 | GC 复杂且不完备 | `tx_manager.go` | 删除 GC；pending 记录在 `MaxRetries` 耗尽后自动删除 |
| 5 | 调用点无需改动 | `contract.resolvers.go`/`optimizer.go` | `SubmitContract` 签名不变，调用方无感 |

### 2026-05-26 修复 5 个逻辑 Bug（已归档）

| # | 问题 | 文件 |
|---|------|------|
| 1 | 合约交易失败回滚整个区块 | `tx_finalize.go` |
| 2 | retryLoop 重试退出逻辑混乱 | `tx_manager.go` |
| 3 | GC 不清理 Pending 死记录 | `tx_manager.go` |
| 4 | 最后一次重试无人跟踪 | `tx_manager.go` |
| 5 | pendingNonces 无清理 | `tx_nonce.go` + `tx_manager.go` |
| 6 | GC 不清理 subscriptions | `tx_manager.go` |

**验证**：所有修改文件均通过 `gofmt` 格式检查。
