package evm

import "github.com/computeflux/tee-provider/pkg/model"

func (e *EvmChain) GetLatestNonce() (uint64, error) { return 0, nil }

func (e *EvmChain) GetRelayEvents(from, to uint64) ([]model.RelayEvent, error) {
	return nil, nil
}
