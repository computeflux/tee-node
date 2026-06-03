//go:build wasm

package main

import (
	"fmt"
	"unsafe"

	"github.com/computeflux/tee-provider/tee-chain/contracts/store"
)

// ── host function imports (对应 engine 侧 host.go 注册的函数) ─

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

const txnBufSize = 128 * 1024 // 128KB

var txnBuf [txnBufSize]byte

// storageError 表示 storage host function 返回的错误码。
type storageError struct {
	code int32
	fn   string
}

func (e *storageError) Error() string {
	return fmt.Sprintf("%s returned error code %d", e.fn, e.code)
}

func txnBufPtr() int32 {
	return int32(uintptr(unsafe.Pointer(&txnBuf[0])))
}

type WasmTxn struct{}

func (txn *WasmTxn) Get(key []byte) ([]byte, error) {
	// 使用 txnBuf 的前 64KB 作为 key，后 64KB 作为 value 输出区
	kp := int32(uintptr(unsafe.Pointer(&txnBuf[0])))
	copy(txnBuf[:], key)
	vp := kp + 65536
	ret := storageGet(kp, int32(len(key)), vp, int32(len(txnBuf)-65536))
	if ret <= 0 {
		return nil, nil
	}
	val := make([]byte, ret)
	copy(val, txnBuf[vp-kp:vp-kp+ret])
	return val, nil
}

func (txn *WasmTxn) Set(key, value []byte) error {
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

func (txn *WasmTxn) ListByPrefix(prefix []byte) (keys, vals [][]byte, err error) {
	kp := int32(uintptr(unsafe.Pointer(&txnBuf[0])))
	copy(txnBuf[:], prefix)
	pp := kp
	vp := kp + 65536 // 前 64KB 存放 key 列表，后 64KB 存放 val 列表
	n := storageScanPrefix(pp, int32(len(prefix)), kp, vp)
	if n <= 0 {
		return nil, nil, nil
	}
	// parse from txnBuf
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

func readU32At(off uint32) uint32 {
	return uint32(txnBuf[off]) | uint32(txnBuf[off+1])<<8 | uint32(txnBuf[off+2])<<16 | uint32(txnBuf[off+3])<<24
}

// ── 身份辅助 ──────────────────────────────────────────────

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
