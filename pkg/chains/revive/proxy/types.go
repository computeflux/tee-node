package proxy

import (
	"fmt"

	"github.com/centrifuge/go-substrate-rpc-client/v4/scale"
)

type Error struct { // Enum
	Unauthorized    *bool // 0
	AddressNotFound *bool // 1
}

func (ty Error) Encode(encoder scale.Encoder) (err error) {
	if ty.Unauthorized != nil {
		err = encoder.PushByte(0)
		if err != nil {
			return err
		}
		return nil
	}

	if ty.AddressNotFound != nil {
		err = encoder.PushByte(1)
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
		ty.Unauthorized = &t
		return
	case 1: // Base
		t := true
		ty.AddressNotFound = &t
		return
	default:
		return fmt.Errorf("unrecognized enum")
	}
}
func (ty *Error) Error() string {
	if ty.Unauthorized != nil {
		return "Unauthorized"
	}

	if ty.AddressNotFound != nil {
		return "AddressNotFound"
	}
	return "Unknown"
}
