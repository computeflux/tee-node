// Package contracts 提供侧链合约的版本化调用接口和路由分发机制。
// 支持两种执行路径：
//  1. 原生 Go 快速路径：版本号与当前二进制一致时，直接调用编译进二进制的 Go 函数
//  2. WASM 兼容路径：版本号与当前二进制不一致时，从数据库加载对应版本的 WASM 字节码执行
package contracts

import (
	"sync"
	"time"

	"github.com/computeflux/tee-provider/pkg/chaintypes"
	dbstore "github.com/computeflux/tee-provider/pkg/store"
	"github.com/computeflux/tee-provider/pkg/util"
	"github.com/computeflux/tee-provider/tee-chain/contracts/module"
	"github.com/computeflux/tee-provider/tee-chain/contracts/module/gateway"
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
	if version == module.NativeVersion {
		return module.Query(call, runtime)
	}

	util.LogWithRed("CONTRACT VESION", "Query with WASM")
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

	if version == module.NativeVersion {
		err := module.Mutation(call, runtime)
		if selector == store.MethodToSelector("Init") && err == nil {
			return SetContractInited(runtime.GetTxn(), string(call.GetName()))
		}
		return err
	}

	util.LogWithYellow("CONTRACT VESION", "Mutation with WASM")
	err := getWasmEngine().ExecuteMutation(version, call, runtime)
	if selector == store.MethodToSelector("Init") && err == nil {
		return SetContractInited(runtime.GetTxn(), string(call.GetName()))
	}
	return err
}

func QueryWithdrawApply(id uint64) (*gateway.WithdrawApply, error) {
	txn := dbstore.DBINS.NewTransaction()
	defer txn.Rollback()
	runtime := NewRuntimeWithVersion(0, time.Now(), txn, chaintypes.UniAddr{}, chaintypes.UniAddr{}, module.NativeVersion)

	ins := gateway.NewGateway(runtime)
	query := gateway.GatewayQuery{Gateway: *ins}
	return query.Withdraw(id)
}
