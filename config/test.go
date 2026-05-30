package config

import "github.com/computeflux/tee-provider/pkg/model"

var TestChainConfig = model.ChainConfigType{
	ChainID:       0,
	ChainType:     "substrate",
	Cipher:        model.SuiteEd25519,
	Urls:          []string{"wss://asset-hub-paseo.ibp.network", "wss://asset-hub-paseo-rpc.n.dwellir.com", "wss://asset-hub-paseo.dotters.network"},
	SubnetAddress: "0xc8b906a60fba4e36685214645a1a944ff749e34c",
	TokenAddress:  "0x546689e0275ac5403a8b9670fb8094ed80946162",
	NetworkLabel:  "Paseo (Asset Hub)",
}

// var TestChainList = []model.ChainConfigType{}
