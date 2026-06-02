# Go 合约编译为 WASM 迁移方案

> 研究文档：分析当前 Go 原生合约迁移至 WASM 执行环境的技术路径、工作量与风险。
> 生成时间：2026-06-02

---

## 1. 背景与目标

### 1.1 当前状态

当前 `tee-chain/contracts/` 下有 4 个原生 Go 合约：

| 合约 | 文件数 | 核心功能 | 存储依赖 |
|------|--------|----------|----------|
| `gov` | ~15 | 治理、代币、提案、投票 | `StoreMapping`, `IndexedList` |
| `auth` | ~8 | 用户认证、社交恢复 | `StoreMapping`, `StoreValue` |
| `gateway` | ~10 | AI Gateway、计费、API Key | `StoreMapping`, `IndexedList`, `IndexedStore`, `StoreList2D` |
| `batch` | ~3 | 批量合约调用 | `StoreMapping`（状态记录） |

所有合约共享以下基础设施：
- `pkg/model/db.go` — PebbleDB 封装（CGO）
- `pkg/model/db_txn.go` — `Txn` 类型（`*pebble.Batch`）
- `pkg/model/contract_*.go` — 存储抽象层（`StoreMapping`, `IndexedList`, `StoreValue`, `StoreList2D`）
- `pkg/util/seal.go` — 加密（`SealWithProductKey` / `Unseal`）
- SCALE codec（`go-substrate-rpc-client`）

### 1.2 目标

将上述 4 个 Go 合约编译为 WASM（`v1`），实现与 `v0` 原生 Go 合约**完全等价**的逻辑和存储行为。

关键约束：
- **保持 Go 语言**（不用 Rust/ink! 重写）
- **完全等价**（v1 逻辑 = v0 逻辑，存储格式不变）
- **编译输出**：`.wasm` 文件，通过 `wasmtime-go` 执行

---

## 2. 技术障碍分析

### 2.1 根本问题：CGO 依赖

Go 编译为 WASM（`GOOS=wasip1 GOARCH=wasm`）时，**CGO 被强制禁用**。

当前合约的 CGO 依赖链：

```
contracts/gov/gov.go
    ├── model.StoreMapping[...]  (pkg/model/contract_mapping.go)
    │       └── *model.Txn
    │               └── *pebble.Batch  (github.com/cockroachdb/pebble)
    │                       └── CGO: RocksDB C++ 代码
    │
    ├── model.IndexedList[...]  (pkg/model/contract_indexer.go)
    │       └── *model.Txn
    │               └── *pebble.Batch
    │
    └── util.SealWithProductKey()  (pkg/util/seal.go)
            └── github.com/edgelesssys/ego (CGO)
```

编译测试结果：
```bash
$ CGO_ENABLED=0 GOOS=wasip1 GOARCH=wasm go build ./tee-chain/contracts/...
# github.com/cockroachdb/pebble/internal/rawalloc
undefined array length maxArrayLen or missing type constraint
# github.com/cockroachdb/pebble
... (大量 CGO 错误)
```

### 2.2 障碍清单

| # | 障碍 | 影响范围 | 解决难度 |
|---|------|----------|----------|
| 1 | **PebbleDB（CGO）** | `pkg/model/db.go`, `db_txn.go`, 所有 `contract_*.go` | 高 — 需重构整个存储层 |
| 2 | **加密库（ego CGO）** | `pkg/util/seal.go` | 中 — 可用纯 Go 替代或 host function |
| 3 | **goleveldb（CGO）** | CometBFT 依赖（间接） | 低 — 不影响合约编译，但影响整体构建 |
| 4 | **泛型使用** | `StoreMapping[K,V]`, `IndexedList[K,V]` | 中 — Go WASM 支持泛型，但需适配新存储层 |
| 5 | **迭代器（Iterator）** | `ListByPrefix`, `DeletekeysByPrefix` | 中 — WASI 无文件系统迭代器，需 host function 模拟 |
| 6 | **protobuf/gogoproto** | `tx.pb.go`, `PeerMsg` | 低 — 纯 Go，WASM 支持 |
| 7 | **SCALE codec** | `go-substrate-rpc-client` | 低 — 纯 Go，WASM 支持 |

---

## 3. 方案对比

### 方案 A：存储层抽象 + Build Tags（推荐）

**核心思想**：用 Go build tags 提供两套存储实现，合约代码零修改。

```
pkg/model/
├── db.go              (build !wasm) — PebbleDB 实现
├── db_wasm.go         (build wasm)  — Host Function 实现
├── db_txn.go          (build !wasm) — *pebble.Batch
├── db_txn_wasm.go     (build wasm)  — 纯 Go 内存映射 + host functions
├── contract_mapping.go       — 泛型存储抽象（不直接依赖 pebble）
├── contract_list.go          — 同上
├── contract_indexer.go       — 同上
└── ...
```

**WASM 模式下的 Txn 实现**：
```go
// db_txn_wasm.go (build wasm)
type Txn struct {
    data map[string][]byte  // 内存缓存（WASM 线性内存内）
}

func (txn *Txn) Get(key []byte) ([]byte, error) {
    // 1. 先查本地缓存
    if v, ok := txn.data[string(key)]; ok {
        return v, nil
    }
    // 2. 通过 host function 查询外部存储
    return hostStorageGet(key)
}

func (txn *Txn) Set(key, value []byte) error {
    // 写入本地缓存（在 Commit 时批量通过 host function 写入）
    txn.data[string(key)] = value
    return nil
}

func (txn *Txn) Commit() error {
    // 批量通过 host function 写入外部存储
    for k, v := range txn.data {
        hostStorageSet([]byte(k), v)
    }
    return nil
}
```

**优点**：
- 合约代码（`gov/`, `auth/`, `gateway/`, `batch/`）**完全不需要修改**
- `contractgen` 生成的 `*_gen.go` 也不需要修改
- 存储层 API 保持一致

**缺点**：
- `db_txn_wasm.go` 中需要实现完整的 `Txn` API（Get, Set, Delete, ListByPrefix, Commit, Rollback）
- `ListByPrefix` / `DeletekeysByPrefix` 在 WASM 模式下需要额外处理（前缀扫描无底层迭代器支持）
- 加密（`util.SealWithProductKey`）需要替代实现

**工作量**：约 2-3 周

---

### 方案 B：条件导入 + 平行存储包

**核心思想**：创建 `pkg/modelwasm` 包，合约文件用 build tags 选择导入。

```
contracts/gov/
├── gov.go             (build !wasm) — import "pkg/model"
├── gov_wasm.go        (build wasm)  — import "pkg/modelwasm"
├── types.go           — 共享类型定义
└── ...
```

**优点**：
- 完全隔离，不影响原生路径
- 可以针对 WASM 优化存储实现

**缺点**：
- 每个合约文件需要写 **两份**（`*_wasm.go`）
- 代码膨胀，维护成本高
- `contractgen` 需要生成两套 dispatch 代码

**工作量**：约 4-6 周

---

### 方案 C：剥离业务逻辑 + WASM Shim

**核心思想**：将合约的纯业务逻辑提取为不依赖存储的函数，WASM 层只做 dispatch。

```go
// 业务逻辑层（纯函数，无存储依赖）
func ExecuteBatch(calls []BatchCall) error {
    for i, call := range calls {
        if len(call.Name) == 0 {
            return fmt.Errorf("batch call %d: empty contract name", i)
        }
        // ...
    }
    return nil
}

// WASM 入口层
//go:wasmexport execute_batch
func wasmExecuteBatch(ptr, len int32) int32 {
    // 从 WASM 内存读取 SCALE 编码的 calls
    // 调用 ExecuteBatch
    // 通过 host functions 读写存储
}
```

**优点**：
- 业务逻辑可测试性强
- WASM 层极简

**缺点**：
- 需要重写所有合约，分离存储逻辑
- 存储访问模式完全改变
- 与 v0 不完全等价

**工作量**：约 6-8 周

---

### 方案 D：Rust/ink! 重写（不采用，但供参考）

**核心思想**：用 Rust ink! 重写所有合约。

**优点**：
- WASM 生态最成熟
- ink! 提供完善的存储抽象
- Substrate 主链也是 Rust，工具链一致

**缺点**：
- 完全重写 4 个合约
- 需要维护两套代码（Go + Rust）
- 团队需要学习 Rust

**工作量**：约 2-3 个月

---

## 4. 推荐方案：方案 A（存储层抽象 + Build Tags）

### 4.1 架构设计

```
┌─────────────────────────────────────────────────────────────────────┐
│                        合约层（零修改）                               │
│  contracts/gov/*.go  contracts/auth/*.go                             │
│  contracts/gateway/*.go  contracts/batch/*.go                        │
│  （完全复用现有代码，包括 *_gen.go）                                   │
└──────────────────────────────┬──────────────────────────────────────┘
                               │
┌──────────────────────────────▼──────────────────────────────────────┐
│                        存储抽象层（统一 API）                          │
│  pkg/model/contract_mapping.go  — StoreMapping[K,V]                  │
│  pkg/model/contract_list.go     — StoreList[K,V]                     │
│  pkg/model/contract_indexer.go  — IndexedList/IndexedStore           │
│  pkg/model/contract_value.go    — StoreValue[V]                      │
│  （泛型存储抽象，不直接依赖 pebble）                                   │
└──────────────────────────────┬──────────────────────────────────────┘
                               │
        ┌──────────────────────┴──────────────────────┐
        │                                          │
┌───────▼────────┐                        ┌────────▼───────┐
│ 原生实现 (!wasm) │                        │ WASM 实现 (wasm) │
│                │                        │                │
│ pkg/model/     │                        │ pkg/model/     │
│   db.go        │                        │   db_wasm.go   │
│   db_txn.go    │                        │   db_txn_wasm.go│
│   （PebbleDB）  │                        │   （host func） │
└────────────────┘                        └────────────────┘
```

### 4.2 关键设计点

#### 4.2.1 Txn 接口化

将 `Txn` 的方法抽象为 interface，两种实现：

```go
// pkg/model/store.go
package model

type KVStore interface {
    Get(key []byte) ([]byte, error)
    Set(key, value []byte) error
    Delete(key []byte) error
    ListByPrefix(prefix []byte) (keys [][]byte, vals [][]byte, err error)
    ListByPrefixLimit(prefix []byte, offset, limit uint64) (keys [][]byte, vals [][]byte, err error)
    DeletekeysByPrefix(prefix []byte) error
    Commit() error
    Rollback() error
}

// Txn 类型改用 interface 包装
type Txn struct {
    store KVStore
}
```

原生实现：
```go
// db_txn.go (build !wasm)
type pebbleStore struct {
    batch *pebble.Batch
}
func (s *pebbleStore) Get(key []byte) ([]byte, error) { ... }
// ...

func (db *DB) NewTransaction() *Txn {
    return &Txn{store: &pebbleStore{batch: db.DB.NewIndexedBatch()}}
}
```

WASM 实现：
```go
// db_txn_wasm.go (build wasm)
type wasmStore struct {
    cache map[string][]byte
}

func (s *wasmStore) Get(key []byte) ([]byte, error) {
    // 1. 查缓存
    if v, ok := s.cache[string(key)]; ok {
        return v, nil
    }
    // 2. host function 查询
    return hostStorageGet(key)
}

func (s *wasmStore) Commit() error {
    for k, v := range s.cache {
        hostStorageSet([]byte(k), v)
    }
    return nil
}
```

#### 4.2.2 Host Functions（WASM 合约内部调用）

WASM 合约需要导入 host functions：

```go
//go:wasmimport env storage_get
func hostStorageGet(keyPtr, keyLen, valPtr, valLen int32) int32

//go:wasmimport env storage_set
func hostStorageSet(keyPtr, keyLen, valPtr, valLen int32) int32

//go:wasmimport env get_height
func hostGetHeight() int64

// ... 其他 host functions
```

> 注：`//go:wasmimport` 是 Go 1.21+ 的标准指令。

#### 4.2.3 加密替代

`util.SealWithProductKey` 依赖 `github.com/edgelesssys/ego`（CGO）。

WASM 模式下需要：
- **方案 A1**：用纯 Go 加密库替代（如 `golang.org/x/crypto/chacha20poly1305`）
- **方案 A2**：加密通过 host function 完成（`env.encrypt` / `env.decrypt`）

推荐 **A2**：将加密逻辑放在 host 侧，WASM 合约不处理密钥管理。

### 4.3 编译流程

```bash
# 1. 构建 v0 原生合约（不变）
go build ./...

# 2. 构建 v1 WASM 合约
cd tee-chain/contracts
CGO_ENABLED=0 GOOS=wasip1 GOARCH=wasm go build \
    -tags=wasm \
    -ldflags="-s -w" \
    -o ../../token_chain_data/runtime_v1.wasm \
    ./
```

> `-ldflags="-s -w"` 可以减小 WASM 体积。

### 4.4 WASM 体积预估

| 内容 | 预估体积 |
|------|----------|
| Go 运行时（WASM） | ~1.5 MB |
| 合约代码 + 存储层 | ~500 KB |
| SCALE codec + utils | ~300 KB |
| **总计** | **~2-3 MB** |

优化方式：
- `-ldflags="-s -w"`：去除调试信息（-30%）
- `-ldflags="-buildid="`：去除 build ID
- 使用 `tinygo`（如果支持所有依赖）：~500 KB（但 tinygo 不支持所有 Go 特性）

---

## 5. 实施步骤

### Phase 1：存储层重构（1 周）

1. 创建 `pkg/model/store.go`，定义 `KVStore` interface
2. 修改 `pkg/model/Txn`，改为包装 `KVStore` 而不是 `*pebble.Batch`
3. 创建 `pkg/model/db_wasm.go` + `db_txn_wasm.go`，实现 host function 存储
4. 验证原生路径编译通过（`go build ./...`）

### Phase 2：加密替代（2-3 天）

1. 创建 `pkg/util/seal_wasm.go`，用 host functions 替代 ego
2. 或改用纯 Go 加密
3. 验证加密/解密一致性

### Phase 3：合约编译验证（2-3 天）

1. 尝试编译 `contracts/batch` 为 WASM（最简单，无复杂存储）
2. 修复编译错误
3. 测试 WASM 执行

### Phase 4：完整合约编译（3-4 天）

1. 编译 `contracts/auth` → WASM
2. 编译 `contracts/gateway` → WASM
3. 编译 `contracts/gov` → WASM
4. 验证功能等价性

### Phase 5：集成测试（2-3 天）

1. 生成 `runtime_v1.wasm`
2. 通过 `RuntimeUpgrade` 上链
3. 测试 v0 和 v1 切换
4. 测试旧区块重放

**总计**：约 2-3 周

---

## 6. 风险与缓解

| 风险 | 可能性 | 影响 | 缓解措施 |
|------|--------|------|----------|
| `ListByPrefix` WASM 实现性能差 | 中 | 高 | 在 host 侧实现前缀扫描缓存 |
| Go WASM 运行时过大（2-3MB） | 高 | 中 | 使用 `-ldflags="-s -w"` 优化；考虑 tinygo |
| `contractgen` 生成的代码不兼容 WASM | 低 | 高 | 提前验证 batch 合约编译 |
| 加密一致性（ego vs 纯 Go） | 中 | 高 | 用 host function 统一加密逻辑 |
| host function 调用开销 | 中 | 中 | benchmark 对比原生 vs WASM 路径 |
| 存储格式兼容性（v0 vs v1） | 低 | 高 | 确保 Txn 序列化格式一致 |

---

## 7. 决策点

在正式开工前，需要确认以下决策：

1. **加密方案**：
   - [ ] 纯 Go 加密替代（`chacha20poly1305`）
   - [ ] Host function 加密（`env.encrypt` / `env.decrypt`）

2. **WASM 体积优化**：
   - [ ] 接受 2-3MB 的 Go WASM
   - [ ] 投入时间用 TinyGo 优化到 <1MB

3. **存储前缀扫描**：
   - [ ] 在 WASM 侧缓存所有 key（内存占用大）
   - [ ] 新增 host function `env.storage_scan_prefix`

4. **contractgen 适配**：
   - [ ] 当前 `*_gen.go` 无需修改
   - [ ] 需要生成 WASM 入口包装

---

## 8. 附录：最小可编译原型

作为概念验证，可以先编译以下最小程序：

```go
// contracts/wasmstub/main.go
//go:build wasm

package main

import (
    _ "github.com/computeflux/tee-provider/tee-chain/contracts/batch"
)

//go:wasmexport version
func version() int32 { return 1 }

func main() {}
```

编译命令：
```bash
CGO_ENABLED=0 GOOS=wasip1 GOARCH=wasm go build \
    -tags=wasm \
    -ldflags="-s -w" \
    -o runtime_v1.wasm \
    ./contracts/wasmstub
```

这个原型可以验证：
1. `contracts/batch` 及其依赖是否能编译为 WASM
2. 哪些包需要 WASM 适配
