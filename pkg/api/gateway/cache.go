package gateway

import (
	"sync"
	"time"

	"github.com/computeflux/tee-provider/pkg/model"
)

// ── In-memory cache to avoid Pebble scan per request ─────

type channelCacheEntry struct {
	channels  []*model.GatewayProviderInfo
	expiresAt time.Time
}

type tokenCacheEntry struct {
	tokens    []*model.GatewayTokenInfo
	expiresAt time.Time
}

var (
	cacheMu    sync.RWMutex
	chanCache  *channelCacheEntry
	tokenCache *tokenCacheEntry
	cacheTTL   = 5 * time.Second
)

// RefreshChannelCache reloads channels from Pebble
func RefreshChannelCache() {
	channels := make([]*model.GatewayProviderInfo, 0)
	var id uint64
	for {
		ch, err := model.GetGatewayProvider(id)
		if err != nil || ch == nil {
			break
		}
		if ch.IsActive {
			channels = append(channels, ch)
		}
		id++
		if id > 10000 {
			break
		}
	}

	cacheMu.Lock()
	chanCache = &channelCacheEntry{
		channels:  channels,
		expiresAt: time.Now().Add(cacheTTL),
	}
	cacheMu.Unlock()
}

// RefreshTokenCache reloads tokens from Pebble
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

// GetCachedChannels returns channels from cache, refreshing if needed
func GetCachedChannels() []*model.GatewayProviderInfo {
	cacheMu.RLock()
	if chanCache != nil && time.Now().Before(chanCache.expiresAt) {
		ch := chanCache.channels
		cacheMu.RUnlock()
		return ch
	}
	cacheMu.RUnlock()

	RefreshChannelCache()

	cacheMu.RLock()
	ch := chanCache.channels
	cacheMu.RUnlock()
	return ch
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
	chanCache = nil
	tokenCache = nil
	cacheMu.Unlock()
}

func findTokenCached(key string) *model.GatewayTokenInfo {
	for _, tk := range GetCachedTokens() {
		if tk.Key == key && tk.IsActive {
			return tk
		}
	}
	return nil
}
