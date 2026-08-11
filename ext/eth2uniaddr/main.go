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

	out, err := ethAddrToUniAddrHex(os.Args[1])
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	fmt.Println(out)
}

// ethAddrToUniAddrHex 将以太坊地址（带/不带 0x 前缀）转换为 SCALE 编码的
// UniAddr（T=2 EVM + 地址字节）的 hex 字符串。
func ethAddrToUniAddrHex(addr string) (string, error) {
	addr = strings.TrimPrefix(addr, "0x")
	addrBytes, err := hex.DecodeString(addr)
	if err != nil {
		return "", fmt.Errorf("invalid hex address: %w", err)
	}
	if len(addrBytes) != 20 {
		return "", fmt.Errorf("ethereum address must be 20 bytes, got %d", len(addrBytes))
	}

	uni := cstore.UniAddr{T: 2, V: addrBytes}
	encoded, err := uni.Encode()
	if err != nil {
		return "", fmt.Errorf("encode UniAddr: %w", err)
	}
	return hex.EncodeToString(encoded), nil
}
