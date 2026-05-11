package main

import (
	"crypto/rand"
	"fmt"

	"github.com/hashicorp/vault/shamir"
)

func main() {
	// 生成64字节sr25519私钥种子（示例）
	privKey := make([]byte, 64)
	_, err := rand.Read(privKey)
	if err != nil {
		panic(err)
	}

	// 拆分私钥，分100份，阈值66
	shares, err := shamir.Split(privKey, 200, 150)
	if err != nil {
		panic(err)
	}

	fmt.Printf("生成了 %d 份私钥份额，阈值为 %d\n", len(shares), 66)

	// 输出部分share作为示例
	for i := range 50 {
		fmt.Printf("Share %d: %x\n", i+1, shares[i])
	}

	// 需要时用 66 份份额恢复私钥
	recovered, err := shamir.Combine(shares[:66])
	if err != nil {
		panic(err)
	}

	fmt.Printf("恢复的私钥与原私钥是否一致: %v\n", string(recovered) == string(privKey))
}
