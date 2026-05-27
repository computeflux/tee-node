package config

import "github.com/computeflux/tee-provider/pkg/model"

var TestChainConfig = model.ChainConfigType{
	ChainID:       0,
	ChainType:     "substrate",
	Cipher:        model.SuiteEd25519,
	Urls:          []string{"wss://asset-hub-paseo.ibp.network", "wss://asset-hub-paseo-rpc.n.dwellir.com", "wss://asset-hub-paseo.dotters.network"},
	SubnetAddress: "0xd521e67f086bde744531e4f94eaaa1304c0fd0c2",
	TokenAddress:  "0xdd74b788be939843568cf0882840615416eede98",
	NetworkLabel:  "Paseo (Asset Hub)",
}

// var TestChainList = []model.ChainConfigType{}
