# 合约存储设计方案

## 1. 设计目标

- **零额外依赖**：完全基于 pebble 键值存储，无外部数据库
- **类型安全**：泛型约束，编译期校验 key/value 类型
- **索引即代码**：通过 struct tag 声明索引，自动维护，无需手写索引逻辑
- **可组合**：基础存储类型可自由组合，适配任意合约的数据结构需求

## 2. 存储类型分层

```
┌─────────────────────────────────────────────────────────────┐
│                     合约层（业务代码）                         │
│  Gateway / Gov / Auth ...                                   │
│         │                                                   │
│         ▼                                                   │
│  ┌──────────────┬──────────────┬─────────────────────────┐  │
│  │ IndexedStore │ IndexedList  │ StoreList2D             │  │
│  │ 带索引mapping │ 带索引列表    │ 二维列表（K1 → Ix → V） │  │
│  └──────┬───────┴──────┬───────┴────────────┬────────────┘  │
│         │              │                    │               │
│  ┌──────▼──────┐ ┌─────▼──────┐      ┌─────▼──────┐       │
│  │ StoreIndex  │ │ StoreIndex │      │ StoreList  │       │
│  │  二级倒排索引│ │ 二级倒排索引│      │  自增列表   │       │
│  └──────┬──────┘ └─────┬──────┘      └─────┬──────┘       │
│         │              │                    │               │
│  ┌──────▼──────────────▼────────────────────▼────────────┐ │
│  │              StoreMapping[K, V]                        │ │
│  │              通用 mapping（key → value）                │ │
│  └────────────────────────────────────────────────────────┘ │
│                            │                                │
│  ┌─────────────────────────▼──────────────────────────────┐ │
│  │              StoreValue[V]                              │ │
│  │              单值存储（固定 key）                        │ │
│  └────────────────────────────────────────────────────────┘ │
│                            │                                │
│  ┌─────────────────────────▼──────────────────────────────┐ │
│  │              Txn（pebble 读写事务）                      │ │
│  └────────────────────────────────────────────────────────┘ │
└─────────────────────────────────────────────────────────────┘
```

## 3. 基础存储类型

### 3.1 Txn — 底层事务

```go
type Txn struct {
    db *pebble.DB
    // ...
}
```

所有存储操作都接收 `*Txn` 参数，保证原子性。

### 3.2 StoreValue[V] — 单值存储

固定 key，存储单个值。常用于全局配置、计数器等。

```go
type StoreValue[V any] struct {
    Namespace string // 命名空间，如 "gov"
    Key       string // 固定 key，如 "issuance"
}

func (s *StoreValue[V]) Get(txn *Txn) (*V, error)
func (s *StoreValue[V]) Set(txn *Txn, val V) error
```

**示例：**
```go
totalIssuance := &model.StoreValue[model.Amount]{Namespace: "gov", Key: "issuance"}
totalIssuance.Set(txn, model.Amount{...})
```

### 3.3 StoreMapping[K, V] — 通用 Mapping

类似 Solidity `mapping(key => value)`，最基础的键值存储。

```go
type StoreMapping[K MappingKey, V any] struct {
    Namespace string // 命名空间，如 "gateway"
    KeyPrefix string // 键前缀，如 "credits_"
}

// MappingKey 约束：string | []byte | uint64 | uint32 | uint16 | uint8 | UniAddr
```

**API：**
```go
func (m *StoreMapping[K, V]) Get(txn *Txn, key K) (*V, error)
func (m *StoreMapping[K, V]) Set(txn *Txn, key K, val V) error
func (m *StoreMapping[K, V]) Delete(txn *Txn, key K) error
func (m *StoreMapping[K, V]) Contains(txn *Txn, key K) (bool, error)
func (m *StoreMapping[K, V]) List(txn *Txn) ([][]byte, []V, error)  // 全量扫描（仅开发调试用）
```

**键编码规则：**
```
完整键 = Namespace + ":" + KeyPrefix + keySuffix(key)

keySuffix 编码：
  string  → 原值
  []byte  → hex.EncodeToString
  uint64  → strconv.FormatUint(v, 10)
  UniAddr → SCALE hex 编码
```

**示例：**
```go
credits := &model.StoreMapping[model.UniAddr, uint64]{Namespace: "gateway", KeyPrefix: "credits_"}
credits.Set(txn, caller, 1000)
```

### 3.4 StoreList[K, V] — 自增列表

自增 id 作为 key（uint8/16/32/64），支持 insert/get/update 和分页 list。

```go
type StoreList[K ListIndex, V any] struct {
    Namespace      string // 命名空间
    KeyPrefixID    string // next_id 前缀，如 "next_model_id"
    KeyPrefixItems string // items 前缀，如 "model_"
}

// ListIndex = uint8 | uint16 | uint32 | uint64
```

**API：**
```go
func (l *StoreList[K, V]) Len(txn *Txn) (K, error)                // 当前长度（下一个将分配的 id）
func (l *StoreList[K, V]) Insert(txn *Txn, value V) (K, error)    // 插入，返回分配的 id
func (l *StoreList[K, V]) Get(txn *Txn, key K) (*V, error)
func (l *StoreList[K, V]) Update(txn *Txn, key K, value V) error
func (l *StoreList[K, V]) Clear(txn *Txn, key K) error           // 删除但不改变 len
func (l *StoreList[K, V]) List(txn *Txn, startKey K, size uint32) ([]K, []V, error)
func (l *StoreList[K, V]) DescList(txn *Txn, startKey *K, size uint32) ([]K, []V, error)
```

**示例：**
```go
models := model.NewStoreList[uint64, ModelInfo]("gateway", "next_model_id", "model_")
id, _ := models.Insert(txn, ModelInfo{...})
info, _ := models.Get(txn, id)
```

### 3.5 StoreList2D[K1, Ix, V] — 二维列表

第一层 key（如用户地址）对应一个独立的自增列表。常用于 "某用户的所有投票"。

```go
type StoreList2D[K1 List2DKey1, Ix ListIndex, V any] struct {
    Namespace       string
    KeyPrefixK1     string // K1 → next_ix 映射前缀
    KeyPrefixItems  string // (K1, ix) → V 前缀
}

// List2DKey1 = string | []byte | uint64 | uint32 | uint16 | uint8 | UniAddr
```

**API：**
```go
func (l *StoreList2D[K1, Ix, V]) Append(txn *Txn, k1 K1, val V) (Ix, error)
func (l *StoreList2D[K1, Ix, V]) Get(txn *Txn, k1 K1, ix Ix) (*V, error)
func (l *StoreList2D[K1, Ix, V]) List(txn *Txn, k1 K1, startIx Ix, size uint32) ([]Ix, []V, error)
func (l *StoreList2D[K1, Ix, V]) Len(txn *Txn, k1 K1) (Ix, error)
```

**示例：**
```go
votesOfUser := model.NewStoreList2D[model.UniAddr, uint32, VoteRef]("gov", "votes_of_user_", "vote_item_")
votesOfUser.Append(txn, caller, VoteRef{ProposalID: 1, VoteIndex: 0})
```

## 4. 索引系统

### 4.1 StoreIndex[K, ID] — 二级倒排索引

将字段值 K 映射到一组主键 ID。底层是 `StoreMapping[K, []ID]`。

```go
type StoreIndex[K IdxKey, ID IdxID] struct {
    inner *StoreMapping[K, []ID]
}

func NewStoreIndex[K IdxKey, ID IdxID](namespace, prefix string) *StoreIndex[K, ID]
func (idx *StoreIndex[K, ID]) Add(txn *Txn, key K, id ID) error
func (idx *StoreIndex[K, ID]) Remove(txn *Txn, key K, id ID) error
func (idx *StoreIndex[K, ID]) GetIDs(txn *Txn, key K) ([]ID, error)
func (idx *StoreIndex[K, ID]) GetIDsByKeys(txn *Txn, keys []K) ([]ID, error)
```

### 4.2 自动索引 — IndexedStore / IndexedList

通过 struct tag `idx` 声明索引字段，写入时自动维护，无需手写索引逻辑。

**支持的索引类型：**

| Tag | 说明 | 适用字段类型 |
|-----|------|-------------|
| `idx:"exact"` | 精确匹配索引 | uint8/16/32/64, **string** |
| `idx:"bucket:a,b,c"` | 分桶范围索引 | uint64 |

**数据结构示例：**
```go
type ModelInfo struct {
    Name            string `json:"name"          idx:"exact"`
    InputModalities uint8  `json:"inputModalities" idx:"exact"`
    ContextLength   uint64 `json:"contextLength"   idx:"bucket:4000,8000,16000,32000"`
    PromptPricing   uint64 `json:"promptPricing"   idx:"bucket:1,5,10,50,100,500"`
}
```

#### IndexedStore[ID, V] — 带索引的 Mapping

```go
type IndexedStore[ID IdxID, V any] struct {
    Primary *StoreMapping[ID, V]
    idx     *idxRegistry[ID, V]
}

func NewIndexedStore[ID IdxID, V any](namespace, prefix string) *IndexedStore[ID, V]

func (s *IndexedStore[ID, V]) Put(txn *Txn, id ID, val V) error      // 写 + 自动索引
func (s *IndexedStore[ID, V]) Update(txn *Txn, id ID, fn func(cur *V)) error  // 更新 + 自动比对索引
func (s *IndexedStore[ID, V]) Delete(txn *Txn, id ID) error         // 删除 + 清理索引
func (s *IndexedStore[ID, V]) Query(txn *Txn, filter map[string]any) ([]ID, error)
```

#### IndexedList[K, V] — 带索引的列表

```go
type IndexedList[K ListIndex, V any] struct {
    *StoreList[K, V]
    idx *idxRegistry[K, V]
}

func NewIndexedList[K ListIndex, V any](namespace, prefixNextID, prefixItems string) *IndexedList[K, V]

func (l *IndexedList[K, V]) Insert(txn *Txn, val V) (K, error)              // 插入 + 自动分配 id + 自动索引
func (l *IndexedList[K, V]) UpdateFn(txn *Txn, key K, fn func(cur *V)) error // 更新 + 自动比对索引
func (l *IndexedList[K, V]) Clear(txn *Txn, key K) error                    // 清除 + 清理索引
func (l *IndexedList[K, V]) Query(txn *Txn, filter map[string]any) ([]K, error)
```

**Query 示例：**
```go
models := model.NewIndexedList[uint64, ModelInfo]("gateway", "next_model_id", "model_")

// uint exact 精确匹配
ids, _ := models.Query(txn, map[string]any{"InputModalities": uint64(3)})

// string exact 精确匹配
ids, _ := models.Query(txn, map[string]any{"Name": "gpt-4"})

// 范围查询（桶合并）
ids, _ := models.Query(txn, map[string]any{
    "ContextLength": model.Range{Min: 4000, Max: 16000},
})

// 组合查询（AND）
ids, _ := models.Query(txn, map[string]any{
    "InputModalities": uint64(3),
    "PromptPricing":   model.Range{Min: 0, Max: 50},
})
```

**实际合约中的 string exact 索引示例：**

```go
// ModelInfo — Name 和 ProviderID 都建了 exact 索引
type ModelInfo struct {
    Name       string `json:"name"       idx:"exact"`
    ProviderID uint64 `json:"providerId" idx:"exact"`
    // ...
}

// ProviderInfo — Name 建了 exact 索引
type ProviderInfo struct {
    Name   string `json:"name" idx:"exact"`
    Region uint8  `json:"region" idx:"exact"`
    // ...
}

// 合约查询方法（直接命中索引，无需全表扫描）
func (g GatewayQuery) GetModelByName(name string) ([]ModelInfo, error) {
    keys, _ := g.models.Query(g.api.GetTxn(), map[string]any{"Name": name})
    // ...
}

func (g GatewayQuery) GetProviderByName(name string) ([]ProviderInfo, error) {
    keys, _ := g.providers.Query(g.api.GetTxn(), map[string]any{"Name": name})
    // ...
}
```

## 5. 存储类型选择指南

| 业务场景 | 推荐类型 | 说明 |
|---------|---------|------|
| 全局配置/开关 | `StoreValue[V]` | 单值，固定 key |
| 账户余额 | `StoreMapping[UniAddr, uint64]` | 一对一映射 |
| 用户 → 实体ID | `StoreMapping[UniAddr, uint64]` | 如 owner → providerID |
| 自增列表（模型、Provider） | `IndexedList[K, V]` | 自动 id + 可索引查询 |
| 需要按字段查询的 mapping | `IndexedStore[ID, V]` | 如 APIKey 按 owner 查询 |
| 某用户的子列表 | `StoreList2D[K1, Ix, V]` | 如某用户的所有投票 |
| 手动维护的索引 | `StoreIndex[K, ID]` | 底层索引，通常不直接使用 |

## 6. 键命名空间规范

```
完整存储键格式：
  {namespace}:{prefix}{suffix}

示例：
  "gateway:credits_0x1234..."           // StoreMapping
  "gateway:next_model_id"               // StoreList next_id
  "gateway:model_0"                     // StoreList item
  "gateway:idx_InputModalities_3"       // exact 索引
  "gateway:idx_ContextLength_4000"      // bucket 索引
  "gov:votes_of_user_v1_0xabcd..."      // StoreList2D K1
  "gov:vote_item_0xabcd..._0"           // StoreList2D item
```

**命名空间分配：**

| Namespace | 合约 |
|-----------|------|
| `gateway` | Gateway 合约 |
| `gov` | Gov 治理合约 |
| `auth` | Auth 认证合约 |
| `TEEKMS` | TEE KMS 本地密钥存储 |

## 7. 类型约束汇总

```go
// 可作为 Mapping key 的类型
MappingKey = string | []byte | uint64 | uint32 | uint16 | uint8 | UniAddr

// 可作为 List 自增 id 的类型
ListIndex = uint8 | uint16 | uint32 | uint64

// 可作为 2D List 第一层 key 的类型
List2DKey1 = string | []byte | uint64 | uint32 | uint16 | uint8 | UniAddr

// 可作为索引键的类型
IdxKey = MappingKey

// 可作为索引引用 ID 的类型
IdxID = uint64 | uint32 | uint16 | uint8 | string | []byte
```
