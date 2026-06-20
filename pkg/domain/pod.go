package domain

import (
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
