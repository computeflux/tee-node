package config

import "github.com/computeflux/tee-provider/pkg/chaintypes"

// 获取最核心的主链，用于 tee chain 安全性的一部分，也作为一个主链
func GetMainChainConfig(env string) *chaintypes.ChainConfigType {
	switch env {
	case "local":
		return &LocalChainConfig
	case "test":
		return &TestChainConfig
	case "main":
		return &MainChainConfig
	}
	return nil
}

// 获取所有的主链
func GetChainConfigs(env string) []chaintypes.ChainConfigType {
	return []chaintypes.ChainConfigType{}
}
