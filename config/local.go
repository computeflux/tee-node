package config

import "github.com/computeflux/tee-provider/pkg/chaintypes"

var LocalChainConfig = chaintypes.ChainConfigType{
	ChainID:       0,
	ChainType:     "substrate",
	Urls:          []string{"wss://xiaobai.asyou.me:30001/ws"},
	SubnetAddress: "",
	TokenAddress:  "",
	NetworkLabel:  "Local",
}
