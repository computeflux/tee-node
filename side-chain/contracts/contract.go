// Package contracts 提供侧链合约的标准调用接口和路由分发机制。
// 本包定义了合约的统一入口，支持 Query（查询）和 Mutation（状态变更）两类操作。
// 各合约（如 dao、token 等）实现相应接口后，通过本包的 Query/Mutation 函数进行统一调用分发。
package contracts

import (
	"fmt"

	"github.com/wetee-dao/tee-dsecret/pkg/model"
	"github.com/wetee-dao/tee-dsecret/pkg/util"
	"github.com/wetee-dao/tee-dsecret/side-chain/contracts/gov"
)

// Query 执行合约的查询操作（只读操作）。
// 查询操作不会修改链上状态，仅返回查询结果，适用于读取账户余额、获取配置等场景。
//
// 参数:
//   - call: 合约调用请求，包含合约名称、方法名和参数
//   - runtime: 合约运行时上下文，提供区块高度、交易信息和调用者身份
//
// 返回:
//   - []byte: 查询结果的字节切片
//   - error: 错误信息，当合约不存在或执行失败时返回
//
// 支持的合约:
//   - "dao": 去中心化自治组织合约，提供提案查询、投票状态等功能
func Query(call *model.ContractCall, runtime model.ContractApi) (data []byte, err error) {
	switch string(call.Name) {
	case "gov":
		// 创建 DAO 合约实例并执行查询
		ins := gov.NewGov(runtime)
		query := gov.GovQuery{Gov: *ins}
		return query.ExecQuery(call)
	default:
		return nil, fmt.Errorf("unsupported contract: %s", string(call.Name))
	}
}

// Mutation 执行合约的状态变更操作（写操作）。
// Mutation 操作会修改链上状态，包括转账、投票、创建提案等需要共识确认的操作。
// 每次 Mutation 调用都会被记录在区块中，成为不可篡改的交易历史。
//
// 参数:
//   - call: 合约调用请求，包含合约名称、方法名和参数
//   - runtime: 合约运行时上下文，提供区块高度、交易信息和调用者身份
//
// 返回:
//   - error: 错误信息，当合约不存在或执行失败时返回
//
// 支持的合约:
//   - "dao": 去中心化自治组织合约，提供提案创建、投票执行等功能
func Mutation(call *model.ContractCall, runtime model.ContractApi) error {
	var selector [4]byte = [4]byte(call.Method[:4])
	if selector == model.MethodToSelector("Init") && IsContractIsInited(runtime.GetTxn(), string(call.Name)) {
		return nil
	}

	var err error
	var method string
	switch string(call.Name) {
	case "gov":
		ins := gov.NewGov(runtime)
		mutation := gov.GovMutation{Gov: *ins}
		err = mutation.ExecCall(call)
		method = gov.MethodMap[selector]
	default:
		return fmt.Errorf("unsupported contract: %s", string(call.Name))
	}

	if selector == model.MethodToSelector("Init") && err == nil {
		return SetContractInited(runtime.GetTxn(), string(call.Name))
	}

	util.LogOk("Contract Call "+string(call.Name)+"."+method+" -> err", err)

	return err
}
