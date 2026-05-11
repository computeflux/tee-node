package main

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/hex"
	"fmt"

	"github.com/cosmos/go-bip39"
	oed "github.com/oasisprotocol/curve25519-voi/primitives/ed25519"
	"github.com/wetee-dao/tee-dsecret/pkg/model"
	"go.dedis.ch/kyber/v4/suites"
)

func main() {
	_, privGo, _ := ed25519.GenerateKey(rand.Reader)
	privOasis, _ := model.StdToOed25519(privGo)
	msg := []byte("hello world")
	sig := ed25519.Sign(privGo, msg)
	sig2 := oed.Sign(privOasis, msg)
	fmt.Println("sig1:", hex.EncodeToString(sig))
	fmt.Println("sig2:", hex.EncodeToString(sig2))

	suite := suites.MustFind("Ed25519")
	privateKey, publicKey, err := model.GenerateEd25519KeyPair(rand.Reader)
	if err != nil {
		fmt.Println(err)
		return
	}

	fmt.Println("privateKey:", privateKey.String())
	fmt.Println("publicKey:", publicKey)

	fmt.Println("publicKey.Point():", publicKey.Point())

	sPriv := privateKey.Scalar()
	fmt.Println("sPriv: ", sPriv)
	sPub := suite.Point().Mul(sPriv, nil)
	fmt.Println("sPub: ", sPub)

}

func seed() {
	// 生成 128 位的随机熵
	entropy, err := bip39.NewEntropy(128)
	if err != nil {
		panic(err)
	}

	// 生成助记词
	mnemonic, err := bip39.NewMnemonic(entropy)
	if err != nil {
		panic(err)
	}

	// 打印助记词
	fmt.Println("助记词:", mnemonic)
}
