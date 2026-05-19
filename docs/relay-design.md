# 主链 → 侧链 Relay 同步 · 完整审计文档

## 一、全景架构 (交易驱动)

```
┌─ 主链 (Polkadot/wrevive) ────────────────────────────────────────────┐
│  Token 合约: 每条 mutation 末尾 emit EventRecord(nonce, type, data)    │
│  Query: get_latest_nonce() / get_event(nonce) / get_chain_id()        │
└───────────────────────────────────────────────────────────────────────┘
        │
        │  ChainApi.GetLatestNonce() / GetRelayEvent(nonce)
        ▼
┌─ 后台 Worker (独立 goroutine) ────────────────────────────────────────┐
│  for each chain:                                                      │
│    latest = chain.GetLatestNonce()                                    │
│    for n = lastSeen+1; n <= latest; n++:                             │
│      target, typ, data = chain.GetRelayEvent(n)                       │
│      tx = buildContractTx(target, typ, data)  ← 包装成 SysCall_Contract│
│      signAndBroadcast(tx)                    ← 提交 mempool           │
└───────────────────────────────────────────────────────────────────────┘
        │
        │  mempool 广播
        ▼
┌─ 出块节点 (PrepareProposal) ─────────────────────────────────────────┐
│  打包 mempool 中的 relay tx 进区块                                      │
└───────────────────────────────────────────────────────────────────────┘
        │
        │  区块广播到所有验证者
        ▼
┌─ 所有节点 (ProcessProposal / FinalizeTx) ─────────────────────────────┐
│                                                                       │
│  case *SysCall_Contract:                                              │
│    if isRelayTx(tx):                                                  │
│      engine.CheckRelayEvent(ev)   ← nonce + Merkle proof 验证         │
│      engine.ApplyRelayEvent(ev)   ← nonce 写入 + dispatch 合约         │
│    else:                                                              │
│      app.ContractMutation(...)    ← 普通合约调用                      │
│                                                                       │
│  每个节点独立验证 → 结果一致 → 区块确认                                  │
└───────────────────────────────────────────────────────────────────────┘
```

## 二、安全保证

| 保证 | 机制 |
|---|---|
| **去中心化验证** | 每笔 relay tx 被所有验证者独立执行 `CheckRelayEvent` |
| **防伪造** | Merkle proof 必须匹配链上 header |
| **防重放** | nonce 严格递增，已处理的 nonce 状态不可覆盖 |
| **防篡改** | tx 签名 + 区块共识 |
| **一致性** | 所有节点执行相同 tx，确定性结果 |

## 三、引擎 API

```go
// CheckTx 阶段: 验证 relay 事件
engine.CheckRelayEvent(ev SyncEvent) error
  ├─ nonce == lastNonce+1 ?
  └─ Merkle proof 匹配 header ?

// FinalizeTx 阶段: 执行 relay 事件
engine.ApplyRelayEvent(ev SyncEvent) error
  ├─ nonce 校验 (二次)
  ├─ 更新 lastNonce
  └─ api.Call(target, method, args)

// 后台 Worker: 构建事件
BuildSyncEvent(chainId, nonce, blockNum, target, typ, data) SyncEvent
```

## 四、文件清单

| 组件 | 文件 | 状态 |
|---|---|---|
| 主链合约 | `revive-contract/revives/Token/src/contract.rs` | ✅ |
| Relay 引擎 | `tee-provider/side-chain/relay/engine.go` | ✅ |
| ChainApi 接口 | `tee-provider/pkg/chains/mod.go` | ✅ |
| revive adapter | `tee-provider/pkg/chains/revive/token.go` | ✅ RPC 实现 |
| SideChain 集成 | `tee-provider/side-chain/consensus.go` | ✅ relay 字段 |
| SideChain 集成 | `tee-provider/side-chain/chains.go` | ✅ engine 初始化 |
| 后台 Worker | 🔲 TODO | 待实现 |

## 五、待完成

| 优先级 | 项目 |
|---|---|
| P1 | 后台 Worker: 轮询主链 → 构造 ContractCall tx → 提交 mempool |
| P1 | CheckTx 集成: `isRelayTx` 判断 + `engine.CheckRelayEvent` |
| P1 | FinalizeTx 集成: relay tx 走 `engine.ApplyRelayEvent` |
| P2 | relay tx 签名 (哪个 key 签?) |
| P2 | EventData 解包 (SCALE tuple → 合约 args) |
