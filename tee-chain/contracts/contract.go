// Package contracts 提供侧链合约的标准调用接口和路由分发机制。
// 支持 Query（查询）和 Mutation（状态变更）。
package contracts

import (
	"fmt"

	"github.com/computeflux/tee-provider/pkg/model"
	"github.com/computeflux/tee-provider/tee-chain/contracts/auth"
	"github.com/computeflux/tee-provider/tee-chain/contracts/gateway"
	"github.com/computeflux/tee-provider/tee-chain/contracts/gov"
)

// Query 执行合约的查询操作（只读操作）。
func Query(call *model.ContractCall, runtime model.ContractApi) (data []byte, err error) {
	switch string(call.Name) {
	case "gov":
		ins := gov.NewGov(runtime)
		return gov.GovQuery{Gov: *ins}.ExecQuery(call)
	case "auth":
		ins := auth.NewAuth(runtime)
		return auth.AuthQuery{Auth: *ins}.ExecQuery(call)
	case "gateway":
		ins := gateway.NewGateway(runtime)
		return gateway.GatewayQuery{Gateway: *ins}.ExecQuery(call)
	default:
		return nil, fmt.Errorf("unsupported contract: %s", string(call.Name))
	}
}

// Mutation 执行合约的状态变更操作（写操作）。
func Mutation(call *model.ContractCall, runtime model.ContractApi) error {
	selector := [4]byte(call.Method[:4])
	if selector == model.MethodToSelector("Init") && IsContractIsInited(runtime.GetTxn(), string(call.Name)) {
		return nil
	}

	var err error
	switch string(call.Name) {
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
	default:
		return fmt.Errorf("unsupported contract: %s", string(call.Name))
	}

	if selector == model.MethodToSelector("Init") && err == nil {
		return SetContractInited(runtime.GetTxn(), string(call.Name))
	}

	// util.LogOk("Contract Call "+string(call.Name)+" -> err", err)
	return err
}
