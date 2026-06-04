# Go 合约编译为 WASM 迁移方案

> 研究文档：分析当前 Go 原生合约迁移至 WASM 执行环境的技术路径、工作量与风险。
> 生成时间：2026-06-02

---

## 1. 背景与目标

### 1.1 当前状态

当前 `tee-chain/contracts/` 下已支持 4 个合约，采用 **Native Go + WASM 双路径执行** 架构：

| 合约 | 文件数 | 核心功能 | 存储依赖 |
|------|--------|----------|----------|
| `gov` | ~15 | 治理、代币、提案、投票 | `StoreMapping`, `IndexedList` |
| `auth` | ~8 | 用户认证、社交恢复 | `StoreMapping`, `StoreValue` |
| `gateway` | ~10 | AI Gateway、计费、API Key | `StoreMapping`, `IndexedList`, `IndexedStore`, `StoreList2D` |
| `batch` | ~3 | 批量合约调用 | `StoreMapping`（状态记录） |

所有合约共享以下基础设施：
- `pkg/model/db.go` — PebbleDB 封装（CGO，原生路径）
- `tee-chain/contracts/store/` — 存储抽象层（`StoreMapping`, `IndexedList`, `StoreValue`, `StoreList2D`）
- `pkg/util/seal.go` — TEE 密封（`SealWithProductKey` / `Unseal`）
- SCALE codec（静态生成，`tools/scale-codec-gen`）

### 1.2 目标

项目已实现 WASM 执行路径（`v1+`），与 `v0` 原生 Go 合约共享同一套存储格式与逻辑。实际采用 **wazero**（纯 Go、零 CGO）作为 WASM 引擎，而非 `wasmtime-go`。

关键约束与现状：
- **保持 Go 语言**（不用 Rust/ink! 重写）
- **存储等价**（v0 与 v1+ 共享 PebbleDB 存储格式）
- **WASM 引擎**：`wazero`（运行时宿主），TinyGo 编译合约运行时（`wasm/runtime/`）
- **编译输出**：`runtime.wasm.gz`（gzip 压缩后存于 DB），通过 `RuntimeUpgrade` 上链激活

---

## 2. 技术障碍分析

### 2.1 根本问题：CGO 依赖（已解决）

Go 编译为 WASM（`GOOS=wasip1 GOARCH=wasm`）时，**CGO 被强制禁用**。项目通过 **wazero host functions** 解决：WASM 合约不直接访问 PebbleDB，而是通过宿主提供的 host import 读写存储。

当前架构：

```
tee-chain/contracts/module/gov/gov.go  (v0 Native)
    ├── tee-chain/contracts/store/StoreMapping[...]
    │       └── *model.Txn → *pebble.Batch  (CGO，仅原生路径)
    
tee-chain/contracts/wasm/runtime/main.go  (v1+ WASM)
    ├── //go:build wasm
    ├── 通过 //go:wasmimport runtime/storage_get 等 host import 访问存储
    └── 实际存储操作由宿主 (wazero) 代理到 PebbleDB
```

编译方式：
```bash
# TinyGo 编译 WASM 运行时（build_runtime_wasm.sh）
tinygo build -target=wasi -buildmode=c-shared -o runtime.wasm ./wasm/runtime/main.go
gzip -k runtime.wasm
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

## 3. 实际架构（已落地）

项目未采用 build tags 双路径方案，而是选择了 **宿主-沙箱分离** 架构。以下第 3 节描述实际实现，第 4-8 节保留原始方案对比供历史参考。

### 3.1 架构图

```
┌─────────────────────────────────────────────────────────────────────┐
│                     宿主 (Host) — tee-node 二进制                      │
│  tee-chain/contracts/wasm/engine.go    ← wazero 引擎                 │
│  tee-chain/contracts/wasm/host.go      ← host functions 注册           │
│  tee-chain/contracts/contract.go       ← 版本路由 (v0 Native / v1+ WASM)│
│  tee-chain/contracts/runtime.go        ← Runtime 上下文               │
└──────────────────────────────┬──────────────────────────────────────┘
                               │ wazero 调用
                               ▼
┌─────────────────────────────────────────────────────────────────────┐
│                  WASM 沙箱 (Guest) — TinyGo 编译                       │
│  tee-chain/contracts/wasm/runtime/main.go                            │
│    ├── SCALE 解码 InContractCall                                     │
│    ├── 通过 //go:wasmimport runtime/storage_get 等访问宿主存储         │
│    └── 调用 contracts/module/call.go 路由到各合约 Query/Mutation       │
└─────────────────────────────────────────────────────────────────────┘
```

### 3.2 关键组件

| 组件 | 文件 | 职责 |
|------|------|------|
| **WASM 引擎** | `wasm/engine.go` | wazero 实例管理；从 DB 加载/缓存 gzip WASM；`ExecuteQuery` / `ExecuteMutation` |
| **Host Functions** | `wasm/host.go` | 注册 `"runtime"` 模块：storage_get/set/delete/scan_prefix/delete_prefix、get_height、get_block_time、get_version、get_caller、get_system_account、is_system |
| **WASM 运行时** | `wasm/runtime/main.go` | TinyGo 编译目标（`//go:build wasm`）；SCALE 解码调用参数，dispatch 到 `module.Query` / `module.Mutation` |
| **版本路由** | `contract.go` | `Query()` / `Mutation()` 根据 `wasm.NativeVersion` 分发到 Native Go（v0）或 WASM（v1+） |
| **Runtime 上下文** | `runtime.go` | `Runtime` 实现 `store.ContractApi`，提供跨合约 `Call()` / `Query()` |

### 3.3 Host Functions 列表

```go
// wasm/host.go 注册的 host imports
runtime.storage_get(ptr, len) -> (ptr, len) packed uint64
runtime.storage_set(ptr, len, ptr, len)
runtime.storage_delete(ptr, len)
runtime.storage_scan_prefix(ptr, len) -> ptr
runtime.storage_delete_prefix(ptr, len)
runtime.get_height() -> uint64
runtime.get_block_time() -> uint64
runtime.get_version() -> uint32
runtime.get_caller() -> (ptr, len) packed uint64
runtime.get_system_account() -> (ptr, len) packed uint64
runtime.is_system() -> uint32
```

### 3.4 编译与部署流程

```bash
# 1. TinyGo 编译 WASM 运行时（hack/build_runtime_wasm.sh）
cd tee-chain/contracts/wasm/runtime
tinygo build -target=wasi -buildmode=c-shared -o runtime.wasm main.go
gzip -k runtime.wasm

# 2. 运行时升级上链（通过 RuntimeUpgrade 交易）
# WASM 字节码存储于 PebbleDB，按版本号索引
# contract_run.go: ApplyRuntimeUpgrade() 激活新版本
```

### 3.5 WASM 体积（实际）

| 内容 | 体积 |
|------|------|
| TinyGo 运行时 + 合约逻辑 | ~数百 KB（原始） |
| gzip 压缩后 | ~数十 KB |
| 存储方式 | `runtime.wasm.gz` 存入 PebbleDB，按 `version` 索引 |

---

## 3+. 与 Native 路径的对比

| 维度 | Native Go (v0) | WASM (v1+) |
|------|----------------|------------|
| 执行引擎 | 直接 Go 函数调用 | wazero 沙箱 |
| 存储访问 | 直接 `*pebble.Batch` | 通过 host function 代理 |
| 跨合约调用 | 直接 `runtime.Call()` | 通过 host `runtime_call` |
| 升级方式 | 需重新编译节点二进制 | `RuntimeUpgrade` 交易热升级 |
| 安全性 | 依赖 TEE 和代码审计 | 增加沙箱隔离层 |
| 性能 | 高（零 overhead） | 中（host call + memory copy） |

---

## 3++. 已完成的实施步骤

- [x] **WASM 引擎集成**：wazero 引擎、host functions、内存管理
- [x] **WASM 运行时编译**：TinyGo 编译脚本 (`hack/build_runtime_wasm.sh`)
- [x] **版本路由**：`contract.go` 按 `NativeVersion` 分发
- [x] **RuntimeUpgrade 支持**：`contract_run.go` 单调版本升级、激活高度记录
- [x] **存储等价**：WASM 与 Native 共享同一套 PebbleDB 存储格式
- [x] **合约模块接入**：`module/call.go` 已支持 WASM dispatch

---

## 4. 历史方案对比（供参考）

### 方案 A：存储层抽象 + Build Tags（原推荐）

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

## 6. 风险与缓解（实际状态）

| 风险 | 状态 | 缓解措施 |
|------|------|----------|
| host function 调用开销 | 已接受 | benchmark 对比显示可接受；Native 路径保留作为默认 |
| WASM 运行时兼容性 | 已解决 | TinyGo + wasi 目标稳定；wazero 纯 Go 无 CGO 依赖 |
| 存储格式兼容性 | 已验证 | v0 与 v1+ 共享同一存储层，key 编码完全一致 |
| 升级回滚 | 已规划 | `RuntimeUpgrade` 严格单调递增；旧版本 WASM 保留于 DB |
| Go WASM 运行时过大 | 已解决 | 使用 TinyGo 替代标准 Go 编译，体积降至数百 KB |
| `ListByPrefix` 性能 | 已解决 | host 侧 `storage_scan_prefix` 直接代理到 Pebble prefix 扫描 |

---

## 7. 决策结论（已确定）

1. **WASM 引擎**：✅ 采用 **wazero**（纯 Go，零 CGO），而非 `wasmtime-go`
2. **编译器**：✅ 采用 **TinyGo**（`-target=wasi`），WASM 体积数百 KB
3. **存储前缀扫描**：✅ host 侧提供 `storage_scan_prefix`，WASM 侧不缓存
4. **contractgen**：✅ `*_gen.go` 无需修改；WASM 入口由 `wasm/runtime/main.go` 统一 dispatch
5. **加密**：✅ WASM 合约不处理 TEE 密封；密封逻辑仅在宿主侧 `pkg/util/tee_seal.go`

---

## 8. 附录：编译验证

### 8.1 编译 WASM 运行时

```bash
cd tee-chain/contracts/wasm/runtime
tinygo build -target=wasi -buildmode=c-shared -o runtime.wasm main.go
gzip -k runtime.wasm
```

### 8.2 引擎测试

```bash
go test ./tee-chain/contracts/wasm/...
```

### 8.3 历史原型（概念验证阶段）

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
