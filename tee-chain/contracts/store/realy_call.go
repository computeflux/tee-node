package store

type WithdrawApply struct {
	ChainID  uint32
	Contract []byte
	User     UniAddr
	Credit   U256
	Nonce    uint64
}
