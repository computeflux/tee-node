// Package wasm 提供 WASM 合约执行所需的 host functions。
// Host functions 是 WASM 合约可以调用的系统 API，包括存储读写、区块信息获取等。
//
// 关键设计：linker 不能缓存。每次执行 WASM 合约时都需要创建新的 linker，
// 因为 host functions 的闭包捕获了 api（包含当前区块的 txn、caller、height 等），
// 而每个区块/每次调用的 api 都是不同的实例。
package wasm

import (
	"fmt"

	"github.com/bytecodealliance/wasmtime-go"
	"github.com/computeflux/tee-provider/pkg/model"
)

// newLinker 创建新的 linker 并注册 host functions。
// 注意：每次调用 WASM 合约时都必须创建新的 linker，因为 api 实例每次不同。
func newLinker(engine *wasmtime.Engine, api model.ContractApi) *wasmtime.Linker {
	linker := wasmtime.NewLinker(engine)
	registerHostFunctions(linker, api)
	return linker
}

// readWasmMemory 从 WASM 线性内存读取指定范围的数据。
// WASM 合约调用 host function 时传入指针和长度，通过此函数读取。
// Caller 实现了 wasmtime.Storelike 接口，可直接传给 Memory.UnsafeData。
func readWasmMemory(caller *wasmtime.Caller, ptr, length int32) []byte {
	if length <= 0 {
		return nil
	}
	memExport := caller.GetExport("memory")
	if memExport == nil || memExport.Memory() == nil {
		return nil
	}
	mem := memExport.Memory()
	// Caller 实现 Storelike 接口（有 Context() 方法），可直接使用
	data := mem.UnsafeData(caller)
	if int(ptr)+int(length) > len(data) {
		return nil
	}
	buf := make([]byte, length)
	copy(buf, data[ptr:ptr+length])
	return buf
}

// writeWasmMemory 向 WASM 线性内存写入数据。
func writeWasmMemory(caller *wasmtime.Caller, ptr int32, data []byte) int32 {
	memExport := caller.GetExport("memory")
	if memExport == nil || memExport.Memory() == nil {
		return -1
	}
	mem := memExport.Memory()
	// Caller 实现 Storelike 接口，可直接使用
	memData := mem.UnsafeData(caller)
	if int(ptr)+len(data) > len(memData) {
		return -1
	}
	copy(memData[ptr:], data)
	return 0
}

// registerHostFunctions 向 linker 注册所有 host functions。
// 这些函数供 WASM 合约调用，实现合约与链的交互。
func registerHostFunctions(linker *wasmtime.Linker, api model.ContractApi) {
	// ── 存储相关 ─────────────────────────────────────────────
	// storage_get(key_ptr, key_len, val_ptr, val_len) -> i32
	// 返回值: 0=成功, -1=内存错误, -2=存储错误, -3=值长度不足
	linker.FuncNew("env", "storage_get",
		wasmtime.NewFuncType([]*wasmtime.ValType{
			wasmtime.NewValType(wasmtime.KindI32), // key_ptr
			wasmtime.NewValType(wasmtime.KindI32), // key_len
			wasmtime.NewValType(wasmtime.KindI32), // val_ptr
			wasmtime.NewValType(wasmtime.KindI32), // val_len
		}, []*wasmtime.ValType{wasmtime.NewValType(wasmtime.KindI32)}),
		func(caller *wasmtime.Caller, args []wasmtime.Val) ([]wasmtime.Val, *wasmtime.Trap) {
			key := readWasmMemory(caller, args[0].I32(), args[1].I32())
			if key == nil {
				return []wasmtime.Val{wasmtime.ValI32(-1)}, nil
			}

			txn := api.GetTxn()
			if txn == nil {
				return []wasmtime.Val{wasmtime.ValI32(-2)}, nil
			}

			val, err := txn.Get(key)
			if err != nil {
				// 未找到视为空值
				return []wasmtime.Val{wasmtime.ValI32(0)}, nil
			}

			valLen := args[3].I32()
			if valLen < int32(len(val)) {
				return []wasmtime.Val{wasmtime.ValI32(-3)}, nil // 缓冲区太小
			}

			if writeWasmMemory(caller, args[2].I32(), val) != 0 {
				return []wasmtime.Val{wasmtime.ValI32(-1)}, nil
			}
			return []wasmtime.Val{wasmtime.ValI32(0)}, nil
		},
	)

	// storage_set(key_ptr, key_len, val_ptr, val_len) -> i32
	// 返回值: 0=成功, -1=内存错误, -2=存储错误
	linker.FuncNew("env", "storage_set",
		wasmtime.NewFuncType([]*wasmtime.ValType{
			wasmtime.NewValType(wasmtime.KindI32), // key_ptr
			wasmtime.NewValType(wasmtime.KindI32), // key_len
			wasmtime.NewValType(wasmtime.KindI32), // val_ptr
			wasmtime.NewValType(wasmtime.KindI32), // val_len
		}, []*wasmtime.ValType{wasmtime.NewValType(wasmtime.KindI32)}),
		func(caller *wasmtime.Caller, args []wasmtime.Val) ([]wasmtime.Val, *wasmtime.Trap) {
			key := readWasmMemory(caller, args[0].I32(), args[1].I32())
			if key == nil {
				return []wasmtime.Val{wasmtime.ValI32(-1)}, nil
			}

			val := readWasmMemory(caller, args[2].I32(), args[3].I32())
			if val == nil {
				return []wasmtime.Val{wasmtime.ValI32(-1)}, nil
			}

			txn := api.GetTxn()
			if txn == nil {
				return []wasmtime.Val{wasmtime.ValI32(-2)}, nil
			}

			if err := txn.Set(key, val); err != nil {
				return []wasmtime.Val{wasmtime.ValI32(-2)}, nil
			}
			return []wasmtime.Val{wasmtime.ValI32(0)}, nil
		},
	)

	// ── 区块信息 ─────────────────────────────────────────────
	linker.FuncNew("env", "get_height",
		wasmtime.NewFuncType([]*wasmtime.ValType{}, []*wasmtime.ValType{wasmtime.NewValType(wasmtime.KindI64)}),
		func(caller *wasmtime.Caller, args []wasmtime.Val) ([]wasmtime.Val, *wasmtime.Trap) {
			return []wasmtime.Val{wasmtime.ValI64(api.GetHeight())}, nil
		},
	)

	linker.FuncNew("env", "get_block_time",
		wasmtime.NewFuncType([]*wasmtime.ValType{}, []*wasmtime.ValType{wasmtime.NewValType(wasmtime.KindI64)}),
		func(caller *wasmtime.Caller, args []wasmtime.Val) ([]wasmtime.Val, *wasmtime.Trap) {
			return []wasmtime.Val{wasmtime.ValI64(api.GetBlockTime())}, nil
		},
	)

	linker.FuncNew("env", "get_version",
		wasmtime.NewFuncType([]*wasmtime.ValType{}, []*wasmtime.ValType{wasmtime.NewValType(wasmtime.KindI32)}),
		func(caller *wasmtime.Caller, args []wasmtime.Val) ([]wasmtime.Val, *wasmtime.Trap) {
			return []wasmtime.Val{wasmtime.ValI32(int32(api.GetVersion()))}, nil
		},
	)

	// ── Caller 信息 ──────────────────────────────────────────
	// get_caller(out_ptr, out_len) -> i32
	linker.FuncNew("env", "get_caller",
		wasmtime.NewFuncType([]*wasmtime.ValType{
			wasmtime.NewValType(wasmtime.KindI32), // out_ptr
			wasmtime.NewValType(wasmtime.KindI32), // out_len
		}, []*wasmtime.ValType{wasmtime.NewValType(wasmtime.KindI32)}),
		func(caller *wasmtime.Caller, args []wasmtime.Val) ([]wasmtime.Val, *wasmtime.Trap) {
			// TODO: 将 caller 地址写入内存
			return []wasmtime.Val{wasmtime.ValI32(-1)}, nil
		},
	)

	linker.FuncNew("env", "is_system",
		wasmtime.NewFuncType([]*wasmtime.ValType{}, []*wasmtime.ValType{wasmtime.NewValType(wasmtime.KindI32)}),
		func(caller *wasmtime.Caller, args []wasmtime.Val) ([]wasmtime.Val, *wasmtime.Trap) {
			var result int32 = 0
			if api.CallbySystem() {
				result = 1
			}
			return []wasmtime.Val{wasmtime.ValI32(result)}, nil
		},
	)

	// ── 日志 ────────────────────────────────────────────────
	linker.FuncNew("env", "log",
		wasmtime.NewFuncType([]*wasmtime.ValType{
			wasmtime.NewValType(wasmtime.KindI32), // msg_ptr
			wasmtime.NewValType(wasmtime.KindI32), // msg_len
		}, []*wasmtime.ValType{}),
		func(caller *wasmtime.Caller, args []wasmtime.Val) ([]wasmtime.Val, *wasmtime.Trap) {
			// TODO: 读取内存中的日志消息并打印
			fmt.Println("[WASM LOG] (not yet implemented)")
			return nil, nil
		},
	)
}
