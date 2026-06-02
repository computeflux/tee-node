package batch

import "github.com/computeflux/tee-provider/pkg/model"

// BatchCall 用于外部输入，SCALE 编码后作为 execute_batch 的参数。
// 与 model.ContractCall 字段对齐，方便 codec 编解码。
type BatchCall struct {
	Name   []byte   `json:"name"`
	Method []byte   `json:"method"`
	Args   [][]byte `json:"args"`
}

// ToContractCall 将 BatchCall 转换为内部 ContractCall。
func (bc BatchCall) ToContractCall() model.ContractCall {
	return model.ContractCall{
		Name:   bc.Name,
		Method: bc.Method,
		Args:   bc.Args,
	}
}
