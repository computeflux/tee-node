package config

import "github.com/computeflux/tee-provider/pkg/model"

var TestChainConfig = model.ChainConfigType{
	ChainID:       0,
	ChainType:     "substrate",
	Cipher:        model.SuiteEd25519,
	Urls:          []string{"wss://asset-hub-paseo-rpc.n.dwellir.com", "wss://asset-hub-paseo.ibp.network", "wss://asset-hub-paseo.dotters.network"},
	SubnetAddress: "0xd9904245829b400b324f24b40a090387634b24da",
	TokenAddress:  "0x0e2fc7fb934916e856c7ba4e4e107673c26110c2",
	NetworkLabel:  "Paseo (Asset Hub)",
}

// var TestChainList = []model.ChainConfigType{}
