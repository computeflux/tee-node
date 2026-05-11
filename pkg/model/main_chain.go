package model

import (
	"encoding/hex"
	"fmt"
	"math/big"

	"github.com/centrifuge/go-substrate-rpc-client/v4/types"
	"github.com/cometbft/cometbft/crypto"
	"github.com/wetee-dao/ink.go/util"
)

// P2PAddr
type P2PAddr struct {
	Ip   Ip
	Port uint16
	Id   [32]byte
}

// Get side chain node id
func (p *P2PAddr) SideChainUrl() string {
	return hex.EncodeToString(crypto.AddressHash(p.Id[:])) + "@" + p.Ip.ToString() + ":" + fmt.Sprint(p.Port)
}

// Ip
type Ip struct {
	Ipv4   util.Option[uint32]
	Ipv6   util.Option[types.U128]
	Domain util.Option[[]byte]
}

// ToString
func (ip *Ip) ToString() string {
	url := ""
	if !ip.Domain.IsNone() {
		url = string(ip.Domain.V)
	} else if !ip.Ipv4.IsNone() {
		ipv4 := ip.Ipv4.V
		url = fmt.Sprintf("%d.%d.%d.%d",
			(ipv4>>24)&0xFF,
			(ipv4>>16)&0xFF,
			(ipv4>>8)&0xFF,
			ipv4&0xFF)
	} else if !ip.Ipv6.IsNone() {
		ipv6 := ip.Ipv6.V
		ipv6Int128 := big.NewInt(0)
		ipv6Int128.SetBytes(ipv6.Bytes())
		url = "[" + fmt.Sprintf("%04x:%04x:%04x:%04x:%04x:%04x:%04x:%04x",
			ipv6Int128.Rsh(ipv6Int128, 112).Uint64(),
			ipv6Int128.Rsh(ipv6Int128, 96).Uint64()&0xFFFF,
			ipv6Int128.Rsh(ipv6Int128, 80).Uint64()&0xFFFF,
			ipv6Int128.Rsh(ipv6Int128, 64).Uint64()&0xFFFF,
			ipv6Int128.Rsh(ipv6Int128, 48).Uint64()&0xFFFF,
			ipv6Int128.Rsh(ipv6Int128, 32).Uint64()&0xFFFF,
			ipv6Int128.Rsh(ipv6Int128, 16).Uint64()&0xFFFF,
			ipv6Int128.Uint64()&0xFFFF) + "]"
	}
	return url
}

type K8sCluster struct {
	Id            uint64
	Name          []byte
	Owner         types.H160
	Level         byte
	RegionId      uint32
	StartBlock    uint32
	StopBlock     util.Option[uint32]
	TerminalBlock util.Option[uint32]
	P2pId         types.AccountID
	Ip            Ip
	Port          uint32
	Status        byte
}

type Validator struct {
	// node id
	NodeID uint64
	// account32
	ValidatorId PubKey
	// account32
	P2pId PubKey
}
