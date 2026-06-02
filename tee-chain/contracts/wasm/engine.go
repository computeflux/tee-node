// Package wasm 提供 WASM 合约执行引擎。
// 使用 wazero（纯 Go WASM 运行时，零 CGO 依赖）执行 WASM 合约。
//
// 调用协议：
//
//	Engine 将 ContractCall 序列化为 call data，注入 host functions。
//	WASM 合约通过 runtime.get_call_data 获取输入，执行后通过 runtime.set_result 返回结果。
//	Engine 调用 WASM 的 execute() 导出函数触发执行。
package wasm

import (
	"context"
	"fmt"
	"sync"

	"github.com/computeflux/tee-provider/pkg/model"
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
}

func NewEngine() *Engine {
	return &Engine{rt: wazero.NewRuntime(context.Background())}
}

func (e *Engine) ensureWASI(ctx context.Context) {
	e.wasiOnce.Do(func() { wasi_snapshot_preview1.MustInstantiate(ctx, e.rt) })
}

func (e *Engine) LoadModule(version uint32) (wazero.CompiledModule, error) {
	if e.cachedMod != nil && e.cachedVer == version {
		return e.cachedMod, nil
	}
	info, err := model.GetJson[model.RuntimeVersionInfo]("", "RUNTIME_META_"+fmt.Sprint(version))
	if err != nil {
		return nil, fmt.Errorf("get runtime meta: %w", err)
	}
	if info == nil {
		return nil, fmt.Errorf("runtime version %d not found", version)
	}
	if len(info.WasmBytes) == 0 {
		return nil, fmt.Errorf("runtime version %d has no wasm bytes", version)
	}
	ctx := context.Background()
	mod, err := e.rt.CompileModule(ctx, info.WasmBytes)
	if err != nil {
		return nil, fmt.Errorf("compile wasm module v%d: %w", version, err)
	}
	e.cachedVer = version
	e.cachedMod = mod
	return mod, nil
}

func (e *Engine) ExecuteQuery(version uint32, call store.ContractCall, api store.ContractApi) ([]byte, error) {
	return e.executeWasmCall(version, call, api)
}

func (e *Engine) ExecuteMutation(version uint32, call store.ContractCall, api store.ContractApi) error {
	_, err := e.executeWasmCall(version, call, api)
	return err
}

func (e *Engine) executeWasmCall(version uint32, call store.ContractCall, api store.ContractApi) ([]byte, error) {
	mod, err := e.LoadModule(version)
	if err != nil {
		return nil, err
	}

	ctx := context.Background()
	callData := serializeCallData(call)
	var result []byte

	hostMod, err := instantiateHostModule(ctx, e.rt, api, callData, &result)
	if err != nil {
		return nil, fmt.Errorf("instantiate host module: %w", err)
	}
	defer hostMod.Close(ctx)

	e.ensureWASI(ctx)

	config := wazero.NewModuleConfig()
	instance, err := e.rt.InstantiateModule(ctx, mod, config)
	if err != nil {
		return nil, fmt.Errorf("instantiate wasm v%d: %w", version, err)
	}
	defer instance.Close(ctx)

	execFn := instance.ExportedFunction("execute")
	if execFn == nil {
		return nil, fmt.Errorf("wasm v%d: missing 'execute' export", version)
	}

	_, err = execFn.Call(ctx)
	if err != nil {
		return nil, fmt.Errorf("wasm v%d execute: %w", version, err)
	}

	return result, nil
}
