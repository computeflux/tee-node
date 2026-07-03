package scheduler

import (
	"context"
	"sync"
	"sync/atomic"
	"time"
)

// ModelSemaphore 限制单个模型的全局并发数。
// 支持软限制：capacity 是目标上限，softCap 是容忍上限。
// 当 inflight < capacity 时正常放行；
// 当 capacity <= inflight < softCap 时允许通过（软限制，不阻塞）；
// 当 inflight >= softCap 时进入 FIFO 等待队列。
type ModelSemaphore struct {
	capacity int64
	softCap  int64
	inflight atomic.Int64
	mu       sync.Mutex
	waiters  []chan struct{} // FIFO 等待队列
}

// NewModelSemaphore 创建信号量。
// capacity <= 0 表示不限流；softRatio <= 0 时使用硬限制（softCap = capacity）。
func NewModelSemaphore(capacity int, softRatio float64) *ModelSemaphore {
	if capacity <= 0 {
		return nil
	}
	softCap := int64(capacity)
	if softRatio > 1.0 {
		softCap = int64(float64(capacity) * softRatio)
		if softCap == int64(capacity) {
			softCap++ // 确保软限制 > 硬限制
		}
	}
	return &ModelSemaphore{capacity: int64(capacity), softCap: softCap}
}

// Acquire 获取信号量。
// 当 s == nil 时(不限流模式),立即返回 noop release。
func (s *ModelSemaphore) Acquire(ctx context.Context) (func(), error) {
	if s == nil {
		return func() {}, nil
	}

	// 快速路径: 在硬限制内
	if s.inflight.Add(1) <= s.capacity {
		return s.boundedRelease(), nil
	}

	// 软限制路径: 在硬限制和软限制之间，允许通过但不竞争
	if s.inflight.Load() <= s.softCap {
		return s.softRelease(), nil
	}

	// 超过软限制，进入 FIFO 等待队列
	s.inflight.Add(-1)

	wake := make(chan struct{})
	s.mu.Lock()
	s.waiters = append(s.waiters, wake)
	s.mu.Unlock()

	for {
		select {
		case <-wake:
			// 被唤醒 → 再次尝试获取
			if s.inflight.Add(1) <= s.softCap {
				return s.boundedRelease(), nil
			}
			s.inflight.Add(-1)
			// 重新排队
			wake2 := make(chan struct{})
			s.mu.Lock()
			s.waiters = append(s.waiters, wake2)
			s.mu.Unlock()
			wake = wake2

		case <-ctx.Done():
			// 从等待队列移除自己
			s.mu.Lock()
			for i, w := range s.waiters {
				if w == wake {
					s.waiters = append(s.waiters[:i], s.waiters[i+1:]...)
					break
				}
			}
			s.mu.Unlock()
			return nil, ErrCanceled
		}
	}
}

// boundedRelease 返回硬限制下的 release 函数（释放后唤醒等待者）。
func (s *ModelSemaphore) boundedRelease() func() {
	return func() {
		s.inflight.Add(-1)
		s.mu.Lock()
		if len(s.waiters) > 0 {
			w := s.waiters[0]
			s.waiters = s.waiters[1:]
			s.mu.Unlock()
			close(w)
			return
		}
		s.mu.Unlock()
	}
}

// softRelease 返回软限制下的 release 函数（仅递减 inflight，不唤醒等待者）。
func (s *ModelSemaphore) softRelease() func() {
	return func() {
		s.inflight.Add(-1)
	}
}

// SemaphoreSet 是所有模型信号量的容器。
type SemaphoreSet struct {
	mu        sync.RWMutex
	cache     map[ModelKey]*ModelSemaphore
	softRatio float64 // 软限制倍率
}

// NewSemaphoreSet 创建空 SemaphoreSet。
func NewSemaphoreSet(softRatio float64) *SemaphoreSet {
	if softRatio <= 0 {
		softRatio = 1.0 // 默认硬限制
	}
	return &SemaphoreSet{cache: make(map[ModelKey]*ModelSemaphore), softRatio: softRatio}
}

// GetOrCreate 根据路由配置返回(可能新建)信号量。
func (s *SemaphoreSet) GetOrCreate(m ModelKey, capacity int) *ModelSemaphore {
	s.mu.RLock()
	if sem, ok := s.cache[m]; ok {
		s.mu.RUnlock()
		return sem
	}
	s.mu.RUnlock()

	s.mu.Lock()
	defer s.mu.Unlock()
	if sem, ok := s.cache[m]; ok {
		return sem
	}
	sem := NewModelSemaphore(capacity, s.softRatio)
	s.cache[m] = sem
	return sem
}

// --- Circuit Breaker ---

// BreakerConfig 是单端点熔断器配置。
type BreakerConfig struct {
	FailureRate    float64       // 错误率阈值,默认 0.5
	MinSampleSize  int           // 最小样本数,默认 20
	OpenDuration   time.Duration // Open 状态持续时间,默认 10s
	HalfOpenProbes int           // Half-open 探测次数,默认 3
}

// DefaultBreakerConfig 返回默认配置。
func DefaultBreakerConfig() BreakerConfig {
	return BreakerConfig{
		FailureRate:    0.5,
		MinSampleSize:  20,
		OpenDuration:   10 * time.Second,
		HalfOpenProbes: 3,
	}
}

// Breaker 是单个端点的熔断器。
// 状态机:Closed → Open → HalfOpen → Closed/Open
type Breaker struct {
	cfg         BreakerConfig
	state       atomic.Int32 // 0=Closed 1=Open 2=HalfOpen
	openUntilNs atomic.Int64
	hoSuccess   atomic.Int32
	hoFailure   atomic.Int32
}

const (
	breakerClosed   int32 = 0
	breakerOpen     int32 = 1
	breakerHalfOpen int32 = 2
)

// NewBreaker 创建熔断器。
func NewBreaker(cfg BreakerConfig) *Breaker {
	return &Breaker{cfg: cfg}
}

// Allow 判断是否允许请求通过。
func (b *Breaker) Allow(metrics *EndpointMetrics) bool {
	st := b.state.Load()
	switch st {
	case breakerClosed:
		// 与 metrics.CircuitOpen 联动(外部 Tick 设置)
		return !metrics.CircuitOpen.Load()

	case breakerOpen:
		if time.Now().UnixNano() >= b.openUntilNs.Load() {
			if b.state.CompareAndSwap(breakerOpen, breakerHalfOpen) {
				b.hoSuccess.Store(0)
				b.hoFailure.Store(0)
			}
			return true
		}
		return false

	case breakerHalfOpen:
		// 只放行 HalfOpenProbes 个探测
		total := b.hoSuccess.Load() + b.hoFailure.Load()
		return total < int32(b.cfg.HalfOpenProbes)
	}
	return false
}

// RecordSuccess 请求成功时调用。
func (b *Breaker) RecordSuccess(metrics *EndpointMetrics, log Logger) {
	if log == nil {
		log = noopLogger{}
	}
	if b.state.Load() == breakerHalfOpen {
		n := b.hoSuccess.Add(1)
		if n >= int32(b.cfg.HalfOpenProbes) {
			b.state.Store(breakerClosed)
			metrics.CircuitOpen.Store(false)
			log.Info("breaker closed after half-open success", "probes_success", n)
		}
	}
}

// RecordFailure 请求失败(transient / rate_limit)时调用。
func (b *Breaker) RecordFailure(metrics *EndpointMetrics, log Logger) {
	if log == nil {
		log = noopLogger{}
	}
	if b.state.Load() == breakerHalfOpen {
		n := b.hoFailure.Add(1)
		if n >= int32(b.cfg.HalfOpenProbes) {
			b.openUntilNs.Store(time.Now().Add(b.cfg.OpenDuration).UnixNano())
			b.state.Store(breakerOpen)
			metrics.CircuitOpen.Store(true)
			log.Warn("breaker reopened after half-open failure", "probes_failure", n, "open_until", time.Now().Add(b.cfg.OpenDuration))
		}
	}
}

// Tick 由后台 goroutine 周期性调用,根据滑动窗口决定是否开/关熔断。
func (b *Breaker) Tick(metrics *EndpointMetrics, log Logger) {
	if log == nil {
		log = noopLogger{}
	}
	if b.state.Load() != breakerClosed {
		return
	}
	// 原子地交换出旧值并清零，避免清零覆盖新请求的数据
	total := metrics.TotalRequests.Swap(0)
	if total < int64(b.cfg.MinSampleSize) {
		return
	}
	_ = metrics.SuccessCount.Swap(0)
	errs429 := metrics.ErrorCount429.Swap(0)
	errs5xx := metrics.ErrorCount5xx.Swap(0)

	errs := errs429 + errs5xx
	if errs == 0 {
		return
	}

	rate := float64(errs) / float64(total)
	if rate >= b.cfg.FailureRate {
		b.openUntilNs.Store(time.Now().Add(b.cfg.OpenDuration).UnixNano())
		b.state.Store(breakerOpen)
		metrics.CircuitOpen.Store(true)
		log.Warn("breaker opened due to high failure rate", "failure_rate", rate, "threshold", b.cfg.FailureRate, "sample_size", total, "errors", errs)
	}
}

// State 返回当前状态字符串(供 metrics 导出)。
func (b *Breaker) State() string {
	switch b.state.Load() {
	case breakerOpen:
		return "open"
	case breakerHalfOpen:
		return "half_open"
	default:
		return "closed"
	}
}

// BreakerSet 是所有端点熔断器的容器。
type BreakerSet struct {
	cfg  BreakerConfig
	mu   sync.RWMutex
	set  map[EndpointKey]*Breaker
	log  Logger
}

// NewBreakerSet 创建 BreakerSet。
func NewBreakerSet(cfg BreakerConfig, log Logger) *BreakerSet {
	if log == nil {
		log = noopLogger{}
	}
	return &BreakerSet{cfg: cfg, set: make(map[EndpointKey]*Breaker), log: log}
}

// GetOrCreate 获取某端点的熔断器(按需新建)。
func (bs *BreakerSet) GetOrCreate(k EndpointKey) *Breaker {
	bs.mu.RLock()
	if b, ok := bs.set[k]; ok {
		bs.mu.RUnlock()
		return b
	}
	bs.mu.RUnlock()

	bs.mu.Lock()
	defer bs.mu.Unlock()
	if b, ok := bs.set[k]; ok {
		return b
	}
	b := NewBreaker(bs.cfg)
	bs.set[k] = b
	bs.log.Debug("breaker created", "endpoint", k.String())
	return b
}

// RunTicker 启动后台 ticker,定期评估所有端点的熔断状态。
// ctx 取消时停止。
func (bs *BreakerSet) RunTicker(ctx context.Context, metrics EndpointMetricsSource, interval time.Duration) {
	bs.log.Info("breaker ticker started", "interval", interval)
	defer bs.log.Info("breaker ticker stopped")
	t := time.NewTicker(interval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			var ticked int
			bs.mu.RLock()
			for k, b := range bs.set {
				m := metrics.GetOrCreate(k)
				if m == nil {
					continue
				}
				b.Tick(m, bs.log)
				ticked++
			}
			bs.mu.RUnlock()
			bs.log.Debug("breaker tick completed", "endpoints_checked", ticked)
		}
	}
}
