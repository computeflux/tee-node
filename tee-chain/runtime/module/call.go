package module

import (
	"github.com/computeflux/tee-provider/tee-chain/runtime/module/auth"
	"github.com/computeflux/tee-provider/tee-chain/runtime/module/batch"
	"github.com/computeflux/tee-provider/tee-chain/runtime/module/gateway"
	"github.com/computeflux/tee-provider/tee-chain/runtime/module/gov"
	"github.com/computeflux/tee-provider/tee-chain/runtime/store"
)

// NativeVersion is the version number for native (non-WASM) contract execution.
const NativeVersion = uint32(6)

// nativeMutation 原生 Go 合约状态变更路由。
func Mutation(call store.ContractCall, runtime store.ContractApi) error {
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
		// 未知合约：静默忽略，保持与旧版 dispatchMutation 行为一致
		return nil
	}
	return err
}

// Query 原生 Go 合约查询路由。
func Query(call store.ContractCall, runtime store.ContractApi) (data []byte, err error) {
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
		// 未知合约：返回空结果，保持与旧版 dispatchQuery 行为一致
		return nil, nil
	}
}
