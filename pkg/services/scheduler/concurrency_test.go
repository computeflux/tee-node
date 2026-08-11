package scheduler

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestBreaker() (*Breaker, *EndpointMetrics) {
	cfg := DefaultBreakerConfig()
	cfg.OpenDuration = 50 * time.Millisecond
	cfg.HalfOpenProbes = 3
	cfg.FailureRate = 0.5
	cfg.MinSampleSize = 2
	return NewBreaker(cfg), &EndpointMetrics{}
}

// ── Breaker 状态机 ────────────────────────────────────────

func TestBreaker_ClosedAllows(t *testing.T) {
	b, m := newTestBreaker()
	assert.True(t, b.Allow(m), "closed state must allow requests")
}

func TestBreaker_OpenBlocksUntilTimeout(t *testing.T) {
	b, m := newTestBreaker()
	m.ErrorCount5xx.Store(10)
	m.TotalRequests.Store(10)
	b.Tick(m, nil)
	assert.Equal(t, breakerOpen, b.state.Load())
	assert.False(t, b.Allow(m), "open state must reject requests")
	assert.True(t, m.CircuitOpen.Load())
}

func TestBreaker_OpenToHalfOpenAfterTimeout(t *testing.T) {
	b, m := newTestBreaker()
	m.ErrorCount5xx.Store(10)
	m.TotalRequests.Store(10)
	b.Tick(m, nil)

	time.Sleep(80 * time.Millisecond) // 等待 openUntil 过期
	assert.True(t, b.Allow(m), "after open duration, first request should be allowed (half-open)")
	assert.Equal(t, breakerHalfOpen, b.state.Load())
}

// 回归测试：half-open 下成功/失败混合时不得卡死。
// 旧逻辑按“总探测数”放行，2 成功 + 1 失败会耗尽探测预算但两个阈值都没达到，
// 导致请求被永久拒绝、状态机卡死在 half-open。
func TestBreaker_HalfOpenMixedProbesNeverStuck(t *testing.T) {
	b, m := newTestBreaker()
	b.state.Store(breakerHalfOpen)

	// 混合结果：S, S, F —— 总数已达 3，但两个计数都 < 3
	b.RecordSuccess(m, nil)
	b.RecordSuccess(m, nil)
	b.RecordFailure(m, nil)

	// 必须仍然放行探测请求（而不是永久拒绝）
	assert.True(t, b.Allow(m), "mixed probes must not exhaust the probe budget")
	assert.Equal(t, breakerHalfOpen, b.state.Load())

	// 继续成功探测 → 达到阈值 → 关闭熔断
	b.RecordSuccess(m, nil)
	assert.Equal(t, breakerClosed, b.state.Load())
	assert.False(t, m.CircuitOpen.Load())
}

func TestBreaker_HalfOpenFailuresReopen(t *testing.T) {
	b, m := newTestBreaker()
	b.state.Store(breakerHalfOpen)

	b.RecordFailure(m, nil)
	b.RecordFailure(m, nil)
	b.RecordFailure(m, nil)

	assert.Equal(t, breakerOpen, b.state.Load())
	assert.True(t, m.CircuitOpen.Load())
	assert.False(t, b.Allow(m))
}

func TestBreaker_HalfOpenSuccessesClose(t *testing.T) {
	b, m := newTestBreaker()
	b.state.Store(breakerHalfOpen)

	b.RecordSuccess(m, nil)
	b.RecordSuccess(m, nil)
	b.RecordSuccess(m, nil)

	assert.Equal(t, breakerClosed, b.state.Load())
	assert.True(t, b.Allow(m))
}

func TestBreaker_TickRespectsMinSample(t *testing.T) {
	b, m := newTestBreaker()
	m.ErrorCount5xx.Store(1)
	m.TotalRequests.Store(1) // < MinSampleSize=2
	b.Tick(m, nil)
	assert.Equal(t, breakerClosed, b.state.Load(), "below min sample must not open breaker")
}

func TestBreaker_TickRateBelowThreshold(t *testing.T) {
	b, m := newTestBreaker()
	m.ErrorCount5xx.Store(1)
	m.TotalRequests.Store(10) // 10% < 50%
	b.Tick(m, nil)
	assert.Equal(t, breakerClosed, b.state.Load())
}

func TestBreaker_StateString(t *testing.T) {
	b, _ := newTestBreaker()
	assert.Equal(t, "closed", b.State())
	b.state.Store(breakerOpen)
	assert.Equal(t, "open", b.State())
	b.state.Store(breakerHalfOpen)
	assert.Equal(t, "half_open", b.State())
}

// ── ModelSemaphore ────────────────────────────────────────

func TestModelSemaphore_NilIsNoop(t *testing.T) {
	release, err := NewModelSemaphore(0, 1.0).Acquire(context.Background())
	require.NoError(t, err)
	release()
	assert.Equal(t, int64(0), (*ModelSemaphore)(nil).Inflight())
}

func TestModelSemaphore_UnderCapacity(t *testing.T) {
	s := NewModelSemaphore(2, 1.0)
	require.NotNil(t, s)

	r1, err := s.Acquire(context.Background())
	require.NoError(t, err)
	r2, err := s.Acquire(context.Background())
	require.NoError(t, err)
	assert.Equal(t, int64(2), s.Inflight())

	r1()
	r2()
	assert.Equal(t, int64(0), s.Inflight())
}

func TestModelSemaphore_BlocksOverCapacity(t *testing.T) {
	s := NewModelSemaphore(1, 1.0)
	r1, err := s.Acquire(context.Background())
	require.NoError(t, err)

	// 第二个请求必须等待
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	start := time.Now()
	_, err = s.Acquire(ctx)
	require.Error(t, err, "over capacity must block until ctx deadline")
	assert.ErrorIs(t, err, ErrCanceled)
	assert.GreaterOrEqual(t, time.Since(start), 150*time.Millisecond)

	// 释放后立即放行
	released := make(chan struct{})
	go func() {
		r2, err := s.Acquire(context.Background())
		require.NoError(t, err)
		r2()
		close(released)
	}()
	r1()
	select {
	case <-released:
	case <-time.After(time.Second):
		t.Fatal("acquire after release must succeed")
	}
}

func TestModelSemaphore_CtxCancelRemovesWaiter(t *testing.T) {
	s := NewModelSemaphore(1, 1.0)
	r1, err := s.Acquire(context.Background())
	require.NoError(t, err)
	defer r1()

	// 占满容量后，等待中的 Acquire 应随 ctx 取消而返回 ErrCanceled
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		_, err := s.Acquire(ctx)
		done <- err
	}()

	time.Sleep(50 * time.Millisecond) // 让等待者进入队列
	cancel()
	select {
	case err := <-done:
		require.ErrorIs(t, err, ErrCanceled, "waiter must be unblocked by ctx cancel")
	case <-time.After(time.Second):
		t.Fatal("Acquire did not return after ctx cancel")
	}

	// 等待者已移除：释放后再 acquire 不应唤醒已取消的等待者（也不 panic）
	r1()
	r2, err := s.Acquire(context.Background())
	require.NoError(t, err)
	r2()
}

// ── SemaphoreSet ──────────────────────────────────────────

func TestSemaphoreSet_GetOrCreate(t *testing.T) {
	ss := NewSemaphoreSet(1.0)
	m := ModelKey("model-a")

	s1 := ss.GetOrCreate(m, 3)
	s2 := ss.GetOrCreate(m, 3)
	assert.Same(t, s1, s2, "same model must reuse the semaphore")

	assert.Equal(t, int64(0), ss.Inflight(m))
	assert.Equal(t, int64(0), ss.Inflight(ModelKey("unknown")))
}

// ── BreakerSet ────────────────────────────────────────────

func TestBreakerSet_GetOrCreate(t *testing.T) {
	bs := NewBreakerSet(DefaultBreakerConfig(), nil)
	k := EndpointKey{Provider: "openai", KeyID: "k1"}

	b1 := bs.GetOrCreate(k)
	b2 := bs.GetOrCreate(k)
	assert.Same(t, b1, b2)
}
