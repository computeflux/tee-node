package config

import "github.com/computeflux/tee-provider/pkg/model"

var TestChainConfig = model.ChainConfigType{
	ChainID:       0,
	ChainType:     "substrate",
	Cipher:        model.SuiteEd25519,
	Urls:          []string{"wss://asset-hub-paseo-rpc.n.dwellir.com", "wss://asset-hub-paseo.ibp.network", "wss://asset-hub-paseo.dotters.network"},
	SubnetAddress: "0xf2cdaaec43e5186d6b29e2863426609fd18fb481",
	TokenAddress:  "0xdffbdd11308a548533ae43062e7116476e75e016",
	NetworkLabel:  "Paseo (Asset Hub)",
}

// var TestChainList = []model.ChainConfigType{}
