// Package wasm provides host functions for WASM contract execution.
// Uses wazero (v1.12+) HostModuleBuilder, registered under the "runtime" module.
//
// Package wasm 提供 WASM 合约执行所需的 host functions。
// 使用 wazero (v1.12+) HostModuleBuilder，注册到独立 "runtime" 模块。
package wasm

import (
	"context"
	"encoding/binary"

	"github.com/computeflux/tee-provider/pkg/types/teetypes"
	"github.com/computeflux/tee-provider/tee-chain/runtime/store"
	"github.com/tetratelabs/wazero"
	wazapi "github.com/tetratelabs/wazero/api"
)

var i32, i64 = wazapi.ValueTypeI32, wazapi.ValueTypeI64

// Error codes returned by host functions to WASM.
// 宿主函数返回给 WASM 的错误码。
const (
	errOK             = uint64(0)          // success
	errMem            = uint64(0xFFFFFFFF) // memory access error
	errStorage        = uint64(0xFFFFFFFE) // storage operation error
	errBufTooSmall    = uint64(0xFFFFFFFD) // output buffer too small
	errKeyNotFound    = uint64(0xFFFFFFFC) // key not found
	errEventInvalid   = uint64(0xFFFFFFFB) // invalid event type (empty / reserved / bad format)
)

// copyBytes deep-copies a byte slice.
// Needed because wazero's mem.Read returns a view into WASM memory that may be reused.
//
// copyBytes 深拷贝字节切片。
// wazero 的 mem.Read 返回 WASM 内存视图，需独立副本以避免被覆盖。
func copyBytes(b []byte) []byte {
	if b == nil {
		return nil
	}
	cp := make([]byte, len(b))
	copy(cp, b)
	return cp
}

// encodeScanPairs writes keys and vals arrays into WASM memory at the given pointers.
// Format: [count: u32LE][len: u32LE][data...][count: u32LE][len: u32LE][data...]
// Returns the number of key-value pairs written.
//
// encodeScanPairs 将 keys/vals 数组写入 WASM 内存指定位置。
// 格式: [数量: u32LE][长度: u32LE][数据...][数量: u32LE][长度: u32LE][数据...]
func encodeScanPairs(mem wazapi.Memory, keysPtr, valsPtr uint32, keys, vals [][]byte) (uint32, bool) {
	buf := make([]byte, 0, 1024)
	buf = binary.LittleEndian.AppendUint32(buf, uint32(len(keys)))
	for _, k := range keys {
		buf = binary.LittleEndian.AppendUint32(buf, uint32(len(k)))
		buf = append(buf, k...)
	}
	if !mem.Write(keysPtr, buf) {
		return 0, false
	}

	buf = buf[:0]
	buf = binary.LittleEndian.AppendUint32(buf, uint32(len(vals)))
	for _, v := range vals {
		buf = binary.LittleEndian.AppendUint32(buf, uint32(len(v)))
		buf = append(buf, v...)
	}
	if !mem.Write(valsPtr, buf) {
		return 0, false
	}

	return uint32(len(keys)), true
}

// instantiateHostModule creates and registers the "runtime" host module.
// s is shared mutable state; host functions access s.api for contract context.
//
// instantiateHostModule 创建并注册 "runtime" host module。
// s 是共享状态指针，host functions 通过 s.api 访问合约上下文。
func instantiateHostModule(ctx context.Context, rt wazero.Runtime, s *hostState) (wazapi.Module, error) {
	b := rt.NewHostModuleBuilder("runtime")
	registerStorage(b, s)
	registerContext(b, s)
	registerIdentity(b, s)
	registerVerifyTEE(b, s)
	registerEmitEvent(b, s)
	return b.Instantiate(ctx)
}

// ── Storage host functions / 存储宿主函数 ─────────────────

func registerStorage(b wazero.HostModuleBuilder, s *hostState) {
	// storage_get: reads value by key from the transaction store.
	// Params: keyPtr, keyLen, valPtr (output), valLen (max output size).
	// Returns: length of value on success, error code on failure.
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

	// storage_set: writes a key-value pair to the transaction store.
	// Params: keyPtr, keyLen, valPtr, valLen. Returns errOK on success.
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

	// storage_delete: deletes a key from the transaction store.
	// Params: keyPtr, keyLen. Returns errOK on success.
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

	// storage_scan_prefix: lists all key-value pairs with the given prefix.
	// Params: prefixPtr, prefixLen, keysPtr (output), valsPtr (output).
	// Returns: number of pairs found.
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
			n, ok := encodeScanPairs(mem, uint32(stack[2]), uint32(stack[3]), keys, vals)
			if !ok {
				stack[0] = errMem
				return
			}
			stack[0] = uint64(n)
		}),
		[]wazapi.ValueType{i32, i32, i32, i32}, []wazapi.ValueType{i32},
	).Export("storage_scan_prefix")

	// storage_delete_prefix: deletes all keys with the given prefix.
	// Params: prefixPtr, prefixLen. Returns errOK on success.
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

// ── Context host functions / 上下文宿主函数 ───────────────

func registerContext(b wazero.HostModuleBuilder, s *hostState) {
	// get_height: returns the current block height.
	b.NewFunctionBuilder().WithGoModuleFunction(
		wazapi.GoModuleFunc(func(_ context.Context, _ wazapi.Module, stack []uint64) {
			stack[0] = uint64(s.api.GetHeight())
		}),
		nil, []wazapi.ValueType{i64},
	).Export("get_height")

	// get_block_time: returns the current block timestamp.
	b.NewFunctionBuilder().WithGoModuleFunction(
		wazapi.GoModuleFunc(func(_ context.Context, _ wazapi.Module, stack []uint64) {
			stack[0] = uint64(s.api.GetBlockTime())
		}),
		nil, []wazapi.ValueType{i64},
	).Export("get_block_time")

	// get_version: returns the current runtime version.
	b.NewFunctionBuilder().WithGoModuleFunction(
		wazapi.GoModuleFunc(func(_ context.Context, _ wazapi.Module, stack []uint64) {
			stack[0] = uint64(s.api.GetVersion())
		}),
		nil, []wazapi.ValueType{i32},
	).Export("get_version")
}

// ── Identity host functions / 身份宿主函数 ─────────────────

func registerIdentity(b wazero.HostModuleBuilder, s *hostState) {
	// get_caller: returns the SCALE-encoded UniAddr of the caller.
	// Params: outPtr, outLen. Returns encoded length on success, error code on failure.
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

	// get_system_account: returns the SCALE-encoded UniAddr of the system account.
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

	// is_system: returns 1 if the caller is the system account, 0 otherwise.
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

// ── TEE signature / TEE 签名校验 ────────────────────────

// registerVerifyTEE 注册 TEE 验签 host 函数，双导出（保持向后兼容）：
//   - verify_tee（旧 7 参）: reportPtr, reportLen, dataPtr, dataLen, t(i64), outPtr, outLen
//     Caller/TeeType 由宿主从执行上下文补齐（当前调用方 / 本机 TEE 类型），
//     链上已部署的旧版 wasm（import 旧 7 参签名）继续可用——
//     wazero 在 instantiate 时校验 import 签名，改签名会直接导致旧 wasm 无法实例化；
//   - verify_tee_v2（新 10 参）: 在 t 之后、outPtr 之前插入 callerPtr, callerLen, teeType(i32)，
//     由合约显式传入报告签发者公钥与 TEE 类型（新版 wasm import 此签名）。
//
// 返回: 成功为编码结果字节长度，失败为错误码。
func registerVerifyTEE(b wazero.HostModuleBuilder, s *hostState) {
	b.NewFunctionBuilder().WithGoModuleFunction(
		wazapi.GoModuleFunc(func(_ context.Context, mod wazapi.Module, stack []uint64) {
			mem := mod.Memory()
			report, _ := mem.Read(uint32(stack[0]), uint32(stack[1]))
			data, _ := mem.Read(uint32(stack[2]), uint32(stack[3]))
			n, code := doVerifyTEE(s, mod, report, data, s.api.GetCaller().V,
				int64(stack[4]), uint32(teetypes.TeeType), uint32(stack[5]), uint32(stack[6]))
			stack[0] = n
			if code != errOK {
				stack[0] = code
				return
			}
		}),
		[]wazapi.ValueType{i32, i32, i32, i32, i64, i32, i32}, []wazapi.ValueType{i32},
	).Export("verify_tee")

	b.NewFunctionBuilder().WithGoModuleFunction(
		wazapi.GoModuleFunc(func(_ context.Context, mod wazapi.Module, stack []uint64) {
			mem := mod.Memory()
			report, _ := mem.Read(uint32(stack[0]), uint32(stack[1]))
			data, _ := mem.Read(uint32(stack[2]), uint32(stack[3]))
			caller, _ := mem.Read(uint32(stack[5]), uint32(stack[6]))
			n, code := doVerifyTEE(s, mod, report, data, caller,
				int64(stack[4]), uint32(stack[7]), uint32(stack[8]), uint32(stack[9]))
			stack[0] = n
			if code != errOK {
				stack[0] = code
				return
			}
		}),
		[]wazapi.ValueType{i32, i32, i32, i32, i64, i32, i32, i32, i32, i32}, []wazapi.ValueType{i32},
	).Export("verify_tee_v2")
}

// doVerifyTEE verify_tee / verify_tee_v2 的共享核心：校验 TEE quote，
// 把验证结果 Result[TeeVerifyInfo, string]（SCALE 编码）写入输出缓冲区（outPtr, outLen）。
// 返回 (编码结果字节长度, errOK)；失败返回 (0, 错误码)。
func doVerifyTEE(s *hostState, mod wazapi.Module, report, data, caller []byte, t int64, teeType uint32, outPtr, outLen uint32) (uint64, uint64) {
	res := s.api.VerifyTEE(report, data, t, caller, teeType)
	encoded, err := res.Encode()
	if err != nil {
		return 0, errStorage
	}
	if outLen < uint32(len(encoded)) {
		return 0, errBufTooSmall
	}
	mod.Memory().Write(outPtr, encoded)
	return uint64(len(encoded)), errOK
}

// ── Event host functions / 事件宿主函数 ─────────────────

// registerEmitEvent 注册 emit_event host 函数：WASM 合约发射交易级事件。
// Params: typePtr, typeLen, attrsPtr, attrsLen。
// attrs 格式: [count: u32LE][len: u32LE][key...][len: u32LE][value...]...
// 返回 errOK on success；事件非法（type 格式/保留名、属性 key/value 格式与长度、
// 属性个数超限）返回 errEventInvalid。校验在宿主侧完成（store.IsValidEvent），
// 防止恶意合约伪造链上系统事件、构造保留 tag（tx.height/tx.hash）或资源放大。
func registerEmitEvent(b wazero.HostModuleBuilder, s *hostState) {
	b.NewFunctionBuilder().WithGoModuleFunction(
		wazapi.GoModuleFunc(func(_ context.Context, mod wazapi.Module, stack []uint64) {
			mem := mod.Memory()
			eventType, _ := mem.Read(uint32(stack[0]), uint32(stack[1]))
			if eventType == nil {
				stack[0] = errMem
				return
			}
			attrsRaw, _ := mem.Read(uint32(stack[2]), uint32(stack[3]))
			if attrsRaw == nil && uint32(stack[3]) > 0 {
				stack[0] = errMem
				return
			}

			attrs := parseEventAttrs(attrsRaw)
			// 校验整个事件（type + 属性 + 个数上限）：拒绝空/非法/系统保留事件名、
			// 含 '.' 或超长的属性 key、超长 value，避免合约伪造系统事件/保留 tag/资源放大
			if !store.IsValidEvent(string(eventType), attrs) {
				stack[0] = errEventInvalid
				return
			}
			s.api.EmitEvent(string(eventType), attrs)
			stack[0] = errOK
		}),
		[]wazapi.ValueType{i32, i32, i32, i32}, []wazapi.ValueType{i32},
	).Export("emit_event")
}

// parseEventAttrs 解析 emit_event 的 attrs 缓冲区。
// 格式: [count: u32LE][len: u32LE][key...][len: u32LE][value...]...
func parseEventAttrs(raw []byte) map[string]string {
	attrs := make(map[string]string)
	if len(raw) < 4 {
		return attrs
	}
	count := binary.LittleEndian.Uint32(raw[:4])
	off := 4
	for i := uint32(0); i < count; i++ {
		if off+4 > len(raw) {
			break
		}
		keyLen := int(binary.LittleEndian.Uint32(raw[off : off+4]))
		off += 4
		if off+keyLen > len(raw) {
			break
		}
		key := string(raw[off : off+keyLen])
		off += keyLen

		if off+4 > len(raw) {
			break
		}
		valLen := int(binary.LittleEndian.Uint32(raw[off : off+4]))
		off += 4
		if off+valLen > len(raw) {
			break
		}
		attrs[key] = string(raw[off : off+valLen])
		off += valLen
	}
	return attrs
}
