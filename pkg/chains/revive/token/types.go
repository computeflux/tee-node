package token

import (
	"fmt"

	"github.com/centrifuge/go-substrate-rpc-client/v4/scale"
	"github.com/centrifuge/go-substrate-rpc-client/v4/types"
)

type Error struct { // Enum
	OnlySubnet                  *bool // 0
	OnlyOwner                   *bool // 1
	AmountMustBeGreaterThanZero *bool // 2
	InsufficientBalance         *bool // 3
	TransferFailed              *bool // 4
	ZeroAddress                 *bool // 5
	ERC20NotSupported           *bool // 6
	ERC20Inactive               *bool // 7
	ERC20TransferFailed         *bool // 8
	NativeDisabled              *bool // 9
}

func (ty Error) Encode(encoder scale.Encoder) (err error) {
	if ty.OnlySubnet != nil {
		err = encoder.PushByte(0)
		if err != nil {
			return err
		}
		return nil
	}

	if ty.OnlyOwner != nil {
		err = encoder.PushByte(1)
		if err != nil {
			return err
		}
		return nil
	}

	if ty.AmountMustBeGreaterThanZero != nil {
		err = encoder.PushByte(2)
		if err != nil {
			return err
		}
		return nil
	}

	if ty.InsufficientBalance != nil {
		err = encoder.PushByte(3)
		if err != nil {
			return err
		}
		return nil
	}

	if ty.TransferFailed != nil {
		err = encoder.PushByte(4)
		if err != nil {
			return err
		}
		return nil
	}

	if ty.ZeroAddress != nil {
		err = encoder.PushByte(5)
		if err != nil {
			return err
		}
		return nil
	}

	if ty.ERC20NotSupported != nil {
		err = encoder.PushByte(6)
		if err != nil {
			return err
		}
		return nil
	}

	if ty.ERC20Inactive != nil {
		err = encoder.PushByte(7)
		if err != nil {
			return err
		}
		return nil
	}

	if ty.ERC20TransferFailed != nil {
		err = encoder.PushByte(8)
		if err != nil {
			return err
		}
		return nil
	}

	if ty.NativeDisabled != nil {
		err = encoder.PushByte(9)
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
		ty.OnlySubnet = &t
		return
	case 1: // Base
		t := true
		ty.OnlyOwner = &t
		return
	case 2: // Base
		t := true
		ty.AmountMustBeGreaterThanZero = &t
		return
	case 3: // Base
		t := true
		ty.InsufficientBalance = &t
		return
	case 4: // Base
		t := true
		ty.TransferFailed = &t
		return
	case 5: // Base
		t := true
		ty.ZeroAddress = &t
		return
	case 6: // Base
		t := true
		ty.ERC20NotSupported = &t
		return
	case 7: // Base
		t := true
		ty.ERC20Inactive = &t
		return
	case 8: // Base
		t := true
		ty.ERC20TransferFailed = &t
		return
	case 9: // Base
		t := true
		ty.NativeDisabled = &t
		return
	default:
		return fmt.Errorf("unrecognized enum")
	}
}
func (ty *Error) Error() string {
	if ty.OnlySubnet != nil {
		return "OnlySubnet"
	}

	if ty.OnlyOwner != nil {
		return "OnlyOwner"
	}

	if ty.AmountMustBeGreaterThanZero != nil {
		return "AmountMustBeGreaterThanZero"
	}

	if ty.InsufficientBalance != nil {
		return "InsufficientBalance"
	}

	if ty.TransferFailed != nil {
		return "TransferFailed"
	}

	if ty.ZeroAddress != nil {
		return "ZeroAddress"
	}

	if ty.ERC20NotSupported != nil {
		return "ERC20NotSupported"
	}

	if ty.ERC20Inactive != nil {
		return "ERC20Inactive"
	}

	if ty.ERC20TransferFailed != nil {
		return "ERC20TransferFailed"
	}

	if ty.NativeDisabled != nil {
		return "NativeDisabled"
	}
	return "Unknown"
}

type EventRecord struct { // Composite
	TargetContract []byte
	EventType      []byte
	EventData      [][]byte
}
type Tuple_25 struct { // Tuple
	F0 bool
	F1 types.U256
	F2 types.U256
}
type Tuple_27 struct { // Tuple
	F0 types.H160
	F1 bool
	F2 types.U256
	F3 types.U256
}
type PendingWithdrawal struct { // Composite
	User   types.H160
	Token  types.H160
	Amount types.U256
	Status byte
}
