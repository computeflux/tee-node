package gateway

import (
	"sync"
	"time"
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
