// signmsg 生成 EIP-191 personal_sign 签名，用于 e2e 测试脚本。
// 用法: go run ./hacks/signmsg <privkey-hex> <message>
// 输出两行: 第一行私钥对应地址(小写), 第二行签名(0x hex, V=27/28 格式)。
package main

import (
	"fmt"
	"os"
	"strings"

	ethcrypto "github.com/ethereum/go-ethereum/crypto"
)

func main() {
	if len(os.Args) != 3 {
		fmt.Fprintln(os.Stderr, "usage: signmsg <privkey-hex> <message>")
		os.Exit(2)
	}
	key, err := ethcrypto.HexToECDSA(strings.TrimPrefix(os.Args[1], "0x"))
	if err != nil {
		fmt.Fprintln(os.Stderr, "bad key:", err)
		os.Exit(1)
	}
	message := os.Args[2]
	msg := fmt.Sprintf("\x19Ethereum Signed Message:\n%d%s", len(message), message)
	hash := ethcrypto.Keccak256Hash([]byte(msg))
	sig, err := ethcrypto.Sign(hash.Bytes(), key)
	if err != nil {
		fmt.Fprintln(os.Stderr, "sign:", err)
		os.Exit(1)
	}
	// 转成 MetaMask 的 27/28 V 值格式
	sig[64] += 27
	addr := strings.ToLower(ethcrypto.PubkeyToAddress(key.PublicKey).Hex())
	fmt.Printf("%s\n0x%x\n", addr, sig)
}
