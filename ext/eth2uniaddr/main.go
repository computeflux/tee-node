// eth2uniaddr converts an Ethereum address to a SCALE-encoded UniAddr hex string.
//
// Usage:
//
//	go run ./ext/eth2uniaddr 0xAb5801a7D398351b8bE11C439e05C5B3259aeC9B
//	go run ./ext/eth2uniaddr Ab5801a7D398351b8bE11C439e05C5B3259aeC9B
//
// Output: hex-encoded SCALE UniAddr (T=2 EVM + address bytes)
package main

import (
	"encoding/hex"
	"fmt"
	"os"
	"strings"

	cstore "github.com/computeflux/tee-provider/tee-chain/runtime/store"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintf(os.Stderr, "Usage: go run ./ext/eth2uniaddr <eth-address>\n")
		fmt.Fprintf(os.Stderr, "Example: go run ./ext/eth2uniaddr 0xAb5801a7D398351b8bE11C439e05C5B3259aeC9B\n")
		os.Exit(1)
	}

	addr := strings.TrimPrefix(os.Args[1], "0x")
	addrBytes, err := hex.DecodeString(addr)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: invalid hex address: %v\n", err)
		os.Exit(1)
	}
	if len(addrBytes) != 20 {
		fmt.Fprintf(os.Stderr, "Error: Ethereum address must be 20 bytes, got %d\n", len(addrBytes))
		os.Exit(1)
	}

	uni := cstore.UniAddr{T: 2, V: addrBytes}

	encoded, err := uni.Encode()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: encode UniAddr: %v\n", err)
		os.Exit(1)
	}

	fmt.Println(hex.EncodeToString(encoded))
}
