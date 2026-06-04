# 主链 → 侧链 Relay 同步 · 审计文档

> 当前: **Phase 1** — 信任 RPC, nonce 防重放  
> 计划: **Phase 2** — header 同步 + Merkle proof

## 一、全景架构 (缓存 + PrepareProposal 驱动)

```
┌─ 主链 (Polkadot/wrevive) ──────────────────────────────────────────┐
│  Token 合约: 仅在 Recharge / Withdraw 时 emit EventRecord            │
│  emit_event 数据格式: Vec<Vec<u8>> (独立 SCALE 编码参数)              │
│  参数: [Encode(UniAddr{t:3, v:user}), Encode(points)]               │
│  Query: get_latest_nonce() / get_events(from,to)                    │
└─────────────────────────────────────────────────────────────────────┘
        │
        │  ChainApi.GetLatestNonce() / GetRelayEvents(from,to)
        │  所有节点后台 Worker 每 3 秒拉取 (批量)
        ▼
┌─ 所有节点 (后台 goroutine, 每 3 秒) ────────────────────────────────┐
│                                                                     │
│  fetchAndCacheRelayEvents():                                         │
│    for each chain in chains.Chains:                                  │
│      latest = chain.GetLatestNonce()                                │
│      if latest == 0: continue                                       │
│      records = chain.GetRelayEvents(start, latest)   ← 批量拉取     │
│      for each rec:                                                   │
│        n = start + i                                                 │
│        ev = BuildSyncEvent(chainID, n, rec)                          │
│        sysCall = MakeRelaySysCall(ev)                                │
│        txBytes = DKG.Sign(sysCall)            ← 预签名              │
│        cache[chainId].append(ev, now, txBytes)                       │
│                                                                     │
│  全链路 util.Log* 日志: 拉取/缓存/打包/验证/dispatch                  │
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
│  verifyRelayTx(txbt, chainExpected):                                 │
│    解码 SysCall_RelaySync                                            │
│    dbLast = LastNonce(chainID)       ← 读已提交 DB                  │
│    expected = max(dbLast+1, chainExpected[chainID])                  │
│    ev.Nonce == expected ?                                           │
│    → chainExpected[chainID] = expected+1   ← 区块内 nonce 跟踪      │
│    失败 → REJECT 区块                                                │
│                                                                     │
│  ProcessTx():                                                        │
│    switch SysCall type:                                              │
│      case *SysCall_RelaySync: 放行 ✓                                 │
│      case *SysCall_Contract:   放行                                  │
│      ...                                                             │
└─────────────────────────────────────────────────────────────────────┘
        │
        ▼
┌─ 所有节点 (FinalizeTx) ────────────────────────────────────────────┐
│                                                                     │
│  case *SysCall_RelaySync:                                           │
│    ApplyRelayEvent(ev)  ← txn 内 nonce 校验 + 写入 + dispatch       │
│    dispatch 用 model.MethodToSelectorBytes 转 Method 为 selector     │
│    (不再重复 CheckRelayEvent, 避免同一区块多事件时读已提交 DB 失败)   │
└─────────────────────────────────────────────────────────────────────┘
```

## 二、事件数据格式

```
Token 合约 emit_event 格式:

  emit_event(
      b"gateway",          // target_contract
      b"Recharge",         // event_type
      vec![                // event_data: Vec<Vec<u8>>
          Encode(&UniAddr { t: 3, v: user.as_ref().to_vec() }),
          Encode(&points),
      ],
  );

侧链 Gateway 方法:
  Recharge(user UniAddr, amount uint64)
  Withdraw(providerID uint64, amount uint64)

dispatch 层将 event_type 转为 4-byte selector:
  model.MethodToSelectorBytes("Recharge") → [0x9b, 0x1e, 0x77, 0xa7]
```

## 三、设计决策

| 决策 | 方案 | 原因 |
|---|---|---|
| 谁拉取事件 | **所有节点**后台拉取 → 本地缓存 | 出块时零网络 IO |
| 谁创建 relay tx | **出块节点** in PrepareProposal | 零竞争 |
| 成熟度 | **≥2s** 缓存才打包 | 确保其他验证者也拿到了 |
| 验证在哪 | ProcessProposal + FinalizeTx(txn内) | 区块级 + txn级双重校验 |
| 签名 | DKG signer, 拉取时预签名 | 出块时零开销 |
| Phase 1 安全 | 信任 RPC + nonce 防重放 | 主链 RPC 自己的 full node |
| 批量查询 | ✅ `GetRelayEvents(from,to)` | 一次 RPC 拉取整个区间 |
| Engine 初始化 | `LoadChains` + `addChain` 懒初始化 | 无 chain 配置时也创建，后续添加链时自动启动 |
| Token 合约 emit | **仅 Recharge / Withdraw** | 配置类操作(SetRate等)不需要跨链 relay |
| Method 路由 | `MethodToSelectorBytes` 转 selector | 与 gateway_gen.go 的 switch 一致 |
| 日志 | 全链路 `util.Log*("RELAY-WORKER"|"RELAY", ...)` | 关键路径均有颜色日志 |

## 四、各阶段 relay 行为

| 阶段 | 谁执行 | 网络 IO | 操作 |
|---|---|---|---|
| 后台 Worker | 所有节点 | ✅ 批量拉取主链 | 缓存 + 预签名 |
| PrepareProposal | 出块节点 | ❌ 读缓存 | 取成熟事件打包 |
| ProcessProposal | 所有节点 | ❌ | nonce 验证(DB+区块内) → 拒绝无效 |
| FinalizeTx | 所有节点 | ❌ | txn 内 nonce 写入 + dispatch |
| Commit | 所有节点 | ❌ | nonce 持久化到 DB |

## 五、Nonce 存储与校验

```
nonce 存储:  "RELAY:nonce_{chainID}" → uint64 (SCALE 编码)

读取路径:
  getLastNonceBg(chainID)    → 读已提交 DB  → ProcessProposal / 后台 Worker
  getLastNonce(txn, chainID) → 读 txn       → FinalizeTx (ApplyRelayEvent)

首次事件: n==nil 时返回 0 而非 error，允许 nonce=1 通过
```

```
同一区块多个 relay 事件时的 nonce 跟踪:

ProcessProposal (verifyRelayTx):
  chainExpected[chainID] 跟踪区块内预期 nonce
  事件 nonce=1: db=0, expected=max(1, -)=1 ✓ → chainExpected[0]=2
  事件 nonce=2: db=0, expected=max(1, 2)=2 ✓ → chainExpected[0]=3

FinalizeTx (ApplyRelayEvent):
  事件 nonce=1: txn=0, expected=1 ✓ → 写 txn nonce=1
  事件 nonce=2: txn=1, expected=2 ✓ → 写 txn nonce=2
  Commit 后 DB nonce=2
```

## 六、安全保证

| 保证 | Phase 1 | Phase 2 |
|---|---|---|
| 防重放 | ✅ nonce 严格递增 | ✅ |
| 防乱序 | ✅ nonce == expected (DB + 区块内) | ✅ |
| 防伪造 | ❌ 信任 RPC | ✅ Merkle proof |
| 防篡改 | ✅ 签名 + 共识 | ✅ |
| 出块节点作恶 | ✅ ProcessProposal 拒绝 | ✅ |

## 七、文件清单

| 组件 | 文件 | 状态 |
|---|---|---|
| 主链合约 | `revive-contract/revives/Token/src/contract.rs` | ✅ 仅 Recharge/Withdraw emit; EventRecord.EventData: Vec<Vec<u8>> |
| Proto 定义 | `tee-provider/pkg/model/tx.proto` | ✅ SysCall_RelaySync |
| Relay 引擎 | `tee-provider/tee-chain/relay/engine.go` | ✅ Phase 1; MethodToSelector 路由; 全链路日志 |
| ChainApi 接口 | `tee-provider/pkg/chains/mod.go` | ✅ GetRelayEvents(from,to) 批量 |
| revive adapter | `tee-provider/pkg/chains/revive/relay.go` | ✅ 批量 RPC; RelayEvent 返回 |
| evm adapter | `tee-provider/pkg/chains/evm/relay.go` | 🔲 stub（当前仅 Substrate/Revive 支持批量事件拉取） |
| 缓存 Worker | `tee-provider/tee-chain/relay_to_tx.go` | ✅ 批量拉取 + 缓存 + DKG 预签名 |
| PrepareProposal | `tee-provider/tee-chain/consensus.go` | ✅ prepareRelayTxs + 日志 |
| ProcessProposal | `tee-provider/tee-chain/consensus.go` | ✅ verifyRelayTx + chainExpected 跟踪 |
| ProcessTx | `tee-provider/tee-chain/tx_process.go` | ✅ SysCall_RelaySync |
| FinalizeTx | `tee-provider/tee-chain/tx_finalize.go` | ✅ ApplyRelayEvent (txn级, 无CheckRelayEvent) |
| SideChain 初始化 | `tee-provider/tee-chain/relay.go` / `relay_to_tx.go` | ✅ engine + worker + 懒初始化 + 日志 |
| 公共类型 | `tee-provider/pkg/model/chain_types.go` | ✅ RelayEvent |
| Sol ABI 类型 | `wrevive/crates/wrevive-api/src/types.rs` | ✅ Address/U256 Sol ABI 委托 SDK 类型 |
| Sol ABI 生成 | `wrevive/crates/wrevive-macro/src/abi.rs` | ✅ payable 标志; Result<T,E>→T 类型映射 |
| DKG | `tee-provider/pkg/dkg/` | ✅ 线程安全 (RLock/Lock 保护共享状态) |

## 八、待完成

| 优先级 | 项目 | 文件 |
|---|---|---|
| P1 | 缓存旧事件清理 (已处理的 nonce) | `relay_to_tx.go` |
| P2 | Header 同步 + Merkle proof (Phase 2) | `relay/engine.go` |
| P2 | 事件数据格式完全对齐 (Token UniAddr → Gateway) | `relay/engine.go` dispatch |
| P3 | EVM Token 合约 relay 支持 | `pkg/chains/evm/relay.go` |
