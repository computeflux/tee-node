# optimizer.go 用量统计丢失风险分析

## 用户问题
> 这种内存形式的统计是否会有丢失5分钟使用的肯恩

## 结论：不会丢失5分钟，但存在两类丢失风险

当前 `ticker` 间隔是 **`500 * time.Millisecond`（500毫秒）**，不是5分钟。所以正常运行的最大内存停留时间是 500ms，不会累积到5分钟。

但存在以下两类丢失风险：

### 风险1：程序崩溃/重启时丢失最多 500ms 的数据

```go
ticker := time.NewTicker(500 * time.Millisecond)
```

- `batch` 在内存中最多停留 500ms 才被 `ticker.C` 触发 flush 到 Pebble
- 如果程序在 `ticker` 触发前崩溃，这 500ms 内的 `usageRecord` 全部丢失
- 500ms 的 QPS 下，若 gateway 每秒处理 1000 请求，500ms 内可能丢失 500 条记录

### 风险2：高并发时 channel 满导致数据被直接丢弃

```go
select {
case usageCh <- usageRecord{...}:
default:
    // Provider full, drop (degraded mode)
}
```

- `usageCh` 容量为 4096
- 当并发超过 4096 的瞬时峰值时，新记录直接进入 `default` 分支被丢弃
- 生产环境高并发下，此问题可能比崩溃更频繁

### 风险3：没有优雅停机 flush

- `usageWg` 只用于等待 goroutine 退出，但 goroutine 退出前不会主动 flush `batch`
- 进程收到 SIGTERM 后，500ms 内的数据直接丢失

---

## 修复方案

### 修复1：去掉 `select default` 丢弃逻辑，改为阻塞式写入

```go
// 旧：channel 满直接丢弃
select {
case usageCh <- usageRecord{...}:
default:
    log drop
}

// 新：阻塞写入，永不丢弃
usageCh <- usageRecord{...}
```

为了配合此改动，需要把 channel 容量从 4096 增大到 65536（可缓存数秒的请求）。

### 修复2：缩短 ticker 间隔到 100ms

```go
ticker := time.NewTicker(100 * time.Millisecond)
```

- 将内存中数据的最大停留时间从 500ms 缩短到 100ms
- 即使崩溃，最多也只丢失 100ms 的数据

### 修复3：增加优雅停机 flush

```go
var usageStopCh = make(chan struct{})

func StopUsageWriter() {
    close(usageStopCh)
    usageWg.Wait()
}
```

在 goroutine 中监听 `stopCh`，收到信号后立即 flush batch：

```go
select {
case r := <-usageCh:
    // 聚合到 batch...
case <-ticker.C:
    flush(batch)
case <-usageStopCh:
    flush(batch)
    return
}
```

### 修复4：增加同步 flush 接口

```go
func FlushUsage() {
    // 发送一个特殊信号到 usageCh，触发立即 flush
    usageCh <- usageRecord{flush: true}
}
```

在关键操作前（如结算前）调用 `FlushUsage()`，确保数据已落盘。

### 修复5：聚合 key 精度不够（潜在问题）

当前 key 是 `day + "_" + tokenID`，意味着同一用户的所有模型用量被合并到一条记录。

如果需要按 model 拆分统计，key 应为：
```go
key := r.day + "_" + uintToStr(r.tokenID) + "_" + uintToStr(r.model.ID)
```

---

## 实施计划

| 优先级 | 改动 | 影响 |
|---|---|---|
| P0 | 去掉 `select default` 丢弃逻辑 | 彻底消除 channel 满导致的数据丢失 |
| P0 | channel 容量 4096 → 65536 | 支撑高并发不阻塞 |
| P1 | ticker 500ms → 100ms | 崩溃时最多丢失 100ms 数据 |
| P1 | 增加 `StopUsageWriter()` + 优雅停机 flush | 重启/升级时不丢数据 |
| P2 | 增加 `FlushUsage()` 接口 | 结算前可强制同步落盘 |
| P2 | key 改为 `day_tokenID_modelID` | 按 model 拆分统计 |
