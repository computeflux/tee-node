package model

import (
	"github.com/centrifuge/go-substrate-rpc-client/v4/types"
	"github.com/ethereum/go-ethereum/ethclient"
	chain "github.com/wetee-dao/ink.go"
)

// ── ChainClient ───────────────────────────────────────────

type ChainClient struct {
	Substrate *chain.ChainClient
	EVM       *ethclient.Client
}

type ChainConfigType struct {
	ChainID       uint32
	ChainType     string
	Cipher        SuiteType
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
type ChainCall struct {
	EVM       EvmCall
	Substrate types.Call
}

type EvmCall struct {
	Method string
	Data   []byte
}

// ── Cross-chain Points Events ─────────────────────────────

// RelayEvent 从主链合约拉取的 relay 事件 | Relay event fetched from main chain contract
type RelayEvent struct {
	TargetContract []byte
	EventType      []byte
	EventData      []byte
}

type RechargeEvent struct {
	User   UniAddr `json:"user"`
	Amount uint64  `json:"amount"`
	TxHash []byte  `json:"txHash"`
	Block  uint64  `json:"block"`
	Chain  string  `json:"chain"`
}

type WithdrawEvent struct {
	User   UniAddr `json:"user"`
	Amount uint64  `json:"amount"`
	TxHash []byte  `json:"txHash"`
	Block  uint64  `json:"block"`
	Chain  string  `json:"chain"`
}

type PointsAccount struct {
	User           UniAddr `json:"user"`
	TotalRecharged uint64  `json:"totalRecharged"`
	TotalSpent     uint64  `json:"totalSpent"`
}

func (p PointsAccount) Balance() uint64 {
	if p.TotalRecharged < p.TotalSpent {
		return 0
	}
	return p.TotalRecharged - p.TotalSpent
}
