package gateway

import (
	"sync"
	"time"

	"github.com/computeflux/tee-provider/pkg/model"
)

// ── Async usage writer ───────────────────────────────────
// 批量写入 Pebble，避免每个请求都同步写磁盘

type usageRecord struct {
	tokenID    uint64
	day        string
	providerID uint64
	modelID    uint64
	tokens     uint64
}

var (
	usageCh   = make(chan usageRecord, 4096)
	usageWg   sync.WaitGroup
	usageOnce sync.Once
)

func initUsageWriter() {
	usageOnce.Do(func() {
		usageWg.Add(1)
		go func() {
			defer usageWg.Done()
			batch := make(map[string]*model.GatewayDailyUsage, 64)
			ticker := time.NewTicker(500 * time.Millisecond)
			defer ticker.Stop()

			for {
				select {
				case r := <-usageCh:
					key := r.day + "_" + uintToStr(r.tokenID)
					if u, ok := batch[key]; ok {
						u.Count++
						u.TokensUsed += r.tokens
						u.PointsUsed += (r.tokens + 999) / 1000
					} else {
						batch[key] = &model.GatewayDailyUsage{
							TokenID: r.tokenID, ProviderID: r.providerID,
							ModelID: r.modelID, Day: r.day,
							Count: 1, TokensUsed: r.tokens,
							PointsUsed: (r.tokens + 999) / 1000,
						}
					}
				case <-ticker.C:
					if len(batch) > 0 {
						for k, u := range batch {
							model.SetJson("GATEWAY", "usage:"+k, u)
						}
						batch = make(map[string]*model.GatewayDailyUsage, 64)
					}
				}
			}
		}()
	})
}

func recordUsageAsync(tokenID uint64, day string, providerID uint64, tokens uint64) {
	initUsageWriter()
	select {
	case usageCh <- usageRecord{tokenID, day, providerID, 0, tokens}:
	default:
		// Channel full, drop (degraded mode) — log for monitoring.
		GATEWAYLOG().Printf("[USAGE] dropped record: token=%d provider=%d tokens=%d (channel full)", tokenID, providerID, tokens)
	}
}

func uintToStr(n uint64) string {
	if n == 0 {
		return "0"
	}
	buf := make([]byte, 20)
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[i:])
}

// ── Cache background refresh ─────────────────────────────

var cacheRefreshOnce sync.Once

func initCacheRefresh() {
	cacheRefreshOnce.Do(func() {
		go func() {
			ticker := time.NewTicker(5 * time.Second)
			defer ticker.Stop()
			for range ticker.C {
				RefreshTokenCache()
			}
		}()
	})
}

// ── Rate limiter cleanup ─────────────────────────────────

var cleanupOnce sync.Once

func initRateLimiterCleanup() {
	cleanupOnce.Do(func() {
		go func() {
			ticker := time.NewTicker(60 * time.Second)
			defer ticker.Stop()
			for range ticker.C {
				limiter.mu.Lock()
				now := time.Now()
				for k, bucket := range limiter.buckets {
					valid := bucket[:0]
					for _, t := range bucket {
						if now.Sub(t) < 5*time.Minute {
							valid = append(valid, t)
						}
					}
					if len(valid) == 0 {
						delete(limiter.buckets, k)
					} else {
						limiter.buckets[k] = valid
					}
				}
				limiter.mu.Unlock()
			}
		}()
	})
}

// ── Body buffer pool ─────────────────────────────────────

var bufferPool = sync.Pool{
	New: func() interface{} {
		return make([]byte, 0, 64*1024)
	},
}

func GetBuffer() []byte {
	return bufferPool.Get().([]byte)[:0]
}

func PutBuffer(buf []byte) {
	if cap(buf) <= 256*1024 {
		bufferPool.Put(buf[:0])
	}
}

// ── Startup init ─────────────────────────────────────────

func InitOptimizations() {
	initUsageWriter()
	initCacheRefresh()
	initRateLimiterCleanup()
	GATEWAYLOG().Printf("[OPT] async usage writer + cache refresh + rate limiter cleanup started")
}
