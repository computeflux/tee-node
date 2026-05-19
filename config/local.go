package config

import "github.com/computeflux/tee-provider/pkg/model"

var LocalChainConfig = model.ChainConfigType{
	ChainID:       0,
	ChainType:     "substrate",
	Urls:          []string{"wss://xiaobai.asyou.me:30001/ws"},
	SubnetAddress: "",
	TokenAddress:  "",
	NetworkLabel:  "Local",
}
