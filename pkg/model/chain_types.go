package model

import (
	"github.com/ethereum/go-ethereum/ethclient"
	chain "github.com/wetee-dao/ink.go"
)

// ── ChainClient ───────────────────────────────────────────

type ChainClient struct {
	Substrate *chain.ChainClient
	EVM       *ethclient.Client
}

// ── Chain-agnostic types ──────────────────────────────────

// ChainCall is a chain-agnostic extrinsic call.
//
//	Substrate: Data contains SCALE-encoded (CallIndex || Args).
//	EVM:       Data contains ABI-encoded calldata.
type ChainCall struct {
	Data   []byte // encoded call payload
	Method string // optional method name (for logging/debugging)
}

// ChainAccountID is a 32-byte account identifier (Substrate AccountId32).
type ChainAccountID [32]byte

// ChainH160 is a 20-byte Ethereum-style address.
type ChainH160 [20]byte

// Hex returns the hex-encoded address with 0x prefix.
func (a ChainH160) Hex() string {
	const hex = "0123456789abcdef"
	out := make([]byte, 40)
	for i := 0; i < 20; i++ {
		out[i*2] = hex[a[i]>>4]
		out[i*2+1] = hex[a[i]&0xf]
	}
	return "0x" + string(out)
}

// ── Cross-chain Points Events ─────────────────────────────

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
