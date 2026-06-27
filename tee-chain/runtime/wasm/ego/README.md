# `wasm/ego` — EGo SGX enclave 复现 engine.go 代码路径

按 EGo 官方 [Build your app](https://docs.edgeless.systems/ego/workflows/build) 工作流构建的最小 example：
调用 `tee-chain/runtime/wasm.Engine.ExecuteQuery` 触发 engine.go 的完整代码路径，
用于在 EGo SGX enclave（或普通 Go）里复现 engine.go 的 bug。

**不重新实现任何 wazero/SCALE 逻辑**，全部委托给 `wasm.Engine`，
这样 engine.go 一旦有 bug（比如原来的 `_initialize` SIGSEGV），这里会原样重现。

**使用 gzip 压缩版 wasm（runtime.wasm.gz）**：比未压缩版（约 547K）小一半（~208K），
适合链上传输。`main.go` 启动时自动检测 gzip magic 并解压，再喂给 `wasm.Engine`。

## 调用方法

```bash
-method runtime_version   # 默认
-method get_public_join
-method default_track
-method sudo
...
```

## 文件

- `main.go` — 唯一源文件，全部委托给 `wasm.Engine`
- `enclave.json` — EGo 配置；`files` 字段把 `runtime.wasm.gz` 嵌入 enclave 内存文件系统
- `test.sh` — 一键构建 & 运行脚本
- `runtime.wasm.gz` — 由 `test.sh` 从 `../runtime/runtime.wasm.gz` 拷贝而来

## 运行

### 1. 一键脚本（推荐）
```bash
cd tee-node/tee-chain/runtime/wasm/ego
./test.sh                  # 用现有 wasm
./test.sh --build          # 先重新编译 wasm 再继续
./test.sh --no-sim         # 关闭 OE_SIMULATION（需真实 SGX 硬件）
```

### 2. 手动步骤
```bash
cd tee-node
bash tee-chain/runtime/wasm/runtime/build.sh
cp tee-chain/runtime/wasm/runtime/runtime.wasm.gz tee-chain/runtime/wasm/ego/
ego-go build -o tee-chain/runtime/wasm/ego/ego ./tee-chain/runtime/wasm/ego/
ego sign tee-chain/runtime/wasm/ego/ego
OE_SIMULATION=1 ego run tee-chain/runtime/wasm/ego/ego
```

### 3. 普通 Go（开发/CI）
```bash
go run ./tee-chain/runtime/wasm/ego/                              # 默认 runtime_version
go run ./tee-chain/runtime/wasm/ego/ -method get_public_join      # 其他查询
```

预期输出：
```
=== ego: run wasm.Engine.ExecuteQuery ===
wasm:   tee-chain/runtime/wasm/runtime/runtime.wasm.gz
method: runtime_version
sgx:    not in SGX enclave (plain Go)
wasm size: 547896 bytes (uncompressed)
✓ result (37 bytes): "storeEncode: unsupported type *uint32"
=== test passed ===
```

## enclave.json 关键配置

```json
"files": [
  {
    "source": "./runtime.wasm.gz",
    "target": "runtime.wasm.gz"
  }
]
```

- **`ego sign`** 会把 `./runtime.wasm.gz`（gzip 压缩版，约 200K，比 raw 547K 小一半）嵌入到 enclave image（in-enclave memfs）
- 嵌入的 wasm **进入 SGX measurement**（MRENCLAVE），无法在 host 上篡改
- enclave 启动后 cwd 就是 `ego/`，运行时 `main.go` 会自动检测 gzip magic (0x1f 0x8b) 并解压，再喂给 `wasm.Engine.CompileWasm`
- 链上 `gov.RuntimeUpgrade` 上传时同理，引擎的 `LoadModule` 会自动解压

**构建前提**：`ego sign` 之前需先把 wasm.gz 拷到 `ego/runtime.wasm.gz`（`test.sh --build` 自动做）。

## 故障排查

| 现象 | 原因 | 解决 |
|------|------|------|
| `_initialize` SIGSEGV | wazero runtime config 用了 Compiler | 改回 `NewRuntimeConfigInterpreter()` |
| `RuntimeMeta ... not found` | LoadModule 找不到 gov runtime_meta_ | 用 `e.SetCache()` 跳过，或预填 storage |
| `module[runtime] not instantiated` | 缺 host import | 调用 `e.ExecuteQuery` 走 ensureHost 路径 |
| SGX 启动报 `OE_ENCLAVE_TYPE` 错误 | 没装 SGX 驱动 | 用 `OE_SIMULATION=1 ego run` 跑仿真 |
| `ego-go: command not found` | 没装 EGo | https://docs.edgeless.systems/ego/getting-started/install |
| `ego sign: file not found runtime.wasm.gz` | 没拷贝 wasm | `cp ../runtime/runtime.wasm.gz .` 或 `./test.sh --build` |