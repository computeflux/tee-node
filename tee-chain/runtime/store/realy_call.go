package store

type WithdrawStatus uint8

const (
	WithdrawPending   WithdrawStatus = 0
	WithdrawCancelled WithdrawStatus = 1
	WithdrawDone      WithdrawStatus = 2
)

// scale-codec-gen:generate
type WithdrawApply struct {
	ChainID  uint32
	Contract []byte
	User     UniAddr
	Credit   U256
	Nonce    uint64
	Status   WithdrawStatus
}
