package gateway

import (
	"sync"
	"time"

	"github.com/computeflux/tee-provider/pkg/model"
)

// ── Token cache (fallback for legacy local keys) ─────────

type tokenCacheEntry struct {
	tokens    []*model.GatewayTokenInfo
	expiresAt time.Time
}

var (
	cacheMu    sync.RWMutex
	tokenCache *tokenCacheEntry
	cacheTTL   = 5 * time.Second
)

// RefreshTokenCache reloads tokens from Pebble (local cache for legacy keys).
// Contract-based API keys are verified on-demand via lookupAPIKeyViaContract.
func RefreshTokenCache() {
	tokens := make([]*model.GatewayTokenInfo, 0)
	var id uint64
	for {
		tk, err := model.GetGatewayToken(id)
		if err != nil || tk == nil {
			break
		}
		if tk.IsActive {
			tokens = append(tokens, tk)
		}
		id++
		if id > 10000 {
			break
		}
	}

	cacheMu.Lock()
	tokenCache = &tokenCacheEntry{
		tokens:    tokens,
		expiresAt: time.Now().Add(cacheTTL),
	}
	cacheMu.Unlock()
}

// GetCachedTokens returns tokens from cache
func GetCachedTokens() []*model.GatewayTokenInfo {
	cacheMu.RLock()
	if tokenCache != nil && time.Now().Before(tokenCache.expiresAt) {
		tk := tokenCache.tokens
		cacheMu.RUnlock()
		return tk
	}
	cacheMu.RUnlock()

	RefreshTokenCache()

	cacheMu.RLock()
	tk := tokenCache.tokens
	cacheMu.RUnlock()
	return tk
}

// InvalidateCache forces a refresh on next access
func InvalidateCache() {
	cacheMu.Lock()
	tokenCache = nil
	cacheMu.Unlock()
}

func findTokenCached(key string) *model.GatewayTokenInfo {
	return FindTokenByKey(key)
}

// FindTokenByKey finds an active API key.
// 1. 优先查询合约（链上注册的 API Key）
// 2. 回退到本地 PebbleDB 缓存 / DB 直接查询（legacy 本地 key / 登录 key）
func FindTokenByKey(key string) *model.GatewayTokenInfo {
	// 优先走合约查询
	if tk := lookupAPIKeyViaContract(key); tk != nil {
		return tk
	}

	// Fallback: local PebbleDB cache
	for _, tk := range GetCachedTokens() {
		// fmt.Printf("[TokenCache] checking key %s against cached token %s (active=%v)\n", key, tk.Key, tk.IsActive)
		if tk.Key == key && tk.IsActive {
			return tk
		}
	}

	// 最后 fallback：直接遍历 DB（登录时 createUserAPIKey 写入的 key）
	return model.FindGatewayTokenByKey(key)
}
