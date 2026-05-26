# TEE 全局 KMS 设计方案：API Key 安全存储与多节点共享

> 最后更新：2026-05-26
> 状态：已落地 — TEE KMS 加密 API Key，链上只存引用标识

## 1. 背景与问题

### 1.1 当前方案的问题

`tee-chain/contracts/gateway/types.go` 中的 `ModelInfo` 结构体不再直接存储明文 API Key：

```go
type ModelInfo struct {
    // ...
    APIKeyRefs []string `json:"apiKeyRefs,omitempty"` // 引用标识列表，指向 EncryptedAPIKey
}
```

**已解决：**
- ✅ 合约状态通过 ABI 查询时，API Key 不再暴露（链上仅存引用标识）
- ✅ TEE 侧链的 pebble 存储中，`EncryptedAPIKey` 被 `sealing key` 保护
- ✅ 满足"最小权限原则"：链上不应存储任何可直接使用的凭证

### 1.2 多节点场景的额外约束

- 用户请求由负载均衡器路由到**任意 TEE 节点**
- 每个节点都必须能独立解密并使用 API Key
- 运行时不能依赖 DKG/阈值加密（首次请求或轮换后高延迟）
- 新节点加入集群时必须能安全获取解密密钥

### 1.3 排除方案

| 方案 | 排除原因 |
|---|---|
| 明文存链上 | 安全风险不可接受 |
| DKG 阈值加密 | 每次解密需 t-of-n 协同，引入 100ms+ 网络延迟，不适用于高频 API 调用 |
| 单节点本地 KMS | 无法满足"任意节点服务"的需求 |

---

## 2. 方案概述

**核心思想：链上加密存储 + TEE 集群共享对称密钥**

```
┌─────────────────────────────────────────────────────────────┐
│                        链上存储                               │
│  ModelInfo.APIKeyRefs = ["openai-prod-v2"]                  │
│  EncryptedAPIKey {                                         │
│      Ref:        "openai-prod-v2"                          │
│      Ciphertext: AES-GCM(GK, "sk-xxxx...")                 │
│      Nonce:      [12]byte                                  │
│  }                                                         │
└─────────────────────────────────────────────────────────────┘
                              ▲
                              │ 区块同步
        ┌─────────────────────┼─────────────────────┐
        │                     │                     │
   ┌────▼────┐           ┌────▼────┐           ┌────▼────┐
   │ Node 0  │           │ Node 1  │           │ Node N  │
   │ ┌─────┐ │◄─────────►│ ┌─────┐ │◄─────────►│ ┌─────┐ │
   │ │ GK  │ │           │ │ GK  │ │           │ │ GK  │ │
   │ │(sealed)│           │ │(sealed)│           │ │(sealed)│
   │ └─────┘ │  RA+TLS   │ └─────┘ │  RA+TLS   │ └─────┘ │
   └─────────┘           └─────────┘           └─────────┘
```

---

## 3. 详细设计

### 3.1 密钥分层

**Layer 1: 全局 KMS 密钥 GK（AES-256-GCM）**

| 属性 | 说明 |
|---|---|
| 生成 | 集群初始化时，由 bootstrap 节点在 enclave 内生成随机密钥 |
| 分发 | 通过 TEE 远程证明（Remote Attestation, RA）+ enclave-to-enclave TLS 通道 |
| 存储 | 每个节点用本地 `sealing key` 加密 GK 后存入 pebble（keyspace `TEEKMS`） |
| 用途 | 加密/解密所有 Provider 的 API Key |
| 轮换 | 支持周期性轮换，旧 GK 废弃后所有密文重新加密 |

**Layer 2: Provider API Key（被 GK 加密后上链）**

| 属性 | 说明 |
|---|---|
| 注册 | Provider 将明文 key 提交给任意 TEE 节点，节点用 GK 加密后写入合约 |
| 使用 | 任意节点读取链上密文，用本地 GK 解密 |
| 轮换 | Provider 提交新 key，节点用 GK 加密后更新链上记录 |

**为什么用对称加密而非非对称？**
- API Key 只需要"加密存储 + 解密使用"，不需要签名/验证
- AES-GCM 比 RSA/AES 混合方案快 100x+
- 所有节点持有相同 GK，对称加密天然适合

### 3.2 链上数据结构

**`tee-chain/contracts/gateway/types.go`：**

```go
// EncryptedAPIKey 链上存储的加密 API Key（明文永不上链）
type EncryptedAPIKey struct {
    Ref        string `json:"ref"`        // 引用标识，如 "openai-prod-v2"
    Ciphertext []byte `json:"ciphertext"` // AES-GCM(GK, apiKey)
    Nonce      []byte `json:"nonce"`      // GCM nonce
}

// ModelInfo 链上只存引用标识
type ModelInfo struct {
    Name            string        `json:"name" idx:"exact"`
    Owner           model.UniAddr `json:"owner"`
    ProviderID      uint64        `json:"providerId" idx:"exact"`
    InputModalities uint8         `json:"inputModalities" idx:"exact"`
    ContextLength   uint64        `json:"contextLength" idx:"range"`
    InPrice         uint64        `json:"inPrice" idx:"range"`
    CachedPrice     uint64        `json:"cachedPrice"`
    OutPrice        uint64        `json:"outPrice"`
    ProviderType    uint16        `json:"providerType"`
    ModelID         string        `json:"modelId"`
    URI             string        `json:"uri,omitempty"`
    APIKeyRefs      []string      `json:"apiKeyRefs,omitempty"` // 引用标识列表
    IsActive        bool          `json:"isActive" idx:"exact"`
    CreatedAt       uint64        `json:"createdAt"`
}
```

> **注意**：旧版文档中的 `PointsPer1K` 和 `PromptPricing` 字段已废弃，当前使用 `InPrice`/`CachedPrice`/`OutPrice` 三维定价模型。

**合约新增方法：**

```go
// RegisterEncryptedAPIKey Provider 注册加密后的 API Key
func (g GatewayMutation) RegisterEncryptedAPIKey(ref string, ciphertext, nonce []byte) error

// GetEncryptedAPIKey 查询加密 API Key
func (g GatewayQuery) GetEncryptedAPIKey(ref string) (*EncryptedAPIKey, error)
```

### 3.3 GK 初始化与分发（绑定到 DKG 组网流程）

```
集群初始化流程：

  Bootstrap 节点（Node 0）
      │
      ▼
  [Enclave 内] 生成随机 32-byte GK
      │
      ▼
  用本地 sealing key 加密 GK → 存 pebble keyspace "TEEKMS"
      │
      ▼
  启动 DKG，成为首个 validator
      │
      ├────────────────────────► Node 1（新节点）
      │                              │
      │                              ▼
      │                         启动，发现本地无 GK
      │                              │
      │                              ▼
      │                         加入 DKG 组网（initConsensus / reConsensus）
      │                              │
      │                              ▼
      │                         DKG 成功后，通过已有 P2P 通道
      │                         广播 kms_sync_req 给所有老节点
      │◄─────────────────────────────┘
      │
      ▼
  老节点验证请求者在 Nodes/NewNodes 列表中
      │
      ▼
  通过 P2P 发送 kms_sync_resp（base64(GK)）
      ├────────────────────────► Node 1
                                  │
                                  ▼
                             [Enclave 内] base64 解码 GK
                                  │
                                  ▼
                             用本地 sealing key 加密 GK → 存 pebble
```

**为什么绑定到 DKG？**
- DKG 组网本身就是低频操作（仅在集群初始化或扩缩容时发生）
- DKG 完成后所有节点已经建立了可信的 P2P 连接
- 无需额外的 RA+TLS 握手，复用已有的 TEE P2P 安全通道
- 请求者身份天然由 DKG 验证（必须在 Nodes / NewNodes 列表中）

**GK 存储格式：**

```go
// pkg/model/tee_kms.go
const KMSKeyspace = "TEEKMS"

// SealedGK 被本地 sealing key 保护的 GK
type SealedGK struct {
    Ciphertext []byte `json:"ciphertext"` // sealing_key(GK)
    SealedAt   int64  `json:"sealedAt"`   // 封印时间戳
    Version    uint32 `json:"version"`    // GK 版本号，用于轮换
}
```

### 3.4 运行时流程

```
用户请求到达（任意节点，如 Node 2）：

  1. 鉴权中间件验证 API Key
         │
         ▼
  2. 路由到对应 Model
         │
         ▼
  3. 读取 ModelInfo.APIKeyRefs
     如：["openai-prod-v2"]
         │
         ▼
  4. 读取链上 EncryptedAPIKey（通过本地 pebble 缓存）
     Ciphertext + Nonce
         │
         ▼
  5. [Enclave 内] 从本地加载 SealedGK
     → unseal 得到 GK（启动时一次，之后内存缓存）
         │
         ▼
  6. [Enclave 内] AES-GCM 解密
     plaintext = AES-GCM.Decrypt(GK, Ciphertext, Nonce)
     耗时：~1μs（纯 CPU 操作）
         │
         ▼
  7. 用明文 API Key 构建 HTTP 请求
     Authorization: Bearer sk-xxxx...
         │
         ▼
  8. 转发到上游 Provider
```

**关键：步骤 5-6 全部在 enclave 内完成，明文永不出 TEE。**

### 3.5 API Key 轮换流程

```
Provider 发现 key 泄露或定期轮换：

  Provider 生成新 key：sk-newxxx
      │
      ▼
  调用合约：RegisterEncryptedAPIKey("openai-prod-v3", ciphertext, nonce)
      │
      ▼
  [合约内] 任意节点收到调用
      │
      ▼
  [Enclave 内] GK 加密新 key
      Ciphertext = AES-GCM.Encrypt(GK, "sk-newxxx")
      │
      ▼
  写入链上 EncryptedAPIKey{"openai-prod-v3", Ciphertext, Nonce}
      │
      ▼
  更新 ModelInfo.APIKeyRefs
      从 ["openai-prod-v2"] 改为 ["openai-prod-v3"]
      │
      ▼
  旧 ref "openai-prod-v2" 可标记为废弃（合约逻辑）
```

### 3.6 GK 轮换流程

```
管理员触发 GK 轮换（如每 90 天）：

  1. 生成新 GK'
      │
      ▼
  2. 遍历所有链上 EncryptedAPIKey
      对每个：Ciphertext_old → AES-GCM.Decrypt(GK, ...) → plaintext
             plaintext → AES-GCM.Encrypt(GK', ...) → Ciphertext_new
      │
      ▼
  3. 批量更新链上密文（通过合约原子操作或批量提交）
      │
      ▼
  4. 通过 RA + TLS 通道分发 GK' 给所有节点
      │
      ▼
  5. 所有节点用新 GK' 替换旧 GK（调用 ClearGlobalKeyCache）
      │
      ▼
  6. 旧 GK 废弃，旧密文不再有效
```

---

## 4. 代码实施方案

### 4.1 新增 `pkg/model/tee_kms.go`

```go
package model

import (
    "crypto/aes"
    "crypto/cipher"
    "crypto/rand"
    "crypto/sha256"
    "encoding/json"
    "fmt"
    "io"
    "time"

    "github.com/computeflux/tee-provider/pkg/util"
)

const (
    KMSKeyspace  = "TEEKMS"
    GKRefKey     = "global_key"
    GKVersionKey = "global_version"
    AESKeySize   = 32
)

// SealedGK 被本地 sealing key 保护的 GK
type SealedGK struct {
    Ciphertext []byte `json:"ciphertext"`
    SealedAt   int64  `json:"sealedAt"`
    Version    uint32 `json:"version"`
}

// GlobalKey 全局 KMS 密钥（仅在 enclave 内存中使用）
type GlobalKey struct {
    Key     []byte `json:"key"`
    Version uint32 `json:"version"`
}

// KMSKey 用 GK 加密后的 API Key
type KMSKey struct {
    Ref        string `json:"ref"`
    Ciphertext []byte `json:"ciphertext"`
    Nonce      []byte `json:"nonce"`
}

// GenerateGlobalKey 生成新的全局密钥
func GenerateGlobalKey() (*GlobalKey, error) {
    key := make([]byte, AESKeySize)
    if _, err := io.ReadFull(rand.Reader, key); err != nil {
        return nil, fmt.Errorf("generate GK: %w", err)
    }
    return &GlobalKey{Key: key, Version: 1}, nil
}

// SealGlobalKey 用本地 sealing key 加密 GK 后存入 pebble
func SealGlobalKey(gk *GlobalKey) error {
    plaintext, err := json.Marshal(gk)
    if err != nil {
        return err
    }
    sealed, err := util.SealWithProductKey(plaintext, []byte("TEEKMS_GK"))
    if err != nil {
        return fmt.Errorf("seal GK: %w", err)
    }
    sg := SealedGK{
        Ciphertext: sealed,
        SealedAt:   time.Now().Unix(),
        Version:    gk.Version,
    }
    b, _ := json.Marshal(sg)
    if err := SetKey(KMSKeyspace, GKRefKey, b); err != nil {
        return err
    }
    verB, _ := json.Marshal(gk.Version)
    return SetKey(KMSKeyspace, GKVersionKey, verB)
}

// UnsealGlobalKey 从 pebble 加载并解封 GK
func UnsealGlobalKey() (*GlobalKey, error) {
    b, err := GetKey(KMSKeyspace, GKRefKey)
    if err != nil {
        return nil, fmt.Errorf("GK not initialized")
    }
    var sg SealedGK
    if err := json.Unmarshal(b, &sg); err != nil {
        return nil, err
    }
    plaintext, err := util.Unseal(sg.Ciphertext, []byte("TEEKMS_GK"))
    if err != nil {
        return nil, fmt.Errorf("unseal GK: %w", err)
    }
    var gk GlobalKey
    if err := json.Unmarshal(plaintext, &gk); err != nil {
        return nil, err
    }
    return &gk, nil
}

// EncryptWithGK 用 GK 加密明文（AAD 绑定 ref + version）
func EncryptWithGK(gk *GlobalKey, ref string, plaintext string) (*KMSKey, error) {
    block, err := aes.NewCipher(gk.Key)
    if err != nil {
        return nil, err
    }
    gcm, err := cipher.NewGCM(block)
    if err != nil {
        return nil, err
    }
    nonce := make([]byte, gcm.NonceSize())
    if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
        return nil, err
    }
    aad := fmt.Sprintf("%s:%d", ref, gk.Version)
    ciphertext := gcm.Seal(nil, nonce, []byte(plaintext), []byte(aad))
    return &KMSKey{Ref: ref, Ciphertext: ciphertext, Nonce: nonce}, nil
}

// DecryptWithGK 用 GK 解密
func DecryptWithGK(gk *GlobalKey, ek *KMSKey) (string, error) {
    block, err := aes.NewCipher(gk.Key)
    if err != nil {
        return "", err
    }
    gcm, err := cipher.NewGCM(block)
    if err != nil {
        return "", err
    }
    aad := fmt.Sprintf("%s:%d", ek.Ref, gk.Version)
    plaintext, err := gcm.Open(nil, ek.Nonce, ek.Ciphertext, []byte(aad))
    if err != nil {
        return "", fmt.Errorf("decrypt failed: %w", err)
    }
    return string(plaintext), nil
}

// ── 单例缓存（运行时内存） ───────────────────────────────

var gkCache *GlobalKey
var gkCacheErr error
var gkCacheInit bool

// GetGlobalKey 获取全局密钥（带内存缓存）
func GetGlobalKey() (*GlobalKey, error) {
    if gkCacheInit {
        return gkCache, gkCacheErr
    }
    gkCache, gkCacheErr = UnsealGlobalKey()
    gkCacheInit = true
    return gkCache, gkCacheErr
}

// ClearGlobalKeyCache 清除缓存（GK 轮换后调用）
func ClearGlobalKeyCache() {
    gkCache = nil
    gkCacheInit = false
}

// HashRef 将 ref 转为确定性 32-byte hash
func HashRef(ref string) []byte {
    h := sha256.Sum256([]byte(ref))
    return h[:]
}

// ── 启动初始化 ───────────────────────────────────────────

// InitGlobalKeyAtStartup 节点启动时调用：
// - 若本地已存在 SealedGK，自动解封并加载到内存缓存
// - 若不存在，返回 error 提示需要从其他节点同步（或首次启动时生成）
func InitGlobalKeyAtStartup() (*GlobalKey, error) {
    gk, err := GetGlobalKey()
    if err == nil {
        return gk, nil
    }
    return nil, fmt.Errorf("global key not found locally: %w (new node must sync from peers or generate)", err)
}

// GenerateAndSealGlobalKey 首次启动时生成 GK 并密封存储。
// 仅应在全网首次部署时由一个节点调用，其他节点应通过 RA+TLS 同步。
func GenerateAndSealGlobalKey() (*GlobalKey, error) {
    gk, err := GenerateGlobalKey()
    if err != nil {
        return nil, err
    }
    if err := SealGlobalKey(gk); err != nil {
        return nil, err
    }
    gkCache = gk
    gkCacheInit = true
    return gk, nil
}

// ImportGlobalKey 从其他 TEE 节点同步后导入 GK。
func ImportGlobalKey(plaintext []byte, version uint32) error {
    gk := &GlobalKey{Key: plaintext, Version: version}
    if err := SealGlobalKey(gk); err != nil {
        return err
    }
    gkCache = gk
    gkCacheInit = true
    return nil
}

// HasGlobalKey 检查本地是否已存储 GK。
func HasGlobalKey() bool {
    _, err := GetKey(KMSKeyspace, GKRefKey)
    return err == nil
}
```

### 4.2 合约修改 `tee-chain/contracts/gateway/types.go`

```go
// EncryptedAPIKey 链上存储的加密 API Key（明文永不上链）
type EncryptedAPIKey struct {
    Ref        string `json:"ref"`        // 引用标识
    Ciphertext []byte `json:"ciphertext"` // AES-GCM(GK, apiKey)
    Nonce      []byte `json:"nonce"`      // GCM nonce
}

// ModelInfo APIKeys → APIKeyRefs（链上只存引用标识）
type ModelInfo struct {
    Name            string        `json:"name" idx:"exact"`
    Owner           model.UniAddr `json:"owner"`
    ProviderID      uint64        `json:"providerId" idx:"exact"`
    InputModalities uint8         `json:"inputModalities" idx:"exact"`
    ContextLength   uint64        `json:"contextLength" idx:"range"`
    InPrice         uint64        `json:"inPrice" idx:"range"`
    CachedPrice     uint64        `json:"cachedPrice"`
    OutPrice        uint64        `json:"outPrice"`
    ProviderType    uint16        `json:"providerType"`
    ModelID         string        `json:"modelId"`
    URI             string        `json:"uri,omitempty"`
    APIKeyRefs      []string      `json:"apiKeyRefs,omitempty"` // 引用标识列表
    IsActive        bool          `json:"isActive" idx:"exact"`
    CreatedAt       uint64        `json:"createdAt"`
}
```

### 4.3 合约修改 `tee-chain/contracts/gateway/gateway.go`

```go
type Gateway struct {
    api model.ContractApi

    credits               *model.StoreMapping[model.UniAddr, uint64]                      `store:"keyPfx:v2_credits"`
    credit_useds          *model.StoreMapping[model.UniAddr, uint64]                      `store:"keyPfx:v2_credit_useds"`
    models                *model.IndexedList[uint64, ModelInfo]                           `store:"keyPfx:v2_models"`
    providers             *model.IndexedList[uint64, ProviderInfo]                        `store:"keyPfx:v2_providers"`
    providerByOwner       *model.StoreMapping[model.UniAddr, uint64]                      `store:"keyPfx:v2_provider_by_owner"`
    providerCredits       *model.StoreMapping[uint64, uint64]                             `store:"keyPfx:v2_provider_credit"`
    apiKeys               *model.IndexedStore[string, APIKeyInfo]                         `store:"keyPfx:v2_api_key"`
    encryptedKeys         *model.StoreMapping[string, EncryptedAPIKey]                    `store:"keyPfx:v2_encrypted_key"`
    rechargeHistory       *model.StoreList2D[model.UniAddr, uint32, RechargeRecord]       `store:"keyPfx:v2_recharge_history"`
    settlementHistory     *model.StoreList2D[uint64, uint32, ProviderSettlementRecord]    `store:"keyPfx:v4_settlement_history"`
    userSettlementHistory *model.StoreList2D[model.UniAddr, uint32, UserSettlementRecord] `store:"keyPfx:v4_user_settlement_history"`
}
```

合约方法（`tee-chain/contracts/gateway/model.go`）：

```go
// RegisterModel 注册模型（apiKeyRefs 替代 apiKeys）
func (g GatewayMutation) RegisterModel(
    name string,
    inputModalities uint8, contextLength uint64,
    inPrice, cachedPrice, outPrice uint64,
    providerType uint16, uri string, apiKeys []string,
) error

// UpdateModel 更新模型
func (g GatewayMutation) UpdateModel(
    ID uint64,
    inputModalities uint8, contextLength uint64,
    inPrice, cachedPrice, outPrice uint64,
    name string, providerType uint16, uri string, apiKeys []string,
) error

// ListModels / ListActiveModels 分页查询（匹配 IndexedList 签名）
func (g GatewayQuery) ListModels(offset uint64, limit uint32) ([]ModelInfo, error)
func (g GatewayQuery) ListActiveModels(offset uint64, limit uint32) ([]ModelInfo, error)

// GetModelCount 模型总数
func (g GatewayQuery) GetModelCount() (uint64, error)

// QueryModels 按条件过滤查询（modalities / contextLength / price）
func (g GatewayQuery) QueryModels(
    modalities uint8, minContext, maxContext, maxPrice uint64,
    offset uint64, limit uint32,
) ([]ModelInfo, error)

// RegisterEncryptedAPIKey Provider 注册加密后的 API Key
func (g GatewayMutation) RegisterEncryptedAPIKey(ref string, ciphertext, nonce []byte) error

// GetEncryptedAPIKey 查询加密 API Key
func (g GatewayQuery) GetEncryptedAPIKey(ref string) (*EncryptedAPIKey, error)
```

### 4.4 Gateway Relay 修改 `pkg/api/gateway/contract.go`

```go
// buildEndpointsFromModel 根据合约 ModelInfo 构建 GatewayEndpoint 列表。
// 使用 TEE KMS 解密链上存储的加密 API Key（明文永不上链）。
func buildEndpointsFromModel(m *cgateway.ModelInfo) []*model.GatewayEndpoint {
    url := m.URI
    if url == "" {
        url = providerDefaultURL(m.ProviderType)
    }
    if len(m.APIKeyRefs) == 0 {
        return nil
    }

    // 获取全局密钥（内存缓存，~0 开销）
    gk, err := model.GetGlobalKey()
    if err != nil {
        GATEWAYLOG().Printf("[GATEWAY] GetGlobalKey failed: %v", err)
        return nil
    }

    bridge := getBridge()

    eps := make([]*model.GatewayEndpoint, 0, len(m.APIKeyRefs))
    for i, ref := range m.APIKeyRefs {
        var apiKey string
        if bridge != nil {
            encKey, err := bridge.GatewayGetEncryptedAPIKey(ref)
            if err != nil || encKey == nil {
                GATEWAYLOG().Printf("[GATEWAY] get encrypted key ref=%s err=%v", ref, err)
                continue
            }
            kmsKey := &model.KMSKey{
                Ref:        encKey.Ref,
                Ciphertext: encKey.Ciphertext,
                Nonce:      encKey.Nonce,
            }
            apiKey, err = model.DecryptWithGK(gk, kmsKey)
            if err != nil {
                GATEWAYLOG().Printf("[GATEWAY] decrypt ref=%s err=%v", ref, err)
                continue
            }
        }
        if apiKey == "" {
            continue
        }
        eps = append(eps, &model.GatewayEndpoint{
            ID:         uint64(i) + 1,
            ProviderID: m.ProviderID,
            URL:        url,
            APIKey:     apiKey,
            Weight:     1,
            IsActive:   true,
        })
    }
    return eps
}
```

### 4.5 `main.go` 启动时 GK 初始化

```go
func main() {
    // Init app db
    db, err := model.NewDB()
    // ...

    // Init TEE KMS Global Key
    gk, err := model.InitGlobalKeyAtStartup()
    if err != nil {
        if os.Getenv("TEE_GENESIS_NODE") == "true" {
            // 全网首次部署：创世节点自动生成并密封 GK
            gk, err = model.GenerateAndSealGlobalKey()
            if err != nil {
                log.Fatalf("failed to generate global key: %v", err)
            }
            // 导出 GK 供备份（新节点现在通过 DKG 自动同步，无需手动设置）
            exported, version, err := model.ExportGlobalKeyBase64()
            if err == nil {
                fmt.Printf("TEE_KMS_BACKUP_KEY=%s TEE_KMS_BACKUP_VERSION=%d\n", exported, version)
            }
        } else {
            // 非 genesis 节点：允许暂时无 GK，DKG 组网成功后会自动从 peers 同步
            util.LogWithYellow("TEE KMS", "Global key not found locally, will sync from peers after DKG join")
        }
    } else {
        util.LogWithYellow("TEE KMS", "Global key loaded from local seal")
    }
    // ...
}
```

### 4.6 合约桥接 `tee-chain/contract.go`

```go
// GatewayGetEncryptedAPIKey 查询加密 API Key。
func (app *TeeChain) GatewayGetEncryptedAPIKey(ref string) (*gateway.EncryptedAPIKey, error) {
    runtime, err := app.gatewayRuntime()
    if err != nil {
        return nil, err
    }
    defer runtime.GetTxn().Rollback()
    return gateway.GatewayQuery{Gateway: *gateway.NewGateway(runtime)}.GetEncryptedAPIKey(ref)
}
```

### 4.7 DKG GK 同步 `pkg/dkg/kms_sync.go`

新增 DKG 消息类型 `kms_sync_req` / `kms_sync_resp`，复用已有的 `DkgMessage` P2P 通道。

```go
// SyncGlobalKeyFromPeers 在 DKG 初始化/重新共识成功后调用。
// 本节点如果没有 GK，则向已知 peers 广播请求，等待任意一个有 GK 的节点响应。
func (dkg *DKG) SyncGlobalKeyFromPeers()

// handleKmsSyncReq 已有 GK 的节点收到新节点的 GK 请求
// - 验证请求者在当前 Nodes / NewNodes 列表中
// - 通过 P2P 直接回复 base64(GK)
func (dkg *DKG) handleKmsSyncReq(msg *model.DkgMessage) error

// handleKmsSyncResp 新节点收到 GK 响应
// - 验证响应者在 Nodes 列表中
// - base64 解码并调用 model.ImportGlobalKey()
func (dkg *DKG) handleKmsSyncResp(msg *model.DkgMessage) error
```

触发点：
```go
// pkg/dkg/consensus.go — finishDkgConsensusStep 成功后
func (dkg *DKG) finishDkgConsensusStep(isok bool, tag string) {
    // ... DKG 状态迁移 ...

    // DKG 组网成功后，触发 GK 同步（新节点从此处获取 Global Key）
    dkg.SyncGlobalKeyFromPeers()

    dkg.SendNewEpochPartialSigToSponsor()
}
```

---

## 5. 节点重启/首次启动场景

| 场景 | 表现 | 操作 |
|------|------|------|
| **正常重启** | 自动 `UnsealGlobalKey()` 加载到内存缓存 | 无感知，直接启动 |
| **首次部署（创世节点）** | 设置 `TEE_GENESIS_NODE=true` | 自动生成 GK 并密封 |
| **新节点加入** | DKG 组网成功后自动从老节点同步 GK | 无需手动操作，P2P 自动完成 |

**⚠️ 关键注意事项：**

| 情况 | 影响 | 处理方式 |
|------|------|----------|
| CPU 更换 / 机器迁移 | 旧 `SealedGK` 无法解封（sealing key 绑定 CPU） | 重新加入 DKG 组网，自动同步 GK |
| pebble 数据目录丢失 | GK 丢失 | 重新加入 DKG 组网，自动同步 GK |
| 全网所有节点同时丢失 GK | 无法恢复 | 灾难场景：需重新生成 GK 并重新加密所有 API Keys |
| 单个节点重启 | 无影响 | 自动从本地解封 |

---

## 6. 安全性分析

| 威胁模型 | 攻击方式 | 防护措施 |
|---|---|---|
| 链上数据泄露 | 查询合约状态 | 链上仅密文和引用标识，无明文 |
| 单节点磁盘窃取 | 物理访问 pebble 文件 | sealing key 绑定 enclave，其他机器无法解封 |
| 单节点内存转储 | 冷启动攻击 / side provider | TEE 内存加密，side provider 缓解由 TEE 硬件提供 |
| 网络嗅探 | 中间人攻击 | RA + attested TLS，证书绑定到 RA 报告 |
| 恶意节点加入 | 伪造 enclave | RA 验证 MRENCLAVE / MRSIGNER，仅合法 enclave 可加入 |
| 旧 GK 泄露 | 历史密钥被破解 | GK 定期轮换，旧密文全部重新加密 |
| 密文替换攻击 | 将 A 的密文替换到 B 的 ref | AAD 包含 ref + version，解密时验证 |

---

## 7. 性能分析

| 操作 | 延迟 | 说明 |
|---|---|---|
| API Key 解密（运行时） | ~1μs | AES-GCM，纯本地 CPU |
| GK unseal（启动时） | ~10μs | 一次 unseal + 内存缓存 |
| 链上密文同步 | 随区块同步 | 无额外延迟 |
| 新节点加入 | ~100ms | 一次 RA + TLS 握手 |
| GK 轮换（批量重加密） | 取决于 key 数量 | 离线操作，不影响运行时 |

---

*文档更新：2026-05-26*
