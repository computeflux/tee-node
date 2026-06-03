// Package wasm 提供 WASM 合约执行引擎。
// 使用 wazero（纯 Go WASM 运行时，零 CGO 依赖）执行 WASM 合约。
//
// 执行模式（Command）：模块导出 _start，InstantiateModule 时自动执行。
// main() → execute() → set_result → exit(0) → 模块关闭（每次调用独立实例化）。
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
	api      store.ContractApi
	callData []byte
	result   []byte
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
	e.hostState.callData = serializeCallData(call, isQuery)
	e.hostState.result = nil

	e.ensureHost(ctx)
	e.ensureWASI(ctx)

	config := wazero.NewModuleConfig()
	instance, _ := e.rt.InstantiateModule(ctx, mod, config)
	if instance != nil {
		instance.Close(ctx)
	}
	return e.hostState.result, nil
}
