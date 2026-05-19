package evm

// ── Relay 同步 (stub — EVM Token 暂未改造 relay) ─────────

func (e *EvmChain) GetLatestNonce() (uint64, error) { return 0, nil }
func (e *EvmChain) GetRelayEvent(nonce uint64) (target, typ, data []byte, err error) {
	return nil, nil, nil, nil
}
func (e *EvmChain) GetChainID() uint32 { return 0 }
