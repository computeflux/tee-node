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

//go:wasmimport runtime cross_contract_query
func crossContractQuery(namePtr, nameLen int32, method int32, argsCount, argsPtr, outPtr, outLen int32) int32

//go:wasmimport runtime cross_contract_call
func crossContractCall(namePtr, nameLen int32, method int32, argsCount, argsPtr, outPtr, outLen int32) int32

var txnBuf [8192]byte

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
	// 使用 txnBuf 的前 4KB 作为 key，后 4KB 作为 value 输出区
	kp := int32(uintptr(unsafe.Pointer(&txnBuf[0])))
	copy(txnBuf[:], key)
	vp := kp + 4096
	ret := storageGet(kp, int32(len(key)), vp, int32(len(txnBuf)-4096))
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
	vp := kp + 4096 // 前 4KB 存放 key 列表，后 4KB 存放 val 列表
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

// ── 上下文辅助 ──────────────────────────────────────────────

// hostReadUniAddr 从 host function 读取 UniAddr 到 addr。
// hostFn 签名: func(outPtr, outLen int32) int32，返回 0=成功。
func hostReadUniAddr(addr *store.UniAddr, hostFn func(int32, int32) int32) {
	bp := txnBufPtr()
	if hostFn(bp, 40) != 0 {
		return
	}
	// 格式: [type: u32 LE][data_len: u32 LE][data...]
	addr.T = readU32At(uint32(bp))
	dLen := readU32At(uint32(bp) + 4)
	if dLen > 0 && dLen <= 32 {
		addr.V = make([]byte, dLen)
		copy(addr.V, txnBuf[8:8+dLen])
	}
}

// ── 跨合约调用辅助 ──────────────────────────────────────────

type crossContractFunc func(namePtr, nameLen int32, method int32, argsCount, argsPtr, outPtr, outLen int32) int32

// hostCrossContract 序列化 call 并通过 host function 发起到宿主的跨合约调用。
func hostCrossContract(call store.ContractCall, hostFn crossContractFunc) ([]byte, error) {
	bp := txnBufPtr()

	// 写入 name
	name := call.GetName()
	copy(txnBuf[:], name)
	namePtr := bp
	nameLen := int32(len(name))

	// 写入 method (4 bytes, 作为 i32)
	m := call.GetMethod()
	var method int32
	if len(m) >= 4 {
		method = int32(uint32(m[0]) | uint32(m[1])<<8 | uint32(m[2])<<16 | uint32(m[3])<<24)
	}

	// 写入 args 指针数组: [arg1_ptr: u32][arg1_len: u32][arg2_ptr: u32][arg2_len: u32]...
	args := call.GetArgs()
	argsPtr := bp + int32(len(name)) + 16
	argsOff := uint32(argsPtr)
	// args 数据紧跟在指针数组后面
	dataOff := argsOff + uint32(len(args))*8
	for i, arg := range args {
		// 写入 arg 指针
		txnBuf[uint32(argsPtr)+uint32(i)*8-bpOff()] = byte(dataOff)
		txnBuf[uint32(argsPtr)+uint32(i)*8+1-bpOff()] = byte(dataOff >> 8)
		txnBuf[uint32(argsPtr)+uint32(i)*8+2-bpOff()] = byte(dataOff >> 16)
		txnBuf[uint32(argsPtr)+uint32(i)*8+3-bpOff()] = byte(dataOff >> 24)
		// 写入 arg 长度
		argLen := uint32(len(arg))
		txnBuf[uint32(argsPtr)+uint32(i)*8+4-bpOff()] = byte(argLen)
		txnBuf[uint32(argsPtr)+uint32(i)*8+5-bpOff()] = byte(argLen >> 8)
		txnBuf[uint32(argsPtr)+uint32(i)*8+6-bpOff()] = byte(argLen >> 16)
		txnBuf[uint32(argsPtr)+uint32(i)*8+7-bpOff()] = byte(argLen >> 24)
		// 写入 arg 数据
		copy(txnBuf[dataOff-bpOff():], arg)
		dataOff += uint32(len(arg))
	}

	// 输出缓冲
	outPtr := argsPtr + int32(len(args))*8 + int32(len(name)) + 32
	outLen := int32(65536 - (outPtr - bp))

	ret := hostFn(namePtr, nameLen, method, int32(len(args)), argsPtr, outPtr, outLen)
	if ret < 0 {
		return nil, nil
	}
	if ret == 0 {
		return nil, nil
	}
	result := make([]byte, ret)
	copy(result, txnBuf[uint32(outPtr)-bpOff():uint32(outPtr)-bpOff()+uint32(ret)])
	return result, nil
}

func bpOff() uint32 { return uint32(txnBufPtr()) }
