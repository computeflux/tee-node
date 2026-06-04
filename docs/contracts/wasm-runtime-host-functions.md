# WASM Runtime Host Functions 方案与实现

> 所有 host functions 统一使用 `runtime` 模块名，涵盖完整存储操作与上下文获取。  
> **注意**：本文档前半部分为设计阶段方案，实际落地实现见第 8 节及 `docs/minimal-wasm-host-functions.md`。

---

## 1. 设计原则

1. **单一模块**：所有 host functions 统一在 `runtime` 模块下，不分散在 `env` / `storage` 等子模块
2. **最小接口**：只暴露 `model.Txn` 已有的方法 + 必要上下文，不新增抽象层
3. **内存约定**：WASM 合约负责内存分配，host 只读写，不管理 WASM 内存生命周期
4. **错误码统一**：返回 i32，0=成功，负数=错误，正数=结果长度

---

## 2. 完整 Host Functions 列表

### 2.1 存储操作（Storage）

| # | 函数 | 签名 | 返回值 |
|---|------|------|--------|
| 1 | `runtime.storage_get` | `(key_ptr, key_len, val_ptr, val_len) -> i32` | 0=成功, -1=内存错误, -2=存储错误, -3=val_len不足, -4=key不存在 |
| 2 | `runtime.storage_set` | `(key_ptr, key_len, val_ptr, val_len) -> i32` | 0=成功, -1=内存错误, -2=存储错误 |
| 3 | `runtime.storage_delete` | `(key_ptr, key_len) -> i32` | 0=成功, -1=内存错误, -2=存储错误 |
| 4 | `runtime.storage_scan_prefix` | `(prefix_ptr, prefix_len, keys_ptr, keys_len, vals_ptr, vals_len) -> i32` | 0=成功, -1=内存错误, -2=存储错误, -3=缓冲区不足, >0=结果对数 |
| 5 | `runtime.storage_scan_prefix_limit` | `(prefix_ptr, prefix_len, offset, limit, keys_ptr, vals_ptr) -> i32` | 同上 |
| 6 | `runtime.storage_delete_prefix` | `(prefix_ptr, prefix_len) -> i32` | 0=成功, -1=内存错误, -2=存储错误 |

**说明**：
- `storage_get`：先查本地缓存，未命中通过 host function 查询，结果写入 `val_ptr`
- `storage_scan_prefix`：返回 `[(key1, val1), (key2, val2), ...]`，序列化后写入 `keys_ptr` 和 `vals_ptr`
  - 序列化格式：`[key1_len: u32][key1_data][key2_len: u32][key2_data]...`
- `storage_scan_prefix_limit`：支持 offset/limit 分页

### 2.2 区块上下文（Context）

| # | 函数 | 签名 | 返回值 |
|---|------|------|--------|
| 7 | `runtime.get_height` | `() -> i64` | 当前区块高度 |
| 8 | `runtime.get_block_time` | `() -> i64` | 当前区块时间戳（秒） |
| 9 | `runtime.get_version` | `() -> i32` | 当前运行时版本号 |

### 2.3 身份与权限（Identity）

| # | 函数 | 签名 | 返回值 |
|---|------|------|--------|
| 10 | `runtime.get_caller` | `(out_ptr, out_len) -> i32` | 0=成功, -1=内存错误, -3=out_len不足 |
| 11 | `runtime.get_system_account` | `(out_ptr, out_len) -> i32` | 同上 |
| 12 | `runtime.is_system` | `() -> i32` | 0=false, 1=true |

**说明**：
- `get_caller` / `get_system_account` 返回 `UniAddr` 的序列化格式：`[type: u32][data_len: u32][data...]`
- `UniAddr` 最大长度：4(type) + 4(len) + 32(data) = 40 bytes

### 2.4 跨合约调用（Inter-Contract）

| # | 函数 | 签名 | 返回值 |
|---|------|------|--------|
| 13 | `runtime.cross_contract_query` | `(name_ptr, name_len, method, args_count, args_ptr, out_ptr, out_len) -> i32` | >=0=结果长度, -1=内存错误, -2=调用错误, -3=out_len不足 |
| 14 | `runtime.cross_contract_call` | `(name_ptr, name_len, method, args_count, args_ptr, out_ptr, out_len) -> i32` | 同上 |

**说明**：
- `method`：4 字节 selector，以 i32 传递
- `args_count`：参数数量
- `args_ptr`：指向 `[arg1_ptr: i32][arg1_len: i32][arg2_ptr: i32][arg2_len: i32]...` 数组
- host 侧构造 `model.ContractCall`，调用 `api.Query()` / `api.Call()`

### 2.5 日志（Logging）

| # | 函数 | 签名 | 返回值 |
|---|------|------|--------|
| 15 | `runtime.log` | `(msg_ptr, msg_len) -> ()` | 无 |

---

## 3. WASM 合约侧调用约定（Rust 示例）

```rust
#[link(wasm_import_module = "runtime")]
extern "C" {
    fn storage_get(key_ptr: i32, key_len: i32, val_ptr: i32, val_len: i32) -> i32;
    fn storage_set(key_ptr: i32, key_len: i32, val_ptr: i32, val_len: i32) -> i32;
    fn storage_delete(key_ptr: i32, key_len: i32) -> i32;
    fn storage_scan_prefix(
        prefix_ptr: i32, prefix_len: i32,
        keys_ptr: i32, keys_len: i32,
        vals_ptr: i32, vals_len: i32
    ) -> i32;
    fn storage_scan_prefix_limit(
        prefix_ptr: i32, prefix_len: i32,
        offset: i64, limit: i64,
        keys_ptr: i32, vals_ptr: i32
    ) -> i32;
    fn storage_delete_prefix(prefix_ptr: i32, prefix_len: i32) -> i32;

    fn get_height() -> i64;
    fn get_block_time() -> i64;
    fn get_version() -> i32;

    fn get_caller(out_ptr: i32, out_len: i32) -> i32;
    fn get_system_account(out_ptr: i32, out_len: i32) -> i32;
    fn is_system() -> i32;

    fn cross_contract_query(
        name_ptr: i32, name_len: i32,
        method: i32,
        args_count: i32, args_ptr: i32,
        out_ptr: i32, out_len: i32
    ) -> i32;
    fn cross_contract_call(
        name_ptr: i32, name_len: i32,
        method: i32,
        args_count: i32, args_ptr: i32,
        out_ptr: i32, out_len: i32
    ) -> i32;

    fn log(msg_ptr: i32, msg_len: i32);
}

// 包装函数
pub fn storage_get(key: &[u8], val_buf: &mut [u8]) -> Result<(), i32> {
    let ret = unsafe {
        storage_get(key.as_ptr() as i32, key.len() as i32,
                    val_buf.as_mut_ptr() as i32, val_buf.len() as i32)
    };
    if ret == 0 { Ok(()) }
    else if ret == -4 { Err(-4) }  // not found
    else { Err(ret) }
}

pub fn get_caller() -> UniAddr {
    let mut buf = [0u8; 40];
    unsafe { get_caller(buf.as_mut_ptr() as i32, 40); }
    UniAddr::decode(&buf)
}
```

---

## 4. Go 合约侧：零修改的实现方式

### 4.1 核心机制

Go 合约代码**不修改**。通过 `pkg/model` 的 build tags 切换底层实现：

```
pkg/model/
├── db_txn.go          (!wasm) — *pebble.Batch
├── db_txn_wasm.go     (wasm)  — 调用 runtime.* host functions
```

### 4.2 WASM 模式下 Txn 的实现

```go
//go:build wasm
package model

//go:wasmimport runtime storage_get
func hostStorageGet(keyPtr, keyLen, valPtr, valLen int32) int32

//go:wasmimport runtime storage_set
func hostStorageSet(keyPtr, keyLen, valPtr, valLen int32) int32

//go:wasmimport runtime storage_delete
func hostStorageDelete(keyPtr, keyLen int32) int32

// ... 其他 host function 声明

type Txn struct {
    // WASM 模式下不需要 pebble.Batch
    // 每次 Get/Set/Delete 直接调用 host function
}

func (txn *Txn) Get(key []byte) ([]byte, error) {
    // WASM 线性内存由 Go 运行时管理
    // Go WASM 编译器自动处理指针到 WASM 内存的映射
    return hostStorageGet(key, ...) // 具体实现由 Go WASM 运行时处理
}

func (txn *Txn) Set(key, value []byte) error {
    return hostStorageSet(key, value, ...)
}

func (txn *Txn) Delete(key []byte) error {
    return hostStorageDelete(key, ...)
}

func (txn *Txn) ListByPrefix(prefix []byte) ([][]byte, [][]byte, error) {
    return hostStorageScanPrefix(prefix, ...)
}

func (txn *Txn) Commit() error  { return nil }
func (txn *Txn) Rollback() error { return nil }
```

### 4.3 关键问题：Go WASM 内存映射

Go 编译为 WASM（`GOOS=wasip1 GOARCH=wasm`）时：
- Go 运行时自带线性内存管理
- Go 的 `[]byte` 指针**就是 WASM 线性内存内的地址**
- `//go:wasmimport` 声明的函数，Go 编译器自动处理参数传递

因此 `hostStorageGet(key, ...)` 中，`key` 的指针就是有效的 WASM 内存地址，host 侧可以直接读取。

---

## 5. Host 侧实现（Go）

```go
// tee-chain/contracts/wasm/host.go

func registerHostFunctions(linker *wasmtime.Linker, api model.ContractApi) {
    txn := api.GetTxn()

    // storage_get
    linker.FuncNew("runtime", "storage_get", ...,
        func(caller *wasmtime.Caller, args []wasmtime.Val) ([]wasmtime.Val, *wasmtime.Trap) {
            key := readWasmMemory(caller, args[0].I32(), args[1].I32())
            val, err := txn.Get(key)
            if err != nil { return []wasmtime.Val{wasmtime.ValI32(-4)}, nil }
            if writeWasmMemory(caller, args[2].I32(), val) != 0 {
                return []wasmtime.Val{wasmtime.ValI32(-3)}, nil
            }
            return []wasmtime.Val{wasmtime.ValI32(0)}, nil
        })

    // storage_set
    linker.FuncNew("runtime", "storage_set", ...,
        func(caller *wasmtime.Caller, args []wasmtime.Val) ([]wasmtime.Val, *wasmtime.Trap) {
            key := readWasmMemory(caller, args[0].I32(), args[1].I32())
            val := readWasmMemory(caller, args[2].I32(), args[3].I32())
            if err := txn.Set(key, val); err != nil {
                return []wasmtime.Val{wasmtime.ValI32(-2)}, nil
            }
            return []wasmtime.Val{wasmtime.ValI32(0)}, nil
        })

    // storage_scan_prefix
    linker.FuncNew("runtime", "storage_scan_prefix", ...,
        func(caller *wasmtime.Caller, args []wasmtime.Val) ([]wasmtime.Val, *wasmtime.Trap) {
            prefix := readWasmMemory(caller, args[0].I32(), args[1].I32())
            keys, vals, err := txn.ListByPrefix(prefix)
            if err != nil { return []wasmtime.Val{wasmtime.ValI32(-2)}, nil }
            // 序列化 keys/vals 写入 WASM 内存
            // ...
            return []wasmtime.Val{wasmtime.ValI32(int32(len(keys)))}, nil
        })

    // get_height, get_version, get_caller, is_system, cross_contract_query, log ...
    // 同上
}
```

---

## 6. 实施步骤（历史计划）

| 步骤 | 内容 | 工作量 |
|------|------|--------|
| 1 | 创建 `pkg/model/db_txn_wasm.go`，实现 `Txn` 的 host function 调用 | 2 天 |
| 2 | 更新 `wasm/host.go`，统一使用 `runtime` 模块名，补全所有函数 | 2 天 |
| 3 | 编译 `contracts/batch` 为 WASM，验证可行性 | 1-2 天 |
| 4 | 编译 `contracts/auth/gateway/gov` 为 WASM | 3-4 天 |
| 5 | 集成测试：RuntimeUpgrade 上链，v0/v1 切换 | 2-3 天 |

**总计：约 2 周**

---

## 7. 决策确认（历史）

1. ✅ Host functions 模块名统一为 `runtime`
2. ✅ 包含完整存储操作：Get/Set/Delete/ScanPrefix/DeletePrefix（`ScanPrefixLimit` 未实现）
3. ❌ 未采用 build tags 切换 `Txn`；改为 **宿主-沙箱分离** 架构（wazero + TinyGo WASM runtime）
4. ✅ WASM 体积通过 TinyGo 优化至数百 KB
5. ✅ 加密 `SealWithProductKey` 仅在宿主侧执行，WASM 合约不涉及


---

## 8. 实际落地实现（2026-06-04 更新）

### 8.1 关键变更

| 项目 | 设计阶段 | 实际实现 |
|------|---------|----------|
| WASM 引擎 | wasmtime-go | **wazero**（纯 Go，零 CGO） |
| 编译器 | Go `GOOS=wasip1` | **TinyGo**（`-target=wasi`） |
| 上下文传递 | 内存布局硬编码 | **host functions**（`get_height` 等） |
| 跨合约调用 | `cross_contract_query/call` host functions | **宿主侧 `runtime.go` 直接递归调用** |
| 日志 | `runtime.log` host function | **未暴露** |
| `ScanPrefixLimit` | 规划中 | **未实现** |

### 8.2 实际 Host Functions 列表

详见 `docs/minimal-wasm-host-functions.md`，核心 11 个函数：

- `runtime.storage_get` / `storage_set` / `storage_delete`
- `runtime.storage_scan_prefix` / `storage_delete_prefix`
- `runtime.get_height` / `get_block_time` / `get_version`
- `runtime.get_caller` / `get_system_account` / `is_system`

### 8.3 核心文件

| 文件 | 职责 |
|------|------|
| `tee-chain/contracts/wasm/engine.go` | wazero 引擎管理、WASM 加载/缓存、ExecuteQuery/ExecuteMutation |
| `tee-chain/contracts/wasm/host.go` | wazero host module 注册（`runtime` 模块，11 个 host functions） |
| `tee-chain/contracts/wasm/runtime/main.go` | TinyGo WASM 入口，`execute(isQuery, ptr, len)` |
| `tee-chain/contracts/wasm/runtime/host.go` | WASM 侧 host import 声明 + `WasmTxn` 实现 `store.StoreTx` |

### 8.4 编译验证

```bash
# TinyGo 编译 WASM 运行时
cd tee-chain/contracts/wasm/runtime
tinygo build -target=wasi -buildmode=c-shared -o runtime.wasm main.go
gzip -k runtime.wasm

# 宿主侧测试
go test ./tee-chain/contracts/wasm/...
```

---

*文档更新：2026-06-04*
