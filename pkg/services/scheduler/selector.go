package scheduler

import (
	"context"
	"fmt"
	"math/rand"
	"sync"
	"time"
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
//
// 注意:不再因为 CircuitOpen=true 直接返回 0。如果在 selector 阶段过滤
// 熔断端点,Breaker.Allow 永远拿不到该端点,也就无法完成
// Open → HalfOpen 的状态转换(死锁)。熔断状态完全由 Breaker 自己
// 在 Allow 中判断,并允许 HalfOpen 探测请求通过。
func computeScore(weight float64, m *EndpointMetrics, coldStartThreshold int) float64 {
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
	log  Logger
}

// Candidate 是评分算法的中间产物。
type Candidate struct {
	Endpoint *Endpoint
	Score    float64
}

// NewSelector 创建 Selector。
func NewSelector() *Selector {
	return &Selector{
		pool: sync.Pool{New: func() any { return make([]Candidate, 0, 16) }},
		rng:  rand.New(rand.NewSource(int64Fast())),
		log:  noopLogger{},
	}
}

// SetLog 给 Selector 绑定日志实现，便于排查无可用端点的原因。
// 注意：SetLog 与 Select 并发调用是数据竞争,必须在所有 Select 调用
// 开始之前完成,或在持有 s.mu 的情况下调用。
func (s *Selector) SetLog(l Logger) {
	if l == nil {
		l = noopLogger{}
	}
	s.mu.Lock()
	s.log = l
	s.mu.Unlock()
}

func newCandidateSlice() []Candidate {
	return make([]Candidate, 0, 16)
}

// Select 从 route.Endpoints 中挑一个可用端点。
// skip 中的 KeyID(已 dead 或本轮已用)会被排除;所有候选都被排除时返回 nil,
// 让调用方据此触发「第二次机会」清空逻辑,而不是空转重试同一批 key。
// 返回 (nil, reason) 表示无可用端点,reason 是供上层透传给调用方的简明原因。
// 选中端点时返回 (ep, "")。
func (s *Selector) Select(ctx context.Context, route *ModelRoute, coldStartThreshold int, skip map[string]bool) (*Endpoint, string) {
	cands := s.pool.Get().([]Candidate)
	cands = cands[:0]
	defer func() { s.pool.Put(cands) }()

	var skippedDisabled, skippedExcluded, skippedModel, skippedConcurrency, skippedScore int
	for _, ep := range route.Endpoints {
		if !ep.Enabled {
			skippedDisabled++
			continue
		}
		if skip[ep.KeyID] {
			skippedExcluded++
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
		// 无可用端点是导致 "retries exhausted / no available endpoint" 的根因，
		// 必须在 Warn 级别带上分项原因(disabled/excluded/model/concurrency/score),
		// 否则上层只看到 "no available endpoint" 无从排查(例如余额耗尽导致
		// 所有端点 Enabled=false → skipped_disabled,见 reloader.modelToEndpoints)。
		s.log.Warn("selector: no candidate endpoint",
			"model", route.Model,
			"endpoints", len(route.Endpoints),
			"skipped_disabled", skippedDisabled,
			"skipped_excluded", skippedExcluded,
			"skipped_model", skippedModel,
			"skipped_concurrency", skippedConcurrency,
			"skipped_score", skippedScore,
		)
		return nil, noEndpointReason(len(route.Endpoints), skippedDisabled, skippedExcluded, skippedModel, skippedConcurrency, skippedScore)
	}
	return weightedPick(cands, s.randFloat64()), ""
}

// noEndpointReason 把过滤分项拼成一句简明原因,既写进 Warn 日志,也透传给
// 调用方的错误消息(retries exhausted ... : <reason>),便于一眼定位根因。
func noEndpointReason(total, disabled, excluded, model, concurrency, score int) string {
	return fmt.Sprintf(
		"all %d endpoint(s) filtered out (disabled=%d[likely insufficient balance/inactive], excluded=%d[dead/rate-limited], model_mismatch=%d, concurrency_full=%d, zero_score=%d)",
		total, disabled, excluded, model, concurrency, score,
	)
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
	return time.Now().UnixNano()
}
