package scheduler

import (
	"context"
	"math/rand"
	"sync"
)

// computeScore 计算端点的综合评分(越大越优先)。
//
//	score = weight × success_rate × load_factor / latency_factor
//	  - weight:        静态权重(配置)
//	  - success_rate:  最近窗口成功率
//	  - load_factor:   1 / (1 + inFlight * 0.1)  已用并发惩罚
//	  - latency_factor:1 + EMA_latency / 1s       延迟惩罚
//
// 冷启动期(样本数 < threshold)只用 weight,避免冷启动抖动。
// 熔断时直接返回 0(被 selector 跳过)。
func computeScore(weight float64, m *EndpointMetrics, coldStartThreshold int) float64 {
	if m.CircuitOpen.Load() {
		return 0
	}
	total := m.TotalRequests.Load()
	if total < int64(coldStartThreshold) {
		return weight
	}
	successRate := float64(m.SuccessCount.Load()) / float64(total)
	if successRate < 0.05 {
		successRate = 0.05
	}
	loadFactor := 1.0 / (1.0 + float64(m.InFlight.Load())*0.1)
	// 延迟项以 1s 为基准(EMA 单位是微秒)
	latencyFactor := 1.0 + float64(m.EMALatencyMicros.Load())/1e6
	return weight * successRate * loadFactor / latencyFactor
}

// Selector 选出某个 model 的最佳端点。
// 内部带 sync.Pool 复用 Candidate slice,降低 GC 压力。
type Selector struct {
	pool sync.Pool
	rng  *rand.Rand
	mu   sync.Mutex
}

// Candidate 是评分算法的中间产物。
type Candidate struct {
	Endpoint *Endpoint
	Score    float64
}

// NewSelector 创建 Selector。
func NewSelector() *Selector {
	return &Selector{
		pool: sync.Pool{New: func() interface{} { return make([]Candidate, 0, 16) }},
		rng:  rand.New(rand.NewSource(int64Fast())),
	}
}

func newCandidateSlice() []Candidate {
	return make([]Candidate, 0, 16)
}

// Select 从 route.Endpoints 中挑一个可用端点。
// 返回 nil 表示无可用端点。
func (s *Selector) Select(ctx context.Context, route *ModelRoute, coldStartThreshold int) *Endpoint {
	cands := s.pool.Get().([]Candidate)
	cands = cands[:0]
	defer func() { s.pool.Put(cands) }()

	var skippedDisabled, skippedModel, skippedConcurrency, skippedScore int
	for _, ep := range route.Endpoints {
		if !ep.Enabled {
			skippedDisabled++
			continue
		}
		if !modelAllowed(ep.Models, route.Model) {
			skippedModel++
			continue
		}
		if ep.Concurrency > 0 && ep.InFlight() >= int64(ep.Concurrency) {
			skippedConcurrency++
			continue
		}
		score := computeScore(ep.Weight, ep.metrics, coldStartThreshold)
		if score <= 0 {
			skippedScore++
			continue
		}
		cands = append(cands, Candidate{Endpoint: ep, Score: score})
	}
	if len(cands) == 0 {
		// 在 debug 级别记录详细原因，生产环境通常不开启
		_ = skippedDisabled + skippedModel + skippedConcurrency + skippedScore
		return nil
	}
	return weightedPick(cands, s.randFloat64())
}

func (s *Selector) randFloat64() float64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.rng.Float64()
}

// weightedPick 按 score 加权随机选一个。
func weightedPick(cands []Candidate, r float64) *Endpoint {
	total := 0.0
	for _, c := range cands {
		total += c.Score
	}
	if total <= 0 {
		return cands[0].Endpoint
	}
	target := r * total
	cum := 0.0
	for _, c := range cands {
		cum += c.Score
		if target <= cum {
			return c.Endpoint
		}
	}
	return cands[len(cands)-1].Endpoint
}

// modelAllowed 检查端点是否支持某模型。
// 空切片 = deny-all;"*" 通配 = allow-all;其他 = 白名单匹配。
func modelAllowed(allowed []ModelKey, target ModelKey) bool {
	if len(allowed) == 0 {
		return false
	}
	for _, m := range allowed {
		if m == "*" || m == target {
			return true
		}
	}
	return false
}

// updateEMA 用指数移动平均更新延迟指标。
// alpha 越小,历史值权重越大(更平滑)。
func updateEMA(metrics *EndpointMetrics, latencyMicros int64, alpha float64) {
	old := metrics.EMALatencyMicros.Load()
	if old == 0 {
		metrics.EMALatencyMicros.Store(latencyMicros)
		return
	}
	newVal := int64(float64(old)*(1-alpha) + float64(latencyMicros)*alpha)
	metrics.EMALatencyMicros.Store(newVal)
}

// int64Fast 返回一个轻量时间源(纳秒),用做 RNG 种子。
func int64Fast() int64 {
	return timeNowUnixNano()
}
