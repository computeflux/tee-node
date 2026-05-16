package util

import (
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
)

// PubKeyToAddr converts a secp256k1 public key to an Ethereum address.
// Accepts uncompressed (65 bytes, 0x04 prefix), raw (64 bytes, no prefix),
// or compressed (33 bytes, 0x02/0x03 prefix) formats.
func PubKeyToAddr(pubKey []byte) common.Address {
	if len(pubKey) == 0 {
		return common.Address{}
	}
	var key []byte
	switch {
	case len(pubKey) == 65 && pubKey[0] == 0x04:
		// Uncompressed: drop the 0x04 prefix, leaving 64 bytes.
		key = pubKey[1:]
	case len(pubKey) == 33 && (pubKey[0] == 0x02 || pubKey[0] == 0x03):
		// Compressed: decompress to 65-byte uncompressed, then drop prefix.
		pk, err := crypto.DecompressPubkey(pubKey)
		if err != nil {
			// Fallback: hash as-is (should not happen with valid keys).
			key = pubKey
		} else {
			key = crypto.FromECDSAPub(pk)[1:] // strip 0x04 prefix
		}
	default:
		key = pubKey
	}
	hash := crypto.Keccak256(key)
	var addr common.Address
	copy(addr[:], hash[12:])
	return addr
}

// WeiToEther converts wei (*big.Int) to a human-readable ether string.
func WeiToEther(wei *big.Int) string {
	f := new(big.Float).SetInt(wei)
	e := new(big.Float).SetFloat64(1e18)
	return new(big.Float).Quo(f, e).Text('f', 6)
}
