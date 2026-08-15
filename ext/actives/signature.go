package actives

import (
	"encoding/hex"
	"fmt"
	"strings"

	ethcrypto "github.com/ethereum/go-ethereum/crypto"
)

// verifyEIP191Signature 验证 MetaMask 等钱包的 EIP-191 personal_sign 签名。
// 恢复出的地址必须与 address 一致（忽略大小写）。
func verifyEIP191Signature(address, signatureHex, message string) bool {
	sigBytes, err := hex.DecodeString(strings.TrimPrefix(signatureHex, "0x"))
	if err != nil || len(sigBytes) != 65 {
		return false
	}

	// EIP-191: "\x19Ethereum Signed Message:\n" + len(message) + message
	msg := fmt.Sprintf("\x19Ethereum Signed Message:\n%d%s", len(message), message)
	hash := ethcrypto.Keccak256Hash([]byte(msg))

	// Transform V from 27/28 to 0/1 for recovery
	if sigBytes[64] >= 27 {
		sigBytes[64] -= 27
	}

	pubKey, err := ethcrypto.SigToPub(hash.Bytes(), sigBytes)
	if err != nil {
		return false
	}

	recoveredAddr := ethcrypto.PubkeyToAddress(*pubKey).Hex()
	return strings.EqualFold(recoveredAddr, address)
}
