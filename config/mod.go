package config

import "github.com/computeflux/tee-provider/pkg/model"

func GetChainConfig(chainType string) *model.ChainConfigType {
	switch chainType {
	case "local":
		return &LocalChainConfig
	case "test":
		return &TestChainConfig
	case "main":
		return &MainChainConfig
	}
	return nil
}
