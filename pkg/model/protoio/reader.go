package protoio

import "errors"

func Unmarshal(buf []byte, pb any) error {
	if p, ok := pb.(VTUnmarshaler); ok {
		return p.UnmarshalVT(buf)
	}
	return errors.New("protoio Unmarshal is not VTUnmarshaler")
}
