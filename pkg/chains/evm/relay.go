package evm

func (e *EvmChain) GetLatestNonce() (uint64, error) { return 0, nil }
func (e *EvmChain) GetRelayEvents(from, to uint64) (targets, types, datas [][]byte, err error) {
	return nil, nil, nil, nil
}
func (e *EvmChain) GetRelayEvent(nonce uint64) (target, typ, data []byte, err error) {
	return nil, nil, nil, nil
}
func (e *EvmChain) GetChainID() uint32 { return 0 }
