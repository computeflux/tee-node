# 交易重试机制说明

## 一、重试触发流程

### 1.1 重试触发时机

重试通过以下方式触发（**基于区块触发，不使用定时器**）：

```
┌─────────────────────────────────────────────────────────┐
│  1. 交易提交失败 (SyncToHub)                            │
│     ↓                                                    │
│  2. 判断错误类型 (IsRetryableError)                     │
│     ↓                                                    │
│  3. 更新状态为 FAILED，保存错误信息                     │
│     ↓                                                    │
│  4. 每个区块 PrepareProposal 时触发检查                │
│     ↓                                                    │
│  5. 检查失败交易是否满足重试条件                        │
│     ↓                                                    │
│  6. 计算重试延迟（指数退避，基于时间）                  │
│     ↓                                                    │
│  7. 执行重试 (retrySubmitTx)                            │
│     ↓                                                    │
│  8. 重新调用 SyncToHub                                  │
└─────────────────────────────────────────────────────────┘
```

### 1.2 详细流程

#### 步骤1: 交易提交失败

在 `SyncToHub` 函数中，当交易提交到主链失败时：

```go
// side-chain/tx_sync.go:64-84
err = client.SignAndSubmit(signer, *call, false, 0)
if err != nil {
    // 更新状态为 FAILED
    status.Status = TxStatusFailed
    status.LastError = err.Error()
    status.RetryCount++
    SaveTxStatus(status)
    
    // 如果是可重试错误，等待重试
    if IsRetryableError(err) {
        util.LogWithYellow("SyncToHub", "retryable error, will retry later")
        return err
    }
}
```

**关键点**:
- 状态更新为 `FAILED`
- 错误信息保存到 `LastError`
- 重试次数 `RetryCount` 递增
- 如果是可重试错误，不调用 `SyncEnd`，等待重试

#### 步骤2: 重试管理器创建

在侧链初始化时，重试管理器被创建（**不再启动后台goroutine**）：

```go
// side-chain/side_chain.go:148-153
if !light {
    // 创建重试管理器（不再使用定时器，改为在区块处理时触发）
    sideChain.retryManager = NewRetryManager(sideChain)
}
```

**关键点**:
- 重试管理器在侧链启动时创建，但不启动后台goroutine
- **不再使用 time.Ticker**
- 重试检查在区块处理时触发

#### 步骤3: 区块触发检查

在每个区块的 `PrepareProposal` 时触发重试检查：

```go
// side-chain/consensus.go:PrepareProposal()
func (app *SideChain) PrepareProposal(...) {
    // 在每个区块准备时，检查并重试失败的交易
    if app.retryManager != nil {
        app.retryManager.CheckAndRetryFailedTxs()
    }
    // ... 其他区块准备逻辑
}
```

**触发时机**:
- 每个区块准备提案时触发（`PrepareProposal`）
- 与区块共识流程同步，无需额外的定时器
- 扫描范围：从 `AsyncBatchState.Done` 到 `AsyncBatchState.Going`
- 限制扫描最近100个交易（避免性能问题）
- 只扫描状态为 `FAILED` 的交易

#### 步骤4: 重试条件检查

对每个失败的交易，检查是否满足重试条件：

```go
// side-chain/tx_retry.go:shouldRetry()
func (rm *RetryManager) shouldRetry(status *TxSubmissionStatus) bool {
    // 1. 状态必须是 FAILED
    if status.Status != TxStatusFailed {
        return false
    }
    
    // 2. 重试次数不能超过最大值（10次）
    if status.RetryCount >= MaxRetryCount {
        return false
    }
    
    // 3. 错误必须是可重试的
    if !IsRetryableError(fmt.Errorf(status.LastError)) {
        return false
    }
    
    return true
}
```

**重试条件**:
- ✅ 状态为 `FAILED`
- ✅ 重试次数 < 10
- ✅ 错误是可重试类型

#### 步骤5: 计算重试延迟（指数退避）

使用指数退避策略计算重试延迟：

```go
// side-chain/tx_retry.go:calculateRetryDelay()
func (rm *RetryManager) calculateRetryDelay(retryCount int) time.Duration {
    // 指数退避: delay = initialDelay * (backoffFactor ^ retryCount)
    delay := float64(InitialRetryDelay) * math.Pow(RetryBackoffFactor, float64(retryCount))
    
    // 限制最大延迟为5分钟
    if delay > float64(MaxRetryDelay) {
        delay = float64(MaxRetryDelay)
    }
    
    return time.Duration(delay)
}
```

**重试延迟表**:
| 重试次数 | 延迟时间 |
|---------|---------|
| 1       | 5秒     |
| 2       | 10秒    |
| 3       | 20秒    |
| 4       | 40秒    |
| 5       | 80秒    |
| 6       | 160秒   |
| 7       | 300秒（5分钟，达到最大值）|
| 8+      | 300秒（保持最大值）|

#### 步骤6: 执行重试

如果满足重试条件且已到重试时间，执行重试：

```go
// side-chain/tx_retry.go:retrySubmitTx()
func (rm *RetryManager) retrySubmitTx(txIndex int64, sigs [][]byte) error {
    // 如果没有签名，重新获取
    if len(sigs) == 0 {
        sigList, err := rm.sideChain.SigListOfTx(txIndex)
        // ... 获取签名
    }
    
    // 重新调用 SyncToHub
    return rm.sideChain.SyncToHub(txIndex, sigs)
}
```

**重试过程**:
1. 更新状态为 `RETRYING`
2. 重新获取签名（如果需要）
3. 调用 `SyncToHub` 重新提交
4. 如果成功，状态更新为 `SUBMITTING` 或 `CONFIRMED`
5. 如果失败，状态更新为 `FAILED`，`RetryCount` 递增

## 二、可重试错误类型

### 2.1 错误判断

```go
// side-chain/tx_status.go:IsRetryableError()
func IsRetryableError(err error) bool {
    errStr := err.Error()
    
    retryableErrors := []string{
        "timeout",              // 超时错误
        "connection",           // 连接错误
        "network",              // 网络错误
        "temporarily unavailable", // 暂时不可用
        "rate limit",           // 限流
        "server error",         // 服务器错误
        "unavailable",          // 不可用
    }
    
    // 检查错误信息中是否包含可重试关键词
    for _, retryable := range retryableErrors {
        if containsIgnoreCase(errStr, retryable) {
            return true
        }
    }
    
    return false
}
```

### 2.2 可重试 vs 不可重试

**可重试错误**（会自动重试）:
- 网络超时
- 连接中断
- 主链暂时不可用
- 限流错误
- 服务器临时错误

**不可重试错误**（不会重试）:
- 交易格式错误
- 签名错误
- 权限错误
- 业务逻辑错误
- 账户余额不足（需要用户处理）

## 三、重试配置

### 3.1 配置参数

```go
const (
    MaxRetryCount      = 10              // 最大重试次数
    InitialRetryDelay  = 5 * time.Second // 初始重试延迟
    MaxRetryDelay      = 5 * time.Minute // 最大重试延迟
    RetryBackoffFactor = 2.0             // 退避因子（指数退避）
)
```

### 3.2 配置说明

- **MaxRetryCount**: 最大重试10次，超过后不再重试
- **InitialRetryDelay**: 第一次重试延迟5秒
- **MaxRetryDelay**: 最大延迟5分钟，避免无限增长
- **RetryBackoffFactor**: 指数退避因子2.0，每次延迟翻倍
- **触发方式**: 每个区块的 `PrepareProposal` 时触发检查（不再使用定时器）

## 四、重试状态流转

```
PENDING → SIGNING → SIGNED → SUBMITTING → FAILED
                                              ↓
                                         RETRYING
                                              ↓
                                         SUBMITTING (重试)
                                              ↓
                                    ┌─────────┴─────────┐
                                    ↓                   ↓
                              CONFIRMED            FAILED (重试失败)
                                                         ↓
                                                    (重试次数+1)
                                                         ↓
                                                    (继续等待重试)
```

## 五、监控和日志

### 5.1 日志输出

重试管理器会输出以下日志：

- **启动**: `RetryManager Start`
- **扫描**: `found N failed transactions to retry`
- **重试**: `retrying tx N, retry count: M`
- **成功**: `retry success for tx N`
- **失败**: `retry failed for tx N: error`
- **超限**: `tx N exceeded max retry count`

### 5.2 状态查询

可以通过以下方式查询交易状态：

```go
status, err := LoadTxStatus(txIndex)
// status.Status: 当前状态
// status.RetryCount: 重试次数
// status.LastError: 最后错误信息
```

## 六、总结

### 6.1 重试触发方式

1. **区块触发**: 每个区块的 `PrepareProposal` 时自动检查失败的交易（不再使用定时器）
2. **条件触发**: 只有满足重试条件的交易才会被重试
3. **延迟触发**: 使用指数退避策略（基于时间），避免频繁重试

### 6.2 重试保障

- ✅ **状态持久化**: 所有状态保存到数据库，系统重启不丢失
- ✅ **幂等性**: 重试不会导致重复提交
- ✅ **智能延迟**: 指数退避避免对主链造成压力
- ✅ **错误分类**: 只重试可恢复的错误
- ✅ **重试上限**: 防止无限重试

### 6.3 注意事项

1. **重试触发时机**: 每个区块准备时触发，与区块共识流程同步
2. **重试有上限**: 最多重试10次
3. **需要签名**: 重试需要重新获取部分签名
4. **延迟控制**: 使用时间延迟（指数退避），确保不会过于频繁重试
5. **主链状态**: 重试前不会检查主链是否已有该交易（可以后续优化）

