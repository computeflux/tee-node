package model

import (
	"hash/fnv"
	"sync"
	"sync/atomic"
)

// ── Endpoint ─────────────────────────────────────────────

type GatewayEndpoint struct {
	ID         uint64 `json:"id"`
	ProviderID uint64 `json:"providerId"`
	URL        string `json:"url"`
	APIKey     string `json:"apiKey"`
	Weight     uint32 `json:"weight"`
	IsActive   bool   `json:"isActive"`
}

// ── CRUD ─────────────────────────────────────────────────

// ── Sticky Balancer ──────────────────────────────────────
//
// 策略：用户永久绑定到一个 Endpoint，最大化缓存命中。
// 只有 Endpoint 故障时才切换。
//
// 路由规则：
//   1. 新用户：按权重分配 Endpoint，分配后永久绑定
//   2. 老用户：永远回到之前绑定的 Endpoint
//   3. Endpoint 故障 → 用户自动迁移到其他 Endpoint
//   4. Endpoint 恢复 → 用户不会自动回来（避免反复迁移）
//
// 效果：
//   - 同一用户的所有请求打到同一 API Key
//   - OpenAI server-side cache 100% 命中
//   - 成本最低

type EndpointBalancer struct {
	mu        sync.RWMutex
	endpoints []*GatewayEndpoint
	userMap   map[string]uint64 // userKey → endpointID (永久绑定)
	weights   []uint32
	totalW    uint32
	inflight  []int64 // 当前并发（监控用）
}

func NewEndpointBalancerFromEndpoints(eps []*GatewayEndpoint) *EndpointBalancer {
	if len(eps) == 0 {
		return nil
	}
	b := &EndpointBalancer{
		endpoints: eps,
		userMap:   make(map[string]uint64),
		weights:   make([]uint32, len(eps)),
		inflight:  make([]int64, len(eps)),
	}

	for i, ep := range eps {
		w := ep.Weight
		if w == 0 {
			w = 1
		}
		b.weights[i] = w
		b.totalW += w
	}
	return b
}

// Pick returns the endpoint for a user.
// Existing users always get their bound endpoint.
// New users get assigned by weighted hash — same weight = same endpoint.
func (b *EndpointBalancer) Pick(userKey string) *GatewayEndpoint {
	if b == nil || len(b.endpoints) == 0 {
		return nil
	}
	if len(b.endpoints) == 1 {
		atomic.AddInt64(&b.inflight[0], 1)
		return b.endpoints[0]
	}

	b.mu.Lock()
	defer b.mu.Unlock()

	// 1. 已绑定用户 → 直接用（除非 endpoint 不可用）
	if epID, ok := b.userMap[userKey]; ok {
		for i, ep := range b.endpoints {
			if ep.ID == epID {
				if ep.IsActive {
					atomic.AddInt64(&b.inflight[i], 1)
					return ep
				}
				// Endpoint 不可用，解除绑定，走新分配
				delete(b.userMap, userKey)
				break
			}
		}
	}

	// 2. 新用户：FNV hash 分配（同一用户永远得到同一结果）
	// 用 hash 而不是轮询，确保重启后分配不变
	idx := b.hashToIndex(userKey)
	b.userMap[userKey] = b.endpoints[idx].ID
	atomic.AddInt64(&b.inflight[idx], 1)
	return b.endpoints[idx]
}

// hashToIndex maps userKey to an endpoint index by weighted hash
func (b *EndpointBalancer) hashToIndex(userKey string) int {
	h := fnv.New32a()
	h.Write([]byte(userKey))
	hash := uint32(h.Sum32()) % b.totalW

	var cumulative uint32
	for i, w := range b.weights {
		cumulative += w
		if hash < cumulative {
			return i
		}
	}
	return len(b.endpoints) - 1
}

// Release decrements inflight count
func (b *EndpointBalancer) Release(endpointID uint64) {
	for i, ep := range b.endpoints {
		if ep.ID == endpointID {
			v := atomic.LoadInt64(&b.inflight[i])
			if v > 0 {
				atomic.AddInt64(&b.inflight[i], -1)
			}
			return
		}
	}
}

// OnEndpointDisabled removes the endpoint from active rotation
func (b *EndpointBalancer) OnEndpointDisabled(endpointID uint64) {
	b.mu.Lock()
	defer b.mu.Unlock()
	// 清除该 endpoint 的所有用户绑定
	for k, v := range b.userMap {
		if v == endpointID {
			delete(b.userMap, k)
		}
	}
}

// Stats returns per-endpoint stats
func (b *EndpointBalancer) Stats() map[uint64]struct {
	Users    int
	Inflight int64
} {
	b.mu.RLock()
	defer b.mu.RUnlock()
	s := make(map[uint64]struct {
		Users    int
		Inflight int64
	})
	userCounts := make(map[uint64]int)
	for _, epID := range b.userMap {
		userCounts[epID]++
	}
	for i, ep := range b.endpoints {
		s[ep.ID] = struct {
			Users    int
			Inflight int64
		}{userCounts[ep.ID], atomic.LoadInt64(&b.inflight[i])}
	}
	return s
}
