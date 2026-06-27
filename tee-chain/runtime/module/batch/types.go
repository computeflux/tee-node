package batch

import (
	"github.com/computeflux/tee-provider/tee-chain/runtime/store"
)

// BatchCall 用于外部输入，SCALE 编码后作为 execute_batch 的参数。
// 与 store.ContractCall 字段对齐，方便 codec 编解码。
// scale-codec-gen:generate
type BatchCall struct {
	Name   []byte   `json:"name"`
	Method []byte   `json:"method"`
	Args   [][]byte `json:"args"`
}

// ToContractCall 将 BatchCall 转换为内部 ContractCall。
func (bc BatchCall) ToContractCall() store.ContractCall {
	return &store.InContractCall{
		Name:   bc.Name,
		Method: bc.Method,
		Args:   bc.Args,
	}
}

const (
	HubCallPending   uint8 = 0
	HubCallCancelled uint8 = 1
	HubCallDone      uint8 = 2
)

// scale-codec-gen:generate
type HubCallIndex struct {
	// 0 => WithdrawApply
	CallType uint8
	CallID   uint64
	ChainID  uint32 `json:"chainId" idx:"exact"`
	Status   uint8  `json:"status" idx:"exact"`
}

type HubCallBatch struct {
	ChainID uint32
	IDs     []uint64
}
