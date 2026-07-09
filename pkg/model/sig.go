package model

import (
	"bytes"
	"encoding/binary"
)

func (c *SysCall) BytesForSig() ([]byte, error) {
	if call := c.GetContract(); call != nil {
		return ContractForSig(call), nil
	}

	return c.MarshalVT()
}

func ContractForSig(call *ContractCall) []byte {
	parts := append([][]byte{[]byte("<Bytes>"), call.Name, call.Method[:]}, call.Args...)
	parts = append(parts, Uint64ToBytes(call.Nonce))
	parts = append(parts, []byte("</Bytes>"))
	return bytes.Join(parts, nil)
}

func Uint64ToBytes(n uint64) []byte {
	b := make([]byte, 8)
	binary.BigEndian.PutUint64(b, n)
	return b
}
