package config

import (
	"github.com/computeflux/tee-provider/pkg/types/chaintypes"
)

// 获取最核心的主链，用于 tee chain 安全性的一部分，也作为一个主链
// 返回的是全局配置的深拷贝，调用方修改返回值不会污染全局配置。
func GetMainChainConfig(env string) *chaintypes.ChainConfigType {
	var cfg chaintypes.ChainConfigType
	switch env {
	case "local":
		cfg = deepCopyChainConfig(LocalChainConfig)
	case "test":
		cfg = deepCopyChainConfig(TestChainConfig)
	case "main":
		cfg = deepCopyChainConfig(MainChainConfig)
	default:
		return nil
	}
	return &cfg
}

// 获取所有的主链
// 返回的是全局配置的深拷贝，调用方修改返回值不会污染全局配置。
func GetChainConfigs(env string) []chaintypes.ChainConfigType {
	switch env {
	case "test":
		// Base Testnet：合约地址未配置（主网未部署时为空）则不启用，避免无效连接。
		if BaseTestChainConfig.TokenAddress == "" {
			return []chaintypes.ChainConfigType{}
		}
		return []chaintypes.ChainConfigType{deepCopyChainConfig(BaseTestChainConfig)}
	case "main":
		if BaseMainChainConfig.TokenAddress == "" {
			return []chaintypes.ChainConfigType{}
		}
		return []chaintypes.ChainConfigType{deepCopyChainConfig(BaseMainChainConfig)}
	default:
		return []chaintypes.ChainConfigType{}
	}
}

// deepCopyChainConfig 返回链配置的深拷贝（含 Urls 底层数组），调用方修改返回值不会污染全局配置。
func deepCopyChainConfig(cfg chaintypes.ChainConfigType) chaintypes.ChainConfigType {
	cfg.Urls = append([]string(nil), cfg.Urls...)
	return cfg
}
