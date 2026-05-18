package config

import "github.com/computeflux/tee-provider/pkg/model"

var LocalChainConfig = model.ChainConfigType{
	ChainType:     "substrate",
	Urls:          []string{"wss://xiaobai.asyou.me:30001/ws"},
	SubnetAddress: "",
	TokenAddress:  "",
	NetworkLabel:  "Local",
}
