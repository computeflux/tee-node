// Package wasm 提供 WASM 合约执行引擎。
// 使用 wazero（纯 Go WASM 运行时，零 CGO 依赖）执行 WASM 合约。
//
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

const NativeVersion = uint32(0)

type Engine struct {
	rt        wazero.Runtime
	cachedVer uint32
	cachedMod wazero.CompiledModule
	wasiOnce  sync.Once
	hostOnce  sync.Once
	hostState hostState
	mu        sync.Mutex
}

type hostState struct {
	api store.ContractApi
}

func NewEngine() *Engine {
	rt := wazero.NewRuntimeWithConfig(context.Background(), wazero.NewRuntimeConfigInterpreter())
	return &Engine{rt: rt}
}

func (e *Engine) ensureWASI(ctx context.Context) {
	e.wasiOnce.Do(func() { wasi_snapshot_preview1.MustInstantiate(ctx, e.rt) })
}

func (e *Engine) ensureHost(ctx context.Context) {
	e.hostOnce.Do(func() {
		instantiateHostModule(ctx, e.rt, &e.hostState)
	})
}

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

	e.hostState.api = api

	// 构造 InContractCall 并 SCALE 编码
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

	// 显式调用 _initialize（reactor 模块必须初始化 Go runtime）
	if initFn := instance.ExportedFunction("_initialize"); initFn != nil {
		if _, err := initFn.Call(ctx); err != nil {
			return nil, fmt.Errorf("wasm _initialize: %w", err)
		}
	}

	// 获取 WASM 内存缓冲区地址
	bufPtr, err := getWasmBuffer(ctx, instance)
	if err != nil {
		return nil, fmt.Errorf("wasm get_buffer: %w", err)
	}

	// 将 SCALE 编码的调用数据写入 WASM 线性内存
	if !instance.Memory().Write(bufPtr, callData) {
		return nil, fmt.Errorf("wasm: failed to write call data to memory")
	}

	// 直接调用导出的 execute(isQuery, ptr, len)，返回 (resultPtr << 32 | resultLen)
	var isQueryFlag uint64
	if isQuery {
		isQueryFlag = 1
	}
	results, err := instance.ExportedFunction("execute").Call(ctx, isQueryFlag, uint64(bufPtr), uint64(len(callData)))
	if err != nil {
		return nil, fmt.Errorf("wasm execute: %w", err)
	}

	// 解析封装的结果: 高 32 位 = 指针, 低 32 位 = 长度
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

// getWasmBuffer 调用 WASM 模块导出的 get_buffer 函数获取缓冲区地址。
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
