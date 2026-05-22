# API Key 设计文档

> 最后更新：2026-05-21
> 重要变更：统一为单一 API Key 鉴权，JWT 和 Legacy Polkadot 已移除

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
    │                              │     userTotal[owner] += tokens│
    │                              │                              │
    ═══════════════════════ 每 5 分钟结算 ═════════════════════════
    │                              │                              │
    │                              │  8. 汇总所有用户用量           │
    │                              │  for each user:              │
    │                              │    cost = (tokens/1000)*price + prompt
    │                              │    records += {user, model, provider, tokens, cost}
    │                              │                              │
    │                              │  9. SettleUsage(records) ───>│
    │                              │           遍历每条:           │
    │                              │           · 校验 provider 归属 │
    │                              │           · 扣用户余额         │
    │                              │           · 加 provider 余额   │
    │                              │           · 记录 DailyUsage    │
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
    Owner     model.UniAddr `json:"owner"`     // 所属用户
    KeyHash   [32]byte      `json:"keyHash"`   // blake2b(apiKey)
    RateLimit uint32        `json:"rateLimit"` // 每分钟最大请求, 0=无限制
    IsActive  bool          `json:"isActive"`
    CreatedAt uint64        `json:"createdAt"`
}

// Settlement 单笔结算记录
type Settlement struct {
    User       model.UniAddr `json:"user"`       // 用户
    ModelID    uint64        `json:"modelId"`    // 使用的模型
    ProviderID uint64        `json:"providerId"` // 服务提供者
    TokensUsed uint64        `json:"tokensUsed"` // 消耗 token 数
    Cost       uint64        `json:"cost"`       // 费用; 0=合约自动计算
}

// DailyUsage 每日用量统计
type DailyUsage struct {
    User       model.UniAddr `json:"user"`
    ProviderID uint64        `json:"providerId"`
    ModelID    uint64        `json:"modelId"`
    Day        string        `json:"day"`
    Count      uint64        `json:"count"`       // 请求次数
    TokensUsed uint64        `json:"tokensUsed"`  // 总 token
    PointsUsed uint64        `json:"pointsUsed"`  // 总费用
}
```

### Mutation（写操作）

| 方法 | Selector | 参数 | 权限 | 说明 |
|---|---|---|---|---|---|
| `RegisterAPIKey` | `0x8edf9e30` | `keyHash [32]byte, rateLimit uint32` | 任意用户 | 注册 API Key 哈希 |
| `RevokeAPIKey` | `0x97288469` | `keyHash [32]byte` | key 拥有者 | 吊销 API Key |
| `SettleUsage` | `0x1b8ec9bd` | `records []Settlement` | Provider | 批量结算用户用量 |

### Query（读操作）

| 方法 | Selector | 参数 | 返回 | 说明 |
|---|---|---|---|---|
| `LookupAPIKey` | `0xb65d9801` | `keyHash [32]byte` | `*APIKeyInfo` | TEE 根据 hash 查用户 |
| `GetBalance` | `0x3744e4b0` | `user UniAddr` | `uint64` | 查询用户余额 |
| `GetProvider` | `0xb34c16d8` | `providerID uint64` | `*ProviderInfo` | 查询 Provider |
| `GetProviderByOwner` | `0x55404847` | 无（自动取 caller） | `uint64` | 查当前用户的 providerID |
| `GetProviderBalance` | `0xd3af56aa` | `providerID uint64` | `uint64` | Provider 可提现余额 |

### 费用计算

```
cost = (tokensUsed + 999) / 1000 * model.PointsPer1K + model.PromptPricing
if cost < 1 { cost = 1 }
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
        
        // 优先查合约
        token := LookupAPIKeyViaContract(key)
        if token == nil {
            // Fallback: 本地 PebbleDB（legacy token）
            token = FindTokenByKeyFallback(key)
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

func recordUsageAsync(tokenID uint64, day string, providerID uint64, tokens uint64) {
    initUsageWriter()
    select {
    case usageCh <- usageRecord{tokenID, day, providerID, 0, tokens}:
    default:
        GatewayLog().Printf("[USAGE] dropped record: token=%d provider=%d tokens=%d (channel full)",
            tokenID, providerID, tokens)
    }
}
```

## 六、安全待办

| 项目 | 状态 | 说明 |
|---|---|---|
| hash 不可逆 | ✅ | blake2b 单向，链上看 hash 无法推 apiKey |
| 抢跑 DoS | ✅ | 攻击者抢注 hash 无法使用 key，用户换 key 即可 |
| 结算重放 | 🔲 TODO | 需 batchNonce/Merkle root + TEE 签名 |
| 用量伪造 | 🔲 TODO | 需 TEE enclave 签名校验 |
| apiKey 熵 | ⚠️ | 用户必须用 `"tee_" + rand(32B)`，禁止弱 key |

## 七、API Key 格式规范

```
格式:  tee_<64 hex chars>
示例:  tee_a1b2c3d4e5f6a7b8c9d0e1f2a3b4c5d6e7f8a9b0c1d2e3f4a5b6c7d8e9f0
熵:    256 bit (32 bytes)
哈希:  blake2b.Sum256(apiKey) → [32]byte 上链
```

## 八、日志记录

所有 Gateway 相关日志（含鉴权）统一写入 `./logs/gateway.log`：

```
2026/05/21 15:00:00 [AUTH] token not found for key prefix: tee_a1b2c3...
2026/05/21 15:00:01 [GATEWAY] contract lookup success: owner=0x... active=true
2026/05/21 15:00:02 [HTTP] POST /v1/chat/completions | 200 | 1.234s | 192.168.1.1
```

---

*文档生成/更新：2026-05-21*
*基于代码版本：`tee-node` 项目当前仓库状态*
