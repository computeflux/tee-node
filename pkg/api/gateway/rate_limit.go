package gateway

import (
	"sync"
	"sync/atomic"
	"time"
)

// ── Endpoint Rate Limiter ─────────────────────────────────
//
// 假设 API key 无限 → RPM 不是瓶颈
// 真正瓶颈：
//   1. HTTP 连接池: 500 conn/host (所有 endpoint 共享 api.openai.com)
//   2. 单 endpoint 并发: 防止一个 key 被请求淹没
//   3. 总并发: 500 (HTTP/1.1 强制)
//
// 每 endpoint 限制调到跟连接池对齐：
//   并发 50/key → 10 个 key 打满 500 连接池

type EndpointRateLimiter struct {
	mu       sync.Mutex
	window   []time.Time
	maxConc  int64 // 最大并发
	inflight int64 // 当前并发
}

var (
	endpointLimiters  = make(map[uint64]*EndpointRateLimiter)
	endpointLimiterMu sync.RWMutex
)

func getEndpointLimiter(endpointID uint64, _ int) *EndpointRateLimiter {
	endpointLimiterMu.Lock()
	defer endpointLimiterMu.Unlock()

	lim, ok := endpointLimiters[endpointID]
	if ok {
		return lim
	}

	lim = &EndpointRateLimiter{
		maxConc: 50, // 单 key 50 并发
	}
	endpointLimiters[endpointID] = lim
	return lim
}

// Allow checks if concurrency limit allows another request
func (l *EndpointRateLimiter) Allow() bool {
	if l == nil {
		return true
	}
	if atomic.AddInt64(&l.inflight, 1) <= atomic.LoadInt64(&l.maxConc) {
		return true
	}
	// Rejected: decrement so inflight doesn't leak
	atomic.AddInt64(&l.inflight, -1)
	return false
}

// Done decrements inflight
func (l *EndpointRateLimiter) Done() {
	if l != nil {
		atomic.AddInt64(&l.inflight, -1)
	}
}

// SetLimits updates max concurrency
func (l *EndpointRateLimiter) SetLimits(_ int, maxConc int64) {
	atomic.StoreInt64(&l.maxConc, maxConc)
}

// Stats returns current state
func (l *EndpointRateLimiter) Stats() (rpm int, inflight int64) {
	return 0, atomic.LoadInt64(&l.inflight)
}

// ── Provider concurrency defaults (unlimited keys scenario) ─

var ProviderRateDefaults = map[string]struct {
	MaxConc int64
}{
	"openai":    {MaxConc: 50},  // 10 keys × 50 = 500 打满连接池
	"anthropic": {MaxConc: 20},  // 25 keys × 20 = 500
	"gemini":    {MaxConc: 100}, // 5 keys × 100 = 500
	"local":     {MaxConc: 100}, // 本地模型通常部署在局域网，并发可更高
	"default":   {MaxConc: 50},
}

func GetProviderLimits(provider string) (rpm int, maxConc int64) {
	if p, ok := ProviderRateDefaults[provider]; ok {
		return 0, p.MaxConc // RPM=0 表示不限
	}
	return 0, 50
}
