// Package wasm 提供 WASM 合约执行引擎。
// 使用 wazero（纯 Go WASM 运行时，零 CGO 依赖）执行 WASM 合约。
//
// 调用协议：
//
//	Engine 将 ContractCall 序列化为 call data，注入 host functions。
//	WASM 合约通过 runtime.get_call_data 获取输入，执行后通过 runtime.set_result 返回结果。
//
// 执行模式（Command）：
//
//	模块导出 _start，InstantiateModule 时自动执行。
//	main() 内部调用 execute()，execute() 通过 set_result 写回结果。
//	main() 返回后 runtime 调用 proc_exit 关闭模块（每次调用独立实例化）。
package wasm

import (
	"bytes"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"sync"

	"github.com/computeflux/tee-provider/pkg/model"
	"github.com/computeflux/tee-provider/tee-chain/contracts/store"
	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/imports/wasi_snapshot_preview1"
)

const NativeVersion = uint32(0)

// hostState 在每次 WASM 调用前更新，host functions 通过指针读取/写入。
type hostState struct {
	api      store.ContractApi
	callData []byte
	result   []byte
}

type Engine struct {
	rt        wazero.Runtime
	cachedVer uint32
	cachedMod wazero.CompiledModule
	wasiOnce  sync.Once
	hostOnce  sync.Once
	hostState hostState
	mu        sync.Mutex // 保护 hostState 和 InstantiateModule 的并发访问
}

func NewEngine() *Engine {
	return &Engine{rt: wazero.NewRuntime(context.Background())}
}

func (e *Engine) ensureWASI(ctx context.Context) {
	e.wasiOnce.Do(func() { wasi_snapshot_preview1.MustInstantiate(ctx, e.rt) })
}

func (e *Engine) ensureHost(ctx context.Context) {
	e.hostOnce.Do(func() {
		instantiateHostModule(ctx, e.rt, &e.hostState)
	})
}

// LoadModule 加载并编译 WASM 模块。先从 DB 读取 RAW 字节，检测是否 gzip 压缩，自动解压后编译为 wazero CompiledModule 并缓存。
func (e *Engine) LoadModule(version uint32) (wazero.CompiledModule, error) {
	if e.cachedMod != nil && e.cachedVer == version {
		return e.cachedMod, nil
	}

	key := "RUNTIME_META_" + fmt.Sprint(version)
	raw, err := model.Get(key)
	if err != nil {
		return nil, fmt.Errorf("get runtime meta %s: %w", key, err)
	}
	if len(raw) == 0 {
		return nil, fmt.Errorf("runtime version %d not found", version)
	}

	// 自动检测 gzip 压缩并解压
	wasmBytes, err := decompressWasm(raw)
	if err != nil {
		return nil, fmt.Errorf("decompress wasm v%d: %w", version, err)
	}
	if len(wasmBytes) == 0 {
		return nil, fmt.Errorf("runtime version %d has no wasm bytes", version)
	}

	ctx := context.Background()
	mod, err := e.rt.CompileModule(ctx, wasmBytes)
	if err != nil {
		return nil, fmt.Errorf("compile wasm module v%d: %w", version, err)
	}
	e.cachedVer = version
	e.cachedMod = mod
	return mod, nil
}

// decompressWasm 自动检测 gzip magic bytes 并解压；非 gzip 数据原样返回。
func decompressWasm(data []byte) ([]byte, error) {
	if len(data) >= 2 && data[0] == 0x1f && data[1] == 0x8b {
		r, err := gzip.NewReader(bytes.NewReader(data))
		if err != nil {
			return nil, fmt.Errorf("gzip reader: %w", err)
		}
		defer r.Close()
		b, err := io.ReadAll(r)
		if err != nil {
			return nil, fmt.Errorf("gzip read: %w", err)
		}
		return b, nil
	}
	return data, nil
}

func (e *Engine) ExecuteQuery(version uint32, call store.ContractCall, api store.ContractApi) ([]byte, error) {
	return e.executeWasmCall(version, call, api, true)
}

func (e *Engine) ExecuteMutation(version uint32, call store.ContractCall, api store.ContractApi) error {
	result, err := e.executeWasmCall(version, call, api, false)
	if err != nil {
		return err
	}
	if len(result) > 0 {
		return fmt.Errorf("wasm v%d mutation: %s", version, string(result))
	}
	return nil
}

func (e *Engine) executeWasmCall(version uint32, call store.ContractCall, api store.ContractApi, isQuery bool) ([]byte, error) {
	mod, err := e.LoadModule(version)
	if err != nil {
		return nil, err
	}

	ctx := context.Background()

	e.mu.Lock()
	defer e.mu.Unlock()

	// 更新共享状态（host functions 通过指针读写）
	e.hostState.api = api
	e.hostState.callData = serializeCallData(call, isQuery)
	e.hostState.result = nil

	// 注册 host module 和 WASI（均为 singleton）
	e.ensureHost(ctx)
	e.ensureWASI(ctx)

	// Command 模式：_start 自动执行 → main() → execute() → set_result → exit(0) → proc_exit 关闭模块
	config := wazero.NewModuleConfig()
	instance, _ := e.rt.InstantiateModule(ctx, mod, config)
	if instance != nil {
		instance.Close(ctx)
	}
	return e.hostState.result, nil
}
