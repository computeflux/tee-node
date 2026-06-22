# 用量统计方案对比：Channel 缓冲 vs 直接落盘 + 定时结算

## 用户提案

> 直接写入到 store，5分钟一次制作链上交易，会不会更好，这样可以避免使用 usageCh

## 当前方案（已修复后）

```
请求 → recordUsageAsync → usageCh → goroutine(batch聚合,100ms flush) → PebbleDB
```

- 每次请求只发送一个 struct 到 channel（约 40 字节 + 指针）
- goroutine 每 100ms 批量 flush 到 PebbleDB
- **崩溃时最多丢失 100ms 的数据**

## 用户提案方案

```
请求 → model.SetJson("usage:key", val) → PebbleDB（同步落盘）
                 ↓
         每5分钟后台 goroutine
                 ↓
    扫描 PebbleDB 中所有 usage 前缀记录
                 ↓
    按 Provider 聚合为 []Settlement
                 ↓
    构造侧链交易调用 SettleUsage
                 ↓
    上链成功后删除本地已结算记录
```

## 方案对比

| 维度 | 当前方案（Channel缓冲） | 用户提案（直接落盘+定时结算） |
|---|---|---|
| **单次请求延迟** | ~0μs（仅 channel 发送） | ~1-5ms（JSON序列化+加密+写Pebble） |
| **高并发性能** | 非常好，channel 可缓存 65536 条 | 受 Pebble 写带宽限制，可能成为瓶颈 |
| **崩溃数据丢失** | 最多丢失 100ms | **不丢失任何数据** |
| **结算准确性** | 依赖 flush 时机，可能和实际时间有 100ms 偏差 | 精确，store 中有完整记录 |
| **代码复杂度** | 低，一个 goroutine + map | 中，需要定时扫描 + 聚合 + 构造交易 + 清理 |
| **多节点部署** | 每个节点独立缓冲，各自 flush | 需要锁/协调，否则多个节点重复结算 |
| **结算频率** | 数据实时落盘，结算可任意时刻触发 | 固定5分钟扫描一次 |
| **清理逻辑** | 无（数据一直保留在 Pebble） | 上链成功后需删除本地记录，否则重复结算 |

## 关键问题

### 1. 高并发下同步写 Pebble 的性能

```go
model.SetJson("GATEWAY", "usage:day_tokenID_modelID", &usage)
```

每次调用需要：
1. JSON 序列化 `GatewayDailyUsage`
2. `util.Seal` 加密（AES-GCM）
3. Pebble `Set`（WAL + MemTable）

在高并发（1000 QPS）下，这会带来明显的 CPU 和 IO 开销。

**建议优化**：
- 关闭 `SetJson` 的加密（`util.Seal`），改为明文存储 — 用量数据不敏感
- 或使用批量 `Set` 接口（Pebble WriteBatch）

### 2. 多节点重复结算

如果有多个 TEE 节点同时运行网关：
- 每个节点都有自己的 PebbleDB（本地存储）
- 每个节点每5分钟扫描自己的用量并上链
- 同一批用量可能被多个节点重复结算

**解决方案**：
- **方案 A**：只有 Leader 节点做结算（需要选举机制）
- **方案 B**：每个节点结算自己处理的用量，侧链通过 nonce 去重（已在安全审计中设计）
- **方案 C**：用量记录中包含节点标识，侧链按节点去重

### 3. 结算后清理

```go
// 结算成功后删除本地记录
for _, key := range settledKeys {
    model.SECRET_DB.Delete([]byte(comboKey("GATEWAY", key)))
}
```

- 清理操作需要在结算交易 Finalize 后才执行
- 如果结算交易失败，记录不能删除
- 需要维护一个"待结算"和"已结算"的状态机

## 混合方案推荐（当前实际实现）

```
请求 → recordUsageAsync → usageCh（仅做聚合，不落盘）
                                 ↓
                     goroutine 每 100ms flush 到 PebbleDB（阻塞写入，channel 容量 65536）
                                 ↓
                     PebbleDB 中保存完整用量记录
                                 ↓
                     每5分钟后台扫描 → 构造 Settlement → 上链
                                 ↓
                     上链成功后删除本地记录
```

**优点**：
1. 请求路径走 channel（零延迟）
2. goroutine 每 100ms 批量 flush，channel 容量 65536，阻塞写入（不丢弃）
3. 崩溃时最多丢失 100ms 数据
4. 5分钟定时结算从 PebbleDB 读取，数据完整

## 结论

用户提案的核心优势是**崩溃不丢数据**，但会带来**每次请求同步写盘的性能开销**。

如果当前系统的 QPS < 100/s，直接落盘完全可行，且代码更简单。如果 QPS > 1000/s，建议使用上述**混合方案**：channel 做请求路径缓冲，goroutine 定期（如 5秒）批量 flush 到 PebbleDB，再定时（5分钟）扫描上链。

**当前状态**：当前系统已采用上述混合方案（channel 缓冲 + 100ms flush + 5分钟定时结算），并针对以下风险做了修复：
- channel 容量 4096 → 65536
- 去掉 `select default` 丢弃逻辑，改为阻塞写入
- ticker 间隔缩短到 100ms

**实施优先级**（历史建议）：
- P0: 如果系统 QPS 不高，可直接改为同步落盘
- P1: 如果 QPS 高，保持 channel 方案，增加 5分钟定时扫描 PebbleDB 上链的后台任务
