package config

import "github.com/computeflux/tee-provider/pkg/model"

var TestChainConfig = model.ChainConfigType{
	ChainType:     "substrate",
	Cipher:        model.SuiteEd25519,
	Urls:          []string{"wss://asset-hub-paseo-rpc.n.dwellir.com", "wss://asset-hub-paseo.ibp.network"},
	SubnetAddress: "0x9312de1b99c69d80112b112bcefd2776c06b1ac6",
	TokenAddress:  "0xe701aeca672b5e1fd177df8096ac8888796e6427",
	NetworkLabel:  "Paseo (Asset Hub)",
}

// var TestChainList = []model.ChainConfigType{}
