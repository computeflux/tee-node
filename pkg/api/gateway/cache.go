package gateway

import (
	"github.com/computeflux/tee-provider/pkg/model"
)

// FindTokenByKey finds an active API key.
// 1. 优先查询合约（链上注册的 API Key）
// 2. 回退到本地 PebbleDB 直接查询（通过 key 索引 O(1)）
func FindTokenByKey(key string) *model.GatewayTokenInfo {
	// 优先走合约查询
	if tk := lookupAPIKeyViaContract(key); tk != nil {
		return tk
	}
	// Fallback: 本地 PebbleDB 通过 key 索引直接查询
	tk, _ := model.GetGatewayTokenByKey(key)
	if tk != nil && tk.IsActive {
		return tk
	}
	return nil
}

// findTokenCached 是内部别名，兼容 auth_middleware.go 调用。
func findTokenCached(key string) *model.GatewayTokenInfo {
	return FindTokenByKey(key)
}

// RefreshTokenCache 已废弃，本地 token 不再使用内存缓存。
func RefreshTokenCache() {}

// InvalidateCache 已废弃，本地 token 不再使用内存缓存。
func InvalidateCache() {}
