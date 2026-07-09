package protoio

import (
	"errors"
	"fmt"
	"io"
)

// WriteMessage writes a varint length-delimited protobuf message with deterministic serialization.
// This ensures consistent hash values across multiple serializations.
func WriteMessage(msg VTMarshaler, w io.Writer) error {
	buf := make([]byte, msg.SizeVT())
	if _, err := msg.MarshalToVT(buf); err != nil {
		return err
	}

	_, err := w.Write(buf)
	return err
}

func Marshal(pb VTMarshaler) ([]byte, error) {
	if pb == nil {
		return nil, errors.New("pb is nil")
	}

	buf := make([]byte, pb.SizeVT())
	if _, err := pb.MarshalToVT(buf); err != nil {
		return nil, err
	}
	return buf, nil
}

type VTMarshaler interface {
	SizeVT() int
	MarshalToVT([]byte) (int, error)
	MarshalVT() ([]byte, error)
}

type VTUnmarshaler interface {
	UnmarshalVT([]byte) error
}

func MarshalVT(tx any) ([]byte, error) {
	if tx == nil {
		return nil, nil
	}

	m, ok := tx.(VTMarshaler)
	if !ok {
		return nil, fmt.Errorf("unsupported tee tx type %T", tx)
	}

	buf := make([]byte, m.SizeVT())
	if _, err := m.MarshalToVT(buf); err != nil {
		return nil, err
	}
	return buf, nil
}
