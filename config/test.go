package config

import "github.com/computeflux/tee-provider/pkg/model"

var TestChainConfig = model.ChainConfigType{
	ChainType:     "substrate",
	Cipher:        model.SuiteEd25519,
	Urls:          []string{"wss://asset-hub-paseo-rpc.n.dwellir.com"},
	SubnetAddress: "0x8ad5985c9114cf948ee3eca182b680d2f33f9198",
	TokenAddress:  "0xb832e0ac0a1979e15a250aba0c8b4dec40803bd0",
	NetworkLabel:  "Paseo (Asset Hub)",
}

// var TestChainList = []model.ChainConfigType{}
