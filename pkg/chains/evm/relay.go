package evm

func (e *EvmChain) GetLatestNonce() (uint64, error) { return 0, nil }
func (e *EvmChain) GetRelayEvent(nonce uint64) (target, typ, data []byte, err error) {
	return nil, nil, nil, nil
}
