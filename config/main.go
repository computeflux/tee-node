package config

import "github.com/computeflux/tee-provider/pkg/model"

var MainChainConfig = model.ChainConfigType{
	ChainType:     "evm",
	Cipher:        model.SuiteBLS12381,
	Urls:          []string{""},
	SubnetAddress: "",
	TokenAddress:  "",
	NetworkLabel:  "Polkadot (Asset Hub)",
}
