# tee-node 总体架构设计

> 基于 docs/ 目录设计文档的综合分析  
> 最后更新：2026-05-26

---

## 1. 项目定位

**`tee-node`**（也称 `tee-provider`）是一个基于 **TEE（可信执行环境）** 的 Web3 基础设施节点，运行 BFT 共识侧链，提供秘密管理、AI API 网关、Web3 认证和阈值密码学服务。

---

## 2. 总体架构

```
┌─────────────────────────────────────────────────────────────┐
│                        外部客户端                             │
│         (OpenAI SDK / Web3Auth 前端 / Pod TEE Worker)        │
└─────────────┬───────────────────────────────┬───────────────┘
              │                               │
    ┌─────────▼──────────┐         ┌──────────▼──────────┐
    │   GraphQL API      │         │   REST API (chi)    │
    │   /gql             │         │   /v1/*             │
    │   · 认证/钱包       │         │   · AI 模型转发      │
    │   · 秘密管理        │         │   · 计费/限流        │
    │   · 链上查询        │         │   · 协议适配         │
    └─────────┬──────────┘         └──────────┬──────────┘
              │                               │
              └───────────────┬───────────────┘
                              │
                   ┌──────────▼──────────┐
                   │     Tee Chain 侧链   │
                   │   (CometBFT BFT共识) │
                   │                      │
                   │  · 交易处理           │
                   │  · 合约执行           │
                   │  · 秘密存储(PebbleDB) │
                   │  · 主链 Relay 同步    │
                   └──────────┬──────────┘
                              │
           ┌──────────────────┼──────────────────┐
           │                  │                  │
    ┌──────▼──────┐  ┌───────▼───────┐  ┌──────▼──────┐
    │   DKG 模块   │  │ 代理重加密     │  │  主链连接    │
    │ 阈值密钥生成 │  │ (NIZK零知识)   │  │ Polkadot/   │
    │  2/3+1 共识  │  │               │  │ EVM 适配     │
    └─────────────┘  └───────────────┘  └─────────────┘
```

---

## 3. 六大功能模块

### 3.1 TEE 侧链 (Tee Chain)

- **共识引擎**：CometBFT BFT（当前 v0.39.3）
- **存储层**：PebbleDB 嵌入式数据库（所有 value TEE 密封）
- **交易类型**：
  - `Empty` — 空交易
  - `EpochStart` / `EpochEnd` — 验证器轮换
  - `SyncTxStart` / `SyncTxEnd` / `SyncTxRetry` — 主链同步交易
  - `HubCall` — 包含 `PodStart`、`PodMint`、`UploadSecret`、`InitDisk`
  - `Contract` — 合约调用（含 nonce 追踪）
  - `RelaySync` — 主链事件同步
  - `RuntimeUpgrade` — WASM 运行时升级（sudo 独占区块）
- **关键机制**：
  - 交易状态机：`PENDING → SIGNING → SIGNED → SUBMITTING → CONFIRMED/FAILED`
  - **交易重试**：基于 `PrepareProposal` 区块触发，指数退避（5s → 300s），最大 10 次
  - **合约执行双路径**：Native Go（v0）+ WASM（v1+，通过 `wazero` 沙箱执行）

### 3.2 DKG + 代理重加密 (Proxy Re-encryption)

- **DKG**：分布式密钥生成（Pedersen DKG，Kyber），阈值 `2/3 + 1`，双套件（Ed25519 + BLS12-381）
- **DSS 阈值签名**：支持 Ed25519 和 BLS12-381 的部分签名与聚合
- **随机数池**：预生成独立随机份额，避免并发签名 nonce 重用
- **秘密生命周期**：
  1. 用户上传 → RSA 解密 → DKG 公钥加密 → 生成 `encCmt` + `encScrt`
  2. 提交侧链交易 → BFT 共识 → PebbleDB 存储（TEE 密封）
  3. Pod 启动 → 广播重加密请求 → 验证节点用 DKG 份额重加密（附 NIZK 证明）
  4. 收集阈值响应 → 验证 NIZK → Lagrange 插值恢复 `xncCmt` → Pod 本地解密
- **安全保证**：NIZK 零知识证明确保重加密正确性；TEE attestation 嵌入 CometBFT 投票扩展

### 3.3 主链 Relay 同步

- **方向**：主链（Polkadot / wrevive / EVM）→ 侧链
- **Phase 1（当前）**：信任 RPC + nonce 严格递增防重放
- **Phase 2（规划）**：Header 同步 + Merkle proof 验证
- **数据流**：
  1. 所有节点后台每 3 秒批量拉取主链 `Recharge` / `Withdraw` 事件
  2. 本地缓存 + DKG 预签名
  3. 出块节点 `PrepareProposal` 取成熟事件（缓存 ≥2s）打包
  4. `ProcessProposal` 验证 nonce（DB + 区块内双重校验）
  5. `FinalizeTx` dispatch 到 Gateway 合约

### 3.4 AI Gateway

- **接口**：兼容 OpenAI SDK 的 REST API
  - `/v1/chat/completions`
  - `/v1/completions`
  - `/v1/embeddings`
  - `/v1/images/generations`
  - `/v1/audio/speech`
  - `/v1/audio/transcriptions`
  - `/v1/models`
- **中间件链**（按执行顺序）：
  1. RecoveryMiddleware — panic 捕获
  2. RequestIDMiddleware — `X-Request-Id` 链路追踪
  3. LoggerMiddleware — 请求日志
  4. GzipDecodeMiddleware — gzip 解压
  5. RateLimitMiddleware — IP 级别滑动窗口限流（60 次/分钟）
  6. CacheMiddleware — 缓存控制
  7. TokenAuthMiddleware — `Authorization: Bearer <apiKey>` 鉴权
  8. DistributeMiddleware — 解析 model → 选 provider → 选 endpoint
- **适配器**：
  | 服务商 | 适配器 | 认证方式 |
  |--------|--------|---------|
  | OpenAI | `OpenAIAdaptor` | `Bearer` |
  | Anthropic | `AnthropicAdaptor` | `x-api-key` |
  | Gemini | `GeminiAdaptor` | `x-goog-api-key` |
  | 其他（DeepSeek/Groq/Mistral/Cohere/XAI/Ollama/MiniMax 等） | `OpenAIAdaptor`（兼容）| `Bearer` |
- **路由策略**：Sticky Balancer（FNV-32a Hash），同一用户绑定同一 endpoint
- **计费模型**（2026-05-26 更新）：
  ```
  uncached = TokenIn - CachedToken
  cost = (uncached * InPrice + CachedToken * CachedPrice + TokenOut * OutPrice) / 1_000_000
  if cost == 0 { continue }
  ```
  异步批量写入（100ms 刷盘，65536 buffer，阻塞写入不丢弃）
- **结算机制**：每5分钟 `settlePendingUsage()` 扫描本地用量，统一构造 `[]Settlement` 一次性上链（DKG 签名 + `callerType=0`）

### 3.5 Web3Auth（迁移中）

- **接口方式**：GraphQL（`graph/auth.graphqls`）+ REST（OpenAI 兼容）
- **认证方式**：
  - OTP 邮箱登录
  - OAuth（Google / GitHub / X）
  - 社交恢复（guardians + threshold）
- **钱包功能**：
  - 基于 email HMAC-SHA256 派生 EVM 地址
  - 交易签名（LegacyTx / EIP-155）/ EIP-191 消息签名
  - 交易密码 AES-GCM 加密保护私钥
- **演进路线**：Phase 1 单节点 ECDSA → Phase 4 DKG 阈值签名（已支持 Ed25519 / BLS12-381）

### 3.6 API Key 管理

- **格式**：`tee_<64 hex chars>`（256 bit 熵）
- **本地索引**：`SetGatewayTokenByKey` / `GetGatewayTokenByKey` O(1) 查询，不再遍历 DB
- **过期机制**：默认 1 天过期（`ExpiresAt` 字段）
- **安全模型**：
  - 链上仅存 `blake2b(apiKey)` 哈希，原始 key 绝不上链
  - 攻击者看到 hash 无法逆推 apiKey
  - 抢注 hash 纯 DoS，用户换 key 即可
- **生命周期**：
  1. 用户离线生成 → 链上 `RegisterAPIKey(keyHash, rateLimit)`
  2. 日常调用带 `Authorization: Bearer <apiKey>`
  3. TEE 查链 `LookupAPIKey(hash)` 验证 `isActive`
  4. 每 5 分钟 `SettleUsage` 批量结算

---

## 4. 演进路线

项目正处于**多项目融合期**（`web3auth` + `token` → `tee-provider`），分 5 个 Phase 实施：

| Phase | 内容 | 周期 | 状态 |
|:---:|------|:---:|:---:|
| 1 | web3auth → GraphQL（OTP/OAuth/钱包/签名） | 2-3 周 | 已落地 |
| 2 | token 中继 → REST（chi + adaptor + 计费） | 2-3 周 | 已落地 |
| 3 | 合约层实现（`contracts/auth/` + `contracts/gateway/` + `contracts/gov/`） | 3-4 周 | 已落地 |
| 4 | DKG 阈值签名替代单节点签名 | 3-4 周 | 已落地（Ed25519 + BLS12-381 双套件） |
| 5 | WASM 运行时升级（Native + WASM 双路径） | 3-4 周 | 已落地（wazero 沙箱） |
| 6 | 统一部署（Dockerfile / K8s / 环境变量 / SGX Gramine） | 1-2 周 | 已落地 |

---

## 5. 关键技术栈

| 层级 | 技术 |
|------|------|
| 共识 / P2P | CometBFT v0.39.3 |
| 数据库 | PebbleDB（全量 TEE 密封） |
| API 层 | GraphQL (gqlgen v0.17) + REST (chi router) |
| 链交互 | Polkadot (`ink.go`)、wrevive、EVM (`ethclient` + abigen) |
| 密码学 | Pedersen DKG、DSS 阈值签名（Ed25519 + BLS12-381）、代理重加密、NIZK |
| TEE | SGX (EGO) / SEV-SNP / TDX |
| WASM 引擎 | wazero（纯 Go，零 CGO） |
| 部署 | Kubernetes / Docker / systemd / SGX Gramine |

---

## 6. 安全特性

1. **分布式密钥**：密钥分散在多个节点，单节点无法恢复完整密钥
2. **阈值加密**：需要超过 2/3 的节点参与才能解密
3. **零知识证明**：重加密过程使用 NIZK 证明，确保正确性
4. **TEE 验证**：生成 TEE 报告，证明代码在可信环境中运行
5. **端到端加密**：从用户上传到 Pod 解密全程加密
6. **Hash 不可逆**：API Key 链上仅存 blake2b 哈希（用户认证 key）；Provider 模型 API Key 明文存储于合约，通过 GatewayQuery 脱敏返回

---

## 7. 参考文档

| 文档 | 说明 |
|------|------|
| `auth/mirgration.md` | web3auth + token 融合方案 |
| `gateway/ai-gateway-design.md` | AI Gateway 详细设计（中间件 / 适配器 / 计费 / 结算） |
| `gateway/api-key-design.md` | API Key 链上注册与认证设计 |
| `gateway/api-history-endpoints.md` | 历史查询接口（充值 / 结算） |
| `contracts/contract-storage-design.md` | 合约存储分层设计（IndexedList / StoreList2D） |
| `contracts/contract-security-analysis.md` | 合约安全审计与 Nonce 设计方案 |
| `gateway/settlement-dual-storage-design.md` | 结算记录双存储设计说明 |
| `chain/relay-design.md` | 主链→侧链 Relay 同步架构 |
| `module-feature-list.md` | 项目模块与功能列表（按目录拆分） |
| `tee-chain-design.md` | 侧链整体架构（DKG / 秘密管理 / 交易处理 / WASM） |
| `tee-tx-retry-design.md` | 交易失败重试机制 |
| `usage-settle-design.md` | 用量上链结算架构设计 |

---

*文档更新：2026-06-04*
