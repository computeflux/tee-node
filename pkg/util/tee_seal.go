package util

import (
	"flag"

	"github.com/edgelesssys/ego/ecrypto"
)

func SealWithProductKey(val []byte, additionalData []byte) ([]byte, error) {
	if flag.Lookup("test.v") == nil {
		return ecrypto.SealWithProductKey(val, additionalData)
	}
	return val, nil
}

func Unseal(ciphertext []byte, additionalData []byte) ([]byte, error) {
	if flag.Lookup("test.v") == nil {
		return ecrypto.Unseal(ciphertext, additionalData)
	}
	return ciphertext, nil
}
