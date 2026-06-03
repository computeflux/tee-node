//go:build wasm

package main

import (
	"unsafe"

	"github.com/computeflux/tee-provider/tee-chain/contracts/module"
	"github.com/computeflux/tee-provider/tee-chain/contracts/store"
)

const bufSize = 128 * 1024 // 128KB

var buf [bufSize]byte

func bufPtr() uint32 { return uint32(uintptr(unsafe.Pointer(&buf[0]))) }

//go:wasmimport runtime get_height
func hostGetHeight() int64

//go:wasmexport ping
func ping() int32 { return 1 }

//go:wasmexport get_buffer
func get_buffer() uint32 { return bufPtr() }

// 返回值: 高 32 位 = 结果指针, 低 32 位 = 结果长度; (0, 0) 表示无结果
//
//go:wasmexport execute
func execute(isQuery uint32, callDataPtr uint32, callDataLen uint32) uint64 {
	if callDataLen == 0 {
		return 0
	}
	// 直接从 WASM 内存读取 SCALE 编码的 InContractCall
	data := unsafe.Slice((*byte)(unsafe.Pointer(uintptr(callDataPtr))), int(callDataLen))

	var call store.InContractCall
	if _, err := call.Decode(data); err != nil {
		bp := bufPtr()
		n := copy(buf[:], err.Error())
		return packResult(bp, uint32(n))
	}

	rt := &wasmRuntime{txn: &WasmTxn{}, height: hostGetHeight()}

	var result []byte
	var err error
	if isQuery != 0 {
		result, err = module.Query(&call, rt)
	} else {
		err = module.Mutation(&call, rt)
	}
	if err != nil {
		result = []byte(err.Error())
	}
	if len(result) == 0 {
		return 0
	}
	bp := bufPtr()
	return packResult(bp, uint32(copy(buf[:], result)))
}

// packResult 将 (ptr, len) 打包为 uint64: 高 32 位 ptr, 低 32 位 len
func packResult(ptr, length uint32) uint64 {
	return uint64(ptr)<<32 | uint64(length)
}

// ── 运行时适配层 ──────────────────────────────────────────

type wasmRuntime struct {
	txn    store.StoreTx
	height int64
}

func (r *wasmRuntime) GetHeight() int64      { return r.height }
func (r *wasmRuntime) GetBlockTime() int64   { return getBlockTime() }
func (r *wasmRuntime) GetVersion() uint32    { return uint32(getVersion()) }
func (r *wasmRuntime) GetTxn() store.StoreTx { return r.txn }

func (r *wasmRuntime) GetCaller() store.UniAddr {
	return readCaller()
}

func (r *wasmRuntime) GetSystemAccount() store.UniAddr {
	return readSystemAccount()
}

func (r *wasmRuntime) CallbySystem() bool { return isSystem() != 0 }

func (r *wasmRuntime) Query(target store.UniAddr, call store.ContractCall) ([]byte, error) {
	return module.Query(call, r)
}

func (r *wasmRuntime) Call(target store.UniAddr, call store.ContractCall) ([]byte, error) {
	return nil, module.Mutation(call, r)
}

func (r *wasmRuntime) QueryNative(call store.ContractCall) ([]byte, error) {
	return module.Query(call, r)
}
