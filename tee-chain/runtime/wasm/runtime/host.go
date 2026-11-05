//go:build wasm

// Package main is the WASM contract runtime.
// It provides the execute() entry point called by the host engine,
// storage via host function imports, and identity helpers.
//
// Package main 是 WASM 合约运行时。
// 提供宿主引擎调用的 execute() 入口、通过 host function import 实现的存储、以及身份辅助函数。
package main

import (
	"fmt"
	"unsafe"

	"github.com/computeflux/tee-provider/tee-chain/runtime/store"
)

// ── Host function imports (registered by engine/host.go) ──
// ── 宿主函数导入（由 engine/host.go 注册） ──

//go:wasmimport runtime storage_get
func storageGet(keyPtr, keyLen, valPtr, valLen int32) int32

//go:wasmimport runtime storage_set
func storageSet(keyPtr, keyLen, valPtr, valLen int32) int32

//go:wasmimport runtime storage_delete
func storageDelete(keyPtr, keyLen int32) int32

//go:wasmimport runtime storage_scan_prefix
func storageScanPrefix(prefixPtr, prefixLen, keysPtr, valsPtr int32) int32

//go:wasmimport runtime storage_delete_prefix
func storageDeletePrefix(prefixPtr, prefixLen int32) int32

//go:wasmimport runtime get_block_time
func getBlockTime() int64

//go:wasmimport runtime get_version
func getVersion() int32

//go:wasmimport runtime get_caller
func getCaller(outPtr, outLen int32) int32

//go:wasmimport runtime get_system_account
func getSystemAccount(outPtr, outLen int32) int32

//go:wasmimport runtime is_system
func isSystem() int32

//go:wasmimport runtime verify_tee_v2
func verifyTEEV2(reportPtr, reportLen, dataPtr, dataLen int32, t int64, callerPtr, callerLen int32, teeType int32, outPtr, outLen int32) int32

//go:wasmimport runtime emit_event
func emitEvent(typePtr, typeLen, attrsPtr, attrsLen int32) int32

// Shared buffer for interop with host functions.
// Layout: [0, txnKeyArea) for keys/prefix (and ListByPrefix key results),
// [txnKeyArea, txnBufSize) for values (and ListByPrefix value results).
//
// 与宿主函数交互的共享缓冲区。
// 布局: [0, txnKeyArea) 存放 key/prefix（含 ListByPrefix 的 key 结果），
// [txnKeyArea, txnBufSize) 存放 value（含 ListByPrefix 的 value 结果）。
const (
	// txnBufSize 必须容纳 runtime_upgrade 上传的整个 SCALE 编码
	// PendingRuntimeUpgrade（含 wasm 字节：当前 raw ≈1.1MB / gzip ≈400KB）。
	// 旧值 256KB 会让 Set 静默截断大 value，链上留下 gunzip 报 unexpected EOF
	// 的残缺 WASM，迁移也随之失败。
	txnBufSize = 4 * 1024 * 1024 // 4MB

	// txnKeyArea 是 key/prefix 区大小；value 区从该偏移开始，
	// 可用空间为 txnBufSize - txnKeyArea。
	txnKeyArea = 256 * 1024 // 256KB
)

var txnBuf [txnBufSize]byte

// Host function error codes (mirror tee-chain/runtime/wasm/host.go).
// 宿主函数错误码（与 tee-chain/runtime/wasm/host.go 保持一致）。
const (
	errKeyNotFound int32 = -4
	errBufTooSmall int32 = -3
	errStorage     int32 = -2
	errMem         int32 = -1
)

// storageError represents an error code returned by a storage host function.
type storageError struct {
	code int32
	fn   string
}

func (e *storageError) Error() string {
	return fmt.Sprintf("%s returned error code %d", e.fn, e.code)
}

// txnBufPtr returns the WASM memory address of the shared buffer.
func txnBufPtr() int32 {
	return int32(uintptr(unsafe.Pointer(&txnBuf[0])))
}

// WasmTxn implements store.StoreTx by delegating to host storage functions.
// All operations go through the shared txnBuf for key/value data transfer.
//
// WasmTxn 通过委托宿主存储函数实现 store.StoreTx 接口。
// 所有操作通过共享 txnBuf 传输 key/value 数据。
type WasmTxn struct{}

func (txn *WasmTxn) Get(key []byte) ([]byte, error) {
	kp := int32(uintptr(unsafe.Pointer(&txnBuf[0])))
	copy(txnBuf[:], key)
	vp := kp + txnKeyArea
	ret := storageGet(kp, int32(len(key)), vp, int32(len(txnBuf)-txnKeyArea))
	if ret == 0 || ret == errKeyNotFound {
		return nil, nil
	}
	if ret < 0 {
		return nil, &storageError{ret, "storage_get"}
	}
	val := make([]byte, ret)
	copy(val, txnBuf[vp-kp:vp-kp+ret])
	return val, nil
}

func (txn *WasmTxn) Set(key, value []byte) error {
	// 共享缓冲区只 txnBufSize 字节：key 从 0 开始、value 从 len(key)+16 开始。
	// 超长 value（如 runtime_upgrade 上传的 WASM）必须显式报错，不能静默截断，
	// 否则链上会写入残缺的 WASM（gunzip 时报 unexpected EOF）。
	if len(key)+16+len(value) > len(txnBuf) {
		return fmt.Errorf("storage_set: key(%d)+value(%d) exceeds shared buffer (%d bytes)",
			len(key), len(value), len(txnBuf))
	}

	kp := int32(uintptr(unsafe.Pointer(&txnBuf[0])))
	copy(txnBuf[:], key)
	vp := kp + int32(len(key)) + 16
	copy(txnBuf[vp-kp:], value)
	ret := storageSet(kp, int32(len(key)), vp, int32(len(value)))
	if ret < 0 {
		return &storageError{ret, "storage_set"}
	}
	return nil
}

func (txn *WasmTxn) Delete(key []byte) error {
	kp := int32(uintptr(unsafe.Pointer(&txnBuf[0])))
	copy(txnBuf[:], key)
	ret := storageDelete(kp, int32(len(key)))
	if ret < 0 {
		return &storageError{ret, "storage_delete"}
	}
	return nil
}

// ListByPrefix lists all key-value pairs whose key starts with the given prefix.
// Results are parsed from the shared buffer written by the host.
//
// ListByPrefix 列出所有 key 以指定前缀开头的键值对。
// 从宿主写入的共享缓冲区解析结果。
func (txn *WasmTxn) ListByPrefix(prefix []byte) (keys, vals [][]byte, err error) {
	kp := int32(uintptr(unsafe.Pointer(&txnBuf[0])))
	copy(txnBuf[:], prefix)
	pp := kp
	vp := kp + txnKeyArea
	n := storageScanPrefix(pp, int32(len(prefix)), kp, vp)
	if n == 0 || n == errKeyNotFound {
		return nil, nil, nil
	}
	if n < 0 {
		return nil, nil, &storageError{n, "storage_scan_prefix"}
	}
	off := uint32(kp - pp)
	count := readU32At(off)
	off += 4
	for i := uint32(0); i < count; i++ {
		l := readU32At(off)
		off += 4
		k := make([]byte, l)
		copy(k, txnBuf[off:off+l])
		off += l
		keys = append(keys, k)
	}
	off = uint32(vp - pp)
	count = readU32At(off)
	off += 4
	for i := uint32(0); i < count; i++ {
		l := readU32At(off)
		off += 4
		v := make([]byte, l)
		copy(v, txnBuf[off:off+l])
		off += l
		vals = append(vals, v)
	}
	return keys, vals, nil
}

func (txn *WasmTxn) DeletekeysByPrefix(prefix []byte) error {
	pp := txnBufPtr()
	copy(txnBuf[:], prefix)
	ret := storageDeletePrefix(pp, int32(len(prefix)))
	if ret < 0 {
		return &storageError{ret, "storage_delete_prefix"}
	}
	return nil
}

func (txn *WasmTxn) Commit() error   { return nil }
func (txn *WasmTxn) Rollback() error { return nil }

// readU32At reads a little-endian uint32 from txnBuf at the given offset.
func readU32At(off uint32) uint32 {
	return uint32(txnBuf[off]) | uint32(txnBuf[off+1])<<8 | uint32(txnBuf[off+2])<<16 | uint32(txnBuf[off+3])<<24
}

// ── Identity helpers / 身份辅助函数 ──────────────────────

// readCaller decodes the caller's UniAddr from the host's get_caller response.
func readCaller() store.UniAddr {
	var addr store.UniAddr
	bp := txnBufPtr()
	n := getCaller(bp, int32(len(txnBuf)))
	if n <= 0 {
		return addr
	}
	addr.Decode(txnBuf[:n])
	return addr
}

// readSystemAccount decodes the system account's UniAddr from the host.
func readSystemAccount() store.UniAddr {
	var addr store.UniAddr
	bp := txnBufPtr()
	n := getSystemAccount(bp, int32(len(txnBuf)))
	if n <= 0 {
		return addr
	}
	addr.Decode(txnBuf[:n])
	return addr
}
