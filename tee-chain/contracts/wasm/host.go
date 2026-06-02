// Package wasm 提供 WASM 合约执行所需的 host functions。
// 使用 wazero (v1.12+) HostModuleBuilder，注册到独立 "runtime" 模块。
package wasm

import (
	"github.com/computeflux/tee-provider/tee-chain/contracts/store"

	"context"
	"encoding/binary"
	"fmt"

	"github.com/tetratelabs/wazero"
	wazapi "github.com/tetratelabs/wazero/api"
)

var i32, i64 = wazapi.ValueTypeI32, wazapi.ValueTypeI64

func readU32At(mem wazapi.Memory, off uint32) uint32 {
	b, _ := mem.Read(off, 4)
	return uint32(b[0]) | uint32(b[1])<<8 | uint32(b[2])<<16 | uint32(b[3])<<24
}

const (
	errOK          = uint64(0)
	errMem         = uint64(0xFFFFFFFF)
	errStorage     = uint64(0xFFFFFFFE)
	errBufTooSmall = uint64(0xFFFFFFFD)
	errKeyNotFound = uint64(0xFFFFFFFC)
)

// ── 序列化 ────────────────────────────────────────────────

func serializeCallData(call store.ContractCall) []byte {
	size := 4 + len(call.GetName()) + 4 + 4
	for _, a := range call.GetArgs() {
		size += 4 + len(a)
	}
	buf := make([]byte, 0, size)
	buf = binary.LittleEndian.AppendUint32(buf, uint32(len(call.GetName())))
	buf = append(buf, call.GetName()...)
	buf = append(buf, call.GetMethod()...)
	buf = binary.LittleEndian.AppendUint32(buf, uint32(len(call.GetArgs())))
	for _, a := range call.GetArgs() {
		buf = binary.LittleEndian.AppendUint32(buf, uint32(len(a)))
		buf = append(buf, a...)
	}
	return buf
}

func encodeScanPairs(mem wazapi.Memory, keysPtr, valsPtr uint32, keys, vals [][]byte) uint32 {
	buf := make([]byte, 0, 1024)
	buf = binary.LittleEndian.AppendUint32(buf, uint32(len(keys)))
	for _, k := range keys {
		buf = binary.LittleEndian.AppendUint32(buf, uint32(len(k)))
		buf = append(buf, k...)
	}
	mem.Write(keysPtr, buf)

	buf = buf[:0]
	buf = binary.LittleEndian.AppendUint32(buf, uint32(len(vals)))
	for _, v := range vals {
		buf = binary.LittleEndian.AppendUint32(buf, uint32(len(v)))
		buf = append(buf, v...)
	}
	mem.Write(valsPtr, buf)

	return uint32(len(keys))
}

// ── Host Module ───────────────────────────────────────────

func instantiateHostModule(ctx context.Context, rt wazero.Runtime, api store.ContractApi, callData []byte, result *[]byte) (wazapi.Module, error) {
	b := rt.NewHostModuleBuilder("runtime")
	txn := api.GetTxn()

	if txn != nil {
		registerStorage(b, txn)
	}
	registerContext(b, api)
	registerIdentity(b, api)
	registerCallIO(b, api, callData, result)
	registerLog(b)

	return b.Instantiate(ctx)
}

// ── 存储 ─────────────────────────────────────────────────

func registerStorage(b wazero.HostModuleBuilder, txn store.StoreTx) {
	b.NewFunctionBuilder().WithGoModuleFunction(
		wazapi.GoModuleFunc(func(_ context.Context, mod wazapi.Module, stack []uint64) {
			mem := mod.Memory()
			key, _ := mem.Read(uint32(stack[0]), uint32(stack[1]))
			if key == nil {
				stack[0] = errMem
				return
			}
			val, err := txn.Get(key)
			if err != nil || len(val) == 0 {
				stack[0] = errKeyNotFound
				return
			}
			if uint32(stack[3]) < uint32(len(val)) {
				stack[0] = errBufTooSmall
				return
			}
			mem.Write(uint32(stack[2]), val)
			stack[0] = errOK
		}),
		[]wazapi.ValueType{i32, i32, i32, i32}, []wazapi.ValueType{i32},
	).Export("storage_get")

	b.NewFunctionBuilder().WithGoModuleFunction(
		wazapi.GoModuleFunc(func(_ context.Context, mod wazapi.Module, stack []uint64) {
			mem := mod.Memory()
			key, _ := mem.Read(uint32(stack[0]), uint32(stack[1]))
			val, _ := mem.Read(uint32(stack[2]), uint32(stack[3]))
			if key == nil || val == nil {
				stack[0] = errMem
				return
			}
			if err := txn.Set(key, val); err != nil {
				stack[0] = errStorage
				return
			}
			stack[0] = errOK
		}),
		[]wazapi.ValueType{i32, i32, i32, i32}, []wazapi.ValueType{i32},
	).Export("storage_set")

	b.NewFunctionBuilder().WithGoModuleFunction(
		wazapi.GoModuleFunc(func(_ context.Context, mod wazapi.Module, stack []uint64) {
			mem := mod.Memory()
			key, _ := mem.Read(uint32(stack[0]), uint32(stack[1]))
			if key == nil {
				stack[0] = errMem
				return
			}
			if err := txn.Delete(key); err != nil {
				stack[0] = errStorage
				return
			}
			stack[0] = errOK
		}),
		[]wazapi.ValueType{i32, i32}, []wazapi.ValueType{i32},
	).Export("storage_delete")

	b.NewFunctionBuilder().WithGoModuleFunction(
		wazapi.GoModuleFunc(func(_ context.Context, mod wazapi.Module, stack []uint64) {
			mem := mod.Memory()
			prefix, _ := mem.Read(uint32(stack[0]), uint32(stack[1]))
			if prefix == nil {
				stack[0] = errMem
				return
			}
			keys, vals, err := txn.ListByPrefix(prefix)
			if err != nil {
				stack[0] = errStorage
				return
			}
			n := encodeScanPairs(mem, uint32(stack[2]), uint32(stack[3]), keys, vals)
			stack[0] = uint64(n)
		}),
		[]wazapi.ValueType{i32, i32, i32, i32}, []wazapi.ValueType{i32},
	).Export("storage_scan_prefix")

	b.NewFunctionBuilder().WithGoModuleFunction(
		wazapi.GoModuleFunc(func(_ context.Context, mod wazapi.Module, stack []uint64) {
			mem := mod.Memory()
			prefix, _ := mem.Read(uint32(stack[0]), uint32(stack[1]))
			if prefix == nil {
				stack[0] = errMem
				return
			}
			if err := txn.DeletekeysByPrefix(prefix); err != nil {
				stack[0] = errStorage
				return
			}
			stack[0] = errOK
		}),
		[]wazapi.ValueType{i32, i32}, []wazapi.ValueType{i32},
	).Export("storage_delete_prefix")
}

// ── 上下文 ───────────────────────────────────────────────

func registerContext(b wazero.HostModuleBuilder, api store.ContractApi) {
	b.NewFunctionBuilder().WithGoModuleFunction(
		wazapi.GoModuleFunc(func(_ context.Context, _ wazapi.Module, stack []uint64) {
			stack[0] = uint64(api.GetHeight())
		}),
		nil, []wazapi.ValueType{i64},
	).Export("get_height")

	b.NewFunctionBuilder().WithGoModuleFunction(
		wazapi.GoModuleFunc(func(_ context.Context, _ wazapi.Module, stack []uint64) {
			stack[0] = uint64(api.GetBlockTime())
		}),
		nil, []wazapi.ValueType{i64},
	).Export("get_block_time")

	b.NewFunctionBuilder().WithGoModuleFunction(
		wazapi.GoModuleFunc(func(_ context.Context, _ wazapi.Module, stack []uint64) {
			stack[0] = uint64(api.GetVersion())
		}),
		nil, []wazapi.ValueType{i32},
	).Export("get_version")
}

// ── 身份权限 ─────────────────────────────────────────────

func registerIdentity(b wazero.HostModuleBuilder, api store.ContractApi) {
	b.NewFunctionBuilder().WithGoModuleFunction(
		wazapi.GoModuleFunc(func(_ context.Context, mod wazapi.Module, stack []uint64) {
			mem := mod.Memory()
			addr := api.GetCaller()
			buf := make([]byte, 8+len(addr.V))
			binary.LittleEndian.PutUint32(buf[0:4], addr.T)
			binary.LittleEndian.PutUint32(buf[4:8], uint32(len(addr.V)))
			copy(buf[8:], addr.V)
			if uint32(stack[1]) < uint32(len(buf)) {
				stack[0] = errBufTooSmall
				return
			}
			mem.Write(uint32(stack[0]), buf)
			stack[0] = errOK
		}),
		[]wazapi.ValueType{i32, i32}, []wazapi.ValueType{i32},
	).Export("get_caller")

	b.NewFunctionBuilder().WithGoModuleFunction(
		wazapi.GoModuleFunc(func(_ context.Context, mod wazapi.Module, stack []uint64) {
			mem := mod.Memory()
			addr := api.GetSystemAccount()
			buf := make([]byte, 8+len(addr.V))
			binary.LittleEndian.PutUint32(buf[0:4], addr.T)
			binary.LittleEndian.PutUint32(buf[4:8], uint32(len(addr.V)))
			copy(buf[8:], addr.V)
			if uint32(stack[1]) < uint32(len(buf)) {
				stack[0] = errBufTooSmall
				return
			}
			mem.Write(uint32(stack[0]), buf)
			stack[0] = errOK
		}),
		[]wazapi.ValueType{i32, i32}, []wazapi.ValueType{i32},
	).Export("get_system_account")

	b.NewFunctionBuilder().WithGoModuleFunction(
		wazapi.GoModuleFunc(func(_ context.Context, _ wazapi.Module, stack []uint64) {
			if api.CallbySystem() {
				stack[0] = 1
			} else {
				stack[0] = 0
			}
		}),
		nil, []wazapi.ValueType{i32},
	).Export("is_system")
}

// ── 调用 IO ──────────────────────────────────────────────

func registerCallIO(b wazero.HostModuleBuilder, api store.ContractApi, callData []byte, result *[]byte) {
	b.NewFunctionBuilder().WithGoModuleFunction(
		wazapi.GoModuleFunc(func(_ context.Context, mod wazapi.Module, stack []uint64) {
			if callData == nil {
				stack[0] = 0
				return
			}
			mem := mod.Memory()
			outLen := uint32(stack[1])
			if outLen < uint32(len(callData)) {
				stack[0] = errBufTooSmall
				return
			}
			mem.Write(uint32(stack[0]), callData)
			stack[0] = uint64(len(callData))
		}),
		[]wazapi.ValueType{i32, i32}, []wazapi.ValueType{i32},
	).Export("get_call_data")

	b.NewFunctionBuilder().WithGoModuleFunction(
		wazapi.GoModuleFunc(func(_ context.Context, mod wazapi.Module, stack []uint64) {
			if result == nil {
				return
			}
			mem := mod.Memory()
			data, _ := mem.Read(uint32(stack[0]), uint32(stack[1]))
			*result = data
		}),
		[]wazapi.ValueType{i32, i32}, nil,
	).Export("set_result")

	// cross_contract_query: 委托引擎侧 native Go 合约执行（version=0），避免 WASM→host→WASM 循环
	b.NewFunctionBuilder().WithGoModuleFunction(
		wazapi.GoModuleFunc(func(_ context.Context, mod wazapi.Module, stack []uint64) {
			mem := mod.Memory()
			name, _ := mem.Read(uint32(stack[0]), uint32(stack[1]))
			method, _ := mem.Read(uint32(stack[2]), uint32(stack[3]))
			argsCount := int32(stack[4])
			argsPtr := uint32(stack[5])
			var args [][]byte
			off := argsPtr
			for i := int32(0); i < argsCount; i++ {
				argLen := readU32At(mem, off)
				off += 4
				arg, _ := mem.Read(off, argLen)
				off += argLen
				args = append(args, arg)
			}
			call := &store.InContractCall{Name: name, Method: method, Args: args}
			data, err := api.Query(store.UniAddr{}, call)
			if err != nil {
				stack[0] = uint64(0xFFFFFFFF)
				return
			}
			outPtr := uint32(stack[6])
			outLen := uint32(stack[7])
			if outLen < uint32(len(data)) {
				stack[0] = uint64(0xFFFFFFFD)
				return
			}
			mem.Write(outPtr, data)
			stack[0] = uint64(len(data))
		}),
		[]wazapi.ValueType{i32, i32, i32, i32, i32, i32, i32, i32}, []wazapi.ValueType{i32},
	).Export("cross_contract_query")
}

// ── 日志 ─────────────────────────────────────────────────

func registerLog(b wazero.HostModuleBuilder) {
	b.NewFunctionBuilder().WithGoModuleFunction(
		wazapi.GoModuleFunc(func(_ context.Context, mod wazapi.Module, stack []uint64) {
			mem := mod.Memory()
			msg, _ := mem.Read(uint32(stack[0]), uint32(stack[1]))
			if msg != nil {
				fmt.Printf("[WASM] %s\n", string(msg))
			}
		}),
		[]wazapi.ValueType{i32, i32}, nil,
	).Export("log")
}
