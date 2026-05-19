# 主链 → 侧链 Relay 同步 · 审计文档

> 当前: **Phase 1** — 信任 RPC, nonce 防重放  
> 计划: **Phase 2** — header 同步 + Merkle proof

## 一、全景架构 (缓存 + PrepareProposal 驱动)

```
┌─ 主链 (Polkadot/wrevive) ──────────────────────────────────────────┐
│  Token 合约: 每条 mutation 末尾 emit EventRecord(nonce, type, data)  │
│  Query: get_latest_nonce() / get_event(nonce) / get_chain_id()      │
└─────────────────────────────────────────────────────────────────────┘
        │
        │  ChainApi.GetLatestNonce() / GetRelayEvent(nonce)
        │  所有节点后台 Worker 每 3 秒拉取
        ▼
┌─ 所有节点 (后台 goroutine, 每 3 秒) ────────────────────────────────┐
│                                                                     │
│  fetchAndCacheRelayEvents():                                         │
│    for each chain:                                                   │
│      latest = chain.GetLatestNonce()                                │
│      for n = lastCached+1; n <= latest; n++:                        │
│        target, typ, data = chain.GetRelayEvent(n)                   │
│        ev = BuildSyncEvent(...)                                      │
│        sysCall = MakeRelaySysCall(ev)                                │
│        txBytes = DKG.Sign(sysCall)            ← 预签名              │
│        cache[chainId].append(ev, now, txBytes)                       │
│                                                                     │
│  所有节点本地缓存主链事件                                             │
└─────────────────────────────────────────────────────────────────────┘
        │
        │  出块节点读缓存, 取 ≥2s 成熟事件
        ▼
┌─ 出块节点 (PrepareProposal) ───────────────────────────────────────┐
│                                                                     │
│  prepareRelayTxs():                                                  │
│    for each chain:                                                   │
│      for each cached event (按 nonce 序):                            │
│        nonce == lastNonce+1 ?                                        │
│        cachedAge >= 2s ?           ← 确保其他节点也拿到了            │
│        → 打包 txBytes 进区块                                         │
│        → 不成熟则 break (后续更不可能)                                │
│                                                                     │
│  打包普通 mempool 交易 → 广播区块                                     │
└─────────────────────────────────────────────────────────────────────┘
        │
        │  区块广播到所有验证者
        ▼
┌─ 所有节点 (ProcessProposal) ───────────────────────────────────────┐
│                                                                     │
│  verifyRelayTx(txbt):                                                │
│    解码 SysCall_RelaySync                                            │
│    CheckRelayEvent(ev)      ← Phase 1: 仅 nonce 校验                │
│    失败 → REJECT 区块                                                │
└─────────────────────────────────────────────────────────────────────┘
        │
        ▼
┌─ 所有节点 (FinalizeTx) ────────────────────────────────────────────┐
│                                                                     │
│  case *SysCall_RelaySync:                                           │
│    ApplyRelayEvent(ev)     ← nonce 写入 + dispatch 合约              │
│                                                                     │
│  case *SysCall_Contract:                                            │
│    ContractMutation(...)   ← 普通合约调用 (不变)                     │
└─────────────────────────────────────────────────────────────────────┘
```

## 二、设计决策

| 决策 | 方案 | 原因 |
|---|---|---|
| 谁拉取事件 | **所有节点**后台拉取 → 本地缓存 | 出块时零网络 IO |
| 谁创建 relay tx | **出块节点** in PrepareProposal | 零竞争 |
| 成熟度 | **≥2s** 缓存才打包 | 确保其他验证者也拿到了 |
| 验证在哪 | ProcessProposal | 区块内校验，无效则拒绝 |
| 签名 | DKG signer, 拉取时预签名 | 出块时零开销 |
| Phase 1 安全 | 信任 RPC + nonce 防重放 | 主链 RPC 自己的 full node |
| 批量查询 | 🔲 TODO | 当前逐条 GetRelayEvent |

## 三、各阶段 relay 行为

| 阶段 | 谁执行 | 网络 IO | 操作 |
|---|---|---|---|
| 后台 Worker | 所有节点 | ✅ 拉取主链 | 缓存 + 预签名 |
| PrepareProposal | 出块节点 | ❌ 读缓存 | 取成熟事件打包 |
| ProcessProposal | 所有节点 | ❌ | nonce 验证 → 拒绝无效 |
| FinalizeTx | 所有节点 | ❌ | nonce 写入 + dispatch |
| Commit | 所有节点 | ❌ | 无 |

## 四、安全保证

| 保证 | Phase 1 | Phase 2 |
|---|---|---|
| 防重放 | ✅ nonce 严格递增 | ✅ |
| 防乱序 | ✅ nonce == last+1 | ✅ |
| 防伪造 | ❌ 信任 RPC | ✅ Merkle proof |
| 防篡改 | ✅ 签名 + 共识 | ✅ |
| 出块节点作恶 | ✅ ProcessProposal 拒绝 | ✅ |

## 五、文件清单

| 组件 | 文件 | 状态 |
|---|---|---|
| 主链合约 | `revive-contract/revives/Token/src/contract.rs` | ✅ |
| Proto 定义 | `tee-provider/pkg/model/tx.proto` | ✅ SysCall_RelaySync |
| Relay 引擎 | `tee-provider/tee-chain/relay/engine.go` | ✅ Phase 1 |
| ChainApi 接口 | `tee-provider/pkg/chains/mod.go` | ✅ |
| revive adapter | `tee-provider/pkg/chains/revive/relay.go` | ✅ RPC |
| evm adapter | `tee-provider/pkg/chains/evm/token.go` | 🔲 stub |
| 缓存 Worker | `tee-provider/tee-chain/relay_worker.go` | ✅ 缓存层 |
| PrepareProposal | `tee-provider/tee-chain/consensus.go` | ✅ 集成 |
| ProcessProposal | `tee-provider/tee-chain/consensus.go` | ✅ verifyRelayTx |
| FinalizeTx | `tee-provider/tee-chain/tx_finalize.go` | ✅ SysCall_RelaySync |
| SideChain 初始化 | `tee-provider/tee-chain/chains.go` | ✅ engine + worker |

## 六、待完成

| 优先级 | 项目 | 文件 |
|---|---|---|
| P1 | 主链合约 `get_events(from,to)` 批量查询 | `contract.rs` |
| P1 | revive adapter 批量 RPC 调用 | `relay.go` |
| P1 | 缓存旧事件清理 (已处理的 nonce) | `relay_worker.go` |
| P2 | Header 同步 + Merkle proof (Phase 2) | `engine.go` |
| P3 | EVM Token 合约 relay 支持 | EVM 合约 |
