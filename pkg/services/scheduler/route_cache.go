package scheduler

import (
	"sync"
	"time"
)

// cacheKey 是 ModelRouteCache 的键，区分 model + provider。
type cacheKey struct {
	model    ModelKey
	provider ProviderID
}

// cachedRoute 缓存一个解析后的 ModelRoute 及其过期时间。
type cachedRoute struct {
	route  *ModelRoute
	expiry time.Time
}

// ModelRouteCache 缓存 resolver 解析出的 ModelRoute，减少每个请求都重建端点列表的开销。
// 它是 scheduler 的运行时组件，indexer 仍只负责索引数据。
type ModelRouteCache struct {
	mu      sync.RWMutex
	entries map[cacheKey]*cachedRoute
	ttl     time.Duration
	log     Logger
}

// NewModelRouteCache 创建缓存。ttl <= 0 时不缓存。
func NewModelRouteCache(ttl time.Duration, log Logger) *ModelRouteCache {
	if log == nil {
		log = noopLogger{}
	}
	return &ModelRouteCache{
		entries: make(map[cacheKey]*cachedRoute),
		ttl:     ttl,
		log:     log,
	}
}

// Get 从缓存取 route。过期或不存在返回 nil。
func (c *ModelRouteCache) Get(model ModelKey, provider ProviderID) *ModelRoute {
	if c == nil || c.ttl <= 0 {
		return nil
	}
	c.mu.RLock()
	entry, ok := c.entries[cacheKey{model: model, provider: provider}]
	c.mu.RUnlock()
	if !ok {
		return nil
	}
	if time.Now().After(entry.expiry) {
		c.log.Debug("route cache expired", "model", model, "provider", provider)
		return nil
	}
	c.log.Debug("route cache hit", "model", model, "provider", provider, "endpoints", len(entry.route.Endpoints))
	return entry.route
}

// Set 把解析后的 route 写入缓存。
func (c *ModelRouteCache) Set(model ModelKey, provider ProviderID, route *ModelRoute) {
	if c == nil || c.ttl <= 0 || route == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.entries[cacheKey{model: model, provider: provider}] = &cachedRoute{
		route:  route,
		expiry: time.Now().Add(c.ttl),
	}
	c.log.Debug("route cache set", "model", model, "provider", provider, "endpoints", len(route.Endpoints))
}

// Clear 清空缓存，用于链上数据刷新后强制重建。
func (c *ModelRouteCache) Clear() {
	if c == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.entries = make(map[cacheKey]*cachedRoute)
	c.log.Info("route cache cleared")
}

