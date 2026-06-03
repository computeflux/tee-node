// Package wasm 提供 WASM 合约执行所需的 host functions。
// 使用 wazero (v1.12+) HostModuleBuilder，注册到独立 "runtime" 模块。
package wasm

import (
	"context"
	"encoding/binary"

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

// copyBytes 复制字节切片（wazero 的 mem.Read 返回 WASM 内存视图，需独立副本）。
func copyBytes(b []byte) []byte {
	if b == nil {
		return nil
	}
	cp := make([]byte, len(b))
	copy(cp, b)
	return cp
}

// ── Host Module ───────────────────────────────────────────
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

// instantiateHostModule 创建并注册 "runtime" host module（singleton，只调用一次）。
// s 是共享状态指针，host functions 通过 s 访问 callData/result/api。
func instantiateHostModule(ctx context.Context, rt wazero.Runtime, s *hostState) (wazapi.Module, error) {
	b := rt.NewHostModuleBuilder("runtime")
	registerStorage(b, s)
	registerContext(b, s)
	registerIdentity(b, s)
	registerCallIO(b, s)
	return b.Instantiate(ctx)
}

// ── 存储 ─────────────────────────────────────────────────

func registerStorage(b wazero.HostModuleBuilder, s *hostState) {
	b.NewFunctionBuilder().WithGoModuleFunction(
		wazapi.GoModuleFunc(func(_ context.Context, mod wazapi.Module, stack []uint64) {
			txn := s.api.GetTxn()
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
			stack[0] = uint64(len(val))
		}),
		[]wazapi.ValueType{i32, i32, i32, i32}, []wazapi.ValueType{i32},
	).Export("storage_get")

	b.NewFunctionBuilder().WithGoModuleFunction(
		wazapi.GoModuleFunc(func(_ context.Context, mod wazapi.Module, stack []uint64) {
			txn := s.api.GetTxn()
			mem := mod.Memory()
			key, _ := mem.Read(uint32(stack[0]), uint32(stack[1]))
			val, _ := mem.Read(uint32(stack[2]), uint32(stack[3]))
			if key == nil || val == nil {
				stack[0] = errMem
				return
			}
			if err := txn.Set(copyBytes(key), copyBytes(val)); err != nil {
				stack[0] = errStorage
				return
			}
			stack[0] = errOK
		}),
		[]wazapi.ValueType{i32, i32, i32, i32}, []wazapi.ValueType{i32},
	).Export("storage_set")

	b.NewFunctionBuilder().WithGoModuleFunction(
		wazapi.GoModuleFunc(func(_ context.Context, mod wazapi.Module, stack []uint64) {
			txn := s.api.GetTxn()
			mem := mod.Memory()
			key, _ := mem.Read(uint32(stack[0]), uint32(stack[1]))
			if key == nil {
				stack[0] = errMem
				return
			}
			if err := txn.Delete(copyBytes(key)); err != nil {
				stack[0] = errStorage
				return
			}
			stack[0] = errOK
		}),
		[]wazapi.ValueType{i32, i32}, []wazapi.ValueType{i32},
	).Export("storage_delete")

	b.NewFunctionBuilder().WithGoModuleFunction(
		wazapi.GoModuleFunc(func(_ context.Context, mod wazapi.Module, stack []uint64) {
			txn := s.api.GetTxn()
			mem := mod.Memory()
			prefix, _ := mem.Read(uint32(stack[0]), uint32(stack[1]))
			if prefix == nil {
				stack[0] = errMem
				return
			}
			keys, vals, err := txn.ListByPrefix(copyBytes(prefix))
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
			txn := s.api.GetTxn()
			mem := mod.Memory()
			prefix, _ := mem.Read(uint32(stack[0]), uint32(stack[1]))
			if prefix == nil {
				stack[0] = errMem
				return
			}
			if err := txn.DeletekeysByPrefix(copyBytes(prefix)); err != nil {
				stack[0] = errStorage
				return
			}
			stack[0] = errOK
		}),
		[]wazapi.ValueType{i32, i32}, []wazapi.ValueType{i32},
	).Export("storage_delete_prefix")
}

// ── 上下文 ───────────────────────────────────────────────

func registerContext(b wazero.HostModuleBuilder, s *hostState) {
	b.NewFunctionBuilder().WithGoModuleFunction(
		wazapi.GoModuleFunc(func(_ context.Context, _ wazapi.Module, stack []uint64) {
			stack[0] = uint64(s.api.GetHeight())
		}),
		nil, []wazapi.ValueType{i64},
	).Export("get_height")

	b.NewFunctionBuilder().WithGoModuleFunction(
		wazapi.GoModuleFunc(func(_ context.Context, _ wazapi.Module, stack []uint64) {
			stack[0] = uint64(s.api.GetBlockTime())
		}),
		nil, []wazapi.ValueType{i64},
	).Export("get_block_time")

	b.NewFunctionBuilder().WithGoModuleFunction(
		wazapi.GoModuleFunc(func(_ context.Context, _ wazapi.Module, stack []uint64) {
			stack[0] = uint64(s.api.GetVersion())
		}),
		nil, []wazapi.ValueType{i32},
	).Export("get_version")
}

// ── 身份权限 ─────────────────────────────────────────────

func registerIdentity(b wazero.HostModuleBuilder, s *hostState) {
	b.NewFunctionBuilder().WithGoModuleFunction(
		wazapi.GoModuleFunc(func(_ context.Context, mod wazapi.Module, stack []uint64) {
			addr := s.api.GetCaller()
			buf, _ := addr.Encode()
			if uint32(stack[1]) < uint32(len(buf)) {
				stack[0] = errBufTooSmall
				return
			}
			mod.Memory().Write(uint32(stack[0]), buf)
			stack[0] = uint64(len(buf))
		}),
		[]wazapi.ValueType{i32, i32}, []wazapi.ValueType{i32},
	).Export("get_caller")

	b.NewFunctionBuilder().WithGoModuleFunction(
		wazapi.GoModuleFunc(func(_ context.Context, mod wazapi.Module, stack []uint64) {
			addr := s.api.GetSystemAccount()
			buf, _ := addr.Encode()
			if uint32(stack[1]) < uint32(len(buf)) {
				stack[0] = errBufTooSmall
				return
			}
			mod.Memory().Write(uint32(stack[0]), buf)
			stack[0] = uint64(len(buf))
		}),
		[]wazapi.ValueType{i32, i32}, []wazapi.ValueType{i32},
	).Export("get_system_account")

	b.NewFunctionBuilder().WithGoModuleFunction(
		wazapi.GoModuleFunc(func(_ context.Context, _ wazapi.Module, stack []uint64) {
			if s.api.CallbySystem() {
				stack[0] = 1
			} else {
				stack[0] = 0
			}
		}),
		nil, []wazapi.ValueType{i32},
	).Export("is_system")
}

// ── 调用 IO ──────────────────────────────────────────────

func registerCallIO(b wazero.HostModuleBuilder, s *hostState) {
}
