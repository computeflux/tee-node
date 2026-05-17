package config

import "github.com/computeflux/tee-provider/pkg/model"

var LocalChainConfig = model.ChainConfigType{
	ChainType:     "dev",
	Urls:          []string{""},
	SubnetAddress: "",
	TokenAddress:  "",
	NetworkLabel:  "Local",
}
