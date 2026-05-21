package gateway

import (
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/computeflux/tee-provider/pkg/services/gateway/relay"
)

// ── Auto Scaler ───────────────────────────────────────────
//
// 用户体验优先：
//   扩容：检测到压力立即扩（1 个 tick），避免用户等 30s
//   缩容：连续 10 分钟低负载才缩，避免误判
//   防抖：扩后冷却 2 分钟，避免抖动

type AutoScaler struct {
	mu                sync.Mutex
	lastScaleTime     time.Time
	scaleDownCount    int
	enabled           bool
	minConns          int
	maxConns          int
	cooldownPeriod    time.Duration
	shrinkQuietPeriod time.Duration // 低负载需持续多久才缩容
}

var scaler = &AutoScaler{
	enabled:           true,
	minConns:          500, // 最小 500，保证基本体验
	maxConns:          10000,
	cooldownPeriod:    2 * time.Minute,  // 扩后冷却
	shrinkQuietPeriod: 10 * time.Minute, // 持续 10 分钟低负载才缩
}

func totalInflight() int64 {
	endpointLimiterMu.RLock()
	defer endpointLimiterMu.RUnlock()
	var total int64
	for _, lim := range endpointLimiters {
		total += atomic.LoadInt64(&lim.inflight)
	}
	return total
}

func (s *AutoScaler) tick() {
	if !s.enabled {
		return
	}

	cfg := relay.GetPoolConfig()
	current := cfg.MaxConnsPerHost
	if current == 0 {
		current = 500
	}

	total := totalInflight()
	utilization := float64(total) / float64(current) * 100

	s.mu.Lock()
	defer s.mu.Unlock()

	// ═══ 扩容：快（1 tick 即扩） ═══════════════════════════
	// 原因：用户遇到 429 比浪费连接更糟糕
	if utilization > 70 && current < s.maxConns {
		// 冷却期内不重复扩（防止抖动）
		if time.Since(s.lastScaleTime) < s.cooldownPeriod {
			return
		}

		// 按利用率计算目标大小（一步到位）
		target := int(float64(current) * (utilization / 50)) // 目标利用率 50%
		if target > s.maxConns {
			target = s.maxConns
		}
		if target <= current {
			target = current * 2
		}
		if target > s.maxConns {
			target = s.maxConns
		}

		cfg.MaxConnsPerHost = target
		relay.ApplyPoolConfig(cfg)
		s.lastScaleTime = time.Now()
		s.scaleDownCount = 0

		GATEWAYLOG().Printf("[SCALER] ▲▲ scale up: %d → %d (util %.0f%%, inflight %d)",
			current, target, utilization, total)
		return
	}

	// ═══ 缩容：慢（持续 10 分钟低负载才缩） ═════════════════
	// 原因：误缩导致用户排队，比浪费连接更糟糕
	if utilization < 15 && current > s.minConns {
		s.scaleDownCount++
		// 10 分钟 = 60 ticks (10s each)
		if s.scaleDownCount >= 60 {
			target := current / 2
			if target < s.minConns {
				target = s.minConns
			}

			cfg.MaxConnsPerHost = target
			relay.ApplyPoolConfig(cfg)
			s.lastScaleTime = time.Now()
			s.scaleDownCount = 0

			GATEWAYLOG().Printf("[SCALER] ▼▼ scale down: %d → %d (util %.0f%%, inflight %d, quiet %d min)",
				current, target, utilization, total, s.shrinkQuietPeriod/time.Minute)
		}
		return
	}

	// 利用率正常，重置缩容计数
	s.scaleDownCount = 0
}

// ── Stats ─────────────────────────────────────────────────

func GetScalerStats() map[string]interface{} {
	cfg := relay.GetPoolConfig()
	total := totalInflight()
	util := float64(0)
	if cfg.MaxConnsPerHost > 0 {
		util = float64(total) / float64(cfg.MaxConnsPerHost) * 100
	}

	scaler.mu.Lock()
	down := scaler.scaleDownCount
	cooldown := time.Since(scaler.lastScaleTime)
	scaler.mu.Unlock()

	state := "stable"
	if util > 70 {
		state = "expanding"
	} else if util < 15 && down > 0 {
		state = fmt.Sprintf("shrinking(%d/60)", down)
	}

	return map[string]interface{}{
		"state":         state,
		"poolSize":      cfg.MaxConnsPerHost,
		"totalInflight": total,
		"utilization":   util,
		"cooldown":      cooldown.String(),
	}
}

func SetScalerEnabled(enabled bool) {
	scaler.mu.Lock()
	scaler.enabled = enabled
	scaler.mu.Unlock()
}

func SetScalerLimits(min, max int) {
	scaler.mu.Lock()
	if min > 0 {
		scaler.minConns = min
	}
	if max > 0 {
		scaler.maxConns = max
	}
	scaler.mu.Unlock()
}

func InitAutoScaler() {
	go func() {
		ticker := time.NewTicker(10 * time.Second)
		defer ticker.Stop()
		for range ticker.C {
			scaler.tick()
		}
	}()
	GATEWAYLOG().Printf("[SCALER] started: %d-%d conns, fast-up + slow-down", scaler.minConns, scaler.maxConns)
}
