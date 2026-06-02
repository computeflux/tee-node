// Package wasm 提供 WASM 合约执行引擎。
// 使用 wasmtime-go 作为底层运行时，支持从数据库加载历史版本的 WASM 字节码并执行。
package wasm

import (
	"fmt"
	"sync"

	"github.com/bytecodealliance/wasmtime-go"
	"github.com/computeflux/tee-provider/pkg/model"
)

// NativeVersion 当前二进制内置的原生合约版本。
// 当执行版本号等于此值时，直接使用原生 Go 函数（快速路径）。
const NativeVersion = uint32(0)

// Engine WASM 执行引擎
type Engine struct {
	mu      sync.RWMutex
	engine  *wasmtime.Engine
	modules map[uint32]*wasmtime.Module // 版本 -> 已编译模块（缓存）
}

// NewEngine 创建新的 WASM 执行引擎。
func NewEngine() *Engine {
	return &Engine{
		engine:  wasmtime.NewEngine(),
		modules: make(map[uint32]*wasmtime.Module),
	}
}

// LoadModule 从数据库加载指定版本的 WASM 模块。
func (e *Engine) LoadModule(version uint32) (*wasmtime.Module, error) {
	e.mu.RLock()
	mod, ok := e.modules[version]
	e.mu.RUnlock()
	if ok {
		return mod, nil
	}

	// 从数据库加载 WASM 字节码
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

	// 编译 WASM 模块
	mod, err = wasmtime.NewModule(e.engine, info.WasmBytes)
	if err != nil {
		return nil, fmt.Errorf("compile wasm module v%d: %w", version, err)
	}

	e.mu.Lock()
	e.modules[version] = mod
	e.mu.Unlock()

	return mod, nil
}

// ExecuteQuery 执行 WASM 合约的查询操作。
// 目前为占位实现，需要定义 WASM 合约 ABI 后完善。
func (e *Engine) ExecuteQuery(version uint32, call *model.ContractCall, api model.ContractApi) ([]byte, error) {
	mod, err := e.LoadModule(version)
	if err != nil {
		return nil, err
	}

	store := wasmtime.NewStore(e.engine)
	// linker 不能缓存：每次调用都需要新的 api/txn 闭包
	linker := newLinker(e.engine, api)

	instance, err := linker.Instantiate(store, mod)
	if err != nil {
		return nil, fmt.Errorf("instantiate wasm v%d: %w", version, err)
	}

	// TODO: 根据 ABI 调用 WASM 导出函数
	_ = instance
	return nil, fmt.Errorf("WASM query not yet implemented for version %d", version)
}

// ExecuteMutation 执行 WASM 合约的状态变更操作。
// 目前为占位实现，需要定义 WASM 合约 ABI 后完善。
func (e *Engine) ExecuteMutation(version uint32, call *model.ContractCall, api model.ContractApi) error {
	mod, err := e.LoadModule(version)
	if err != nil {
		return err
	}

	store := wasmtime.NewStore(e.engine)
	// linker 不能缓存：每次调用都需要新的 api/txn 闭包
	linker := newLinker(e.engine, api)

	instance, err := linker.Instantiate(store, mod)
	if err != nil {
		return fmt.Errorf("instantiate wasm v%d: %w", version, err)
	}

	// TODO: 根据 ABI 调用 WASM 导出函数
	_ = instance
	return fmt.Errorf("WASM mutation not yet implemented for version %d", version)
}
