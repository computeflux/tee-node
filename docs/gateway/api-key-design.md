# API Key 设计文档

> 最后更新：2026-05-26
> 重要变更：统一为单一 API Key 鉴权，JWT 和 Legacy Polkadot 已移除；Token 增加 `Addr` 和 `ExpiresAt` 字段

## 一、泳道图

```
用户 (Alice)                  TEE Provider                    Gateway 合约 (链上)
    │                              │                              │
    │  1. 离线生成 apiKey           │                              │
    │  raw ← rand(32B)             │                              │
    │  apiKey = "tee_" + hex(raw)  │                              │
    │  keyHash = blake2b(apiKey)   │                              │
    │                              │                              │
    │  2. RegisterAPIKey ──────────│─────────────────────────────>│
    │     (keyHash, rateLimit)     │           存储: hex(keyHash) → {owner, keyHash, rateLimit, isActive}
    │                              │                              │
    │  3. 保存 apiKey 到本地        │                              │
    │                              │                              │
    ═══════════════════════════ 日常调用 ═══════════════════════════
    │                              │                              │
    │  4. POST /v1/chat            │                              │
    │     Header: Authorization: Bearer tee_xxx...               │
    │     Body: {model, messages}  │                              │
    │                              │  5. hash = blake2b(apiKey)   │
    │                              │     LookupAPIKey(hash) ─────>│
    │                              │                   返回 {owner, isActive, rateLimit}
    │                              │<─────────────────────────────│
    │                              │                              │
    │                              │  6. 鉴权: isActive?          │
    │                              │    限流: rateLimit?          │
    │                              │    转发请求到 AI 模型         │
    │                              │                              │
    │<── 模型响应 ─────────────────│                              │
    │                              │                              │
    │                              │  7. 记录用量:                │
    │                              │     usage[tokenID_modelID] += {TokenIn, TokenOut, CachedToken}│
    │                              │                              │
    ═══════════════════════ 每 5 分钟结算 ═════════════════════════
    │                              │                              │
    │                              │  8. 汇总所有用量记录           │
    │                              │  for each record:            │
    │                              │    cost = (uncached*InPrice + cached*CachedPrice + tokenOut*OutPrice) / 1_000_000│
    │                              │    settlements += {user, model, provider, TokenIn, tokenOut, cached, count}│
    │                              │                              │
    │                              │  9. SettleUsage(settlements) ─>│
    │                              │           遍历每条:           │
    │                              │           · 校验 provider/model 存在 │
    │                              │           · 扣用户余额（credit_useds）│
    │                              │           · 加 provider 余额   │
    │                              │           · 双写 settlement 历史 │
    │                              │<─────────────────────────────│
    │                              │                              │
    ═══════════════════════ 吊销 ══════════════════════════════════
    │                              │                              │
    │ 10. RevokeAPIKey(keyHash) ───│─────────────────────────────>│
    │                              │           isActive = false    │
    │                              │                              │
    │                              │  下次 LookupAPIKey → 拒绝     │
    │                              │                              │
```

## 二、安全模型

```
┌─────────────────────────────────────────────────────┐
│                    安全边界                           │
│                                                     │
│  链上 (公开)              链下 (保密)                 │
│  ┌──────────────┐       ┌──────────────────┐       │
│  │ keyHash      │       │ apiKey 明文       │       │
│  │ (blake2b)    │ ←─── │ "tee_a1b2c3..."  │       │
│  │              │ 不可逆 │                  │       │
│  │ owner        │       │ 仅用户 + TEE 知晓 │       │
│  │ isActive     │       └──────────────────┘       │
│  │ rateLimit    │                                  │
│  └──────────────┘                                  │
│                                                     │
│  攻击者看到 keyHash → 无法推出 apiKey (blake2b)      │
│  攻击者抢注 keyHash → 纯 DoS, 无法使用 key           │
└─────────────────────────────────────────────────────┘
```

## 三、统一鉴权原则

### 3.1 唯一鉴权方式

**所有接口统一使用 `Authorization: Bearer <apiKey>`**：

| 接口类型 | 鉴权头 | 说明 |
|---------|--------|------|
| REST Gateway | `Authorization: Bearer tee_xxx...` | AI 模型请求转发 |
| GraphQL | `Authorization: Bearer tee_xxx...` | Web3Auth / 秘密管理 |

### 3.2 鉴权流程

```
Authorization: Bearer <apiKey>
    │
    v
 提取 apiKey（去掉 "Bearer " 前缀）
    │
    v
 blake2b.Sum256(apiKey) → keyHash
    │
    v
 GatewayLookupAPIKey(keyHash) [链上合约查询]
    │
    v
 验证 isActive == true
 验证 rateLimit 未超限
    │
    v
 鉴权通过，继续处理请求
```

### 3.3 已移除的鉴权方式

| 鉴权方式 | 状态 | 说明 |
|---------|:---:|:---|
| JWT (GraphQL) | ❌ 已移除 | 原 `X-Token` header，现统一为 Bearer |
| Legacy Polkadot | ❌ 已移除 | 原 `X-Signature` header，现统一为 Bearer |
| API Key (REST) | ✅ 保留并统一 | 现为唯一鉴权方式 |

## 四、合约接口

### 类型定义

```go
// APIKeyInfo 用户 API Key（仅存哈希，原始 key 绝不上链）
type APIKeyInfo struct {
    OwnerKey  uint64        `json:"ownerKey" idx:"exact"` // blake2b(owner.V)[:8] 转 uint64，用于索引
    Owner     model.UniAddr `json:"owner"`
    KeyHash   []byte        `json:"keyHash"`   // blake2b(apiKey), 32 bytes
    RateLimit uint32        `json:"rateLimit"` // 每分钟最大请求, 0=无限制
    IsActive  bool          `json:"isActive"`
    CreatedAt uint64        `json:"createdAt"`
}

// Settlement 单笔结算记录（用于批量提交）
type Settlement struct {
    User         model.UniAddr `json:"user"`         // 用户
    ModelID      uint64        `json:"modelId"`      // 使用的模型
    ProviderID   uint64        `json:"providerId"`   // 服务提供者
    TokenIn     uint64        `json:"TokenIn"`     // prompt tokens
    TokenOut     uint64        `json:"tokenOut"`     // completion tokens
    CachedToken  uint64        `json:"cachedToken"`  // cached prompt tokens
    Count        uint16        `json:"count"`        // 请求次数
}

// ProviderSettlementRecord Provider 视角结算记录
type ProviderSettlementRecord struct {
    ProviderID  uint64        `json:"providerId"`
    User        model.UniAddr `json:"user"`
    ModelID     uint64        `json:"modelId"`
    TokenIn    uint64        `json:"TokenIn"`
    TokenOut    uint64        `json:"tokenOut"`
    CachedToken uint64        `json:"cachedToken"`
    Cost        uint64        `json:"cost"`
    Count       uint16        `json:"count"`
    Time        int64         `json:"time"`
}

// UserSettlementRecord User 视角结算记录
type UserSettlementRecord struct {
    ProviderID  uint64        `json:"providerId"`
    ModelID     uint64        `json:"modelId"`
    TokenIn    uint64        `json:"TokenIn"`
    TokenOut    uint64        `json:"tokenOut"`
    CachedToken uint64        `json:"cachedToken"`
    Cost        uint64        `json:"cost"`
    Count       uint16        `json:"count"`
    Time        int64         `json:"time"`
}
```

> **注意**：`DailyUsage` 类型已废弃，合约中不再写入每日用量。用量通过 `SettleUsage` 结算后直接转为 `SettlementRecord` 双写存储。

### Mutation（写操作）

| 方法 | Selector | 参数 | 权限 | 说明 |
|---|---|---|---|---|---|
| `RegisterAPIKey` | `0xf8ea1a7b` | `keyHash [32]byte, rateLimit uint32` | 任意用户 | 注册 API Key 哈希 |
| `RevokeAPIKey` | `0x7ed802da` | `keyHash [32]byte` | key 拥有者 | 吊销 API Key |
| `SettleUsage` | `0x2808831a` | `records []Settlement` | System / Provider Owner | 批量结算用户用量 |

### Query（读操作）

| 方法 | Selector | 参数 | 返回 | 说明 |
|---|---|---|---|---|
| `LookupAPIKey` | `0xc533a84f` | `keyHash [32]byte` | `*APIKeyInfo` | TEE 根据 hash 查用户 |
| `GetCredit` | `0x04601fca` | `user UniAddr` | `int64` | 查询用户余额（总充值 - 已使用） |
| `GetProvider` | `0xb34c16d8` | `providerID uint64` | `*ProviderInfo` | 查询 Provider |
| `GetProviderByOwner` | `0x55c04847` | 无（自动取 caller） | `*Option[uint64]` | 查当前用户的 providerID |
| `GetProviderCredit` | `0xad4c38cd` | `providerID uint64` | `uint64` | Provider 可提现余额 |
| `GetSettlementHistory` | `0x74d599fb` | `providerID, offset, limit` | `[]ProviderSettlementRecord` | Provider 结算历史 |
| `GetUserSettlementHistory` | `0x56b906c0` | `user, offset, limit` | `[]UserSettlementRecord` | 用户消费历史 |

### 费用计算

```
uncached = TokenIn - CachedToken
cost = (uncached * InPrice + CachedToken * CachedPrice + TokenOut * OutPrice) / 1_000_000
if cost == 0 { continue }  // 跳过零费用记录
```

## 五、TEE 侧实现

### 5.1 合约查询桥接

```go
// pkg/api/gateway/contract.go

// hashAPIKey 对明文 API Key 做 blake2b-256 哈希
func hashAPIKey(key string) []byte {
    h := blake2b.Sum256([]byte(key))
    return h[:]
}

// LookupAPIKeyViaContract 通过合约查询 API Key 信息
func LookupAPIKeyViaContract(key string) *model.GatewayTokenInfo {
    bridge := getBridge()
    hash := hashAPIKey(key)
    
    info, err := bridge.GatewayLookupAPIKey(hash)
    if err != nil || info == nil {
        return nil
    }
    
    return &model.GatewayTokenInfo{
        Owner:     fmt.Sprintf("%x", info.Owner.V),
        Key:       key,
        RateLimit: info.RateLimit,
        IsActive:  info.IsActive,
    }
}
```

### 5.2 鉴权中间件

```go
// pkg/api/gateway/auth_middleware.go

func TokenAuthMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        key := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
        if key == "" {
            writeError(w, 401, "missing api key", "auth_error")
            return
        }
        
        // 优先查本地 key 索引（O(1)）
        token := FindGatewayTokenByKey(key)
        if token == nil {
            // Fallback: 链上合约查询
            token = LookupAPIKeyViaContract(key)
        }
        
        if token == nil || !token.IsActive {
            writeError(w, 401, "invalid api key", "auth_error")
            return
        }
        
        ctx := context.WithValue(r.Context(), ctxToken, token)
        next.ServeHTTP(w, r.WithContext(ctx))
    })
}
```

### 5.3 异步用量记录

```go
// pkg/api/gateway/optimizer.go

func recordUsageAsync(user model.UniAddr, tokenID uint64, model *cgateway.ModelInfoWithID, in uint64, out uint64, cached uint64) {
    initUsageWriter()
    usageCh <- usageRecord{user, tokenID, model, in, out, cached}
}
```

## 六、Token 本地索引

```go
// pkg/model/gateway_store.go

// SetGatewayTokenByKey 以 API Key 为索引写入 token 信息，供 O(1) 直接查询。
func SetGatewayTokenByKey(t *GatewayTokenInfo) error {
    return SetJson("GATEWAY", fmt.Sprintf("token_by_key:%s", t.Key), t)
}

// GetGatewayTokenByKey 通过 API Key 直接查询 token（O(1)，不遍历）。
func GetGatewayTokenByKey(key string) (*GatewayTokenInfo, error) {
    return GetJson[GatewayTokenInfo]("GATEWAY", fmt.Sprintf("token_by_key:%s", key))
}

// FindGatewayTokenByKey 通过 key 索引直接查询（O(1)），同时检查是否过期。
func FindGatewayTokenByKey(key string) *GatewayTokenInfo {
    tk, err := GetGatewayTokenByKey(key)
    if err != nil || tk == nil {
        return nil
    }
    if !tk.IsActive || tk.IsExpired() {
        return nil
    }
    return tk
}
```

## 七、安全待办

| 项目 | 状态 | 说明 |
|---|---|---|
| hash 不可逆 | ✅ | blake2b 单向，链上看 hash 无法推 apiKey |
| 抢跑 DoS | ✅ | 攻击者抢注 hash 无法使用 key，用户换 key 即可 |
| Token 过期 | ✅ | `ExpiresAt` 字段，默认 1 天过期 |
| 结算重放 | 🔲 TODO | 需 batchNonce/Merkle root + TEE 签名 |
| 用量伪造 | 🔲 TODO | 需 TEE enclave 签名校验 |
| apiKey 熵 | ⚠️ | 用户必须用 `"tee_" + rand(32B)`，禁止弱 key |

## 八、API Key 格式规范

```
格式:  tee_<64 hex chars>
示例:  tee_a1b2c3d4e5f6a7b8c9d0e1f2a3b4c5d6e7f8a9b0c1d2e3f4a5b6c7d8e9f0
熵:    256 bit (32 bytes)
哈希:  blake2b.Sum256(apiKey) → [32]byte 上链
过期:  默认 1 天（可配置）
```

## 九、日志记录

所有 Gateway 相关日志（含鉴权）统一写入 `./logs/gateway.log`：

```
2026/05/21 15:00:00 [AUTH] token not found for key prefix: tee_a1b2c3...
2026/05/21 15:00:01 [GATEWAY] contract lookup success: owner=0x... active=true
2026/05/21 15:00:02 [HTTP] POST /v1/chat/completions | 200 | 1.234s | 192.168.1.1
2026/05/21 15:05:00 [SETTLE] batch submitted 42 records across 3 providers
```

---

*文档更新：2026-06-04*
*基于代码版本：`tee-node` 项目当前仓库状态*
