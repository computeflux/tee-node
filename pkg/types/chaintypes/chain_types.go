package chaintypes

import (
	"github.com/centrifuge/go-substrate-rpc-client/v4/types"
	"github.com/computeflux/tee-provider/pkg/types/crypto"
	"github.com/computeflux/tee-provider/tee-chain/contracts/store"
)

// ── ChainClient ───────────────────────────────────────────
type ChainConfigType struct {
	ChainID       uint32
	ChainType     string
	Cipher        crypto.SuiteType
	Urls          []string
	SubnetAddress string
	TokenAddress  string
	CloudAddress  string // Substrate Cloud contract address
	BLSAddress    string // BLS gateway contract address
	NetworkLabel  string
}

// ── Chain-agnostic types ──────────────────────────────────

// ChainCall is a chain-agnostic extrinsic call.
//
//	Substrate: Data contains SCALE-encoded (CallIndex || Args).
//	EVM:       Data contains ABI-encoded calldata.
//
// scale-codec-gen:generate
type ChainCall struct {
	EVM       EvmCall
	Substrate types.Call
}

// scale-codec-gen:generate
type EvmCall struct {
	Method string
	Data   []byte
}

// ── Cross-chain Points Events ─────────────────────────────

// RelayEvent 从主链合约拉取的 relay 事件 | Relay event fetched from main chain contract
type RelayEvent struct {
	TargetContract []byte
	EventType      []byte
	EventData      [][]byte
}

type RechargeEvent struct {
	User   store.UniAddr `json:"user"`
	Amount uint64        `json:"amount"`
	TxHash []byte        `json:"txHash"`
	Block  uint64        `json:"block"`
	Chain  string        `json:"chain"`
}

type WithdrawEvent struct {
	User   store.UniAddr `json:"user"`
	Amount uint64        `json:"amount"`
	TxHash []byte        `json:"txHash"`
	Block  uint64        `json:"block"`
	Chain  string        `json:"chain"`
}

type PointsAccount struct {
	User           store.UniAddr `json:"user"`
	TotalRecharged uint64        `json:"totalRecharged"`
	TotalSpent     uint64        `json:"totalSpent"`
}
