# optimizer.go 用量统计丢失风险分析

> 最后更新：2026-06-20
> 状态：已修复 — channel 容量 65536、阻塞写入、ticker 100ms；`StopUsageWriter` / `FlushUsage` 未实施并已删除

## 结论

当前 `ticker` 间隔是 **`100 * time.Millisecond`（100毫秒）**，channel 容量为 **65536**（约可缓存数秒请求），且采用**阻塞式写入**（不会丢弃）。

正常运行的最大内存停留时间是 100ms，channel 可缓存大量请求不丢数据。

但存在以下风险：

### 风险1：程序崩溃/重启时丢失最多 100ms 的数据

```go
ticker := time.NewTicker(100 * time.Millisecond)
```

- `batch` 在内存中最多停留 100ms 才被 `ticker.C` 触发 flush 到 Pebble
- 如果程序在 `ticker` 触发前崩溃，这 100ms 内的 `usageRecord` 全部丢失
- 100ms 的 QPS 下，若 gateway 每秒处理 1000 请求，100ms 内可能丢失 100 条记录

### 风险2（已修复）：channel 满导致丢弃

```go
// 旧：channel 满直接丢弃（4096 容量）
select {
case usageCh <- usageRecord{...}:
default:
    // Provider full, drop (degraded mode)
}

// 新：阻塞写入，永不丢弃（65536 容量）
usageCh <- usageRecord{...}
```

- `usageCh` 容量已增大到 **65536**
- 阻塞式写入，channel 满时 goroutine 会等待而不是丢弃
- 生产环境高并发下不再丢数据

### 风险3：没有优雅停机 flush（当前状态）

- `usageWg` 只用于等待 goroutine 退出，但 goroutine 退出前不会主动 flush `batch`
- 进程收到 SIGTERM 后，100ms 内的数据直接丢失

---

## 修复方案（已实施 + 待实施）

### 修复1：去掉 `select default` 丢弃逻辑，改为阻塞式写入 ✅

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

### 修复2：channel 容量 4096 → 65536 ✅

```go
usageCh = make(chan usageRecord, 65536)
```

### 修复3：缩短 ticker 间隔到 100ms ✅

```go
ticker := time.NewTicker(100 * time.Millisecond)
```

### 修复4：增加优雅停机 flush（不再实施）

原计划增加 `StopUsageWriter()` 监听停止信号并在退出前 flush，但该函数未被任何调用方使用，已在死代码清理中删除。当前仍依赖 100ms ticker 落盘，崩溃/重启时最多丢失 100ms 数据。

### 修复5：增加同步 flush 接口（不再实施）

原计划增加 `FlushUsage()` 在关键操作前强制落盘，但该函数同样未被使用，已删除。

### 修复6：聚合 key 精度不够（潜在问题）

当前 key 是 `tokenID + "_" + modelID`，意味着同一用户的所有 Provider 用量被合并到一条记录。

结算时按 `ProviderID` 区分，但聚合 key 本身不包含 ProviderID。由于 `modelID` 天然属于特定 `ProviderID`，实际不影响结算准确性。

---

## 实施计划

| 优先级 | 改动 | 状态 |
|---|---|---|
| P0 | 去掉 `select default` 丢弃逻辑 | ✅ 已完成 |
| P0 | channel 容量 4096 → 65536 | ✅ 已完成 |
| P1 | ticker 500ms → 100ms | ✅ 已完成 |
| P1 | 增加 `StopUsageWriter()` + 优雅停机 flush | ❌ 不再实施（函数已删除） |
| P2 | 增加 `FlushUsage()` 接口 | ❌ 不再实施（函数已删除） |

---

*文档更新：2026-06-20*
