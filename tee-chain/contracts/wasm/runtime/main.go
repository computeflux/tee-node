//go:build wasm

package main

import (
	"encoding/binary"
	"unsafe"

	"github.com/computeflux/tee-provider/tee-chain/contracts/module/auth"
	"github.com/computeflux/tee-provider/tee-chain/contracts/module/batch"
	"github.com/computeflux/tee-provider/tee-chain/contracts/module/gateway"
	"github.com/computeflux/tee-provider/tee-chain/contracts/module/gov"
	"github.com/computeflux/tee-provider/tee-chain/contracts/store"
)

var buf [8192]byte

func bufPtr() uint32 { return uint32(uintptr(unsafe.Pointer(&buf[0]))) }

//go:wasmimport runtime get_call_data
func getCallData(outPtr, outLen int32) int32

//go:wasmimport runtime set_result
func setResult(ptr, len int32)

//go:wasmimport runtime get_height
func hostGetHeight() int64

//go:wasmexport ping
func ping() int32 { return 1 }

func main() {
	// Go wasip1 仅支持 command 模式（_start → 执行 → exit）。
	// main() 中直接调用 execute()，让合约执行发生在 _start 期间。
	// execute() 通过 runtime.set_result 将结果写回宿主，
	// 之后 Go runtime 调用 proc_exit 关闭模块（每次查询独立实例化）。
	execute()
}

//go:wasmexport execute
func execute() {
	bp := bufPtr()
	n := getCallData(int32(bp), int32(len(buf)))
	if n <= 1 {
		return
	}
	data := buf[:n]

	// 第一个字节: 0x00=Query, 0x01=Mutation
	isQuery := data[0] == 0x00
	off := 1

	nameLen := binary.LittleEndian.Uint32(data[off:])
	off += 4
	name := string(data[off : off+int(nameLen)])
	off += int(nameLen)
	method := data[off : off+4]
	off += 4
	argsCount := binary.LittleEndian.Uint32(data[off:])
	off += 4
	args := make([][]byte, argsCount)
	for i := uint32(0); i < argsCount; i++ {
		argLen := binary.LittleEndian.Uint32(data[off:])
		off += 4
		args[i] = data[off : off+int(argLen)]
		off += int(argLen)
	}

	rt := &wasmRuntime{txn: &WasmTxn{}, height: hostGetHeight()}
	call := &store.InContractCall{Name: []byte(name), Method: method, Args: args}

	var result []byte
	var err error
	if isQuery {
		result, err = dispatchQuery(name, method, args, rt, call)
	} else {
		err = dispatchMutation(name, call, rt)
	}
	if err != nil {
		result = []byte(err.Error())
	}
	setResult(int32(bp), int32(copy(buf[:], result)))
}

func dispatchQuery(name string, method []byte, args [][]byte, rt *wasmRuntime, call store.ContractCall) ([]byte, error) {
	switch name {
	case "gov":
		ins := gov.NewGov(rt)
		return gov.GovQuery{Gov: *ins}.ExecQuery(call)
	case "auth":
		ins := auth.NewAuth(rt)
		return auth.AuthQuery{Auth: *ins}.ExecQuery(call)
	case "gateway":
		ins := gateway.NewGateway(rt)
		return gateway.GatewayQuery{Gateway: *ins}.ExecQuery(call)
	case "batch":
		ins := batch.NewBatch(rt)
		return batch.BatchQuery{Batch: *ins}.ExecQuery(call)
	}
	return nil, nil
}

func dispatchMutation(name string, call store.ContractCall, rt *wasmRuntime) error {
	switch name {
	case "gov":
		ins := gov.NewGov(rt)
		return gov.GovMutation{Gov: *ins}.ExecCall(call)
	case "auth":
		ins := auth.NewAuth(rt)
		return auth.AuthMutation{Auth: *ins}.ExecCall(call)
	case "gateway":
		ins := gateway.NewGateway(rt)
		return gateway.GatewayMutation{Gateway: *ins}.ExecCall(call)
	case "batch":
		ins := batch.NewBatch(rt)
		return batch.BatchMutation{Batch: *ins}.ExecCall(call)
	}
	return nil
}

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
	return doCrossContractQuery(call)
}

func (r *wasmRuntime) Call(target store.UniAddr, call store.ContractCall) ([]byte, error) {
	return doCrossContractCall(call)
}

func (r *wasmRuntime) QueryNative(call store.ContractCall) ([]byte, error) {
	return doCrossContractQuery(call)
}
