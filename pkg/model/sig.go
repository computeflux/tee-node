package model

import (
	"bytes"
)

func (c *SysCall) BytesForSig() ([]byte, error) {
	if call := c.GetContract(); call != nil {
		return ContractForSig(call), nil
	}

	data, err := c.XXX_Marshal(nil, true)
	if err != nil {
		return nil, err
	}
	return data, nil

}

func ContractForSig(call *ContractCall) []byte {
	parts := append([][]byte{[]byte("<Bytes>"), call.Name, call.Method[:]}, call.Args...)
	parts = append(parts, []byte("</Bytes>"))
	return bytes.Join(parts, nil)
}
