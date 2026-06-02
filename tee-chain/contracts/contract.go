// Package contracts 提供侧链合约的版本化调用接口和路由分发机制。
// 支持两种执行路径：
//  1. 原生 Go 快速路径：版本号与当前二进制一致时，直接调用编译进二进制的 Go 函数
//  2. WASM 兼容路径：版本号与当前二进制不一致时，从数据库加载对应版本的 WASM 字节码执行
package contracts

import (
	"fmt"
	"sync"

	"github.com/computeflux/tee-provider/tee-chain/contracts/module/auth"
	"github.com/computeflux/tee-provider/tee-chain/contracts/module/batch"
	"github.com/computeflux/tee-provider/tee-chain/contracts/module/gateway"
	"github.com/computeflux/tee-provider/tee-chain/contracts/module/gov"
	"github.com/computeflux/tee-provider/tee-chain/contracts/store"
	"github.com/computeflux/tee-provider/tee-chain/contracts/wasm"
)

var (
	wasmEngine     *wasm.Engine
	wasmEngineOnce sync.Once
)

// getWasmEngine 获取全局 WASM 执行引擎（懒加载）。
func getWasmEngine() *wasm.Engine {
	wasmEngineOnce.Do(func() {
		wasmEngine = wasm.NewEngine()
	})
	return wasmEngine
}

// Query 执行合约的查询操作（只读操作）。
// 根据 version 选择执行路径：
//   - version == wasm.NativeVersion：使用原生 Go 快速路径
//   - version != wasm.NativeVersion：使用 WASM 兼容路径
func Query(version uint32, call store.ContractCall, runtime store.ContractApi) (data []byte, err error) {
	if version == wasm.NativeVersion {
		return nativeQuery(call, runtime)
	}
	return getWasmEngine().ExecuteQuery(version, call, runtime)
}

// Mutation 执行合约的状态变更操作（写操作）。
// 根据 version 选择执行路径：
//   - version == wasm.NativeVersion：使用原生 Go 快速路径
//   - version != wasm.NativeVersion：使用 WASM 兼容路径
func Mutation(version uint32, call store.ContractCall, runtime store.ContractApi) error {
	selector := [4]byte(call.GetMethod()[:4])
	if selector == store.MethodToSelector("Init") && IsContractIsInited(runtime.GetTxn(), string(call.GetName())) {
		return nil
	}

	if version == wasm.NativeVersion {
		err := nativeMutation(call, runtime)
		if selector == store.MethodToSelector("Init") && err == nil {
			return SetContractInited(runtime.GetTxn(), string(call.GetName()))
		}
		return err
	}

	err := getWasmEngine().ExecuteMutation(version, call, runtime)
	if selector == store.MethodToSelector("Init") && err == nil {
		return SetContractInited(runtime.GetTxn(), string(call.GetName()))
	}
	return err
}

// nativeQuery 原生 Go 合约查询路由。
func nativeQuery(call store.ContractCall, runtime store.ContractApi) (data []byte, err error) {
	switch string(call.GetName()) {
	case "gov":
		ins := gov.NewGov(runtime)
		return gov.GovQuery{Gov: *ins}.ExecQuery(call)
	case "auth":
		ins := auth.NewAuth(runtime)
		return auth.AuthQuery{Auth: *ins}.ExecQuery(call)
	case "gateway":
		ins := gateway.NewGateway(runtime)
		return gateway.GatewayQuery{Gateway: *ins}.ExecQuery(call)
	case "batch":
		ins := batch.NewBatch(runtime)
		return batch.BatchQuery{Batch: *ins}.ExecQuery(call)
	default:
		return nil, fmt.Errorf("unsupported contract: %s", string(call.GetName()))
	}
}

// nativeMutation 原生 Go 合约状态变更路由。
func nativeMutation(call store.ContractCall, runtime store.ContractApi) error {
	var err error
	switch string(call.GetName()) {
	case "gov":
		ins := gov.NewGov(runtime)
		mutation := gov.GovMutation{Gov: *ins}
		err = mutation.ExecCall(call)
	case "auth":
		ins := auth.NewAuth(runtime)
		mutation := auth.AuthMutation{Auth: *ins}
		err = mutation.ExecCall(call)
	case "gateway":
		ins := gateway.NewGateway(runtime)
		mutation := gateway.GatewayMutation{Gateway: *ins}
		err = mutation.ExecCall(call)
	case "batch":
		ins := batch.NewBatch(runtime)
		mutation := batch.BatchMutation{Batch: *ins}
		err = mutation.ExecCall(call)
	default:
		return fmt.Errorf("unsupported contract: %s", string(call.GetName()))
	}
	return err
}
