package config

import "github.com/computeflux/tee-provider/pkg/types/chaintypes"

// 获取最核心的主链，用于 tee chain 安全性的一部分，也作为一个主链
// 返回的是全局配置的深拷贝，调用方修改返回值不会污染全局配置。
func GetMainChainConfig(env string) *chaintypes.ChainConfigType {
	var cfg chaintypes.ChainConfigType
	switch env {
	case "local":
		cfg = LocalChainConfig
	case "test":
		cfg = TestChainConfig
	case "main":
		cfg = MainChainConfig
	default:
		return nil
	}
	// 深拷贝切片字段：浅拷贝会共享底层数组，调用方修改 Urls 会污染全局配置
	cfg.Urls = append([]string(nil), cfg.Urls...)
	return &cfg
}

// 获取所有的主链
// 注意：当前为占位实现，暂无子链配置数据源。
func GetChainConfigs(env string) []chaintypes.ChainConfigType {
	return []chaintypes.ChainConfigType{}
}
