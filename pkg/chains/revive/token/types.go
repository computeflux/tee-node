package token

import (
	"fmt"

	"github.com/centrifuge/go-substrate-rpc-client/v4/scale"
)

type Error struct { // Enum
	OnlySubnet                  *bool // 0
	OnlyOwner                   *bool // 1
	AmountMustBeGreaterThanZero *bool // 2
	InsufficientBalance         *bool // 3
	TransferFailed              *bool // 4
	ZeroAddress                 *bool // 5
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
	return "Unknown"
}
