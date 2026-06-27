package model

import "github.com/computeflux/tee-provider/tee-chain/runtime/store"

func (m *ContractCall) ToStore() store.ContractCall {
	return &store.InContractCall{
		Name:   m.GetName(),
		Method: m.GetMethod(),
		Args:   m.GetArgs(),
	}
}
