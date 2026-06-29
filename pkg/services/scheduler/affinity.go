package scheduler

import (
	"sync"
	"time"
)

// SessionAffinity 维护 session → endpoint 的亲和映射。
// 同一 session 的连续请求会尽量复用上次成功的端点，以命中上游缓存。
type SessionAffinity struct {
	mu     sync.RWMutex
	cache  map[string]affinityEntry
	ttl    time.Duration
}

type affinityEntry struct {
	key    EndpointKey
	expiry time.Time
}

// NewSessionAffinity 创建亲和性管理器。ttl 为 0 时使用默认 5 分钟。
func NewSessionAffinity(ttl time.Duration) *SessionAffinity {
	if ttl <= 0 {
		ttl = 5 * time.Minute
	}
	return &SessionAffinity{
		cache: make(map[string]affinityEntry),
		ttl:   ttl,
	}
}

// Get 获取 session 对应的 endpoint key。过期返回 nil。
func (a *SessionAffinity) Get(sessionID string) *EndpointKey {
	if a == nil || sessionID == "" {
		return nil
	}
	a.mu.RLock()
	ent, ok := a.cache[sessionID]
	a.mu.RUnlock()
	if !ok || time.Now().After(ent.expiry) {
		// 过期则删除
		if ok {
			a.mu.Lock()
			if e, still := a.cache[sessionID]; still && time.Now().After(e.expiry) {
				delete(a.cache, sessionID)
			}
			a.mu.Unlock()
		}
		return nil
	}
	return &ent.key
}

// Set 记录 session 成功使用的 endpoint。
func (a *SessionAffinity) Set(sessionID string, key EndpointKey) {
	if a == nil || sessionID == "" {
		return
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	a.cache[sessionID] = affinityEntry{key: key, expiry: time.Now().Add(a.ttl)}
}

// Delete 删除某个 session 的亲和记录。
func (a *SessionAffinity) Delete(sessionID string) {
	if a == nil || sessionID == "" {
		return
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	delete(a.cache, sessionID)
}

// Cleanup 清理所有过期条目。
func (a *SessionAffinity) Cleanup() {
	if a == nil {
		return
	}
	now := time.Now()
	a.mu.Lock()
	defer a.mu.Unlock()
	for k, v := range a.cache {
		if now.After(v.expiry) {
			delete(a.cache, k)
		}
	}
}

// Len 返回当前缓存条目数。
func (a *SessionAffinity) Len() int {
	if a == nil {
		return 0
	}
	a.mu.RLock()
	defer a.mu.RUnlock()
	return len(a.cache)
}
