package config

import "github.com/computeflux/tee-provider/pkg/model"

var MainChainConfig = model.ChainConfigType{
	ChainID:       0,
	ChainType:     "evm",
	Cipher:        model.SuiteBLS12381,
	Urls:          []string{""},
	SubnetAddress: "",
	TokenAddress:  "",
	NetworkLabel:  "Polkadot (Asset Hub)",
}
