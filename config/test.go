package config

import "github.com/computeflux/tee-provider/pkg/model"

var TestChainConfig = model.ChainConfigType{
	ChainID:       0,
	ChainType:     "substrate",
	Cipher:        model.SuiteEd25519,
	Urls:          []string{"wss://asset-hub-paseo.ibp.network", "wss://asset-hub-paseo-rpc.n.dwellir.com", "wss://asset-hub-paseo.dotters.network"},
	SubnetAddress: "0x828effd251b5477f19ef38838681063a0f979ecd",
	TokenAddress:  "0x05f0600ac0076154e5c471f291c1678e74c3c0f3",
	NetworkLabel:  "Paseo (Asset Hub)",
}

// var TestChainList = []model.ChainConfigType{}
