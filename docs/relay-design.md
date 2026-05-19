# 主链 → 侧链 Relay 同步 · 审计文档

## 一、全景架构 (PrepareProposal 驱动)

```
┌─ 主链 (Polkadot/wrevive) ────────────────────────────────────────────┐
│  Token 合约: 每条 mutation 末尾 emit EventRecord(nonce, type, data)    │
│  Query: get_latest_nonce() / get_event(nonce) / get_chain_id()        │
└───────────────────────────────────────────────────────────────────────┘
        │
        │  ChainApi.GetLatestNonce() / GetRelayEvent(nonce)
        │  仅出块节点调用 (PrepareProposal)
        ▼
┌─ 出块节点 (PrepareProposal) ─────────────────────────────────────────┐
│                                                                       │
│  prepareRelayTxs():                                                   │
│    for each chain:                                                    │
│      latest = chain.GetLatestNonce()                ← 网络 IO         │
│      for n = lastNonce+1; n <= latest; n++:                          │
│        target, typ, data = chain.GetRelayEvent(n)   ← 网络 IO         │
│        ev = BuildSyncEvent(chainId, n, ...)                           │
│        sysCall = MakeRelaySysCall(ev)              → SysCall_RelaySync │
│        txBytes = DKG.Sign(sysCall)                                    │
│        prepend txBytes to block                                       │
│                                                                       │
│  打包普通 mempool 交易                                                │
│  广播区块                                                             │
└───────────────────────────────────────────────────────────────────────┘
        │
        │  区块广播到所有验证者
        ▼
┌─ 所有节点 (FinalizeTx) ──────────────────────────────────────────────┐
│                                                                       │
│  case *SysCall_RelaySync:                                             │
│    ev = FromRelaySysCall(p.RelaySync)                                 │
│    engine.CheckRelayEvent(ev)      ← nonce + Merkle proof 验证        │
│    engine.ApplyRelayEvent(ev)      ← nonce 写入 + dispatch 合约        │
│                                                                       │
│  case *SysCall_Contract:                                              │
│    app.ContractMutation(...)       ← 普通合约调用 (不变)               │
└───────────────────────────────────────────────────────────────────────┘
```

## 二、设计决策

| 决策 | 方案 | 原因 |
|---|---|---|
| 谁创建 relay tx | **出块节点** in PrepareProposal | 零竞争、无 mempool 垃圾、即时执行 |
| relay tx 如何携带 | **SysCall_RelaySync** (proto variant) | 专用类型，易扩展 |
| nonce 递增在哪 | 最后一笔 relay tx 成功后才递增 | 区块原子性 |
| 谁签名 relay tx | **DKG signer** | 与 HubCall 一致 |
| relay tx 验证在哪 | FinalizeTx | 所有节点执行前校验 |
| 与现有方案区别 | 不经过 mempool，不竞争 | 原方案所有节点都提交 relay tx 到 mempool |

## 三、安全保证

| 保证 | 机制 |
|---|---|
| **去中心化验证** | 所有验证者 FinalizeTx 中独立校验 nonce + proof |
| **防伪造** | Merkle proof 必须匹配链上 header |
| **防重放** | nonce 严格递增，状态写入后不可重复 |
| **防篡改** | tx 签名 (DKG) + 区块共识 |
| **出块节点作恶** | 提交无效 relay tx → 区块被拒绝 |
| **零竞争** | 只有 proposer 创建 relay tx |

## 四、文件清单

| 组件 | 文件 | 状态 |
|---|---|---|
| 主链合约 | `revive-contract/revives/Token/src/contract.rs` | ✅ |
| Proto 定义 | `tee-provider/pkg/model/tx.proto` | ✅ SysCall_RelaySync |
| Relay 引擎 | `tee-provider/side-chain/relay/engine.go` | ✅ |
| ChainApi 接口 | `tee-provider/pkg/chains/mod.go` | ✅ |
| revive adapter | `tee-provider/pkg/chains/revive/token.go` | ✅ RPC |
| evm adapter | `tee-provider/pkg/chains/evm/token.go` | 🔲 stub |
| PrepareProposal | `tee-provider/side-chain/consensus.go` | ✅ 集成 relay |
| Relay Worker | `tee-provider/side-chain/relay_worker.go` | ✅ prepareRelayTxs |
| FinalizeTx | `tee-provider/side-chain/tx_finalize.go` | ✅ SysCall_RelaySync |
| TeeChain 初始化 | `tee-provider/side-chain/chains.go` | ✅ engine init |

## 五、Proto 定义

```proto
message SysCall {
  oneof payload {
    ...
    RelaySync relay_sync = 8;
  }
}

message RelaySync {
  uint32 chain_id = 1;
  uint64 nonce = 2;
  uint64 block_num = 3;
  bytes target_contract = 4;
  bytes event_type = 5;
  repeated bytes event_data = 6;
  repeated bytes proof = 7;
  uint32 proof_index = 8;
}
```

## 六、待完成

| 优先级 | 项目 | 文件 |
|---|---|---|
| P1 | relay tx 在 CheckTx 中预验证 (减少 FinalizeTx 压力) | `tx_check.go` |
| P2 | Header 同步 (SubmitHeader) | `relay_worker.go` |
| P2 | EventData 解包 (SCALE tuple → 合约 args) | `engine.go` |
| P3 | EVM Token 合约 relay 支持 | EVM 合约 |
