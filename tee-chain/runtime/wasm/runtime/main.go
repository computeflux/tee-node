//go:build wasm

package main

import (
	"fmt"
	"unsafe"

	"github.com/computeflux/tee-provider/tee-chain/runtime/module"
	"github.com/computeflux/tee-provider/tee-chain/runtime/store"
)

const bufSize = 128 * 1024 // 128KB

var buf [bufSize]byte

//go:wasmimport runtime get_height
func hostGetHeight() int64

//go:wasmexport ping
func ping() int32 { return 1 }

//go:wasmexport get_buffer
func get_buffer() uint32 { return uint32(uintptr(unsafe.Pointer(&buf[0]))) }

// execute is the main entry point called by the host.
// Parameters:
//
//	isQuery      - 0 = mutation, non-zero = query
//	callDataPtr  - pointer to SCALE-encoded InContractCall in WASM memory
//	callDataLen  - byte length of the encoded data
//
// Returns packed uint64:
//
//	high 32 bits = result pointer in WASM memory
//	low 32 bits  = result byte length
//	0 means no result
//
// execute 是宿主调用的主入口。
// 参数: isQuery(0=变更,非0=查询), callDataPtr(SCALE编码数据指针), callDataLen(字节长度)
// 返回打包 uint64: 高32位=结果指针, 低32位=结果长度, 0表示无结果
//
//go:wasmexport execute
func execute(isQuery uint32, callDataPtr uint32, callDataLen uint32) uint64 {
	if callDataLen == 0 {
		return 0
	}
	data := unsafe.Slice((*byte)(unsafe.Pointer(uintptr(callDataPtr))), int(callDataLen))

	var call store.InContractCall
	if _, err := call.Decode(data); err != nil {
		n := copy(buf[:], err.Error())
		return packResult(uint32(uintptr(unsafe.Pointer(&buf[0]))), uint32(n))
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
	return packResult(uint32(uintptr(unsafe.Pointer(&buf[0]))), uint32(copy(buf[:], result)))
}

// packResult packs (ptr, length) into a uint64: high 32 bits = ptr, low 32 bits = length.
func packResult(ptr, length uint32) uint64 {
	return uint64(ptr)<<32 | uint64(length)
}

// wasmRuntime adapts the WASM environment to the contract API interface.
// wasmRuntime 将 WASM 环境适配为合约 API 接口。

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

// VerifyTEE 通过 host verify_tee 函数校验客户端 TEE 报告，
// 返回 host 写回的 Result[TeeVerifyInfo, string]（SCALE 解码）。
func (r *wasmRuntime) VerifyTEE(report []byte) store.Result[store.TeeVerifyInfo, string] {
	var reportPtr int32
	if len(report) > 0 {
		reportPtr = int32(uintptr(unsafe.Pointer(&report[0])))
	}
	bp := txnBufPtr()
	n := verifyTEE(reportPtr, int32(len(report)), bp, int32(len(txnBuf)))
	if n <= 0 {
		return store.NewErr[store.TeeVerifyInfo, string](fmt.Sprintf("verify_tee: host returned %d", n))
	}
	var res store.Result[store.TeeVerifyInfo, string]
	if _, err := res.Decode(txnBuf[:n]); err != nil {
		return store.NewErr[store.TeeVerifyInfo, string](fmt.Sprintf("verify_tee: decode result: %v", err))
	}
	return res
}

func (r *wasmRuntime) Query(target store.UniAddr, call store.ContractCall) ([]byte, error) {
	return module.Query(call, r)
}

func (r *wasmRuntime) Call(target store.UniAddr, call store.ContractCall) ([]byte, error) {
	return nil, module.Mutation(call, r)
}

func (r *wasmRuntime) QueryNative(call store.ContractCall) ([]byte, error) {
	return module.Query(call, r)
}
