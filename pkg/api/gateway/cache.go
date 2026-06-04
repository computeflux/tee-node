package gateway

import "github.com/computeflux/tee-provider/pkg/store"

// FindTokenByKey finds an active API key.
// 1. 优先查询合约（链上注册的 API Key）
// 2. 回退到本地 PebbleDB 直接查询（通过 key 索引 O(1)）
func FindTokenByKey(key string) *store.GatewayTokenInfo {
	// 优先走合约查询 api_tee_xxxxxx
	if tk := lookupAPIKeyViaContract(key); tk != nil {
		return tk
	}

	// Fallback: 本地 PebbleDB 通过 key 索引直接查询
	tk, err := store.GetGatewayTokenByKey(key)
	if err != nil {
		GATEWAYLOG().Printf("[AUTH] local DB lookup error for %s: %v", key[:min(16, len(key))], err)
	}

	if tk == nil {
		GATEWAYLOG().Printf("[AUTH] local DB not found for %s", key[:min(16, len(key))])
	} else if !tk.IsActive {
		GATEWAYLOG().Printf("[AUTH] token inactive for %s", key[:min(16, len(key))])
	} else {
		return tk
	}

	return nil
}

// findTokenCached 是内部别名，兼容 auth_middleware.go 调用。
func findTokenCached(key string) *store.GatewayTokenInfo {
	return FindTokenByKey(key)
}

// findTokenCachedFn 可被测试替换为 mock。
var findTokenCachedFn func(string) *store.GatewayTokenInfo = findTokenCached
