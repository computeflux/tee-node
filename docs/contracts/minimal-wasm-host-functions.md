# WASM Host Functions 实际实现

> 本文档描述 `tee-chain/contracts/wasm/` 中实际落地的 host functions 设计。  
> 最后更新：2026-06-04

---

## 1. 设计原则

1. **单一模块**：所有 host functions 注册在 `runtime` 模块下
2. **最小接口**：仅暴露存储操作 + 必要上下文，不暴露跨合约调用 host function（由宿主侧 `runtime.go` 直接处理）
3. **内存约定**：WASM 侧通过固定共享缓冲区（`txnBuf` 128KB）与宿主交换数据
4. **引擎**：宿主侧使用 **wazero**（纯 Go，零 CGO），而非 wasmtime

---

## 2. 实际 Host Functions 列表

### 2.1 存储操作（Storage）

| # | 函数 | 签名 | 返回值 |
|---|------|------|--------|
| 1 | `runtime.storage_get` | `(key_ptr, key_len, val_ptr, val_len) -> i32` | >0=值长度, 0=不存在, 负数=错误码 |
| 2 | `runtime.storage_set` | `(key_ptr, key_len, val_ptr, val_len) -> i32` | 0=成功, 负数=错误码 |
| 3 | `runtime.storage_delete` | `(key_ptr, key_len) -> i32` | 0=成功, 负数=错误码 |
| 4 | `runtime.storage_scan_prefix` | `(prefix_ptr, prefix_len, keys_ptr, vals_ptr) -> i32` | >=0=结果对数, 负数=错误码 |
| 5 | `runtime.storage_delete_prefix` | `(prefix_ptr, prefix_len) -> i32` | 0=成功, 负数=错误码 |

**说明**：
- `storage_scan_prefix` 结果通过 `encodeScanPairs` 序列化写入 WASM 内存：
  - 格式：`[count: u32LE][len: u32LE][data...]`（keys 和 vals 各一份）
- 宿主侧错误码：`errOK=0`, `errMem=0xFFFFFFFF`, `errStorage=0xFFFFFFFE`, `errBufTooSmall=0xFFFFFFFD`, `errKeyNotFound=0xFFFFFFFC`

### 2.2 区块上下文（Context）

| # | 函数 | 签名 | 返回值 |
|---|------|------|--------|
| 6 | `runtime.get_height` | `() -> i64` | 当前区块高度 |
| 7 | `runtime.get_block_time` | `() -> i64` | 当前区块时间戳（秒） |
| 8 | `runtime.get_version` | `() -> i32` | 当前运行时版本号 |

### 2.3 身份与权限（Identity）

| # | 函数 | 签名 | 返回值 |
|---|------|------|--------|
| 9 | `runtime.get_caller` | `(out_ptr, out_len) -> i32` | >0=编码后长度, 负数=错误码 |
| 10 | `runtime.get_system_account` | `(out_ptr, out_len) -> i32` | 同上 |
| 11 | `runtime.is_system` | `() -> i32` | 1=true, 0=false |

**说明**：
- `get_caller` / `get_system_account` 返回 SCALE 编码的 `UniAddr`

---

## 3. WASM 侧实现（TinyGo）

文件：`tee-chain/contracts/wasm/runtime/host.go`

```go
//go:build wasm

package main

//go:wasmimport runtime storage_get
func storageGet(keyPtr, keyLen, valPtr, valLen int32) int32

//go:wasmimport runtime storage_set
func storageSet(keyPtr, keyLen, valPtr, valLen int32) int32

//go:wasmimport runtime storage_delete
func storageDelete(keyPtr, keyLen int32) int32

//go:wasmimport runtime storage_scan_prefix
func storageScanPrefix(prefixPtr, prefixLen, keysPtr, valsPtr int32) int32

//go:wasmimport runtime storage_delete_prefix
func storageDeletePrefix(prefixPtr, prefixLen int32) int32

//go:wasmimport runtime get_block_time
func getBlockTime() int64

//go:wasmimport runtime get_version
func getVersion() int32

//go:wasmimport runtime get_caller
func getCaller(outPtr, outLen int32) int32

//go:wasmimport runtime get_system_account
func getSystemAccount(outPtr, outLen int32) int32

//go:wasmimport runtime is_system
func isSystem() int32
```

**共享缓冲区**：WASM 侧使用 128KB 固定缓冲区 `txnBuf`，前 64KB 用于 key/prefix，后 64KB 用于 value，避免每次调用在 WASM 内存中动态分配。

---

## 4. 宿主侧实现（wazero）

文件：`tee-chain/contracts/wasm/host.go`

使用 `wazero.HostModuleBuilder` 注册 `runtime` 模块：

```go
func instantiateHostModule(ctx context.Context, rt wazero.Runtime, s *hostState) (wazapi.Module, error) {
    b := rt.NewHostModuleBuilder("runtime")
    registerStorage(b, s)
    registerContext(b, s)
    registerIdentity(b, s)
    return b.Instantiate(ctx)
}
```

- `registerStorage` — `storage_get/set/delete/scan_prefix/delete_prefix`
- `registerContext` — `get_height/get_block_time/get_version`
- `registerIdentity` — `get_caller/get_system_account/is_system`

---

## 5. 与早期设计文档的差异

| 项目 | 早期设计 | 实际实现 |
|------|---------|----------|
| WASM 引擎 | wasmtime-go | **wazero**（纯 Go，零 CGO） |
| 编译器 | Go `GOOS=wasip1` | **TinyGo**（`-target=wasi`） |
| 上下文传递 | 内存布局硬编码 | **host functions**（`get_height` 等） |
| 跨合约调用 | `cross_contract_query/call` host functions | **宿主侧 `runtime.go` 直接递归调用**，WASM 侧不暴露 |
| 日志 | `runtime.log` host function | **未暴露**，WASM 侧无日志 host function |
| `storage_scan_prefix_limit` | 规划中 | **未实现**，当前仅 `storage_scan_prefix` |

---

## 6. 编译验证

```bash
# 编译 WASM 运行时（hack/build_runtime_wasm.sh）
cd tee-chain/contracts/wasm/runtime
tinygo build -target=wasi -buildmode=c-shared -o runtime.wasm main.go
gzip -k runtime.wasm

# 宿主侧测试
go test ./tee-chain/contracts/wasm/...
```

---

*文档更新：2026-06-04*
