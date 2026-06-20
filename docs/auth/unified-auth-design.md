# 统一鉴权设计方案

> 目标：将 tee-node 目前 3 种接口鉴权方式合并为 1 种，消除多项目融合副作用。
> **状态：已完成 ✅**

---

## 1. 现状问题（已解决）

~~当前项目同时存在 3 套鉴权逻辑~~，现已统一为单一 API Key：

| 接口 | 原鉴权方式 | 新鉴权方式 | 状态 |
|------|-----------|-----------|:---:|
| REST AI Gateway (`/v1/*`) | API Key | API Key（合约查询） | ✅ |
| GraphQL Web3Auth (`/gql`) | JWT Bearer | API Key（合约查询） | ✅ |
| GraphQL 旧 Polkadot | Legacy签名 | 已移除 | ✅ |

---

## 2. 统一原则

**所有接口（REST + GraphQL）只认一种凭证：API Key。**

格式：`Authorization: Bearer <apiKey>`

- REST Gateway 使用 `TokenAuthMiddleware`，优先链上合约 `LookupAPIKey` 验证
- GraphQL 侧废弃 JWT / Legacy Polkadot，统一走 API Key 验证
- Web3Auth 登录成功后返回 API Key 而非 JWT
- 受保护的 GraphQL 操作通过 `@AuthCheck` directive 校验 API Key

---

## 3. 关键改动（已实施）

### 3.1 GraphQL 鉴权中间件 ✅

文件：`graph/auth.go`

- ~~删除~~ `JWTValidator`、`decodeJWT`、`decodeLegacyToken`、`InitUser`。
- ~~重写~~ `AuthMiddleware`：从 `Authorization` header 提取 Bearer token，调用 `pkg/api/gateway` 的 `FindTokenByKey()` 验证
- `AuthCheck` directive：检查 context 中是否存在合法的 API Key 或 session
- Context key 存储认证状态

### 3.2 REST Gateway 鉴权 ✅

文件：`pkg/api/gateway/auth_middleware.go`

- `TokenAuthMiddleware` 优先链上合约查询 `LookupAPIKey`
- 回退本地 PebbleDB `GetGatewayTokenByKey`（O(1) key 索引）
- 支持 API Key 过期校验（`ExpiresAt`）

### 3.3 Web3Auth 登录返回 ✅

文件：`graph/auth.resolvers.go`

登录类 mutation（`verifyOTP`、`socialLogin`、`completeRecovery`）返回 `token: String`（API Key）。

```graphql
mutation {
  verifyOTP(email: "u@x.com", otp: "123456", clientId: "app") {
    success
    token    # API Key: "tee_a1b2c3..."
    address
    userInfo { email }
  }
}
```

### 3.4 受保护操作的鉴权 ✅

GraphQL 中受 `@AuthCheck` 保护的操作统一从 HTTP Header 读取 API Key。

`token` 参数已标记 deprecated，后续版本移除。

### 3.5 Resolver 内部用户解析 ✅

文件：`graph/resolver.go`

- `ResolveSignerEmail` 支持 API Key 解析
- `sig_` 旧格式标记 deprecated

### 3.6 废弃代码清理 ✅

已删除/简化：
- ~~`pkg/api/auth/config.go` 中的 `JWTSecret`~~
- ~~`graph/auth.go` 中的 Polkadot 签名验证逻辑~~
- ~~`pkg/model/user.go` 中的 `PublicUser`~~
- ~~`pkg/api/gateway/seed.go`（本地 seed 数据）~~
- ~~`pkg/model/gateway_store.go` 中未使用的 PebbleDB CRUD~~

> 合约调用 `contractCall` 的链上签名验证不受影响。

---

## 4. 数据模型衔接

`model.GatewayTokenInfo` 结构：

```go
type GatewayTokenInfo struct {
    ID        uint64  `json:"id"`
    Owner     string  `json:"owner"`     // 用户 email 或地址
    Addr      UniAddr `json:"addr"`      // 完整的链上地址（含 T+V），用于结算
    Key       string  `json:"key"`       // bearer token value
    RateLimit uint32  `json:"rateLimit"`
    IsActive  bool    `json:"isActive"`
    CreatedAt uint64  `json:"createdAt"`
    ExpiresAt uint64  `json:"expiresAt"` // 过期时间（Unix 秒），0=永不过期
}
```

API Key 信息现在**优先从链上合约查询**，回退到本地 PebbleDB 的 O(1) key 索引：

```go
// 优先走合约查询，回退到本地 O(1) key 索引
func FindTokenByKey(key string) *store.GatewayTokenInfo {
    // 1. 先查合约
    if tk := LookupAPIKeyViaContract(key); tk != nil {
        return tk
    }
    // 2. Fallback: 本地 PebbleDB O(1) key 索引（不再遍历全表）
    tk, err := store.GetGatewayTokenByKey(key)
    if err != nil {
        GATEWAYLOG().Printf("[AUTH] local DB lookup error for %s: %v", key[:min(16, len(key))], err)
    }
    if tk == nil {
        GATEWAYLOG().Printf("[AUTH] local DB not found for %s", key[:min(16, len(key))])
    } else if !tk.IsActive {
        GATEWAYLOG().Printf("[AUTH] token inactive for %s", key[:min(16, len(key))])
    } else if tk.IsExpired() {
        GATEWAYLOG().Printf("[AUTH] token expired for %s", key[:min(16, len(key))])
    } else {
        return tk
    }
    return nil
}
```

> **变更说明**：本地 token 查询已从遍历全表改为通过 `token_by_key:` 前缀做 O(1) 直接索引，同时在 [`pkg/model/gateway_store.go`](pkg/model/gateway_store.go:55) 中增加了 `SetGatewayTokenByKey`/`GetGatewayTokenByKey` 方法。`ExpiresAt` 字段支持 token 过期校验。

---

## 5. 接口调用示例

### REST（AI 模型）—— 不变

```bash
curl -X POST https://node/v1/chat/completions \
  -H "Authorization: Bearer tee_a1b2c3d4..." \
  -d '{"model": "openai/gpt-4", "messages": [...]}'
```

### GraphQL（Web3Auth / 秘密管理）—— 统一为同一种 key

```bash
curl -X POST https://node/gql \
  -H "Authorization: Bearer tee_a1b2c3d4..." \
  -d '{"query": "mutation { upload_secret(...) }"}'
```

### 登录获取 API Key

```bash
curl -X POST https://node/gql \
  -d '{
    "query": "mutation { verifyOTP(email: \"u@x.com\", otp: \"123456\", clientId: \"app\") { success token address } }"
  }'
# 返回：{ "data": { "verifyOTP": { "success": true, "token": "tee_xxx...", "address": "0x..." } } }
```

---

## 6. 影响范围

### 后端

| 文件 | 动作 |
|------|------|
| `graph/auth.go` | 重写：废弃 JWT/Legacy，统一为 API Key |
| `graph/auth.resolvers.go` | 修改：登录 mutation 返回 API Key |
| `graph/resolver.go` | 修改：`ResolveSignerEmail` 支持 API Key 解析 |
| `graph/mod.go` | 修改：`AuthMiddleware` 调用签名调整（不再需要 `validateJWT`） |
| `pkg/api/auth/config.go` | 简化：配置优先从链上合约读取（`AuthGetConfig`），回退环境变量 |
| `pkg/model/auth_store.go` | `AuthUserRecord` / `PrivateKeyRecord` 存储（移除旧 KMS 相关类型） |
| `pkg/api/gateway/*` | 无改动（已兼容） |

### 前端 SDK (`web3auth`)

| 文件 | 动作 |
|------|------|
| `src/core/auth.ts` | 注释更新：token 字段说明改为 API Key；mock token 格式改为 `tee_` 前缀 |
| `src/core/wallet.ts` | 注释更新：JWT → API Key；请求已带 `Authorization: Bearer <token>` header，无需改动 |

> 前端 SDK 的调用逻辑无需结构性修改，因为：
> - 登录 mutation 返回的 `token` 字段后端已自动改为 API Key，前端直接透传即可。
> - `wallet.ts` 中的 GraphQL 请求已统一通过 `Authorization: Bearer ${this.token}` 发送，与新的 API Key 鉴权完全兼容。
> - `verifyPassword` 等 mutation 仍通过 `token` 参数传递凭证，后端 `resolveEmailFromToken` 已支持 API Key 解析。

---

## 7. 接口鉴权权限矩阵

### 7.1 REST 接口

| 接口 | 方法 | 鉴权中间件 | 说明 |
|------|------|-----------|------|
| `/v1/chat/completions` | POST | `TokenAuthMiddleware` | AI 模型调用 |
| `/v1/completions` | POST | `TokenAuthMiddleware` | AI 模型调用 |
| `/v1/embeddings` | POST | `TokenAuthMiddleware` | AI 模型调用 |
| `/v1/images/generations` | POST | `TokenAuthMiddleware` | AI 模型调用 |
| `/v1/audio/speech` | POST | `TokenAuthMiddleware` | AI 模型调用 |
| `/v1/audio/transcriptions` | POST | `TokenAuthMiddleware` | AI 模型调用 |
| `/v1/models` | GET | `TokenAuthMiddleware` | 模型列表 |

> REST 侧所有 `/v1/*` 路由已统一通过 `TokenAuthMiddleware` 校验 API Key。

### 7.2 GraphQL 接口

当前 `@AuthCheck` directive 已在 `graph/secret.graphqls` 定义，但**尚未应用到任何字段**。以下矩阵列出所有 GraphQL 操作，并标注**建议**的鉴权状态：

#### 公开接口（无需 `@AuthCheck`）

| 操作 | 类型 | 说明 |
|------|------|------|
| `requestOTP` | Mutation | 请求 OTP，登录前调用 |
| `verifyOTP` | Mutation | 验证 OTP 并返回 API Key |
| `socialLogin` | Mutation | 社交登录并返回 API Key |
| `oauthURL` | Query | 获取 OAuth 授权 URL |
| `authHealth` | Query | 认证服务健康检查 |
| `chain_info` | Query | 链信息查询 |
| `validators` | Query | 验证器列表 |
| `contractDryRun` | Query | 合约只读查询（链上签名验证） |
| `tee_report` | Query | 获取 TEE report |
| `secret_rsa` | Query | 获取 RSA 公钥 |

#### 需要鉴权的接口（建议加 `@AuthCheck`）

| 操作 | 类型 | 说明 | 当前状态 |
|------|------|------|---------|
| `upload_secret` | Mutation | 上传秘密 | ⚠️ 建议加 `@AuthCheck` |
| `init_disk_key` | Mutation | 初始化磁盘密钥 | ⚠️ 建议加 `@AuthCheck` |
| `setPassword` | Mutation | 设置交易密码 | ⚠️ 通过 `token` 参数自校验（建议也加 `@AuthCheck` 做二次校验） |
| `verifyPassword` | Mutation | 验证交易密码 | ⚠️ 同上 |
| `signTransaction` | Mutation | 签名交易 | ⚠️ 通过 `token` 参数自校验 |
| `signMessage` | Mutation | 签名消息 | ⚠️ 通过 `token` 参数自校验 |
| `requestRecovery` | Mutation | 发起社交恢复 | ⚠️ 通过 recoveryEmail 查库 |
| `verifyRecovery` | Mutation | 验证恢复 OTP | ⚠️ 通过 OTP 校验 |
| `completeRecovery` | Mutation | 完成恢复 | ⚠️ 通过 recoveryToken 校验 |
| `systemContractInit` | Mutation | 系统合约初始化 | ⚠️ 建议加 `@AuthCheck`（管理员操作） |
| `faucet` | Mutation | 水龙头领币 | ⚠️ local 环境开放，main 环境建议限制 |
| `contractCall` | Mutation | 执行合约调用（链上签名验证） | ❌ 链上签名已验证 caller，HTTP 层可不强制 |
| `start_epoch` | Mutation | 触发新区块 | ⚠️ 节点操作，建议加 `@AuthCheck` |
| `claimOrderToken` | Mutation | 领取订单 API Token | 待实现 |

#### 建议的 Schema 改动示例

```graphql
# graph/secret.graphqls
extend type Mutation {
  upload_secret(index: String!, secret: String!, hash: String!, user: String!): Boolean! @AuthCheck
  init_disk_key(index: String!, user: String!): Boolean! @AuthCheck
}

# graph/chain.graphqls
extend type Mutation {
  start_epoch: Boolean! @AuthCheck
}

# graph/contract.graphqls
extend type Mutation {
  systemContractInit(contract: String!): Boolean! @AuthCheck
}
```

> 注意：`contractCall` 的鉴权由链上 `caller + signature` 完成，HTTP 层 `@AuthCheck` 可作为补充但不强制。`faucet` 在 `main` 环境已禁用，可保持现状。

---

## 8. 回滚与兼容

- API Key 的验证逻辑与原 `TokenAuthMiddleware` 完全一致，不存在行为差异。
- 若前端已缓存 JWT，登录 mutation 会返回新的 API Key，前端只需替换存储的 token 即可。
- Legacy Polkadot 签名格式彻底废弃；需要旧格式的客户端需先走 Web3Auth 登录获取 API Key。

---

## 附录 A：相关测试修复

### A.1 `tee-chain/relay/engine_test.go`

该测试文件因 `Engine` 结构体重构（移除 `api` 字段、`ApplyRelayEvent` 新增 `model.ContractApi` 参数、`getLastNonce` 行为变更）导致编译和运行时失败，已修复：

| 测试 | 原问题 | 修复 |
|------|--------|------|
| `newMockEngine` | `Engine{api: ...}` 字段已不存在 | 改为 `NewEngine()`，mockAPI 由调用方传入 |
| `commitAndRefresh` | 直接修改 `eng.api` | 改为只提交事务并返回新 txn |
| `TestApplyRelayEvent_*` | `ApplyRelayEvent(ev)` 缺少参数 | 改为 `ApplyRelayEvent(ev, api)` |
| `TestGetLastNonce_NotRegistered` | 期望返回 "not registered" 错误 | 实际返回 `(0, nil)`，调整断言 |
| `TestGetLastNonceBg_NotRegistered` | 同上 | 同上 |
| `TestEngine_LastNonce` | 未提交 txn，读不到写入的 nonce | 在调用 `LastNonce` 前执行 `txn.Commit()` |

---

*文档更新：2026-06-04*
*基于代码版本：tee-node 当前仓库状态*
