# 最小化 Host Functions WASM 方案

> 核心思想：只需暴露 `Txn.Get` / `Txn.Set` 两个底层存储函数，WASM 合约即可通过闭包访问 PebbleDB。

---

## 1. 问题简化

之前的方案过度设计：把 Runtime 的每个方法（get_height, get_caller, is_system...）都暴露为 host function。

实际上合约只需要两件事：
1. **读写存储** — `Txn.Get(key)` / `Txn.Set(key, val)`
2. **获取上下文** — height, version, caller, system account

而第 2 点（上下文）可以在合约启动时**一次性传入 WASM 内存**，不需要持续通过 host function 调用。

---

## 2. 最小 Host Functions 集合

只需要 **2 个** host function：

| Host Function | 签名 | 作用 |
|--------------|------|------|
| `runtime.storage_get` | `(key_ptr, key_len, val_ptr, val_len) -> i32` | 读取存储 |
| `runtime.storage_set` | `(key_ptr, key_len, val_ptr, val_len) -> i32` | 写入存储 |

WASM 合约侧，这两个函数被包装为 `KVStore` interface：

```go
// contracts/wasmcompat/store.go
//go:build wasm

package wasmcompat

//go:wasmimport runtime storage_get
func hostGet(keyPtr, keyLen, valPtr, valLen int32) int32

//go:wasmimport runtime storage_set
func hostSet(keyPtr, keyLen, valPtr, valLen int32) int32

type HostStore struct{}

func (s *HostStore) Get(key []byte) ([]byte, error) {
    // 1. 在 WASM 内存分配 key 缓冲区
    // 2. 调用 hostGet
    // 3. 从 WASM 内存读取返回值
}

func (s *HostStore) Set(key, value []byte) error {
    // 同上
}
```

---

## 3. Txn 的双模式实现

`pkg/model/db_txn.go` 用 build tags 提供两套实现：

### 原生模式（`!wasm`）— 不变

```go
// pkg/model/db_txn.go
//go:build !wasm

type Txn struct {
    in *pebble.Batch
}

func (txn *Txn) Get(key []byte) ([]byte, error) {
    v, _, err := txn.in.Get(key)
    return util.Unseal(v, nil)
}

func (txn *Txn) Set(key, value []byte) error {
    val, _ := util.SealWithProductKey(value, nil)
    return txn.in.Set(key, val, pebble.Sync)
}
```

### WASM 模式（`wasm`）— 通过 host functions

```go
// pkg/model/db_txn_wasm.go
//go:build wasm

type Txn struct {
    // WASM 模式下不需要 pebble.Batch
    // Get/Set 直接调用 host functions
}

func (txn *Txn) Get(key []byte) ([]byte, error) {
    return hostStorageGet(key)   // 调用 wasmimport
}

func (txn *Txn) Set(key, value []byte) error {
    return hostStorageSet(key, value)  // 调用 wasmimport
}

func (txn *Txn) Commit() error  { return nil }  // host 侧自动提交
func (txn *Txn) Rollback() error { return nil }
```

---

## 4. 合约代码：零修改

以 `contracts/batch/batch.go` 为例：

```go
func (b BatchMutation) ExecuteBatch(calls []BatchCall) error {
    for _, call := range calls {
        // v0 原生：b.api.Call() -> native Query/Mutation -> pebble.Batch
        // v1 WASM：b.api.Call() -> wasm Engine -> host functions -> pebble.Batch
        // 
        // 合约自身代码完全不知道运行在哪种环境！
        _, err := b.api.Call(b.api.GetCaller(), call.ToContractCall())
        if err != nil {
            return err
        }
    }
    return nil
}
```

**关键**：`b.api.Call()` 在 WASM 环境下会递归回到 `wasm.Engine.ExecuteMutation`，后者创建新的 host linker，继续通过 `storage_get/set` 访问同一 `model.Txn`。

---

## 5. 上下文传递（height, caller, version...）

不需要为每个上下文字段暴露 host function。在 WASM 合约实例化时，一次性写入线性内存：

```
WASM 线性内存布局（启动时由 host 写入）：

[0x0000..0x0007]  int64  height
[0x0008..0x000B]  int32  version
[0x000C..0x0013]  int64  block_time
[0x0014..0x0033]  bytes  caller (UniAddr: type[4] + data[28])
[0x0034..0x0053]  bytes  system_account (同上)
```

WASM 合约内部直接读取这些固定地址：

```go
func GetHeight() int64 {
    return *(*int64)(unsafe.Pointer(uintptr(0)))
}

func GetVersion() uint32 {
    return *(*uint32)(unsafe.Pointer(uintptr(8)))
}
```

这比每次调用 `env.get_height()` 快得多，且不需要额外的 host function。

---

## 6. 跨合约调用（Query / Call）

跨合约调用需要递归执行。当前设计是：

```
WASM 合约 A
    ├── 调用 env.cross_contract_query(...)  ← host function
    │       └── host 侧：contracts.Query(version, call, runtime)
    │               ├── v0 → nativeQuery() → pebble
    │               └── v1 → wasm.Engine.ExecuteQuery() → 加载 WASM B
    └── WASM 合约 B 执行...
```

`cross_contract_query/call` 是必需的 host function（2 个），否则 WASM 合约无法调用其他合约。

**最终 host function 总数：4 个**

| # | Host Function | 必要性 |
|---|--------------|--------|
| 1 | `runtime.storage_get` | ✅ 必须 — 读写存储 |
| 2 | `runtime.storage_set` | ✅ 必须 — 读写存储 |
| 3 | `runtime.cross_contract_query` | ✅ 必须 — 跨合约查询 |
| 4 | `runtime.cross_contract_call` | ✅ 必须 — 跨合约调用 |

上下文（height, version, caller）通过**内存布局**传递，不需要 host functions。

---

## 7. 实施步骤（最小化版本）

### Step 1: 创建 `db_txn_wasm.go`（1 天）

```go
//go:build wasm
package model

//go:wasmimport runtime storage_get
func hostStorageGet(keyPtr, keyLen, valPtr, valLen int32) int32

//go:wasmimport runtime storage_set
func hostStorageSet(keyPtr, keyLen, valPtr, valLen int32) int32

type Txn struct{}

func (txn *Txn) Get(key []byte) ([]byte, error) {
    // 在 WASM 内存分配 key 和 val 缓冲区
    // 调用 hostStorageGet
}

func (txn *Txn) Set(key, value []byte) error { ... }
func (txn *Txn) Delete(key []byte) error { ... }
func (txn *Txn) Commit() error { return nil }
func (txn *Txn) Rollback() error { return nil }
```

### Step 2: WASM 上下文内存布局（1 天）

在 `wasm.Engine.ExecuteQuery/ExecuteMutation` 中，实例化 WASM 前写入上下文到线性内存：

```go
func writeContextToWasmMemory(mem *wasmtime.Memory, store *wasmtime.Store, api model.ContractApi) {
    data := mem.UnsafeData(store)
    // [0:8] height
    binary.LittleEndian.PutUint64(data[0:8], uint64(api.GetHeight()))
    // [8:12] version
    binary.LittleEndian.PutUint32(data[8:12], api.GetVersion())
    // ... caller, system_account
}
```

### Step 3: 编译验证（1-2 天）

```bash
CGO_ENABLED=0 GOOS=wasip1 GOARCH=wasm go build -tags=wasm ./contracts/...
```

### Step 4: 集成测试（1-2 天）

生成 `runtime_v1.wasm`，通过 `RuntimeUpgrade` 上链测试。

**总计：约 1 周**

---

## 8. 风险

| 风险 | 说明 | 缓解 |
|------|------|------|
| `ListByPrefix` 缺失 | `Txn.ListByPrefix` 需要迭代器，WASM host functions 只有单 key Get/Set | 新增 `storage_scan_prefix` host function（返回 key 列表） |
| 加密 `SealWithProductKey` | 依赖 ego（CGO） | WASM 模式下用纯 Go chacha20poly1305，或 host function 加密 |
| 内存布局硬编码 | WASM 合约依赖固定内存地址读取上下文 | 约定 ABI，编译时检查 |
| Go WASM 体积 | 2-3MB | `-ldflags="-s -w"` |
