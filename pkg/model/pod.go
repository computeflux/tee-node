package model

import (
	"encoding/json"
	"fmt"
	"math/big"

	"github.com/centrifuge/go-substrate-rpc-client/v4/types"
)

type PodVersion struct {
	PodId    uint64
	Version  uint32
	LastMint uint32
	Status   uint8
}

type Container struct { // Composite
	Image   []byte
	Command Command
	Port    []Service
	Cpu     uint32
	Mem     uint32
	Disk    []ContainerDisk
	Gpu     uint32
	Env     []Env
}

type ContainerDisk struct { // Composite
	Id   uint32
	Path []byte
	Size uint32
}

// type Disk struct { // Composite
// 	Path DiskClass
// 	Size uint32
// }

type DiskClass struct { // Enum
	SSD *[]byte // 0
}

type Service struct { // Enum
	Tcp        *uint16 // 0
	Udp        *uint16 // 1
	Http       *uint16 // 2
	Https      *uint16 // 3
	ProjectTcp *uint16 // 4
	ProjectUdp *uint16 // 5
}

type Command struct { // Enum
	SH   *[]byte // 0
	BASH *[]byte // 1
	ZSH  *[]byte // 2
	NONE *bool   // 3
}

type Pod struct {
	PodId               uint64
	Meta                []byte
	Containers          []Container
	Owner               types.H160
	Ptype               PodType
	TeeType             TEEType
	Version             uint32
	Status              uint8
	LastMintBlockNumber uint32
	SkipUtil            uint32
	DeploySkipUtil      uint32
}

type PodType struct { // Enum
	CPU    *bool // 0
	GPU    *bool // 1
	SCRIPT *bool // 2
}

type TEEType struct { // Enum
	SGX *bool // 0
	CVM *bool // 1
}

func GetUrlFromIp(ip Ip) string {
	url := ""
	if !ip.Domain.IsNone() {
		url = "/dns4/" + string(ip.Domain.V)
	} else if !ip.Ipv4.IsNone() {
		ipv4 := ip.Ipv4.V
		url = "/ip4/" + fmt.Sprintf("%d.%d.%d.%d",
			(ipv4>>24)&0xFF,
			(ipv4>>16)&0xFF,
			(ipv4>>8)&0xFF,
			ipv4&0xFF)
	} else if !ip.Ipv6.IsNone() {
		ipv6 := ip.Ipv6.V
		ipv6Int128 := big.NewInt(0)
		ipv6Int128.SetBytes(ipv6.Bytes())
		url = "/ip6/" + fmt.Sprintf("%04x:%04x:%04x:%04x:%04x:%04x:%04x:%04x",
			ipv6Int128.Rsh(ipv6Int128, 112).Uint64(),
			ipv6Int128.Rsh(ipv6Int128, 96).Uint64()&0xFFFF,
			ipv6Int128.Rsh(ipv6Int128, 80).Uint64()&0xFFFF,
			ipv6Int128.Rsh(ipv6Int128, 64).Uint64()&0xFFFF,
			ipv6Int128.Rsh(ipv6Int128, 48).Uint64()&0xFFFF,
			ipv6Int128.Rsh(ipv6Int128, 32).Uint64()&0xFFFF,
			ipv6Int128.Rsh(ipv6Int128, 16).Uint64()&0xFFFF,
			ipv6Int128.Uint64()&0xFFFF)
	}
	return url
}

type Env struct { // Enum
	Env *struct { // 0
		F0 []byte
		F1 []byte
	}
	File *struct { // 1
		F0 []byte
		F1 []byte
	}
	Encrypt *struct { // 2
		F0 []byte
		F1 uint64
	}
}

func CopyWithJSON[T any, V any](from T) V {
	bt, _ := json.Marshal(from)
	to := new(V)
	json.Unmarshal(bt, to)
	return *to
}
