# tee-provider — TEE Subnet for Web3 Infrastructure

tee-provider 是一个基于 **TEE (Trusted Execution Environment)** 的去中心化子网（subnet），由多个运行在 TEE 中的节点组成，通过 **CometBFT** 共识协议维护子网内状态一致，并通过 **DKG (Distributed Key Generation)** 和 **代理重加密 (Proxy Re-encryption)** 提供安全的密钥管理与秘密分发能力。

tee-provider 子网可以作为**任意公链的子网**运行，将链下安全计算、认证服务和 API 网关能力带入任何生态。

---

## 架构总览

```
┌──────────────────────────────────────────────────────────────────┐
│                    任意公链 (L1 / L2)                             │
│  Polkadot · Ethereum · Solana · BNB Chain · 任何链               │
│                                                                  │
│  ┌────────────────────────────────────────────────────────────┐  │
│  │  tee-provider 注册合约 (Registry · Staking · Slashing)      │  │
│  │  - 节点注册 / 注销                                          │  │
│  │  - TEE 远程证明验证                                          │  │
│  │  - 子网状态锚定                                              │  │
│  └────────────────────────┬───────────────────────────────────┘  │
└───────────────────────────┼──────────────────────────────────────┘
                            │ 跨链通信 / 状态同步
                            │
╔═══════════════════════════╧══════════════════════════════════════╗
║                 tee-provider 子网 (CometBFT)                     ║
║                                                                  ║
║  ┌────────────────────────────────────────────────────────────┐  ║
║  │                    共识层 (Consensus)                       │  ║
║  │  - CometBFT ABCI 应用                                       │  ║
║  │  - TEE 远程证明 (Vote Extensions)                            │  ║
║  │  - 验证人选举 & 纪元管理                                      │  ║
║  └────────────────────────┬───────────────────────────────────┘  ║
║                           │                                      ║
║  ┌────────────────────────┴───────────────────────────────────┐  ║
║  │                   安全计算层 (Crypto)                        │  ║
║  │  - DKG (分布式密钥生成) — 阈值签名                             │  ║
║  │  - Proxy Re-encryption — 代理重加密                            │  ║
║  │  - TEE Attestation — SGX / TDX / SEV 远程证明                │  ║
║  └────────────────────────┬───────────────────────────────────┘  ║
║                           │                                      ║
║  ┌────────────────────────┴───────────────────────────────────┐  ║
║  │                    服务层 (Services)                         │  ║
║  │                                                              │  ║
║  │  ┌─────────────────┐  ┌──────────────┐  ┌───────────────┐   │  ║
║  │  │  Web3Auth 服务   │  │ API 网关服务  │  │  秘密管理服务  │   │  ║
║  │  │                 │  │              │  │               │   │  ║
║  │  │ · OTP 邮箱认证  │  │ · Token 管理 │  │ · 加密环境变量 │   │  ║
║  │  │ · 社交 OAuth    │  │ · 多供应商   │  │ · 磁盘密钥     │   │  ║
║  │  │ · 钱包管理      │  │   Relay     │  │ · TEE 报告     │   │  ║
║  │  │ · 交易签名(TEE) │  │ · 速率限制   │  │ · RSA 密钥     │   │  ║
║  │  │ · 密钥派生(TEE) │  │ · 计费结算   │  │               │   │  ║
║  │  │ · 社交恢复      │  │ · 链上订单   │  │               │   │  ║
║  │  └─────────────────┘  └──────────────┘  └───────────────┘   │  ║
║  └────────────────────────────────────────────────────────────┘  ║
║                                                                  ║
║  ┌────────────────────────────────────────────────────────────┐  ║
║  │                   P2P 网络层 (Networking)                    │  ║
║  │  - 节点发现 / 种子节点                                       │  ║
║  │  - 消息路由 / Gossip                                         │  ║
║  │  - DKG 信道 / BFT Bridge                                     │  ║
║  └────────────────────────────────────────────────────────────┘  ║
╚══════════════════════════════════════════════════════════════════╝
                            │
        ┌───────────────────┼───────────────────┐
        ▼                   ▼                   ▼
   ┌─────────┐        ┌──────────┐        ┌──────────┐
   │ Node 0  │        │  Node 1  │  ...   │  Node N  │
   │ (TEE)   │        │  (TEE)   │        │  (TEE)   │
   │ SGX/TDX │        │ SGX/TDX  │        │ SGX/TDX  │
   └─────────┘        └──────────┘        └──────────┘
```

---

## 项目模块

### 1. 侧链 / 共识 (`side-chain/`)

子网核心，基于 CometBFT 实现：

| 模块 | 说明 |
|------|------|
| `consensus.go` | ABCI 应用，处理区块生命周期 (CheckTx, PrepareProposal, ProcessProposal, FinalizeBlock, Commit) |
| `mod.go` | 侧链初始化、节点启动、验证人配置 |
| `epoch.go` | 纪元管理，自动生成 HubCall 同步到主链 |
| `hub_sync.go` | 主链状态同步（验证人集合、链配置） |
| `chains.go` | 多链连接管理（添加 / 删除 / 查询外部链） |
| `tx_*.go` | 交易处理管线（Prepare → Process → Partial Sign → Sign → Finalize） |

### 2. 密码学层 (`pkg/`)

| 子包 | 说明 |
|------|------|
| `pkg/dkg` | 分布式密钥生成 (DKG)，基于 kyber 实现阈值签名 |
| `pkg/proxy-reenc` | 代理重加密：将加密秘密从 DKG 公钥安全转换为客户端公钥 |
| `pkg/chains` | 公链接口抽象（支持 Substrate / 任意链） |
| `pkg/util` | 工具函数、日志、SS58 编解码 |

### 3. GraphQL API (`graph/`)

为外部客户端提供查询和操作接口：

```graphql
# 链信息
query { chain_info { network_label urls } validators }

# 秘密管理
mutation { upload_secret(index, secret, hash, user) init_disk_key(index, user) }
query { tee_report(hash) secret_rsa }

# 合约调用
mutation { contractCall(caller, callerType, contract, method, args, signature) }
query { contractDryRun(caller, callerType, contract, mut, method, args) }
```

### 4. TEE 远程证明

- 基于 **Edgeless ego** (`github.com/edgelesssys/ego`) — Intel SGX
- 基于 **go-sev-guest** (`github.com/google/go-sev-guest`) — AMD SEV
- Vote Extensions 中嵌入 TEE 证明，每个区块都经过 TEE 验证
- 公私钥对绑定到特定 TEE 实例

---

## 功能整合方案 — 合约 + DKG 深度设计

### A. Web3Auth 后端 → tee-provider (DKG 私钥存储)

将 `web3auth/back` 的认证服务迁移到 tee-provider，**核心升级是由单服务器 HMAC 密钥派生改为 DKG 阈值分片存储**。

| web3auth 功能 | tee-provider 对应层 | TEE 安全增强 |
|---------------|--------------------|-------------|
| OTP 邮箱认证 | HTTP: `services/auth/otp.go` | OTP 在 TEE 内生成和验证 |
| 社交 OAuth 登录 | HTTP: `services/auth/oauth.go` | Token 交换在 TEE 内完成 |
| 密钥派生 (email→seed→privkey) | 合约: `contracts/auth/` + `services/wallet/derive.go` | **私钥分片存储，DKG 阈值签名** |
| 交易签名 | 合约: `contracts/auth/` + `services/wallet/signer.go` | 阈值签名在 enclave 内执行 |
| 消息签名 | 同上 | 同上 |
| 密码加密私钥 | `services/wallet/encrypt.go` | 加密在 TEE 内完成 |
| JWT 签发/验证 | HTTP: `services/auth/jwt.go` | 密钥存储在 TEE 保护内存 |
| 社交恢复 | 合约: `contracts/auth/` + `services/recovery/` | 恢复流程由共识保障 |

### B. Token API 转发 → tee-provider (积分模式)

将 `token` 的 Relay 功能迁移到 tee-provider，**采用积分模式：用户充值积分，按各 Channel 设定的汇率扣费**。

| token 功能 | tee-provider 对应层 | 去中心化增强 |
|------------|--------------------|-------------|
| API Token 管理 | 合约: `contracts/gateway/` + HTTP: `services/gateway/token.go` | 状态上链，子网共识 |
| 多供应商 Relay | HTTP: `services/gateway/relay.go` | 路由策略由合约配置 |
| 速率限制 | HTTP: `services/gateway/ratelimit.go` | 分布式速率限制 |
| 积分充值 | 合约: `contracts/gateway/` + HTTP: `services/gateway/points.go` | **链上充值/扣费** |
| 汇率配置 | 合约: `contracts/gateway/` (每个 Channel 自行设定) | **供应商自主定价** |
| 用户管理 | 合约: `contracts/gateway/` + HTTP: `services/gateway/user.go` | DID 身份 |
| Channel 管理 | 合约: `contracts/gateway/` + HTTP: `services/gateway/channel.go` | 配置变更需共识 |
| Dashboard Web UI | `dashboard/` (静态托管) | API 指向 tee-provider |

**积分模式核心逻辑**：

```
用户充值 100 积分
     │
     ▼
调用 GPT-4:   Channel A 设定 10 积分/1K tokens → 消耗 1000 tokens = 扣 10 积分
调用 Claude:  Channel B 设定 5 积分/1K tokens  → 消耗 1000 tokens = 扣 5 积分
调用 国产模型: Channel C 设定 1 积分/1K tokens  → 消耗 1000 tokens = 扣 1 积分

✓ 每个 Channel 独立设定自己的 model → points_per_1k 汇率
✓ 用户一个积分余额通用所有模型，按各 Channel 汇率自动折算
✓ 不再有套餐/Product 概念，纯粹的"充值积分 → 按量消费"
```

---

## 合约层设计 — Auth + Gateway 上链

tee-provider 内建智能合约系统（`side-chain/contracts/`），遵循统一的路由模式：
`contracts/contract.go` 按合约名 `call.Name` 分发 → 各合约包实现 `Mutation(call)` / `Query(call)`。

现有入口（扩展后）：

```go
// side-chain/contracts/contract.go
func Mutation(call *model.ContractCall, runtime model.ContractApi) error {
    switch string(call.Name) {
    case "gov":     // 已有：治理/代币/提案
        ins := gov.NewGov(runtime)
        return gov.GovMutation{Gov: *ins}.ExecCall(call)
    case "auth":    // 新增：Web3Auth 合约
        ins := auth.NewAuth(runtime)
        return auth.AuthMutation{Auth: *ins}.ExecCall(call)
    case "gateway": // 新增：API 网关合约
        ins := gateway.NewGateway(runtime)
        return gateway.GatewayMutation{Gateway: *ins}.ExecCall(call)
    }
}
```

### 合约目录结构（规划）

```
side-chain/contracts/
├── contract.go              # 合约路由入口
├── runtime.go               # 合约运行时上下文
├── gov/                     # 已有：治理合约
│   ├── gov.go, gov_gen.go, types.go
│   ├── member.go, token.go, vote.go, proposal.go, track.go, treasury.go
│   └── curve.go, helper.go
├── auth/                    # 新增：Web3Auth 合约
│   ├── auth.go              # 合约主体 (结构体 + Init)
│   ├── auth_gen.go          # 代码生成 (ExecCall / ExecQuery / MethodMap)
│   ├── types.go             # 类型定义 (UserInfo, RecoveryConfig, ...)
│   ├── user.go              # 用户注册/查询
│   ├── recovery.go          # 社交恢复逻辑
│   └── abis/
│       └── auth.json        # 合约 ABI
└── gateway/                 # 新增：API 网关合约
    ├── gateway.go           # 合约主体
    ├── gateway_gen.go       # 代码生成
    ├── types.go             # 类型定义 (ModelInfo, ChannelInfo, TokenInfo, ...)
    ├── model.go             # 模型注册 / 更新价格
    ├── channel.go           # Channel 创建 / 关联模型
    ├── token.go             # API Token CRUD
    ├── points.go            # 积分充值 / 消费 / 提现
    └── abis/
        └── gateway.json     # 合约 ABI
```

---

## Auth 合约 — DKG 私钥分片存储

**核心思路**：用户私钥通过 Shamir 秘密共享分为 n 份，每个 TEE 节点持有一份，签名时 t-of-n 阈值聚合。**完整私钥从未在任何单一节点出现**。

### 密钥生命周期

```
① 认证 → ② 派生 → ③ 分片 → ④ 存储 → ⑤ 签名

① OTP/Social 认证
     │
     ▼
② email → seed = HMAC-SHA256(email, SECRET)    [在 TEE 内，内存中]
   seed → privKey (ECDSA)                       [在 TEE 内，瞬时]
     │
     ▼
③ Shamir 秘密共享 (t-of-n):
   f(x) = privKey + a₁x + a₂x² + ... + aₜ₋₁xᵗ⁻¹
   分片 sᵢ = f(i),  i ∈ {1..n}
   承诺 Cᵢ = sᵢ·G  (用于后续验证)
     │
     ▼
④ 各节点存储 sᵢ (TEE 密封加密)
   链上 Auth 合约锚定: (user, share_hash, commitments_root)
     │
     ▼
⑤ 签名请求:
   各节点: σᵢ = tss_sign(sᵢ, tx_hash)
   聚合:   σ = tss_aggregate({σᵢ}),  t ≥ threshold
   → 返回完整签名
```

### Auth 合约方法

**Mutation（写操作）**：

| 方法 | Selector | 参数 | 说明 |
|------|----------|------|------|
| `Init` | `0x53e39003` | — | 初始化 Auth 合约 (DKG 公钥、阈值) |
| `RegisterUser` | `0x...` | `(email_hash, share_commitment, recovery_email_hash)` | 注册用户，锚定分片承诺 |
| `UpdateShareCommitment` | `0x...` | `(email_hash, new_commitment)` | 更新分片承诺 (key rotation) |
| `SetRecovery` | `0x...` | `(email_hash, recovery_config)` | 设置社交恢复配置 |
| `RequestRecovery` | `0x...` | `(email_hash, new_commitment)` | 发起恢复 (需 ≥2 守护者批准) |
| `ApproveRecovery` | `0x...` | `(email_hash, guardian_sig)` | 守护者批准恢复 |
| `CompleteRecovery` | `0x...` | `(email_hash)` | 完成恢复 (重置分片) |

**Query（读操作）**：

| 方法 | Selector | 参数 | 返回 |
|------|----------|------|------|
| `GetUser` | `0x...` | `email_hash` | `UserInfo` |
| `HasKey` | `0x...` | `email_hash` | `bool` |
| `GetRecovery` | `0x...` | `email_hash` | `RecoveryConfig` |
| `GetRecoveryStatus` | `0x...` | `email_hash` | `RecoveryStatus` |

### Auth 合约状态模型

```go
type Auth struct {
    api           model.ContractApi
    users         *model.StoreMapping[H160, UserInfo]          // email_hash → UserInfo
    shareRoots    *model.StoreMapping[H160, H256]              // email_hash → share_merkle_root
    recoveries    *model.StoreMapping[H160, RecoveryConfig]     // email_hash → Recovery
    recoveryReqs  *model.StoreMapping[H160, RecoveryRequest]   // 进行中的恢复请求
    dkgPubKey     *model.StoreValue[[]byte]                    // DKG 公钥
    threshold     *model.StoreValue[uint32]                    // 阈值 t
    totalNodes    *model.StoreValue[uint32]                    // 总节点数 n
}

type UserInfo struct {
    EmailHash       H160      // email 的哈希 (隐私保护)
    ShareRoot       H256      // 分片承诺 Merkle 根
    DerivedAddress  []byte     // 派生地址 (如 Ethereum address)
    RegisteredAt    uint64     // 注册区块高度
}

type RecoveryConfig struct {
    Guardians       []H160    // 守护者 email_hash 列表
    Threshold       uint32    // 恢复所需最小守护者数
    RecoveryEmail   []byte    // 恢复邮箱 (加密存储)
}
```

### 签名流程（HTTP → 合约 → DKG）

```
POST /wallet/sign-transaction
  │
  ├─ ① JWT 认证 → 获取 email_hash
  ├─ ② Query Auth.GetUser(email_hash) → 确认密钥存在
  ├─ ③ 通过 P2P 向所有 DKG 节点广播 SignRequest{email_hash, tx_hash}
  │
  ├─ 各节点 (在 TEE 内):
  │   ├─ 从 TEE 密封存储加载分片 sᵢ
  │   ├─ 验证分片承诺: sᵢ·G == Cᵢ
  │   ├─ 计算部分签名: σᵢ = TSS_Sign(sᵢ, tx_hash)
  │   └─ 返回 PartialSig{σᵢ, TEE_attestation}
  │
  ├─ ④ 收集 ≥ threshold 个有效的 σᵢ
  ├─ ⑤ 聚合: σ = TSS_Aggregate({σᵢ})
  └─ ⑥ 返回完整签名 σ
```

---

## Gateway 合约 — 积分制 API 网关

**核心思路**：模型独立注册（每个模型有自己的价格和供应商），Channel 通过 modelIds 关联已注册的模型。用户充值积分，Relay 时按模型价格自动扣费。

### 积分流转模型

```
+-- 模型与 Channel 注册（供应商）--------------------------+
|                                                        |
|  ① 注册模型 (RegisterModel)                             |
|     provider -> Gateway.RegisterModel(name, pointsPer1K)|
|     -> modelId=1, owner=provider, 模型独立存在           |
|                                                        |
|  ② 创建 Channel (CreateChannel)                         |
|     provider -> Gateway.CreateChannel(name, baseUrl,    |
|                     [modelId1, modelId2], priority)     |
|     Channel 关联已注册的模型 ID 列表                     |
+--------------------------------------------------------+

+-- 积分消费（Relay 自动触发）----------------------------+
|                                                        |
|  ③ 用户充值 (Recharge)                                  |
|     user.balance += amount                             |
|                                                        |
|  ④ 消费 (Consume)                                      |
|     model = Gateway.GetModel(modelId)                   |
|     cost = ceil(tokensUsed / 1000 * model.PointsPer1K)  |
|     user.balance -= cost                               |
|     model.providerBalance += cost                      |
|                                                        |
|  ⑤ 提现 (Withdraw)                                      |
|     provider -> Gateway.Withdraw(modelId, amount)       |
|     model.providerBalance -= amount                     |
+--------------------------------------------------------+
```

### Gateway 合约方法

**Mutation**：

| 方法 | 说明 |
|------|------|
| `Init` | 初始化 Gateway 合约 |
| `RegisterModel(owner, name, pointsPer1K)` | **供应商注册模型，设定积分价格** -> `modelId` |
| `UpdateModel(modelId, pointsPer1K)` | 更新模型价格 (仅 owner) |
| `DeactivateModel(modelId)` | 下架模型 (仅 owner) |
| `CreateChannel(owner, name, baseUrl, modelIds, priority)` | 供应商创建 Channel，关联已注册模型 |
| `UpdateChannel(channelId, modelIds, priority)` | 更新 Channel 关联的模型 |
| `Recharge(user, amount)` | 用户充值积分 (仅 sudo/管理员调用) |
| `Consume(tokenId, channelId, modelId, tokensUsed)` | Relay 扣费：按模型价格扣积分 |
| `CreateToken(owner, rateLimit)` | 创建 API Token (不绑定模型) |
| `RevokeToken(tokenId)` | 吊销 Token |
| `Withdraw(modelId, amount)` | 供应商提取该模型的收益 |

**Query**：

| 方法 | 说明 |
|------|------|
| `GetBalance(user)` | 查询用户积分余额 |
| `GetModel(modelId)` | 查询模型详情 (name, pointsPer1K, owner) |
| `ListModels(startKey, size)` | 分页列出所有已注册模型 |
| `GetToken(tokenId)` | 查询 Token 详情 |
| `ListUserTokens(user, startKey, size)` | 分页列出用户 Token |
| `GetChannel(channelId)` | 查询 Channel 及关联的 modelIds |
| `ListActiveChannels(modelId)` | 列出支持某模型的可用 Channel (按优先级) |
| `GetUsage(tokenId, day)` | 查询某天用量 |

### Gateway 合约状态模型

```go
type Gateway struct {
    api              model.ContractApi
    // 用户积分余额: user -> balance
    balances         *model.StoreMapping[UniAddr, Amount]
    // 模型: modelId -> ModelInfo (独立注册，每个模型有自己的价格)
    models           *model.StoreMapping[uint64, ModelInfo]
    // 模型收益: modelId -> provider balance (按模型累积)
    providerBalances *model.StoreMapping[uint64, Amount]
    // Channel: channelId -> ChannelInfo
    channels         *model.StoreMapping[uint64, ChannelInfo]
    // API Token
    tokens           *model.StoreMapping[uint64, TokenInfo]
    userTokens       *model.StoreList2D[UniAddr, uint64, uint64]
    // 用量记录: "{tokenId}_{day}" -> DailyUsage
    usages           *model.StoreMapping[string, DailyUsage]
    nextModelId      *model.StoreValue[uint64]
    nextChannelId    *model.StoreValue[uint64]
    nextTokenId      *model.StoreValue[uint64]
}

// ModelInfo 独立注册的模型，有自己的积分价格
type ModelInfo struct {
    Name        string      // 模型名称，如 "gpt-4"
    Owner       UniAddr     // 供应商地址
    PointsPer1K uint64      // 每 1000 tokens 消耗的积分数
    IsActive    bool
    CreatedAt   uint64
}

type ChannelInfo struct {
    Name       string
    BaseURL    string       // 供应商 API 地址
    ModelIds   []uint64     // 关联的模型 ID 列表
    Priority   uint32       // 路由优先级
    IsActive   bool
    Owner      UniAddr      // 供应商地址
}

type TokenInfo struct {
    Owner      UniAddr
    RateLimit  uint32       // 每分钟最大请求数
    IsActive   bool
    CreatedAt  uint64
}

type DailyUsage struct {
    TokenId    uint64
    ChannelId  uint64
    ModelId    uint64
    Day        string       // "2025-01-15"
    Count      uint64       // 调用次数
    TokensUsed uint64       // 消耗的 token 数
    PointsUsed uint64       // 消耗的积分数
}
```

### 积分消费流程（Relay 中间件）

```
客户端 POST /v1/chat/completions
  Authorization: Bearer <api-key>
  Body: { "model": "gpt-4", ... }
  |
  v
+-- tee-provider HTTP Gateway ---------------------------------+
|                                                              |
| ① 解析 api-key -> tokenId -> owner                          |
| ② Query Gateway.GetBalance(owner)                            |
|    -> 余额不足则拒绝                                          |
| ③ 根据请求 model 名称查找 modelId                            |
|    Query Gateway.GetModel(modelId) -> ModelInfo              |
| ④ Query Gateway.ListActiveChannels(modelId)                  |
|    -> 按 Priority 选择最优 Channel                           |
| ⑤ 转发请求到 Channel.BaseURL -> 收到响应                     |
| ⑥ 计算费用: cost = ceil(tokensUsed / 1000                    |
|                         * model.PointsPer1K)                 |
| ⑦ Mutation Gateway.Consume(tokenId, channelId,              |
|                              modelId, tokensUsed)            |
|    -> user.balance -= cost                                   |
|    -> model.providerBalance += cost                          |
| ⑧ 返回响应给客户端                                           |
+--------------------------------------------------------------+
  |
  v
供应商 API (OpenAI / Claude / ...)
```

### 注册与路由示例

```
① 供应商 A 注册模型:
   RegisterModel(owner=A, name="gpt-4",      pointsPer1K=30)  -> modelId=1
   RegisterModel(owner=A, name="gpt-3.5",    pointsPer1K=3)   -> modelId=2

② 供应商 B 注册模型:
   RegisterModel(owner=B, name="claude-3",   pointsPer1K=25)  -> modelId=3

③ 供应商 C 注册模型:
   RegisterModel(owner=C, name="qwen-turbo", pointsPer1K=1)   -> modelId=4

④ 供应商 A 创建 Channel:
   CreateChannel(owner=A, name="OpenAI通道", baseUrl="https://api.openai.com",
                 modelIds=[1, 2], priority=10)  -> channelId=1

⑤ 用户调用 gpt-4:
   请求 model="gpt-4" -> 匹配 modelId=1 -> 路由到 channelId=1
   扣费: 1000 tokens * 30 积分/1K = 30 积分

⑥ 供应商 A 提现:
   Withdraw(modelId=1, amount=300)  // 从 gpt-4 收益中提取
```

> **模型与 Channel 解耦**：模型通过 `RegisterModel` 独立注册并定价，Channel 通过 `modelIds` 关联模型。同一供应商可以用多个 Channel 提供不同模型组合，同一模型也可以被多个 Channel 引用（不同供应商各自定价）。

---

## DKG 私钥存储与现有代理重加密的复用

tee-provider 已有成熟的代理重加密基础设施 (`pkg/proxy-reenc`)，用于对用户上传的秘密（环境变量、磁盘密钥）进行安全分发。DKG 私钥存储**复用同一套 DKG 密钥材料**：

| 密码学服务 | 使用的 DKG 材料 | 用途 |
|-----------|---------------|------|
| Proxy Re-encryption（现有） | DKG 公钥 + 份额 `sᵢ` | 将加密秘密从 DKG 公钥安全转换为客户端公钥 |
| Threshold ECDSA（新增） | DKG 公钥 + 份额 `sᵢ` | 用户钱包密钥的阈值签名 |
| TEE Attestation（现有） | TEE 报告 | 证明计算在真实 enclave 中执行 |

**统一安全模型**：一个 DKG 网络同时提供「代理重加密」和「阈值签名」两种密码学服务，由同一组验证人运行，共享同一个信任假设（≥2/3 诚实 + TEE 安全）。

---

## 目录结构 (规划)

```
tee-provider/
├── main.go                          # 入口
├── go.mod / go.sum
├── Dockerfile
│
├── side-chain/                      # 侧链核心 (CometBFT ABCI)
│   ├── mod.go                       # 初始化
│   ├── consensus.go                 # 共识状态机
│   ├── chains.go                    # 多链管理
│   ├── epoch.go                     # 纪元
│   ├── hub_sync.go                  # 主链同步
│   ├── secret_api.go                # 秘密请求处理
│   ├── secret_reply.go              # 秘密回复处理
│   ├── secret_store.go              # 秘密存储
│   ├── tx_*.go                      # 交易处理
│   └── contracts/                   # 链上合约 ABI
│
├── pkg/
│   ├── api/                         # 外部 API (待扩展)
│   │   ├── dao/                     # DAO 接口
│   │   ├── auth/                    # Web3Auth 接口 ← 新增
│   │   ├── gateway/                 # API 网关接口 ← 新增
│   │   └── middleware/              # 中间件 (CORS / Auth / RateLimit)
│   │
│   ├── chains/                      # 链抽象层
│   ├── dkg/                         # 分布式密钥生成
│   ├── model/                       # 数据模型 + Pebble DB
│   ├── network/                     # P2P 网络
│   ├── proxy-reenc/                 # 代理重加密
│   └── util/                        # 工具
│
├── services/                        # 服务层 ← 新增
│   ├── auth/                        # Web3Auth 服务
│   │   ├── otp.go                   # OTP 生成/验证
│   │   ├── oauth.go                 # 社交登录
│   │   ├── jwt.go                   # JWT 签发/验证
│   │   └── email.go                 # 邮件发送
│   │
│   ├── wallet/                      # 钱包服务
│   │   ├── derive.go                # 密钥派生 (email → privkey)
│   │   ├── signer.go                # 交易/消息签名
│   │   └── encrypt.go               # 密码加密私钥
│   │
│   ├── gateway/                     # API 网关服务
│   │   ├── relay.go                 # 请求中继
│   │   ├── token.go                 # API Token 管理
│   │   ├── points.go                # 积分充值/消费/提现
│   │   ├── ratelimit.go             # 速率限制
│   │   └── channel.go               # Channel + 汇率管理
│   │
│   └── recovery/                    # 社交恢复
│
├── graph/                           # GraphQL API (现有)
│   ├── mod.go
│   ├── resolver.go
│   ├── *.graphqls                   # Schema
│   └── *.resolvers.go               # Resolver
│
├── ext/                             # 外部工具
├── hack/                            # 部署 / 构建脚本
├── docs/                            # 文档
└── tools/                           # 工具
```

---

## 部署架构

```
┌─────────────────────────────────────────────────────────────┐
│                      部署环境                                │
│                                                             │
│  ┌───────────────────────────────────────────────────────┐  │
│  │                 Kubernetes 集群                        │  │
│  │                                                       │  │
│  │  ┌──────────┐  ┌──────────┐  ┌──────────┐            │  │
│  │  │  tee-0   │  │  tee-1   │  │  tee-2   │  ...       │  │
│  │  │  (TEE)   │  │  (TEE)   │  │  (TEE)   │            │  │
│  │  │  SGX     │  │  SGX     │  │  SGX     │            │  │
│  │  └──────────┘  └──────────┘  └──────────┘            │  │
│  │                                                       │  │
│  │  需要:                                                │  │
│  │  - Intel SGX DCAP 驱动                                │  │
│  │  - Ego 运行时                                         │  │
│  │  - PCCS (Provisioning Certification Caching Service)  │  │
│  └───────────────────────────────────────────────────────┘  │
│                                                             │
│  ┌───────────────────────────────────────────────────────┐  │
│  │              外部依赖                                  │  │
│  │  - Polkadot / 任意链 的 RPC 端点                       │  │
│  │  - SMTP 服务器 (邮件发送)                               │  │
│  │  - OAuth 提供商 (Google / GitHub / Twitter)            │  │
│  │  - AI 供应商 API (OpenAI / Claude / ...)               │  │
│  └───────────────────────────────────────────────────────┘  │
└─────────────────────────────────────────────────────────────┘
```

---

## 启动方式

```bash
# 环境变量
export CHAIN_ENV=local               # 主链环境: local / test / main
export GQL_PORT=61000                # GraphQL 端口
export SIDE_CHAIN_PORT=61001         # P2P 端口
export SIDE_CHAIN_ADDR=0.0.0.0       # P2P 监听地址

# 邮箱配置 (Web3Auth 服务)
export SMTP_HOST=smtp.example.com
export SMTP_PORT=587
export SMTP_FROM=auth@example.com
export SMTP_USER=auth@example.com
export SMTP_PASSWORD=your-password
export MOCK_EMAIL=true               # 开发模式: 控制台打印 OTP

# OAuth 配置
export GOOGLE_CLIENT_ID=xxx
export GOOGLE_CLIENT_SECRET=xxx
export GITHUB_CLIENT_ID=xxx
export GITHUB_CLIENT_SECRET=xxx
export TWITTER_CLIENT_ID=xxx
export TWITTER_CLIENT_SECRET=xxx

# TEE 模式
export OE_MODE=SIM                   # 开发: SIM / 生产: HW

# 启动
go run main.go
```

---

## 与各项目的关系

| 项目 | 关系 |
|------|------|
| **web3auth** | `web3auth/back` 功能全部迁移到 tee-provider 的 `services/auth/` 和 `services/wallet/`；`web3auth/front` 保持不变，API 地址指向 tee-provider |
| **token** | `token` 的中继和 API 管理功能迁移到 tee-provider 的 `services/gateway/`；dashboard 静态页面由 tee-provider 托管；合约交互复用现有的 `graph/contract.resolvers.go` |
| **contract** | 合约部署在目标链上，tee-provider 通过 `pkg/chains` 模块与之交互。新增 Auth / Gateway 相关合约 |
| **dapp** | 前端不变，API 基地址指向 tee-provider 子网的任一节点 |
| **dashboard** | 管理面板不变，API 基地址指向 tee-provider 子网 |

---

## 安全模型

```
┌──────────────────────────────────────────────────────────┐
│                    信任模型                               │
│                                                          │
│  假设:                                                    │
│  ✗ 不信任任何单个节点                                      │
│  ✓ 信任 TEE 硬件 (Intel SGX / AMD SEV)                    │
│  ✓ 信任 > 2/3 的节点是诚实的 (BFT 假设)                     │
│  ✓ 信任 DKG 阈值 (t-of-n，至少 t 个节点诚实)                │
│                                                          │
│  攻击面:                                                  │
│  - TEE 侧信道攻击 → 依赖硬件厂商安全更新                     │
│  - 共识攻击 (≥1/3 恶意) → CometBFT 安全保证                │
│  - DKG 合谋 (<threshold) → 门限密码学保证                   │
│  - 网络分区 → 依赖主链最终性                                │
└──────────────────────────────────────────────────────────┘
```

---

## License

MIT
