package config

import "github.com/computeflux/tee-provider/pkg/model"

var TestChainConfig = model.ChainConfigType{
	ChainID:       0,
	ChainType:     "substrate",
	Cipher:        model.SuiteEd25519,
	Urls:          []string{"wss://asset-hub-paseo.ibp.network", "wss://asset-hub-paseo-rpc.n.dwellir.com", "wss://asset-hub-paseo.dotters.network"},
	SubnetAddress: "0x4b9dc2fea1576f638776406491794125f8175535",
	TokenAddress:  "0x3a9d7e869419aa3c076cf80328751063d62a6a31",
	NetworkLabel:  "Paseo (Asset Hub)",
}

// var TestChainList = []model.ChainConfigType{}
