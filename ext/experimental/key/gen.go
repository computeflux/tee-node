package main

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/hex"
	"fmt"

	"github.com/computeflux/tee-provider/pkg/crypto"
	oed "github.com/oasisprotocol/curve25519-voi/primitives/ed25519"
	"go.dedis.ch/kyber/v4/suites"
)

func main() {
	_, privGo, _ := ed25519.GenerateKey(rand.Reader)
	privOasis, _ := crypto.StdToOed25519(privGo)
	msg := []byte("hello world")
	sig := ed25519.Sign(privGo, msg)
	sig2 := oed.Sign(privOasis, msg)
	fmt.Println("sig1:", hex.EncodeToString(sig))
	fmt.Println("sig2:", hex.EncodeToString(sig2))

	suite := suites.MustFind("Ed25519")
	privateKey, publicKey, err := crypto.GenerateEd25519KeyPair(rand.Reader)
	if err != nil {
		fmt.Println(err)
		return
	}

	fmt.Println("privateKey:", privateKey.String())
	fmt.Println("publicKey:", publicKey)

	fmt.Println("publicKey.Point():", publicKey.Point())

	sPriv := privateKey.Scalar(crypto.SuiteEd25519)
	fmt.Println("sPriv: ", sPriv)
	sPub := suite.Point().Mul(sPriv, nil)
	fmt.Println("sPub: ", sPub)

}
