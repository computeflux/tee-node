package cloud

import (
	"fmt"

	"github.com/centrifuge/go-substrate-rpc-client/v4/scale"
	"github.com/centrifuge/go-substrate-rpc-client/v4/types"
	"github.com/wetee-dao/ink.go/util"
)

type Error struct { // Enum
	SetCodeFailed              *bool // 0
	MustCallByGovContract      *bool // 1
	WorkerNotOnline            *bool // 2
	WorkerLevelNotEnough       *bool // 3
	RegionNotMatch             *bool // 4
	NotPodOwner                *bool // 5
	PodKeyNotExist             *bool // 6
	PodStatusError             *bool // 7
	InvalidSideChainCaller     *bool // 8
	DelFailed                  *bool // 9
	NotFound                   *bool // 10
	PodNotFound                *bool // 11
	PodCodeNotFound            *bool // 12
	WorkerIdNotFound           *bool // 13
	WorkerNotFound             *bool // 14
	LevelPriceNotFound         *bool // 15
	AssetNotFound              *bool // 16
	BalanceNotEnough           *bool // 17
	PayFailed                  *bool // 18
	PodInstantiateFailed       *bool // 19
	ArbitrationNotFound        *bool // 20
	ArbitrationAlreadyResolved *bool // 21
	WorkerMortgageCheckFailed  *bool // 22
	InvalidFeeRate             *bool // 23
	InsufficientPrepayment     *bool // 24
	PodAlreadySettled          *bool // 25
	CallFailed                 *bool // 26
}

func (ty Error) Encode(encoder scale.Encoder) (err error) {
	if ty.SetCodeFailed != nil {
		err = encoder.PushByte(0)
		if err != nil {
			return err
		}
		return nil
	}

	if ty.MustCallByGovContract != nil {
		err = encoder.PushByte(1)
		if err != nil {
			return err
		}
		return nil
	}

	if ty.WorkerNotOnline != nil {
		err = encoder.PushByte(2)
		if err != nil {
			return err
		}
		return nil
	}

	if ty.WorkerLevelNotEnough != nil {
		err = encoder.PushByte(3)
		if err != nil {
			return err
		}
		return nil
	}

	if ty.RegionNotMatch != nil {
		err = encoder.PushByte(4)
		if err != nil {
			return err
		}
		return nil
	}

	if ty.NotPodOwner != nil {
		err = encoder.PushByte(5)
		if err != nil {
			return err
		}
		return nil
	}

	if ty.PodKeyNotExist != nil {
		err = encoder.PushByte(6)
		if err != nil {
			return err
		}
		return nil
	}

	if ty.PodStatusError != nil {
		err = encoder.PushByte(7)
		if err != nil {
			return err
		}
		return nil
	}

	if ty.InvalidSideChainCaller != nil {
		err = encoder.PushByte(8)
		if err != nil {
			return err
		}
		return nil
	}

	if ty.DelFailed != nil {
		err = encoder.PushByte(9)
		if err != nil {
			return err
		}
		return nil
	}

	if ty.NotFound != nil {
		err = encoder.PushByte(10)
		if err != nil {
			return err
		}
		return nil
	}

	if ty.PodNotFound != nil {
		err = encoder.PushByte(11)
		if err != nil {
			return err
		}
		return nil
	}

	if ty.PodCodeNotFound != nil {
		err = encoder.PushByte(12)
		if err != nil {
			return err
		}
		return nil
	}

	if ty.WorkerIdNotFound != nil {
		err = encoder.PushByte(13)
		if err != nil {
			return err
		}
		return nil
	}

	if ty.WorkerNotFound != nil {
		err = encoder.PushByte(14)
		if err != nil {
			return err
		}
		return nil
	}

	if ty.LevelPriceNotFound != nil {
		err = encoder.PushByte(15)
		if err != nil {
			return err
		}
		return nil
	}

	if ty.AssetNotFound != nil {
		err = encoder.PushByte(16)
		if err != nil {
			return err
		}
		return nil
	}

	if ty.BalanceNotEnough != nil {
		err = encoder.PushByte(17)
		if err != nil {
			return err
		}
		return nil
	}

	if ty.PayFailed != nil {
		err = encoder.PushByte(18)
		if err != nil {
			return err
		}
		return nil
	}

	if ty.PodInstantiateFailed != nil {
		err = encoder.PushByte(19)
		if err != nil {
			return err
		}
		return nil
	}

	if ty.ArbitrationNotFound != nil {
		err = encoder.PushByte(20)
		if err != nil {
			return err
		}
		return nil
	}

	if ty.ArbitrationAlreadyResolved != nil {
		err = encoder.PushByte(21)
		if err != nil {
			return err
		}
		return nil
	}

	if ty.WorkerMortgageCheckFailed != nil {
		err = encoder.PushByte(22)
		if err != nil {
			return err
		}
		return nil
	}

	if ty.InvalidFeeRate != nil {
		err = encoder.PushByte(23)
		if err != nil {
			return err
		}
		return nil
	}

	if ty.InsufficientPrepayment != nil {
		err = encoder.PushByte(24)
		if err != nil {
			return err
		}
		return nil
	}

	if ty.PodAlreadySettled != nil {
		err = encoder.PushByte(25)
		if err != nil {
			return err
		}
		return nil
	}

	if ty.CallFailed != nil {
		err = encoder.PushByte(26)
		if err != nil {
			return err
		}
		return nil
	}
	return fmt.Errorf("unrecognized enum")
}

func (ty *Error) Decode(decoder scale.Decoder) (err error) {
	variant, err := decoder.ReadOneByte()
	if err != nil {
		return err
	}
	switch variant {
	case 0: // Base
		t := true
		ty.SetCodeFailed = &t
		return
	case 1: // Base
		t := true
		ty.MustCallByGovContract = &t
		return
	case 2: // Base
		t := true
		ty.WorkerNotOnline = &t
		return
	case 3: // Base
		t := true
		ty.WorkerLevelNotEnough = &t
		return
	case 4: // Base
		t := true
		ty.RegionNotMatch = &t
		return
	case 5: // Base
		t := true
		ty.NotPodOwner = &t
		return
	case 6: // Base
		t := true
		ty.PodKeyNotExist = &t
		return
	case 7: // Base
		t := true
		ty.PodStatusError = &t
		return
	case 8: // Base
		t := true
		ty.InvalidSideChainCaller = &t
		return
	case 9: // Base
		t := true
		ty.DelFailed = &t
		return
	case 10: // Base
		t := true
		ty.NotFound = &t
		return
	case 11: // Base
		t := true
		ty.PodNotFound = &t
		return
	case 12: // Base
		t := true
		ty.PodCodeNotFound = &t
		return
	case 13: // Base
		t := true
		ty.WorkerIdNotFound = &t
		return
	case 14: // Base
		t := true
		ty.WorkerNotFound = &t
		return
	case 15: // Base
		t := true
		ty.LevelPriceNotFound = &t
		return
	case 16: // Base
		t := true
		ty.AssetNotFound = &t
		return
	case 17: // Base
		t := true
		ty.BalanceNotEnough = &t
		return
	case 18: // Base
		t := true
		ty.PayFailed = &t
		return
	case 19: // Base
		t := true
		ty.PodInstantiateFailed = &t
		return
	case 20: // Base
		t := true
		ty.ArbitrationNotFound = &t
		return
	case 21: // Base
		t := true
		ty.ArbitrationAlreadyResolved = &t
		return
	case 22: // Base
		t := true
		ty.WorkerMortgageCheckFailed = &t
		return
	case 23: // Base
		t := true
		ty.InvalidFeeRate = &t
		return
	case 24: // Base
		t := true
		ty.InsufficientPrepayment = &t
		return
	case 25: // Base
		t := true
		ty.PodAlreadySettled = &t
		return
	case 26: // Base
		t := true
		ty.CallFailed = &t
		return
	default:
		return fmt.Errorf("unrecognized enum")
	}
}
func (ty *Error) Error() string {
	if ty.SetCodeFailed != nil {
		return "SetCodeFailed"
	}

	if ty.MustCallByGovContract != nil {
		return "MustCallByGovContract"
	}

	if ty.WorkerNotOnline != nil {
		return "WorkerNotOnline"
	}

	if ty.WorkerLevelNotEnough != nil {
		return "WorkerLevelNotEnough"
	}

	if ty.RegionNotMatch != nil {
		return "RegionNotMatch"
	}

	if ty.NotPodOwner != nil {
		return "NotPodOwner"
	}

	if ty.PodKeyNotExist != nil {
		return "PodKeyNotExist"
	}

	if ty.PodStatusError != nil {
		return "PodStatusError"
	}

	if ty.InvalidSideChainCaller != nil {
		return "InvalidSideChainCaller"
	}

	if ty.DelFailed != nil {
		return "DelFailed"
	}

	if ty.NotFound != nil {
		return "NotFound"
	}

	if ty.PodNotFound != nil {
		return "PodNotFound"
	}

	if ty.PodCodeNotFound != nil {
		return "PodCodeNotFound"
	}

	if ty.WorkerIdNotFound != nil {
		return "WorkerIdNotFound"
	}

	if ty.WorkerNotFound != nil {
		return "WorkerNotFound"
	}

	if ty.LevelPriceNotFound != nil {
		return "LevelPriceNotFound"
	}

	if ty.AssetNotFound != nil {
		return "AssetNotFound"
	}

	if ty.BalanceNotEnough != nil {
		return "BalanceNotEnough"
	}

	if ty.PayFailed != nil {
		return "PayFailed"
	}

	if ty.PodInstantiateFailed != nil {
		return "PodInstantiateFailed"
	}

	if ty.ArbitrationNotFound != nil {
		return "ArbitrationNotFound"
	}

	if ty.ArbitrationAlreadyResolved != nil {
		return "ArbitrationAlreadyResolved"
	}

	if ty.WorkerMortgageCheckFailed != nil {
		return "WorkerMortgageCheckFailed"
	}

	if ty.InvalidFeeRate != nil {
		return "InvalidFeeRate"
	}

	if ty.InsufficientPrepayment != nil {
		return "InsufficientPrepayment"
	}

	if ty.PodAlreadySettled != nil {
		return "PodAlreadySettled"
	}

	if ty.CallFailed != nil {
		return "CallFailed"
	}
	return "Unknown"
}

type AssetInfo struct { // Enum
	Native *[]byte   // 0
	ERC20  *struct { // 1
		F0 []byte
		F1 types.H256
	}
}

func (ty AssetInfo) Encode(encoder scale.Encoder) (err error) {
	if ty.Native != nil {
		err = encoder.PushByte(0)
		if err != nil {
			return err
		}
		err = encoder.Encode(*ty.Native)
		if err != nil {
			return err
		}
		return nil
	}

	if ty.ERC20 != nil {
		err = encoder.PushByte(1)
		if err != nil {
			return err
		}

		err = encoder.Encode(ty.ERC20.F0)
		if err != nil {
			return err
		}

		err = encoder.Encode(ty.ERC20.F1)
		if err != nil {
			return err
		}

		return nil
	}
	return fmt.Errorf("unrecognized enum")
}

func (ty *AssetInfo) Decode(decoder scale.Decoder) (err error) {
	variant, err := decoder.ReadOneByte()
	if err != nil {
		return err
	}
	switch variant {
	case 0: // Inline
		ty.Native = new([]byte)
		err = decoder.Decode(ty.Native)
		if err != nil {
			return err
		}
		return
	case 1: // Tuple
		ty.ERC20 = &struct {
			F0 []byte
			F1 types.H256
		}{}

		err = decoder.Decode(&ty.ERC20.F0)
		if err != nil {
			return err
		}

		err = decoder.Decode(&ty.ERC20.F1)
		if err != nil {
			return err
		}

		return
	default:
		return fmt.Errorf("unrecognized enum")
	}
}

type PodType struct { // Enum
	CPU    *bool // 0
	GPU    *bool // 1
	SCRIPT *bool // 2
}

func (ty PodType) Encode(encoder scale.Encoder) (err error) {
	if ty.CPU != nil {
		err = encoder.PushByte(0)
		if err != nil {
			return err
		}
		return nil
	}

	if ty.GPU != nil {
		err = encoder.PushByte(1)
		if err != nil {
			return err
		}
		return nil
	}

	if ty.SCRIPT != nil {
		err = encoder.PushByte(2)
		if err != nil {
			return err
		}
		return nil
	}
	return fmt.Errorf("unrecognized enum")
}

func (ty *PodType) Decode(decoder scale.Decoder) (err error) {
	variant, err := decoder.ReadOneByte()
	if err != nil {
		return err
	}
	switch variant {
	case 0: // Base
		t := true
		ty.CPU = &t
		return
	case 1: // Base
		t := true
		ty.GPU = &t
		return
	case 2: // Base
		t := true
		ty.SCRIPT = &t
		return
	default:
		return fmt.Errorf("unrecognized enum")
	}
}

type TEEType struct { // Enum
	SGX *bool // 0
	CVM *bool // 1
}

func (ty TEEType) Encode(encoder scale.Encoder) (err error) {
	if ty.SGX != nil {
		err = encoder.PushByte(0)
		if err != nil {
			return err
		}
		return nil
	}

	if ty.CVM != nil {
		err = encoder.PushByte(1)
		if err != nil {
			return err
		}
		return nil
	}
	return fmt.Errorf("unrecognized enum")
}

func (ty *TEEType) Decode(decoder scale.Decoder) (err error) {
	variant, err := decoder.ReadOneByte()
	if err != nil {
		return err
	}
	switch variant {
	case 0: // Base
		t := true
		ty.SGX = &t
		return
	case 1: // Base
		t := true
		ty.CVM = &t
		return
	default:
		return fmt.Errorf("unrecognized enum")
	}
}

type Pod struct { // Composite
	Name          []byte
	Owner         types.H160
	PodAddress    types.H160
	Ptype         PodType
	StartBlock    uint32
	TeeType       TEEType
	Level         byte
	PayAssetId    uint32
	PrepaidAmount types.U256
	EndBlock      uint32
	IsSettled     bool
	SettledAmount types.U256
}
type Command struct { // Enum
	SH   *[]byte // 0
	BASH *[]byte // 1
	ZSH  *[]byte // 2
	NONE *bool   // 3
}

func (ty Command) Encode(encoder scale.Encoder) (err error) {
	if ty.SH != nil {
		err = encoder.PushByte(0)
		if err != nil {
			return err
		}
		err = encoder.Encode(*ty.SH)
		if err != nil {
			return err
		}
		return nil
	}

	if ty.BASH != nil {
		err = encoder.PushByte(1)
		if err != nil {
			return err
		}
		err = encoder.Encode(*ty.BASH)
		if err != nil {
			return err
		}
		return nil
	}

	if ty.ZSH != nil {
		err = encoder.PushByte(2)
		if err != nil {
			return err
		}
		err = encoder.Encode(*ty.ZSH)
		if err != nil {
			return err
		}
		return nil
	}

	if ty.NONE != nil {
		err = encoder.PushByte(3)
		if err != nil {
			return err
		}
		return nil
	}
	return fmt.Errorf("unrecognized enum")
}

func (ty *Command) Decode(decoder scale.Decoder) (err error) {
	variant, err := decoder.ReadOneByte()
	if err != nil {
		return err
	}
	switch variant {
	case 0: // Inline
		ty.SH = new([]byte)
		err = decoder.Decode(ty.SH)
		if err != nil {
			return err
		}
		return
	case 1: // Inline
		ty.BASH = new([]byte)
		err = decoder.Decode(ty.BASH)
		if err != nil {
			return err
		}
		return
	case 2: // Inline
		ty.ZSH = new([]byte)
		err = decoder.Decode(ty.ZSH)
		if err != nil {
			return err
		}
		return
	case 3: // Base
		t := true
		ty.NONE = &t
		return
	default:
		return fmt.Errorf("unrecognized enum")
	}
}

type Service struct { // Enum
	Tcp        *uint16 // 0
	Udp        *uint16 // 1
	Http       *uint16 // 2
	Https      *uint16 // 3
	ProjectTcp *uint16 // 4
	ProjectUdp *uint16 // 5
}

func (ty Service) Encode(encoder scale.Encoder) (err error) {
	if ty.Tcp != nil {
		err = encoder.PushByte(0)
		if err != nil {
			return err
		}
		err = encoder.Encode(*ty.Tcp)
		if err != nil {
			return err
		}
		return nil
	}

	if ty.Udp != nil {
		err = encoder.PushByte(1)
		if err != nil {
			return err
		}
		err = encoder.Encode(*ty.Udp)
		if err != nil {
			return err
		}
		return nil
	}

	if ty.Http != nil {
		err = encoder.PushByte(2)
		if err != nil {
			return err
		}
		err = encoder.Encode(*ty.Http)
		if err != nil {
			return err
		}
		return nil
	}

	if ty.Https != nil {
		err = encoder.PushByte(3)
		if err != nil {
			return err
		}
		err = encoder.Encode(*ty.Https)
		if err != nil {
			return err
		}
		return nil
	}

	if ty.ProjectTcp != nil {
		err = encoder.PushByte(4)
		if err != nil {
			return err
		}
		err = encoder.Encode(*ty.ProjectTcp)
		if err != nil {
			return err
		}
		return nil
	}

	if ty.ProjectUdp != nil {
		err = encoder.PushByte(5)
		if err != nil {
			return err
		}
		err = encoder.Encode(*ty.ProjectUdp)
		if err != nil {
			return err
		}
		return nil
	}
	return fmt.Errorf("unrecognized enum")
}

func (ty *Service) Decode(decoder scale.Decoder) (err error) {
	variant, err := decoder.ReadOneByte()
	if err != nil {
		return err
	}
	switch variant {
	case 0: // Inline
		ty.Tcp = new(uint16)
		err = decoder.Decode(ty.Tcp)
		if err != nil {
			return err
		}
		return
	case 1: // Inline
		ty.Udp = new(uint16)
		err = decoder.Decode(ty.Udp)
		if err != nil {
			return err
		}
		return
	case 2: // Inline
		ty.Http = new(uint16)
		err = decoder.Decode(ty.Http)
		if err != nil {
			return err
		}
		return
	case 3: // Inline
		ty.Https = new(uint16)
		err = decoder.Decode(ty.Https)
		if err != nil {
			return err
		}
		return
	case 4: // Inline
		ty.ProjectTcp = new(uint16)
		err = decoder.Decode(ty.ProjectTcp)
		if err != nil {
			return err
		}
		return
	case 5: // Inline
		ty.ProjectUdp = new(uint16)
		err = decoder.Decode(ty.ProjectUdp)
		if err != nil {
			return err
		}
		return
	default:
		return fmt.Errorf("unrecognized enum")
	}
}

type ContainerDisk struct { // Composite
	Id   uint64
	Path []byte
}
type PodEnv struct { // Enum
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

func (ty PodEnv) Encode(encoder scale.Encoder) (err error) {
	if ty.Env != nil {
		err = encoder.PushByte(0)
		if err != nil {
			return err
		}

		err = encoder.Encode(ty.Env.F0)
		if err != nil {
			return err
		}

		err = encoder.Encode(ty.Env.F1)
		if err != nil {
			return err
		}

		return nil
	}

	if ty.File != nil {
		err = encoder.PushByte(1)
		if err != nil {
			return err
		}

		err = encoder.Encode(ty.File.F0)
		if err != nil {
			return err
		}

		err = encoder.Encode(ty.File.F1)
		if err != nil {
			return err
		}

		return nil
	}

	if ty.Encrypt != nil {
		err = encoder.PushByte(2)
		if err != nil {
			return err
		}

		err = encoder.Encode(ty.Encrypt.F0)
		if err != nil {
			return err
		}

		err = encoder.Encode(ty.Encrypt.F1)
		if err != nil {
			return err
		}

		return nil
	}
	return fmt.Errorf("unrecognized enum")
}

func (ty *PodEnv) Decode(decoder scale.Decoder) (err error) {
	variant, err := decoder.ReadOneByte()
	if err != nil {
		return err
	}
	switch variant {
	case 0: // Tuple
		ty.Env = &struct {
			F0 []byte
			F1 []byte
		}{}

		err = decoder.Decode(&ty.Env.F0)
		if err != nil {
			return err
		}

		err = decoder.Decode(&ty.Env.F1)
		if err != nil {
			return err
		}

		return
	case 1: // Tuple
		ty.File = &struct {
			F0 []byte
			F1 []byte
		}{}

		err = decoder.Decode(&ty.File.F0)
		if err != nil {
			return err
		}

		err = decoder.Decode(&ty.File.F1)
		if err != nil {
			return err
		}

		return
	case 2: // Tuple
		ty.Encrypt = &struct {
			F0 []byte
			F1 uint64
		}{}

		err = decoder.Decode(&ty.Encrypt.F0)
		if err != nil {
			return err
		}

		err = decoder.Decode(&ty.Encrypt.F1)
		if err != nil {
			return err
		}

		return
	default:
		return fmt.Errorf("unrecognized enum")
	}
}

type Container struct { // Composite
	Image   []byte
	Command Command
	Port    []Service
	Cpu     uint32
	Mem     uint32
	Disk    []ContainerDisk
	Gpu     uint32
	Env     []PodEnv
}
type Tuple_36 struct { // Tuple
	F0 uint64
	F1 Container
}
type Tuple_38 struct { // Tuple
	F0 uint64
	F1 Pod
	F2 []Tuple_36
	F3 byte
}
type Tuple_41 struct { // Tuple
	F0 uint64
	F1 uint32
	F2 uint32
	F3 byte
}
type Secret struct { // Composite
	K      []byte
	Hash   types.H256
	Minted bool
}
type Tuple_45 struct { // Tuple
	F0 uint64
	F1 Secret
}
type Disk struct { // Enum
	SecretSSD *struct { // 0
		F0 []byte
		F1 []byte
		F2 uint32
	}
}

func (ty Disk) Encode(encoder scale.Encoder) (err error) {
	if ty.SecretSSD != nil {
		err = encoder.PushByte(0)
		if err != nil {
			return err
		}

		err = encoder.Encode(ty.SecretSSD.F0)
		if err != nil {
			return err
		}

		err = encoder.Encode(ty.SecretSSD.F1)
		if err != nil {
			return err
		}

		err = encoder.Encode(ty.SecretSSD.F2)
		if err != nil {
			return err
		}

		return nil
	}
	return fmt.Errorf("unrecognized enum")
}

func (ty *Disk) Decode(decoder scale.Decoder) (err error) {
	variant, err := decoder.ReadOneByte()
	if err != nil {
		return err
	}
	switch variant {
	case 0: // Tuple
		ty.SecretSSD = &struct {
			F0 []byte
			F1 []byte
			F2 uint32
		}{}

		err = decoder.Decode(&ty.SecretSSD.F0)
		if err != nil {
			return err
		}

		err = decoder.Decode(&ty.SecretSSD.F1)
		if err != nil {
			return err
		}

		err = decoder.Decode(&ty.SecretSSD.F2)
		if err != nil {
			return err
		}

		return
	default:
		return fmt.Errorf("unrecognized enum")
	}
}

type Tuple_56 struct { // Tuple
	F0 uint64
	F1 Disk
}
type Ip struct { // Composite
	Ipv4   util.Option[uint32]
	Ipv6   util.Option[types.U128]
	Domain util.Option[[]byte]
}
type K8sClusterInfo struct { // Composite
	Name          []byte
	Owner         types.H160
	Level         byte
	RegionId      uint32
	StartBlock    uint32
	StopBlock     util.Option[uint32]
	TerminalBlock util.Option[uint32]
	Ip            Ip
	Port          uint32
	Status        byte
}
type Tuple_66 struct { // Tuple
	F0 uint64
	F1 K8sClusterInfo
	F2 []byte
}
type Tuple_71 struct { // Tuple
	F0 Container
	F1 []util.Option[Disk]
}
type Tuple_72 struct { // Tuple
	F0 uint64
	F1 Tuple_71
}
type Tuple_74 struct { // Tuple
	F0 uint64
	F1 Pod
	F2 []Tuple_72
	F3 uint32
	F4 uint32
	F5 byte
}
type ArbitrationStatus struct { // Enum
	Pending  *bool // 0
	Approved *bool // 1
	Rejected *bool // 2
}

func (ty ArbitrationStatus) Encode(encoder scale.Encoder) (err error) {
	if ty.Pending != nil {
		err = encoder.PushByte(0)
		if err != nil {
			return err
		}
		return nil
	}

	if ty.Approved != nil {
		err = encoder.PushByte(1)
		if err != nil {
			return err
		}
		return nil
	}

	if ty.Rejected != nil {
		err = encoder.PushByte(2)
		if err != nil {
			return err
		}
		return nil
	}
	return fmt.Errorf("unrecognized enum")
}

func (ty *ArbitrationStatus) Decode(decoder scale.Decoder) (err error) {
	variant, err := decoder.ReadOneByte()
	if err != nil {
		return err
	}
	switch variant {
	case 0: // Base
		t := true
		ty.Pending = &t
		return
	case 1: // Base
		t := true
		ty.Approved = &t
		return
	case 2: // Base
		t := true
		ty.Rejected = &t
		return
	default:
		return fmt.Errorf("unrecognized enum")
	}
}

type Arbitration struct { // Composite
	Id           uint64
	PodId        uint64
	WorkerId     uint64
	Claimant     types.H160
	Amount       types.U256
	Reason       []byte
	Status       ArbitrationStatus
	ResultAmount types.U256
	CreatedAt    uint32
	ResolvedAt   util.Option[uint32]
}
type Tuple_81 struct { // Tuple
	F0 uint64
	F1 Arbitration
}
type Tuple_84 struct { // Tuple
	F0 Pod
	F1 []Tuple_36
	F2 uint32
	F3 byte
}
type EditType struct { // Enum
	INSERT *bool   // 0
	UPDATE *uint64 // 1
	REMOVE *uint64 // 2
}

func (ty EditType) Encode(encoder scale.Encoder) (err error) {
	if ty.INSERT != nil {
		err = encoder.PushByte(0)
		if err != nil {
			return err
		}
		return nil
	}

	if ty.UPDATE != nil {
		err = encoder.PushByte(1)
		if err != nil {
			return err
		}
		err = encoder.Encode(*ty.UPDATE)
		if err != nil {
			return err
		}
		return nil
	}

	if ty.REMOVE != nil {
		err = encoder.PushByte(2)
		if err != nil {
			return err
		}
		err = encoder.Encode(*ty.REMOVE)
		if err != nil {
			return err
		}
		return nil
	}
	return fmt.Errorf("unrecognized enum")
}

func (ty *EditType) Decode(decoder scale.Decoder) (err error) {
	variant, err := decoder.ReadOneByte()
	if err != nil {
		return err
	}
	switch variant {
	case 0: // Base
		t := true
		ty.INSERT = &t
		return
	case 1: // Inline
		ty.UPDATE = new(uint64)
		err = decoder.Decode(ty.UPDATE)
		if err != nil {
			return err
		}
		return
	case 2: // Inline
		ty.REMOVE = new(uint64)
		err = decoder.Decode(ty.REMOVE)
		if err != nil {
			return err
		}
		return
	default:
		return fmt.Errorf("unrecognized enum")
	}
}

type ContainerInput struct { // Composite
	Etype     EditType
	Container Container
}
