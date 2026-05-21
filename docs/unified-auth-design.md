# 统一鉴权设计方案

> 目标：将 tee-node 目前 3 种接口鉴权方式合并为 1 种，消除多项目融合副作用。

---

## 1. 现状问题

当前项目同时存在 3 套鉴权逻辑：

| 接口 | 鉴权方式 | 代码位置 |
|------|---------|---------|
| REST AI Gateway (`/v1/*`) | API Key (`Authorization: Bearer <key>`) | [`pkg/api/gateway/auth_middleware.go`](../pkg/api/gateway/auth_middleware.go) |
| GraphQL Web3Auth (`/gql`) | JWT Bearer (`Authorization: Bearer <jwt>`) | [`graph/auth.go`](../graph/auth.go) |
| GraphQL 旧 Polkadot | Legacy签名 (`hex\|\|hex`) | [`graph/auth.go`](../graph/auth.go) |

这导致：
- 前端/SDK 需要同时携带两套凭证。
- 鉴权中间件分散在两处，维护成本高。
- 登录后拿到的是 JWT，调用 AI 模型时却需要 API Key，体验割裂。

---

## 2. 统一原则

**所有接口（REST + GraphQL）只认一种凭证：API Key。**

格式：`Authorization: Bearer <apiKey>`

- REST Gateway 已天然支持，保持不变。
- GraphQL 侧废弃 JWT / Legacy Polkadot 格式，改为走同一套 API Key 验证。
- Web3Auth 登录成功后**不再返回 JWT**，而是返回/绑定到一个 API Key。
- 受保护的 GraphQL 操作通过 `@AuthCheck` directive 校验 API Key。

---

## 3. 关键改动

### 3.1 GraphQL 鉴权中间件

文件：`graph/auth.go`

- 删除 `JWTValidator`、`decodeJWT`、`decodeLegacyToken`、`InitUser`。
- 重写 `AuthMiddleware`：从 `Authorization` header 提取 Bearer token，调用 `pkg/api/gateway` 已有的 `findTokenCached()` 验证 `GatewayTokenInfo`。
- `AuthCheck` directive：检查 context 中是否存在合法的 `*model.GatewayTokenInfo`；若不存在返回 401 错误。
- Context key `loginStatCtxKey` 改为存储 `*model.GatewayTokenInfo`（而非 `*model.PublicUser`）。

### 3.2 REST Gateway 鉴权

文件：`pkg/api/gateway/auth_middleware.go`

- `TokenAuthMiddleware` 逻辑已符合要求，**无需改动**。
- 但需将 `findTokenCached` 与相关缓存逻辑抽取到 `pkg/api/auth` 或保持 `gateway` 包导出，供 GraphQL 侧复用。

> 当前 `findTokenCached` 与 `GetCachedTokens` 已是 `gateway` 包级别导出函数，GraphQL 侧可直接调用。无需额外抽取。

### 3.3 Web3Auth 登录返回

文件：`graph/auth.resolvers.go`

登录类 mutation（`verifyOTP`、`socialLogin`、`completeRecovery`）当前返回 `token: String`（JWT）。

改为：
1. 登录验证通过后，检查该用户是否已有 API Key。
2. 若无，自动创建一条 `GatewayTokenInfo`（Owner=email，Key=随机生成）。
3. 返回的 `token` 字段即为 API Key 明文（用户需保存）。

```graphql
mutation {
  verifyOTP(email: "u@x.com", otp: "123456", clientId: "app") {
    success
    token    # 现在是 API Key，如 "tee_a1b2c3..."
    address
    userInfo { email }
  }
}
```

### 3.4 受保护操作的鉴权

GraphQL 中受 `@AuthCheck` 保护的操作（如 `upload_secret`、`signTransaction` 等）不再需要传 `token` 参数，而是统一从 HTTP Header 读取 API Key。

- 保留 `token` 参数作为向后兼容的备选（内部与 header 取到的值做一致性校验，若不一致返回错误）。
- 或者一次性清理：删除所有 mutation 的 `token` 参数，完全依赖 header。

本方案采用**渐进清理**：先让 header 成为主鉴权源，`token` 参数暂时保留但可选；后续版本再移除参数。

### 3.5 Resolver 内部用户解析

文件：`graph/resolver.go`

- `ResolveSignerEmail` 当前支持 JWT 和 `sig_` token。
- 改为：从传入的 token 字符串（若为 API Key）查 `FindGatewayTokenByKey`，取 `Owner` 字段作为用户标识（email）。
- 若传入的是 `sig_` 旧格式，保留兼容逻辑，但标记为 deprecated。

### 3.6 废弃代码清理

可删除/简化：
- `pkg/api/auth/config.go` 中的 `JWTSecret`（JWT 不再使用）。
- `graph/auth.go` 中的 Polkadot 签名验证逻辑（`decodeLegacyToken`）。
- `pkg/model/user.go` 中的 `PublicUser`（若 GraphQL 不再使用）。

> 注意：合约调用 `contractCall` 使用的是链上签名验证（caller + signature），与 HTTP 层鉴权无关，**不受影响**。

---

## 4. 数据模型衔接

`model.GatewayTokenInfo` 当前结构：

```go
type GatewayTokenInfo struct {
    Owner     string `json:"owner"`     // 当前多为 "admin" 或简单标识
    Key       string `json:"key"`       // bearer token value
    RateLimit uint32 `json:"rateLimit"`
    IsActive  bool   `json:"isActive"`
    CreatedAt uint64 `json:"createdAt"`
    ID        uint64 `json:"id"`
}
```

Web3Auth 用户登录后生成的 API Key，`Owner` 字段填入用户 email（或地址）。这样：
- REST Gateway 的计费/限流仍然工作。
- GraphQL 的 `@AuthCheck` 也能识别用户身份。

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

| 文件 | 动作 |
|------|------|
| `graph/auth.go` | 重写：废弃 JWT/Legacy，统一为 API Key |
| `graph/auth.resolvers.go` | 修改：登录 mutation 返回 API Key |
| `graph/resolver.go` | 修改：`ResolveSignerEmail` 支持 API Key 解析 |
| `graph/mod.go` | 修改：`AuthMiddleware` 调用签名调整（不再需要 `validateJWT`） |
| `pkg/api/auth/config.go` | 简化：移除 `JWTSecret` 等不再使用的配置 |
| `pkg/model/user.go` | 可选：移除 `PublicUser`（如确认无其他引用） |
| `pkg/api/gateway/*` | 无改动（已兼容） |

---

## 7. 回滚与兼容

- API Key 的验证逻辑与原 `TokenAuthMiddleware` 完全一致，不存在行为差异。
- 若前端已缓存 JWT，登录 mutation 会返回新的 API Key，前端只需替换存储的 token 即可。
- Legacy Polkadot 签名格式彻底废弃；需要旧格式的客户端需先走 Web3Auth 登录获取 API Key。

---

*文档版本：2026-05-21*
*基于代码版本：tee-node 当前仓库状态*
