# web3auth + token → tee-provider 融合方案

本文档详细描述如何将 `web3auth/back` 和 `token` 项目的代码融合到 `tee-provider` 中。**web3auth 通过 GraphQL 接口暴露**，复用 tee-provider 现有的 GraphQL 基础设施。

---

## 目录

1. [总体策略](#1-总体策略)
2. [Phase 1: web3auth 迁移 (GraphQL)](#2-phase-1-web3auth-迁移-graphql)
3. [Phase 2: token 中继迁移](#3-phase-2-token-中继迁移)
4. [Phase 3: 合约层实现](#4-phase-3-合约层实现)
5. [Phase 4: DKG 阈值签名](#5-phase-4-dkg-阈值签名)
6. [Phase 5: 统一部署](#6-phase-5-统一部署)
7. [附录: 文件迁移对照表](#7-附录-文件迁移对照表)

---

## 1. 总体策略

### 1.1 原则

| 原则 | 说明 |
|------|------|
| 最小破坏 | 优先直接复制 + 适配，不重写能用的代码 |
| 统一存储 | web3auth 的 Pebble Storage 合并到 `pkg/model` |
| **GraphQL 优先** | web3auth 全部功能通过 GraphQL 暴露，不复用 REST handler |
| 统一配置 | 环境变量统一到 `util.GetEnv()` 模式 |
| 合约上链 | 认证状态、API Token、计费逐步上链 |

### 1.2 接口模式

| 项目 | 原接口 | tee-provider 接口 |
|------|--------|------------------|
| web3auth | REST (chi): `POST /auth/*`, `POST /wallet/*` | **GraphQL**: mutation/query 走 `/gql` |
| token | REST (gin): `POST /v1/chat/completions` 等 | REST (chi): 保持 `/v1/*`（兼容 OpenAI SDK） |
| tee-provider | GraphQL: `/gql` | 不变，扩展 schema |

---

## 2. Phase 1: web3auth 迁移 (GraphQL)

### 2.1 目标

web3auth 全部功能通过 **GraphQL** 暴露，不创建 REST 路由。复用现有 `graph/` 目录的 Schema + Resolver 模式。

### 2.2 架构对比

```
之前 web3auth (REST):
  前端 → POST /auth/request-otp     { email, clientId }
       → POST /auth/verify-otp      { email, otp, clientId }
       → POST /wallet/sign-transaction  { ...tx, passwordHash }

tee-provider (GraphQL):
  前端 → POST /gql
       mutation {
         requestOTP(email: "u@x.com", clientId: "app") { success error }
         verifyOTP(email: "u@x.com", otp: "123456") { success token address userInfo { email } }
         signTransaction(token: "..", tx: { from, to, value, data }) { status result }
       }
```

### 2.3 文件迁移清单

```
web3auth/back/                         tee-provider/
────────────────────────────────────────────────────────────
config/config.go          → pkg/api/auth/config.go         (去掉端口，只保留业务配置)
model/types.go            → 输入类型写入 graph/auth.graphqls
                            内部类型保留在 pkg/api/auth/types.go
storage/store.go          → pkg/model/auth_store.go         (合并到主 Pebble)
service/otp.go            → services/auth/otp.go            (不变)
service/email.go          → services/auth/email.go          (不变)
service/oauth.go          → services/auth/oauth.go          (不变)
service/auth.go           → services/wallet/derive.go       (拆分)
                          → services/auth/jwt.go
                          → services/wallet/signer.go
                          → services/wallet/encrypt.go
handler/otp.go            → 废弃 (逻辑移入 graph/auth.resolvers.go)
handler/social.go         → 废弃
handler/wallet.go         → 废弃
handler/recovery.go       → 废弃
handler/helpers.go        → 废弃
```

### 2.4 GraphQL Schema 定义

tee-provider 现有 GraphQL 按模块拆分 schema 文件，auth 也按此模式：

**`graph/auth.graphqls`**：

```graphql
# ── 类型定义 ────────────────────────────────────────────

type AuthResponse {
  success: Boolean!
  userInfo: UserInfo
  token: String
  address: String
  error: String
}

type UserInfo {
  email: String!
  name: String
  profileImage: String
  verifier: String
  verifierId: String
  typeOfLogin: String
  aggregateVerifier: String
  authConnection: String
  authConnectionId: String
}

type OAuthURL {
  authUrl: String!
}

type WalletResult {
  status: String!
  result: String
  error: String
}

type RecoveryResult {
  status: String!
  recoveryToken: String
  token: String
  address: String
  email: String
  error: String
}

# ── 输入类型 ────────────────────────────────────────────

input TxInput {
  from: String!
  to: String!
  value: String
  data: String
  gas: String
  gasPrice: String
  nonce: String
  chainId: String
  passwordHash: String
}

# ── Mutation ────────────────────────────────────────────

extend type Mutation {
  """请求 OTP 验证码"""
  requestOTP(email: String!, clientId: String!): AuthResponse!

  """验证 OTP 并完成登录，返回 token + 用户信息"""
  verifyOTP(email: String!, otp: String!, clientId: String!): AuthResponse!

  """社交登录 (provider: google / github / twitter)"""
  socialLogin(
    provider: String!
    idToken: String
    code: String
    codeVerifier: String
  ): AuthResponse!

  """设置交易密码"""
  setPassword(token: String!, passwordHash: String!): WalletResult!

  """签名交易 (Ethereum / 任意 EVM 链)"""
  signTransaction(token: String!, tx: TxInput!): WalletResult!

  """签名消息"""
  signMessage(token: String!, message: String!, from: String!): WalletResult!

  """发起社交恢复 — 向恢复邮箱发送 OTP"""
  requestRecovery(recoveryEmail: String!): RecoveryResult!

  """验证恢复 OTP — 返回临时 recoveryToken"""
  verifyRecovery(recoveryEmail: String!, otp: String!): RecoveryResult!

  """完成恢复 — 用 recoveryToken 重置密码"""
  completeRecovery(recoveryToken: String!, passwordHash: String!): RecoveryResult!
}

# ── Query ───────────────────────────────────────────────

extend type Query {
  """获取 OAuth 授权 URL (用于服务端重定向流)"""
  oauthURL(provider: String!): OAuthURL!

  """服务健康检查"""
  authHealth: String!
}
```

### 2.5 GraphQL Resolver 实现

tee-provider 已有的 resolver 模式（见 `graph/secret.resolvers.go`、`graph/chain.resolvers.go`）是通过 `gqlgen` 生成 `graph/generated.go`，业务逻辑写在 `*Resolvers` 方法中。

**`graph/resolver.go` 注入依赖**：

```go
package graph

import (
    teechain "github.com/computeflux/tee-provider/tee-chain"
    "github.com/computeflux/tee-provider/pkg/api/auth"
    "github.com/computeflux/tee-provider/services/auth"
    "github.com/computeflux/tee-provider/services/wallet"
)

type Resolver struct{
    TeeChain *teechain.TeeChain
    AuthCfg   *auth.AuthConfig
    WalletSvc *wallet.WalletService
    JWTSvc    *auth.JWTService
    EmailSvc  auth.EmailService
    OAuthSvc  *auth.OAuthService
    OTPSvc    *auth.OTPService
}
```

**`graph/auth.resolvers.go`**：

```go
package graph

import (
    "context"
    "github.com/computeflux/tee-provider/pkg/api/auth"
    "github.com/computeflux/tee-provider/pkg/model"
    "github.com/computeflux/tee-provider/services/wallet"
)

// ── Mutation resolvers ───────────────────────────────────

func (r *mutationResolver) RequestOTP(ctx context.Context, email, clientID string) (*auth.AuthResponse, error) {
    otp, err := r.OTPSvc.Generate(email, clientID)
    if err != nil {
        return &auth.AuthResponse{Success: false, Error: err.Error()}, nil
    }
    if err := r.EmailSvc.SendOTP(email, otp); err != nil {
        return &auth.AuthResponse{Success: false, Error: "failed to send email"}, nil
    }
    return &auth.AuthResponse{Success: true}, nil
}

func (r *mutationResolver) VerifyOTP(ctx context.Context, email, otp, clientID string) (*auth.AuthResponse, error) {
    if err := r.OTPSvc.Verify(email, otp); err != nil {
        return &auth.AuthResponse{Success: false, Error: err.Error()}, nil
    }

    provider := "email_passwordless"
    address := r.WalletSvc.DeriveAddress(email)
    userInfo := r.buildUserInfo(email, provider)
    token, err := r.JWTSvc.IssueToken(email, address, provider)
    if err != nil {
        return &auth.AuthResponse{Success: false, Error: "failed to issue token"}, nil
    }

    // 持久化到 Pebble
    model.PutAuthUser(&model.AuthUser{
        Email: email, Name: userInfo.Name, Address: address,
        AuthProvider: provider, LastLogin: time.Now().Unix(),
    })
    model.PutAuthSession(&model.AuthSession{
        Token: token, Email: email, Address: address,
        AuthProvider: provider, CreatedAt: time.Now().Unix(),
    })

    return &auth.AuthResponse{
        Success: true, UserInfo: userInfo, Token: token, Address: address,
    }, nil
}

func (r *mutationResolver) SocialLogin(ctx context.Context, provider string,
    idToken, code, codeVerifier *string) (*auth.AuthResponse, error) {

    var oauthUser *auth.OAuthUserInfo
    var err error

    switch provider {
    case "google":
        if idToken != nil && *idToken != "" {
            oauthUser, err = r.OAuthSvc.VerifyGoogleIDToken(*idToken)
        } else if code != nil && *code != "" {
            oauthUser, err = r.OAuthSvc.ExchangeGoogleCode(*code)
        }
    case "github":
        if code != nil && *code != "" {
            oauthUser, err = r.OAuthSvc.ExchangeGitHubCode(*code)
        }
    case "twitter", "x":
        v := ""
        if codeVerifier != nil { v = *codeVerifier }
        oauthUser, err = r.OAuthSvc.ExchangeXCode(*code, v)
    default:
        return &auth.AuthResponse{Success: false, Error: "unsupported provider: " + provider}, nil
    }

    if err != nil {
        return &auth.AuthResponse{Success: false, Error: err.Error()}, nil
    }

    address := r.WalletSvc.DeriveAddress(oauthUser.Email)
    token, _ := r.JWTSvc.IssueToken(oauthUser.Email, address, provider)
    userInfo := &auth.UserInfo{
        Email: oauthUser.Email, Name: oauthUser.Name,
        ProfileImage: oauthUser.AvatarURL,
        TypeOfLogin: provider, Verifier: provider, VerifierID: oauthUser.ProviderID,
    }
    return &auth.AuthResponse{Success: true, UserInfo: userInfo, Token: token, Address: address}, nil
}

func (r *mutationResolver) SignTransaction(ctx context.Context, tokenStr string, tx auth.TxInput) (*auth.WalletResult, error) {
    claims, err := r.JWTSvc.ParseToken(tokenStr)
    if err != nil {
        return &auth.WalletResult{Status: "error", Error: "unauthorized"}, nil
    }
    raw, err := r.WalletSvc.SignTx(claims.Email, &wallet.TxRequest{
        From: tx.From, To: tx.To, Value: tx.Value, Data: tx.Data,
        Gas: tx.Gas, GasPrice: tx.GasPrice, Nonce: tx.Nonce,
        ChainID: tx.ChainID, PasswordHash: tx.PasswordHash,
    })
    if err != nil {
        return &auth.WalletResult{Status: "error", Error: err.Error()}, nil
    }
    return &auth.WalletResult{Status: "ok", Result: raw}, nil
}

func (r *mutationResolver) SignMessage(ctx context.Context, tokenStr, message, from string) (*auth.WalletResult, error) {
    claims, err := r.JWTSvc.ParseToken(tokenStr)
    if err != nil {
        return &auth.WalletResult{Status: "error", Error: "unauthorized"}, nil
    }
    sig, err := r.WalletSvc.SignMessage(claims.Email, message)
    if err != nil {
        return &auth.WalletResult{Status: "error", Error: err.Error()}, nil
    }
    return &auth.WalletResult{Status: "ok", Result: sig}, nil
}

func (r *mutationResolver) SetPassword(ctx context.Context, tokenStr, passwordHash string) (*auth.WalletResult, error) {
    claims, err := r.JWTSvc.ParseToken(tokenStr)
    if err != nil {
        return &auth.WalletResult{Status: "error", Error: "unauthorized"}, nil
    }
    if err := r.WalletSvc.EncryptPrivateKey(claims.Email, passwordHash); err != nil {
        return &auth.WalletResult{Status: "error", Error: err.Error()}, nil
    }
    return &auth.WalletResult{Status: "ok"}, nil
}

// ── Query resolvers ──────────────────────────────────────

func (r *queryResolver) OAuthURL(ctx context.Context, provider string) (*auth.OAuthURL, error) {
    switch provider {
    case "google":  return &auth.OAuthURL{AuthUrl: r.OAuthSvc.GoogleAuthURL("")}, nil
    case "github":  return &auth.OAuthURL{AuthUrl: r.OAuthSvc.GitHubAuthURL("")}, nil
    default:        return nil, fmt.Errorf("unsupported provider: %s", provider)
    }
}

func (r *queryResolver) AuthHealth(ctx context.Context) (string, error) {
    return "ok", nil
}

// ── helpers ──────────────────────────────────────────────

func (r *mutationResolver) buildUserInfo(email, provider string) *auth.UserInfo {
    name := email
    if i := strings.Index(email, "@"); i > 0 {
        name = email[:i]
    }
    return &auth.UserInfo{
        Email: email, Name: name, TypeOfLogin: provider,
        Verifier: provider, VerifierID: email,
    }
}
```

### 2.6 gqlgen 代码生成

1. 将 `auth.graphqls` 放入 `graph/`
2. 更新 `graph/generated.go` 中的 `NewExecutableSchema` — gqlgen 自动包含 `*.graphqls`
3. 确认 `graph/resolver.go` 中的 `Resolver` 包含所有新增字段
4. `go generate ./...` 重新生成

### 2.7 main.go 修改

```go
func main() {
    // ... 现有初始化 (db, p2p, dkg, teechain) 不变 ...

    // 新增：初始化 Web3Auth 服务
    authCfg := auth.LoadConfig()
    walletSvc := wallet.NewWalletService(authCfg)
    jwtSvc := auth.NewJWTService(authCfg)
    emailSvc := auth.NewEmailService(authCfg)
    oauthSvc := auth.NewOAuthService(authCfg)
    otpSvc := auth.NewOTPService(authCfg)

    // 创建 Resolver，注入所有依赖
    resolver := &graph.Resolver{
        TeeChain: teeChain,
        AuthCfg:   authCfg,
        WalletSvc: walletSvc,
        JWTSvc:    jwtSvc,
        EmailSvc:  emailSvc,
        OAuthSvc:  oauthSvc,
        OTPSvc:    otpSvc,
    }

    // 启动 GraphQL
    go graph.StartServerWithResolver(teeChain, gqlPort, resolver)

    // ... 信号等待 ...
}
```

`graph/mod.go` 中 `StartServer` 改为接受 `*Resolver` 参数：

```go
func StartServerWithResolver(s *teechain.TeeChain, port int, resolver *Resolver) {
    teeChain = s
    router := chi.NewRouter()
    router.Use(AuthMiddleware())
    router.Use(cors.Handler(cors.Options{...}))

    srv := handler.NewDefaultServer(NewExecutableSchema(Config{
        Resolvers: resolver,
        Directives: NewDirectiveRoot(),
    }))

    router.Handle("/", playground.Handler("WeTEE", "/gql"))
    router.Handle("/gql", srv)

    http.ListenAndServe(":"+fmt.Sprint(port), router)
}
```

### 2.8 验收

- [ ] `requestOTP` — 返回 `{ success: true }`（mock 模式控制台打印 OTP）
- [ ] `verifyOTP` — 返回 `{ success, token, address, userInfo }`
- [ ] `socialLogin(provider:"google", idToken:"...")` — 社交登录成功
- [ ] `signTransaction` — 返回签名后的 raw tx
- [ ] `signMessage` — 返回签名
- [ ] `setPassword` — 设置密码成功
- [ ] `requestRecovery` / `verifyRecovery` / `completeRecovery` — 恢复流程
- [ ] `authHealth` — 返回 `"ok"`
- [ ] 现有 query `chain_info`、`validators`、`tee_report` 等不受影响

---

## 3. Phase 2: token 中继迁移

### 3.1 目标

token 的 AI API 中继功能迁移到 tee-provider，**保持 REST 接口**（兼容 OpenAI SDK），gin → chi 适配。

### 3.2 文件迁移清单

```
token/                                 tee-provider/
────────────────────────────────────────────────────────────
relay/adaptor/             → services/gateway/adaptor/       (18个适配器，完全不变)
relay/adaptor.go           → services/gateway/adaptor.go     (工厂方法，不变)
relay/controller/          → services/gateway/relay/controller.go
relay/relaymode/           → services/gateway/relay/relaymode.go

middleware/distributor.go  → pkg/api/gateway/middleware.go   (gin → chi)
middleware/auth.go         → pkg/api/gateway/auth_mw.go      (gin → chi)
middleware/rate-limit.go   → pkg/api/gateway/ratelimit.go    (gin → chi)

# 以下由合约替代，不迁移
model/token.go / channel.go / product.go    → contracts/gateway/
controller/billing.go / token.go / user.go  → contracts/gateway/
```

### 3.3 gin → chi 中间件适配

```go
// pkg/api/gateway/middleware.go
package gateway

func TokenAuthMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        key := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
        if key == "" {
            http.Error(w, `{"error":"missing api key"}`, 401)
            return
        }
        // Phase 2: 从内存/Pebble 验证 (Phase 3 改为合约查询)
        token := lookupToken(key)
        if token == nil || !token.IsActive {
            http.Error(w, `{"error":"invalid api key"}`, 401)
            return
        }
        ctx := context.WithValue(r.Context(), ctxToken, token)
        next.ServeHTTP(w, r.WithContext(ctx))
    })
}

func DistributeMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        // 从请求体解析 model name
        bodyBytes, _ := io.ReadAll(r.Body)
        r.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
        var reqBody struct{ Model string `json:"model"` }
        json.Unmarshal(bodyBytes, &reqBody)

        channel := selectBestChannel(reqBody.Model)
        if channel == nil {
            http.Error(w, `{"error":"no available channel"}`, 503)
            return
        }
        ctx := context.WithValue(r.Context(), ctxChannel, channel)
        next.ServeHTTP(w, r.WithContext(ctx))
    })
}
```

### 3.4 Relay Handler

```go
// pkg/api/gateway/relay_handler.go
func HandleRelay(w http.ResponseWriter, r *http.Request) {
    channel := r.Context().Value(ctxChannel).(*ChannelInfo)

    bodyBytes, _ := io.ReadAll(r.Body)
    r.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))

    adaptor := gateway.GetAdaptor(channel.ApiType)
    resp, err := adaptor.DoRequest(r.Context(), channel.BaseURL, channel.Key, bodyBytes)
    if err != nil {
        http.Error(w, `{"error":"`+err.Error()+`"}`, 502)
        return
    }

    // Phase 3: 合约扣费
    // usage := adaptor.GetUsage(resp)
    // teeChain.ContractGatewayConsume(tokenId, channelId, modelId, usage.TokensUsed)

    w.Header().Set("Content-Type", "application/json")
    w.Write(resp.Body)
}

func RegisterGatewayRoutes(r chi.Router) {
    r.Route("/v1", func(r chi.Router) {
        r.Use(TokenAuthMiddleware)
        r.Use(DistributeMiddleware)
        r.Post("/chat/completions", HandleRelay)
        r.Post("/completions", HandleRelay)
        r.Post("/embeddings", HandleRelay)
        r.Post("/images/generations", HandleRelay)
        r.Post("/audio/speech", HandleRelay)
        r.Post("/audio/transcriptions", HandleRelay)
        r.Get("/models", HandleListModels)
    })
}
```

### 3.5 main.go 集成

```go
func main() {
    // ... GraphQL (含 auth) ...
    // ... Gateway REST (与 GraphQL 共用 chi router) ...

    go func() {
        r := chi.NewRouter()
        r.Use(corsMiddleware("*"))

        // GraphQL (含 web3auth)
        r.Handle("/gql", gqlHandler)
        r.Handle("/", playgroundHandler)

        // Gateway REST (兼容 OpenAI SDK)
        gateway.RegisterGatewayRoutes(r)

        http.ListenAndServe(":61000", r)
    }()
}
```

---

## 4. Phase 3: 合约层实现

### 4.1 Auth 合约 (`contracts/auth/`)

```go
type Auth struct {
    api         model.ContractApi
    users       *model.StoreMapping[H160, UserInfo]        // email_hash → UserInfo
    shareRoots  *model.StoreMapping[H160, H256]            // 分片承诺根
    recoveries  *model.StoreMapping[H160, RecoveryConfig]  // 恢复配置
    dkgPubKey   *model.StoreValue[[]byte]
    threshold   *model.StoreValue[uint32]
}

type UserInfo struct {
    EmailHash      H160
    ShareRoot      H256
    DerivedAddress []byte
    RegisteredAt   uint64
}

// Mutation
func (a AuthMutation) Init() error { ... }
func (a AuthMutation) RegisterUser(emailHash H160, shareRoot H256, address []byte) error { ... }
func (a AuthMutation) SetRecovery(emailHash H160, cfg RecoveryConfig) error { ... }
func (a AuthMutation) RequestRecovery(emailHash H160) error { ... }
func (a AuthMutation) CompleteRecovery(emailHash H160, newShareRoot H256) error { ... }

// Query
func (a AuthQuery) GetUser(emailHash H160) (*UserInfo, error) { ... }
func (a AuthQuery) HasKey(emailHash H160) (bool, error) { ... }
func (a AuthQuery) GetRecovery(emailHash H160) (*RecoveryConfig, error) { ... }
```

### 4.2 Gateway 合约 (`contracts/gateway/`)

```go
type Gateway struct {
    api              model.ContractApi
    balances         *model.StoreMapping[UniAddr, Amount]       // 用户积分
    models           *model.StoreMapping[uint64, ModelInfo]     // 独立注册的模型
    providerBalances *model.StoreMapping[uint64, Amount]        // 模型收益
    channels         *model.StoreMapping[uint64, ChannelInfo]   // Channel
    tokens           *model.StoreMapping[uint64, TokenInfo]     // API token
    usages           *model.StoreMapping[string, DailyUsage]
}

type ModelInfo struct {
    Name        string
    Owner       UniAddr
    PointsPer1K uint64
    IsActive    bool
}

type ChannelInfo struct {
    Name     string
    BaseURL  string
    ModelIds []uint64      // 关联已注册的模型
    Priority uint32
    IsActive bool
    Owner    UniAddr
}

// Mutation
func (g GatewayMutation) RegisterModel(owner UniAddr, name string, pointsPer1K uint64) error { ... }
func (g GatewayMutation) CreateChannel(owner UniAddr, name, baseUrl string, modelIds []uint64, prio uint32) error { ... }
func (g GatewayMutation) Recharge(user UniAddr, amount Amount) error { ... }
func (g GatewayMutation) Consume(tokenId, channelId, modelId uint64, tokensUsed uint64) error { ... }
func (g GatewayMutation) Withdraw(modelId uint64, amount Amount) error { ... }
func (g GatewayMutation) CreateToken(owner UniAddr, rateLimit uint32) error { ... }

// Query
func (g GatewayQuery) GetBalance(user UniAddr) (*Amount, error) { ... }
func (g GatewayQuery) GetModel(modelId uint64) (*ModelInfo, error) { ... }
func (g GatewayQuery) GetChannel(channelId uint64) (*ChannelInfo, error) { ... }
func (g GatewayQuery) ListActiveChannels(modelId uint64) ([]uint64, error) { ... }
```

### 4.3 合约注册

```go
// contracts/contract.go
func Mutation(call *model.ContractCall, runtime model.ContractApi) error {
    switch string(call.Name) {
    case "gov":     return gov.GovMutation{Gov: *gov.NewGov(runtime)}.ExecCall(call)
    case "auth":    return auth.AuthMutation{Auth: *auth.NewAuth(runtime)}.ExecCall(call)
    case "gateway": return gateway.GatewayMutation{Gateway: *gateway.NewGateway(runtime)}.ExecCall(call)
    }
}
func Query(call *model.ContractCall, runtime model.ContractApi) ([]byte, error) {
    switch string(call.Name) {
    case "gov":     return gov.GovQuery{Gov: *gov.NewGov(runtime)}.ExecQuery(call)
    case "auth":    return auth.AuthQuery{Auth: *auth.NewAuth(runtime)}.ExecQuery(call)
    case "gateway": return gateway.GatewayQuery{Gateway: *gateway.NewGateway(runtime)}.ExecQuery(call)
    }
}
```

---

## 5. Phase 4: DKG 阈值签名

Phase 1 中 `signTransaction` resolver 使用单节点 ECDSA。Phase 4 升级为 DKG 阈值签名：

```go
// graph/auth.resolvers.go — Phase 4 版本
func (r *mutationResolver) SignTransaction(ctx context.Context, tokenStr string, tx auth.TxInput) (*auth.WalletResult, error) {
    claims, err := r.JWTSvc.ParseToken(tokenStr)
    emailHash := sha256.Sum256([]byte(claims.Email))

    // 1. 从 Auth 合约查询用户
    user := r.TeeChain.ContractAuthQuery("GetUser", emailHash[:])
    if user == nil {
        return &auth.WalletResult{Status: "error", Error: "user not registered"}, nil
    }

    // 2. 向所有 DKG 节点广播部分签名请求
    req := &model.SignRequest{EmailHash: emailHash[:], TxHash: txHash(tx)}
    partialSigs := r.TeeChain.BroadcastPartialSign(req)

    // 3. 聚合
    fullSig := tss.Aggregate(partialSigs, int(user.Threshold))
    return &auth.WalletResult{Status: "ok", Result: fullSig}, nil
}
```

---

## 6. Phase 5: 统一部署

### 6.1 环境变量

```bash
# tee-provider 核心
CHAIN_ENV=local
GQL_PORT=61000
SIDE_CHAIN_PORT=61001
SIDE_CHAIN_ADDR=0.0.0.0

# Web3Auth
JWT_SECRET=<random>
SEED_SECRET=<random>
MOCK_EMAIL=true
SMTP_HOST=smtp.example.com
SMTP_PORT=587
SMTP_FROM=auth@example.com
SMTP_USER=auth@example.com
SMTP_PASSWORD=<password>
SMTP_FROM_NAME="WeTEE Auth"
GOOGLE_CLIENT_ID=<id>
GOOGLE_CLIENT_SECRET=<secret>
GOOGLE_REDIRECT_URL=https://tee.example.com/auth/callback/google
GITHUB_CLIENT_ID=<id>
GITHUB_CLIENT_SECRET=<secret>
GITHUB_REDIRECT_URL=https://tee.example.com/auth/callback/github
TWITTER_CLIENT_ID=<id>
TWITTER_CLIENT_SECRET=<secret>
TWITTER_REDIRECT_URL=https://tee.example.com/auth/callback/twitter

# TEE
OE_MODE=SIM
```

### 6.2 端口规划

| 端口 | 服务 | 接口 |
|------|------|------|
| 61000 | HTTP (统一) | `/gql` (GraphQL: auth + chain + secret), `/v1/*` (REST: gateway) |
| 61001 | P2P | CometBFT 节点通信 |
| 61001+1 | RPC | CometBFT RPC |

---

## 7. 附录: 文件迁移对照表

### web3auth → tee-provider

| 源 | 目标 | 变更 |
|----|------|------|
| `config/config.go` | `pkg/api/auth/config.go` | `envStr/Int/Bool` → `util.GetEnv*` |
| `model/types.go` | `graph/auth.graphqls` + `pkg/api/auth/types.go` | 输入 → schema，内部 → Go struct |
| `storage/store.go` | `pkg/model/auth_store.go` | 复用主 Pebble，前缀不变 |
| `service/otp.go` | `services/auth/otp.go` | sync.Map → Pebble |
| `service/email.go` | `services/auth/email.go` | 不变 |
| `service/oauth.go` | `services/auth/oauth.go` | 不变 |
| `service/auth.go` | `services/wallet/derive.go` `services/auth/jwt.go` `services/wallet/signer.go` `services/wallet/encrypt.go` | **拆分为 4 文件** |
| `handler/*` | **废弃** | 逻辑移入 `graph/auth.resolvers.go` |

### token → tee-provider

| 源 | 目标 | 变更 |
|----|------|------|
| `relay/adaptor/` (18个) | `services/gateway/adaptor/` | 不变 |
| `relay/adaptor.go` | `services/gateway/adaptor.go` | 不变 |
| `relay/controller/` | `services/gateway/relay/controller.go` | 不变 |
| `relay/relaymode/` | `services/gateway/relay/relaymode.go` | 不变 |
| `middleware/distributor.go` | `pkg/api/gateway/middleware.go` | gin → chi |
| `middleware/auth.go` | `pkg/api/gateway/auth_mw.go` | gin → chi |
| `controller/relay.go` | `pkg/api/gateway/relay_handler.go` | gin → chi |
| `model/{token,channel,product}.go` | **合约替代** | Phase 3 |
| `controller/{billing,token,user}.go` | **合约替代** | Phase 3 |

---

## 实施顺序

```
Phase 1 (2-3周):  web3auth → GraphQL
  ├── 定义 graph/auth.graphqls
  ├── 迁移 services (otp/email/oauth/derive/jwt/signer/encrypt)
  ├── 合并 storage → pkg/model/auth_store.go
  ├── 实现 graph/auth.resolvers.go
  ├── 更新 graph/resolver.go 注入依赖
  ├── gqlgen 重新生成
  └── 集成验证

Phase 2 (2-3周):  token 中继 → REST
  ├── 迁移 adaptor 目录 (不变)
  ├── gin → chi 中间件适配
  ├── Relay handler 实现
  └── 集成到 main.go chi router

Phase 3 (3-4周):  合约实现
  ├── contracts/auth/ (RegisterUser, GetUser, ...)
  ├── contracts/gateway/ (RegisterModel, Consume, ...)
  ├── 注册到 contract.go
  └── Relay 集成合约扣费

Phase 4 (3-4周):  DKG 阈值签名
  ├── P2P SignRequest/PartialSignature 消息
  ├── TSS 签名聚合
  └── signTransaction resolver 改为 DKG 模式

Phase 5 (1周):  统一部署
  ├── Dockerfile 更新
  ├── 环境变量统一
  └── K8s 配置
```
