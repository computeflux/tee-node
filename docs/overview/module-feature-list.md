# TEE Node 模块与功能列表

> 本文档按目录/功能域拆分项目模块，列出各模块的核心职责与功能点。  
> 最后更新：2026-06-04（基于最新代码）

---

## 一、项目概述

- **项目名称**：TEE Provider Node（`github.com/computeflux/tee-provider`）
- **技术栈**：Go 1.26 / CometBFT v0.39.3 / WASM (wazero) / GraphQL (gqlgen)
- **定位**：基于可信执行环境（TEE）的去中心化子网节点，提供安全链下计算、Web3 身份认证、AI API 网关和阈值密码学服务。
- **测试进度**：详见 [`../kimi-testing-progress.md`](../kimi-testing-progress.md)（197 个新增测试，P0/P1 已完成）

---

## 二、模块拆分与功能列表

### 1. 应用入口与配置层

| 模块 | 路径 | 核心功能 |
|------|------|----------|
| **主入口** | `main.go` | 初始化 PebbleDB、加载多套件密钥（Ed25519 / secp256k1 / BLS12381）、连接主链、启动 CometBFT 共识与 DKG 服务、加载 WASM 运行时、启动 Web3Auth 与 GraphQL/REST 服务器 |
| **环境配置** | `config/` | 按环境（`local` / `test` / `main`）管理链 RPC 地址、合约地址、网络标识、密码学套件（Ed25519 / BLS12381） |

---

### 2. 核心库层（`pkg/`）

#### 2.1 区块链抽象层（`pkg/chains/`）

| 子模块 | 功能 |
|--------|------|
| **链抽象接口** | `pkg/chains/mod.go` | 定义 `ChainApi` / `MainChainApi` 接口；工厂函数 `ConnectMainChain()` / `ConnectChain()` 按配置分发到 EVM 或 Substrate 实现 |
| **EVM 客户端** | `pkg/chains/evm/` | `EvmChain` 实现双接口；`ethclient` 连接管理、Token/Subnet/BLS 合约绑定、多 RPC 故障转移、ECDSA transactor、BLS 网关合约批量提交 (`submit` / `batchSubmit`)、revert reason 解码；`relay.go` 当前为 stub |
| **Substrate/Revive 客户端** | `pkg/chains/revive/` | 基于 `ink.go` 的 Substrate 链客户端；Subnet/Token/Proxy 合约调用封装、代理调用、账户映射 (`MakeMapAccountCall`)、批量事件拉取 (`GetRelayEvents`) |

#### 2.2 分布式密钥生成（`pkg/dkg/`）

> ✅ 测试覆盖：`persist_test.go`（11 个）、`random_pool_test.go`（7 个）

| 功能项 | 说明 |
|--------|------|
| **Pedersen DKG** | 基于 `go.dedis.ch/kyber/v4` 的分布式密钥生成，支持 Ed25519 + BLS12-381 双套件 |
| **Epoch 共识** | 验证人集合变更时触发多轮 DKG 共识（`TryEpochConsensus` / `startConsensus` / `reConsensus`），含超时与状态迁移 |
| **Deal/Response 处理** | `sendDealMessage` / `handleDeal` / `handleDealResp` — 处理 Pedersen deal bundles 与 response bundles |
| **DSS 阈值签名** | `DssSigner` 支持 Ed25519（Schnorr/EdDSA）和 BLS12-381（`tbls` + EIP-2537 格式化）的 `Sign()` / `PartialSign()` |
| **随机数池** | `RandomSharePool` 预生成独立随机份额，避免 Ed25519 DSS 并发签名时的 nonce 重用 |
| **Epoch 部分签名** | `SendNewEpochPartialSigToSponsor` / `RevPartialSig` — 收集阈值部分签名完成链上 epoch 提交 |
| **P2P 消息路由** | `handleDkg()` 路由 `consensus` / `deal` / `deal_resp` / `random_deal_batch` 等消息；`handleSecretSave` 用于 P2P 秘密持久化 |
| **状态持久化** | DKG 状态保存/恢复至 PebbleDB (`persist.go`) |

#### 2.3 代理重加密（`pkg/proxy-reenc/`）

| 功能项 | 说明 |
|--------|------|
| **加密** | `EncryptSecret` — 使用 DKG 聚合公钥加密秘密（ElGamal） |
| **重加密** | `Reencrypt` — 基于 DKG 份额将密文转换为目标用户公钥 |
| **零知识验证** | `Verify` — NIZK 证明验证重加密份额正确性 |
| **恢复与解密** | `Recover` + `DecryptSecret` — Lagrange 插值恢复重加密承诺，目标用户最终解密 |

#### 2.4 数据模型与存储（`pkg/model/`）

> ✅ 测试覆盖：`gateway_endpoint_test.go`（9 个）、`u128_test.go`（10 个）、`key_ss58_test.go`（6 个）

| 子模块 | 功能 |
|--------|------|
| **数据库** | `db.go` — PebbleDB 封装（`DBINS` 单例），所有 value 经 TEE `SealWithProductKey` 密封；提供 JSON / Protobuf / SCALE 编解码辅助 |
| **密钥管理** | `key_*.go` — Ed25519 / secp256k1 / BLS12-381 私钥/公钥封装、SS58 编码、H160 / AccountID 转换 |
| **签名** | `sig.go` — 签名类型与验证辅助 |
| **TEE 远程证明** | `tee.go` / `tee_sgx.go` / `tee_snp.go` / `tee_tdx.go` — 平台检测（SGX via EGO / SEV-SNP / TDX），`IssueReport()` / `VerifyReport()` |
| **P2P 消息** | `p2p.go` / `message.go` — 广播/定向发送辅助、KVS 结构 |
| **DKG 模型** | `dkg_deal.go` / `dkg_dss.go` / `dkg_secret.go` / `dkg_side_key.go` — DealBundle、DistKeyShare、ConsensusMsg、SecretBox 等类型 |
| **网关模型** | `gateway_endpoint.go` / `gateway_store.go` — EndpointBalancer（FNV-32a 粘性哈希）、GatewayTokenInfo / GatewayDailyUsage |
| **认证存储** | `auth_store.go` — AuthUser / AuthSession / SignSession / EncryptedKey / RecoveryConfig / ChallengeRecord 的 PebbleDB CRUD |
| **链类型** | `chain_types.go` / `main_chain.go` — ChainClient、ChainConfigType、ChainCall、RelayEvent、RechargeEvent、WithdrawEvent、PointsAccount |
| **BLS12-381 工具** | `bls12381/` — EIP-2537 预编译格式、G1 映射、哈希、验证 |
| **Protobuf IO** | `protoio/` — 带长度前缀帧的 Protobuf 读写 |
| **其他** | `pod.go`（K8s Pod 类型）、`runtime_version.go`、`u128.go`、`contract_conv.go` |

#### 2.5 P2P 网络（`pkg/network/`）

| 子模块 | 功能 |
|--------|------|
| **Peer 抽象** | `peer.go` — `Peer` 接口（`Send` / `Sub` / `AvailableNodes` / `AllNodes`） |
| **CometBFT 桥接** | `bft-brigde/` — `BTFReactor` 自定义 Reactor，管理 `dkg` / `block-partial-sign` / `secret` 通道；信封路由、PeerID↔ValidatorPubkey 映射 |
| **本地 Peer** | `local/peer.go` — 本地开发/测试用的 Peer 实现 |

#### 2.6 Web3 认证服务（`pkg/services/auth/`）

> ✅ 测试覆盖：`jwt_test.go`（8 个）、`otp_test.go`（10+ 个）、`oauth_test.go`（14 个）

| 功能项 | 说明 |
|--------|------|
| **JWT 服务** | `jwt.go` — 签发/验证 JWT（标准 24h，恢复 10min），使用 `jwt/v5` |
| **OAuth 服务** | `oauth.go` — Google、GitHub、X/Twitter 社交登录，Code 交换与 Token 验证 |
| **OTP 服务** | `otp.go` — OTP 生成/验证（防暴力破解、频率限制、日上限） |
| **邮件服务** | `email.go` — SMTP 邮件发送，支持 TLS/STARTTLS |

#### 2.7 钱包服务（`pkg/services/wallet/`）

> ✅ 测试覆盖：`wallet_test.go`（25 个）

| 功能项 | 说明 |
|--------|------|
| **密钥派生** | `derive.go` — HMAC-SHA256 确定性种子 → ECDSA 私钥派生；服务端 AES-GCM 密钥包裹 |
| **加密** | `encrypt.go` — 用户密码基 AES-GCM 私钥加密 |
| **交易签名** | `signer.go` — Ethereum 交易签名（LegacyTx、EIP-155）、EIP-191 消息签名；支持加密密钥和种子派生双路径 |

#### 2.8 AI API 中继（`pkg/services/gateway_relay/`）

| 子模块 | 功能 |
|--------|------|
| **适配器接口** | `adaptor.go` — `Adaptor` 接口与工厂 `GetAdaptor()`（OpenAI=0 / Anthropic=1 / Gemini=2）；`GetRelayMode()` / `GetModelPointsRate()` |
| **OpenAI 适配** | `adaptor_openai.go` — 透传 + URI 去重 + SSE 流式 |
| **Anthropic 适配** | `adaptor_anthropic.go` — OpenAI↔Anthropic 格式双向转换 |
| **Gemini 适配** | `adaptor_gemini.go` — OpenAI↔Gemini 格式双向转换 + 安全设置追加 |
| **模型定义** | `model/` — OpenAI 兼容请求/响应类型（Chat、Image、Embedding、Audio） |
| **API/通道类型** | `apitype/define.go`、`channeltype/define.go` — 常量与 URL 辅助 |
| **RelayContext** | `context.go` — 兼容 `net/http` 的 context，替代 gin.Context，支持 body 读/重置、SSE 流式 |
| **Meta** | `meta.go` — Provider URL、API Key、stream flag、model name |
| **连接池** | `pool.go` — HTTP 连接池 / Transport 管理 |
| **Provider 检测** | `helper.go` — URL→Provider 识别 |

#### 2.9 合约 API 桥接（`pkg/api/`）

| 子模块 | 功能 |
|--------|------|
| **Auth 桥接** | `pkg/api/auth/` — `AuthConfig` 加载（优先链上合约，回退环境变量，内存缓存）；桥接 `tee-chain/contracts/module/auth` 的查询/存储/配置方法 |
| **Gateway 桥接** | `pkg/api/gateway/` — OpenAI 兼容 REST 路由注册 (`RegisterGatewayRoutes`)、`HandleRelay` 转发编排、`HandleListModels`；中间件链（Recovery / RequestID / Logger / GzipDecode / RateLimit / Cache / TokenAuth / Distribute）；Endpoint 并发限流 (`EndpointRateLimiter`)；链上合约查询桥接 + 模型缓存 |

#### 2.10 工具库（`pkg/util/`）

| 功能 | 环境变量读取、彩色日志、TEE Seal/Unseal（EGO `ecrypto`）、Gramine 辅助、Kyber 曲线转换、地址格式化、文件 IO、深拷贝、类型辅助、CGO stub |

---

### 3. 侧链核心（`tee-chain/`）

| 模块 | 文件 | 功能 |
|------|------|------|
| **ABCI 应用** | `consensus.go` | 核心 `TeeChain` 结构体，实现 `Info` / `InitChain` / `CheckTx` / `PrepareProposal` / `ProcessProposal` / `FinalizeBlock` / `Commit` / `ExtendVote` / `VerifyVoteExtension`（TEE attestation 嵌入投票扩展） |
| **节点初始化** | `mod.go` | `InitTeeChain()` 启动 CometBFT 节点、P2P Reactor、DKG hooks、验证人密钥加载、创世配置 |
| **应用状态** | `app.go` | `AppState`（Size / Height / RuntimeVersion）的加载/持久化 |
| **Epoch 管理** | `epoch.go` | 三阶段 epoch 过渡（P1 DKG 共识 → P2 提交下 epoch → P3 `EpochEnd` 切换验证人）；`SetEpoch` / `GetEpoch` / `GetValidators` |
| **交易准备** | `tx_prepare.go` | `PrepareTx()` — 调用者去重、Nonce 校验、合约 Nonce 追踪、`RuntimeUpgrade` 独占区块、HubCall 批处理、HubSyncStep1 |
| **交易检查** | `tx_check.go` | `checkTx()` — 签名验证、合约 Nonce dry-run、验证人成员检查 |
| **交易处理** | `tx_process.go` | `ProcessTx()` — 验证 `ProcessProposal` 阶段，拒绝无效合约 Nonce 或 dry-run 失败 |
| **交易最终化** | `tx_finalize.go` | `FinalizeTx()` — 执行 `EpochStart/End`、`SyncTxStart/End/Retry`、`HubCall`、`Contract`、`RelaySync`、`RuntimeUpgrade` |
| **交易签名** | `tx_sign.go` | `signTx` / `SubmitTx` / `SubmitCallFromNode` / `SignAndBuildRelayTx` |
| **部分签名** | `tx_partial_sign.go` | DKG 部分签名收集，主链 batch call 聚合签名与提交 (`SyncToHub`) |
| **Nonce 管理** | `tx_nonce.go` | 每调用者 Nonce 管理（`GetNextCallerNonce`、`TxnSetCallerNonce`） |
| **交易池** | `tx_pool.go` | 合约交易池（`SubmitContract` / `SubmitContractWithDedup`）；后台 `applyTxLoop` 批量 drain 为 `batch.execute_batch` |
| **主链同步** | `relay.go` / `relay_to_tx.go` / `tx_to_realy.go` | 后台 worker 每 3s 拉取主链事件 → 本地缓存 + DKG 预签名 → `PrepareProposal` 取成熟事件（≥2s）打包 → `ProcessProposal` nonce 校验 → `FinalizeTx` dispatch |
| **Relay 引擎** | `relay/engine.go` | `Engine.CheckRelayEvent()`（nonce 校验）、`ApplyRelayEvent()`（dispatch 到目标合约）、`MakeRelaySysCall` / `FromRelaySysCall` |
| **秘密存储** | `secret_store.go` | `Encrypt(data)`（DKG 公钥 + 代理重加密）；`SaveSecret` / `GetSecrets` / `SaveDiskKey` / `GetDiskKeys` |
| **秘密 API** | `secret_api.go` | `BroadcastReencryptReq()` — 广播 `PodStart`、收集阈值 `DecryptSharesResp`、恢复秘密；`HandleReencryptReq()` / `VerifyReencryptResp()` |
| **秘密序列化** | `secret_reply.go` | `EncodeDecryptShare` / `DecodeDecryptShare` — protobuf ↔ kyber `share.PubShare` |
| **合约路由** | `contract.go` / `contract_run.go` | `Query()` / `Mutation()` 按版本分发到 Native Go（v0）或 WASM（v1+）；`RuntimeVersion` 的激活高度管理、严格单调升级 |
| **工具** | `util.go` / `tx_to_realy.go` | 常量、全局状态、日志辅助、Hub 同步状态机（`HubSyncStep1/2/End`） |

---

### 4. 智能合约系统（`tee-chain/contracts/`）

> ✅ 测试覆盖：`store_primitive_test.go`（35 个）、`batch_test.go`（7 个）、`auth_test.go`（17 个）、`gateway_test.go`（21 个）、`gov_test.go`（20 个）

| 模块 | 功能 |
|------|------|
| **合约路由器** | `contract.go` | 根据运行时版本分发：Native Go（`wasm.NativeVersion == 0`，快路径）或 WASM 引擎（兼容路径）；合约 init 去重 |
| **运行时上下文** | `runtime.go` | `Runtime` 实现 `store.ContractApi`，提供 height、time、caller、sudo、跨合约 `Call()` / `Query()` |
| **存储原语** | `store/` | `StoreValue` / `StoreMapping` / `StoreList` / `StoreList2D` / `IndexedList` / `IndexedStore` / `StoreIndex`；SCALE 编解码；前缀扫描索引 |
| **WASM 引擎** | `wasm/` | 基于 **wazero**（纯 Go、零 CGO）的沙箱引擎；从 DB 加载/缓存 gzip 压缩的 WASM；`ExecuteQuery` / `ExecuteMutation` SCALE 编码调用、内存读写；host functions：`storage_get/set/delete/scan_prefix/delete_prefix`、`get_height`、`get_block_time`、`get_version`、`get_caller`、`get_system_account`、`is_system` |
| **WASM 运行时** | `wasm/runtime/` | TinyGo 编译的 WASM 二进制（`main.go` //go:build wasm），SCALE 解码 `InContractCall`，通过 host import 回调查询/变更 |
| **治理合约** | `module/gov/` | 成员管理、代币、提案、投票轨道、国库支出、曲线参数、公开加入门控 |
| **认证合约** | `module/auth/` | 用户注册（email hash）、社交恢复（guardians + threshold）、OAuth/SMTP 配置、加密用户/密钥记录、`AuthSecret`（TEE 内部查询） |
| **网关合约** | `module/gateway/` | Provider 注册、模型注册、API Key 哈希管理、信用充值/提现（relay-gated）、用量结算（nonce 防重放）、索引查询（exact / range） |
| **批处理合约** | `module/batch/` | `execute_batch(calls []BatchCall)` — 原子顺序执行子调用 |
| **合约调用路由** | `module/call.go` | 所有模块的 `Query` / `Mutation` / `Secret` 统一路由入口 |
| **合约 ABI** | `abis/` | `auth.json` / `batch.json` / `gateway.json` / `gov.json` — 供外部客户端使用 |

---

### 5. 外部 API 层（`graph/`）

| 模块 | 功能 |
|------|------|
| **服务器框架** | `mod.go` | Chi 路由 + CORS、GraphQL Playground + Handler（gqlgen，支持 WS/GET/POST/Multipart）、REST 路由挂载、可选 TLS |
| **依赖注入** | `resolver.go` | `Resolver` 容器：TeeChain、WalletSvc、EmailSvc、OAuthSvc、OTPSvc；`ResolveSignerEmail` |
| **认证中间件** | `auth.go` / `auth_helpers.go` | `AuthCheck` directive + `AuthMiddleware`：Bearer Token / API Key 解码、上下文注入、合约 `LookupAPIKey` 验证、EIP-191 签名验证、HMAC、email 解析 |
| **Auth GraphQL** | `auth.graphqls` / `auth.resolvers.go` | OTP 发送/验证、OAuth、钱包密码与签名、社交恢复、JWT、order-token 领取 |
| **Chain GraphQL** | `chain.graphqls` / `chain.resolvers.go` | 链信息、验证人列表、epoch 触发（`start_epoch`） |
| **Contract GraphQL** | `contract.graphqls` / `contract.resolvers.go` | 合约 dry-run、nonce 查询、水龙头、faucet、系统合约初始化、通用合约调用 |
| **Secret GraphQL** | `secret.graphqls` / `secret.resolvers.go` | 秘密上传、磁盘密钥初始化、TEE report、RSA 公钥查询 |
| **合约辅助** | `contract.go` | `DecodeCaller`、`decodeArgs`、`SubmitContractCall`、`ContractDryRun` |
| **REST 扩展** | `rest_handler.go` | `/api/blockchain/token`（dApp SDK stub） |
| **RSA 辅助** | `secret.go` / `secret_test.go` | RSA-PKCS#1v15 解密辅助 |
| **生成代码** | `generated.go` / `directives.go` | gqlgen 生成代码与 directive 根接线 |

**REST 路由（OpenAI 兼容）：**
- `/v1/models`、`/v1/chat/completions`、`/v1/completions`、`/v1/embeddings`、`/v1/images/generations`、`/v1/audio/*`

---

### 6. 构建与代码生成工具

| 模块 | 路径 | 功能 |
|------|------|------|
| **构建脚本** | `hack/` | EGO(SGX) 构建签名、WASM 运行时构建（TinyGo）、Go generate、SCALE 编解码生成、合约 ABI 生成、Protobuf 生成、CometBFT 创世/节点密钥生成、K8s 部署、EVM 合约部署脚本、节点部署脚本 |
| **合约代码生成器** | `tools/contractgen/` | Go AST 扫描 `DaoMutation` / `DaoQuery` 方法，自动生成 `ExecCall` / `ExecQuery` 分发方法、构造函数、方法选择器映射、ink! 兼容 JSON ABI |
| **SCALE 编解码生成器** | `tools/scale-codec-gen/` | 为 `// scale-codec-gen:generate` 注解结构体自动生成 `Encode()` / `Decode([]byte)` 方法 |
| **ink! ABI TS 生成器** | `tools/ink-abi-ts-gen/` | ink! ABI v6 JSON → TypeScript 类型定义 + 运行时 SCALE-to-JS decoder |

---

### 7. 实验与扩展（`ext/`）

| 子目录 | 功能 |
|--------|------|
| `experimental/dss/` | BLS12-381 阈值签名 + 链上 EIP-2537 验证研究（Hardhat/Go 测试，Sepolia 验证） |
| `experimental/mtls/` | Ed25519 mTLS 原型服务器 |
| `genkey/` | 密钥生成工具 |
| `setblspubkey/` | 独立 CLI 工具，向 EVM Subnet 合约提交聚合 BLS 公钥 |
| `tailscale/` | Tailscale 跨区域 TEE 节点代理（节点生命周期、Peer 发现、服务发现、请求路由） |

---

## 三、核心能力矩阵

| 核心能力 | 涉及模块 |
|----------|----------|
| **BFT 共识** | `tee-chain/consensus.go`、`tee-chain/mod.go`（CometBFT ABCI） |
| **TEE 远程证明** | `pkg/model/tee_*.go`（嵌入 CometBFT `ExtendVote` / `VerifyVoteExtension`） |
| **阈值密码学** | `pkg/dkg/` + `pkg/proxy-reenc/` |
| **智能合约（Native + WASM 双路径）** | `tee-chain/contracts/` |
| **Web3 身份认证** | `pkg/services/auth/` + `pkg/services/wallet/` + `tee-chain/contracts/module/auth/` |
| **AI API 网关** | `pkg/api/gateway/` + `pkg/services/gateway_relay/` + `tee-chain/contracts/module/gateway/` |
| **主链同步（Relay）** | `pkg/chains/` + `tee-chain/relay*.go` + `tee-chain/tx_prepare.go` |
| **外部接口** | `graph/`（GraphQL + OpenAI 兼容 REST） |
