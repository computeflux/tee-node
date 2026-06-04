// Package wasm provides the WASM contract execution engine.
// Uses wazero (pure-Go WASM runtime, zero CGO) to execute WASM contracts.
//
// Execution mode (Reactor): modules are compiled with -buildmode=c-shared, exporting _initialize.
// After InstantiateModule, the host calls execute(ptr, len) directly without _start.
//
// Package wasm 提供 WASM 合约执行引擎。
// 使用 wazero（纯 Go WASM 运行时，零 CGO 依赖）执行 WASM 合约。
// 执行模式（Reactor）：模块使用 -buildmode=c-shared 编译，导出 _initialize。
// InstantiateModule 后宿主直接调用 execute(ptr, len)，无需 _start。
package wasm

import (
	"bytes"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"sync"

	"github.com/computeflux/tee-provider/pkg/model"
	"github.com/computeflux/tee-provider/pkg/util"
	"github.com/computeflux/tee-provider/tee-chain/contracts/store"
	"github.com/tetratelabs/wazero"
	wazapi "github.com/tetratelabs/wazero/api"
	"github.com/tetratelabs/wazero/imports/wasi_snapshot_preview1"
)

// NativeVersion is the version number for native (non-WASM) contract execution.
const NativeVersion = uint32(0)

// Engine is the WASM contract execution engine.
// It manages WASM runtime lifecycle, module compilation, caching, and invocation.
type Engine struct {
	rt        wazero.Runtime        // WASM runtime instance
	cachedVer uint32                // cached module version
	cachedMod wazero.CompiledModule // cached compiled module
	wasiOnce  sync.Once             // ensures WASI is instantiated only once
	hostOnce  sync.Once             // ensures host module is registered only once
	hostState hostState             // shared state passed to host functions
	mu        sync.Mutex            // protects concurrent calls
}

// hostState holds per-call mutable state shared with host functions.
// The api field is set before each invocation under the mutex lock.
type hostState struct {
	api store.ContractApi
}

// NewEngine creates a new WASM execution engine with interpreter mode.
func NewEngine() *Engine {
	rt := wazero.NewRuntimeWithConfig(context.Background(), wazero.NewRuntimeConfigInterpreter())
	return &Engine{rt: rt}
}

// ensureWASI instantiates WASI snapshot preview1 in the runtime (once).
func (e *Engine) ensureWASI(ctx context.Context) {
	e.wasiOnce.Do(func() { wasi_snapshot_preview1.MustInstantiate(ctx, e.rt) })
}

// ensureHost registers the "runtime" host module in the runtime (once).
// All host functions (storage, context, identity) are registered here.
func (e *Engine) ensureHost(ctx context.Context) {
	e.hostOnce.Do(func() {
		instantiateHostModule(ctx, e.rt, &e.hostState)
	})
}

// LoadModule loads a compiled WASM module for the given version from DB,
// decompressing gzip data if needed. Results are cached by version.
func (e *Engine) LoadModule(version uint32) (wazero.CompiledModule, error) {
	if e.cachedMod != nil && e.cachedVer == version {
		return e.cachedMod, nil
	}

	info, err := model.GetJson[model.RuntimeVersionInfo]("", "RUNTIME_META_"+fmt.Sprint(version))
	if err != nil {
		util.LogError("wasm.LoadModule", fmt.Errorf("get runtime meta v%d: %w", version, err))
		return nil, fmt.Errorf("get runtime meta: %w", err)
	}
	if info == nil {
		util.LogError("wasm.LoadModule", fmt.Errorf("runtime version %d not found", version))
		return nil, fmt.Errorf("runtime version %d not found", version)
	}

	wasmBytes, err := decompressWasm(info.WasmBytes)
	if err != nil {
		util.LogError("wasm.LoadModule", fmt.Errorf("decompress v%d: %w", version, err))
		return nil, fmt.Errorf("decompress wasm v%d: %w", version, err)
	}

	ctx := context.Background()
	mod, err := e.rt.CompileModule(ctx, wasmBytes)
	if err != nil {
		util.LogError("wasm.LoadModule", fmt.Errorf("compile v%d: %w", version, err))
		return nil, fmt.Errorf("compile wasm module v%d: %w", version, err)
	}
	e.cachedVer = version
	e.cachedMod = mod
	return mod, nil
}

// decompressWasm decompresses gzip-compressed WASM bytes.
// Returns the original data if it is not gzip-compressed.
func decompressWasm(data []byte) ([]byte, error) {
	if len(data) >= 2 && data[0] == 0x1f && data[1] == 0x8b {
		r, err := gzip.NewReader(bytes.NewReader(data))
		if err != nil {
			return nil, err
		}
		defer r.Close()
		return io.ReadAll(r)
	}
	return data, nil
}

// CompileWasm compiles raw WASM bytes into a CompiledModule. For testing.
func (e *Engine) CompileWasm(wasmBytes []byte) (wazero.CompiledModule, error) {
	return e.rt.CompileModule(context.Background(), wasmBytes)
}

// SetCache sets the cached module for the given version. For testing.
func (e *Engine) SetCache(version uint32, mod wazero.CompiledModule) {
	e.cachedVer = version
	e.cachedMod = mod
}

// ExecuteQuery executes a read-only contract call via WASM.
func (e *Engine) ExecuteQuery(version uint32, call store.ContractCall, api store.ContractApi) ([]byte, error) {
	return e.executeWasmCall(version, call, api, true)
}

// ExecuteMutation executes a state-changing contract call via WASM.
// Returns an error if the WASM returns a non-empty result (treated as error message).
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

// executeWasmCall is the core WASM invocation path.
//
// Flow:
//  1. SCALE-encode the contract call into InContractCall
//  2. Instantiate the WASM module (calls _initialize for reactor init)
//  3. Write encoded call data into WASM linear memory
//  4. Call execute(isQuery, dataPtr, dataLen) → returns (resultPtr << 32 | resultLen)
//  5. Read result bytes from WASM memory at the returned pointer
func (e *Engine) executeWasmCall(version uint32, call store.ContractCall, api store.ContractApi, isQuery bool) ([]byte, error) {
	mod, err := e.LoadModule(version)
	if err != nil {
		return nil, err
	}

	ctx := context.Background()

	e.mu.Lock()
	defer e.mu.Unlock()

	e.hostState.api = api

	// Build InContractCall and SCALE-encode it
	inCall := &store.InContractCall{
		Name:   call.GetName(),
		Method: call.GetMethod(),
		Args:   call.GetArgs(),
	}
	callData, err := inCall.Encode()
	if err != nil {
		return nil, fmt.Errorf("wasm encode call: %w", err)
	}

	e.ensureHost(ctx)
	e.ensureWASI(ctx)

	config := wazero.NewModuleConfig()
	instance, err := e.rt.InstantiateModule(ctx, mod, config)
	if err != nil {
		return nil, fmt.Errorf("wasm instantiate: %w", err)
	}
	defer instance.Close(ctx)

	// Explicitly call _initialize for reactor-mode modules
	// _initialize 显式调用，初始化 Go runtime
	if initFn := instance.ExportedFunction("_initialize"); initFn != nil {
		if _, err := initFn.Call(ctx); err != nil {
			return nil, fmt.Errorf("wasm _initialize: %w", err)
		}
	}

	// Get the WASM-side buffer address for writing call data
	// 获取 WASM 端缓冲区地址
	bufPtr, err := getWasmBuffer(ctx, instance)
	if err != nil {
		return nil, fmt.Errorf("wasm get_buffer: %w", err)
	}

	// Write SCALE-encoded call data into WASM linear memory
	// 将 SCALE 编码写入 WASM 线性内存
	if !instance.Memory().Write(bufPtr, callData) {
		return nil, fmt.Errorf("wasm: failed to write call data to memory")
	}

	// Call execute(isQuery, dataPtr, dataLen)
	// Returns packed uint64: high 32 bits = result pointer, low 32 bits = result length
	// 调用 execute，返回打包 uint64: 高 32 位结果指针, 低 32 位结果长度
	var isQueryFlag uint64
	if isQuery {
		isQueryFlag = 1
	}
	results, err := instance.ExportedFunction("execute").Call(ctx, isQueryFlag, uint64(bufPtr), uint64(len(callData)))
	if err != nil {
		return nil, fmt.Errorf("wasm execute: %w", err)
	}

	// Unpack and read result from WASM memory
	// 解包并从 WASM 内存读取结果
	packed := results[0]
	if packed == 0 {
		return nil, nil
	}
	resultPtr := uint32(packed >> 32)
	resultLen := uint32(packed & 0xFFFFFFFF)
	result, ok := instance.Memory().Read(resultPtr, resultLen)
	if !ok {
		return nil, fmt.Errorf("wasm: failed to read result from memory")
	}
	return copyBytes(result), nil
}

// getWasmBuffer calls the WASM module's exported get_buffer function
// to obtain the address of the shared buffer in WASM linear memory.
func getWasmBuffer(ctx context.Context, mod wazapi.Module) (uint32, error) {
	fn := mod.ExportedFunction("get_buffer")
	if fn == nil {
		return 0, fmt.Errorf("get_buffer not exported")
	}
	results, err := fn.Call(ctx)
	if err != nil {
		return 0, err
	}
	if len(results) == 0 {
		return 0, fmt.Errorf("get_buffer returned no results")
	}
	return uint32(results[0]), nil
}
