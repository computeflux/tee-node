package scheduler

import (
	"context"
	"errors"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// testLogger 是测试用的日志捕获器。
type testLogger struct {
	mu      sync.Mutex
	entries []logEntry
}

type logEntry struct {
	level string
	msg   string
	kvs   map[string]any
}

func newTestLogger() *testLogger { return &testLogger{entries: make([]logEntry, 0, 64)} }

func (l *testLogger) Debug(msg string, kvs ...any) { l.record("DEBUG", msg, kvs) }
func (l *testLogger) Info(msg string, kvs ...any)  { l.record("INFO", msg, kvs) }
func (l *testLogger) Warn(msg string, kvs ...any)  { l.record("WARN", msg, kvs) }
func (l *testLogger) Error(msg string, kvs ...any) { l.record("ERROR", msg, kvs) }

func (l *testLogger) record(level, msg string, kvs []any) {
	l.mu.Lock()
	defer l.mu.Unlock()
	m := make(map[string]any, len(kvs)/2)
	for i := 0; i+1 < len(kvs); i += 2 {
		m[kvs[i].(string)] = kvs[i+1]
	}
	l.entries = append(l.entries, logEntry{level: level, msg: msg, kvs: m})
}

func (l *testLogger) Find(level, msgContains string) []logEntry {
	l.mu.Lock()
	defer l.mu.Unlock()
	var out []logEntry
	for _, e := range l.entries {
		if e.level == level && strings.Contains(e.msg, msgContains) {
			out = append(out, e)
		}
	}
	return out
}

func (l *testLogger) Count(level, msgContains string) int {
	return len(l.Find(level, msgContains))
}

func (l *testLogger) Reset() {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.entries = l.entries[:0]
}

// --- 场景 1: 正常流程 ---
func TestIntegration_HappyPath(t *testing.T) {
	log := newTestLogger()
	reg := NewRegistry()
	routes := []*ModelRoute{{
		Model: "gpt-4o", MaxConcurrency: 10,
		Endpoints: []*Endpoint{
			{Provider: "openai", KeyID: "key-1", KeyValue: "v", Weight: 1.0, Enabled: true, Models: []ModelKey{"*"}, Concurrency: 100},
		},
	}}
	reg.Replace(routes)

	cfg := DefaultConfig()
	cfg.Log = log
	sch := New(cfg, reg, NoopObservability())
	sch.AttachMetrics()
	sch.RegisterProvider("openai", newMockClient())

	res, err := sch.Schedule(context.Background(), Invoke{Model: "gpt-4o", Body: []byte(`{}`)})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(res.Body) != `{"ok":true}` {
		t.Errorf("bad body: %s", res.Body)
	}

	// 验证日志
	if log.Count("DEBUG", "schedule begin") != 1 {
		t.Errorf("expected 1 'schedule begin' debug log, got %d", log.Count("DEBUG", "schedule begin"))
	}
	if log.Count("DEBUG", "endpoint selected") != 1 {
		t.Errorf("expected 1 'endpoint selected' debug log, got %d", log.Count("DEBUG", "endpoint selected"))
	}
	if log.Count("INFO", "endpoint call success") != 1 {
		t.Errorf("expected 1 'endpoint call success' info log, got %d", log.Count("INFO", "endpoint call success"))
	}
	if log.Count("INFO", "schedule success") != 1 {
		t.Errorf("expected 1 'schedule success' info log, got %d", log.Count("INFO", "schedule success"))
	}
	if log.Count("INFO", "provider registered") != 1 {
		t.Errorf("expected 1 provider registration log, got %d", log.Count("INFO", "provider registered"))
	}
}

// --- 场景 2: Session 亲和性 ---
func TestIntegration_SessionAffinity(t *testing.T) {
	log := newTestLogger()
	reg := NewRegistry()
	routes := []*ModelRoute{{
		Model: "gpt-4o", MaxConcurrency: 10,
		Endpoints: []*Endpoint{
			{Provider: "openai", KeyID: "key-a", KeyValue: "v", Weight: 1.0, Enabled: true, Models: []ModelKey{"*"}, Concurrency: 100},
			{Provider: "openai", KeyID: "key-b", KeyValue: "v", Weight: 1.0, Enabled: true, Models: []ModelKey{"*"}, Concurrency: 100},
			{Provider: "openai", KeyID: "key-c", KeyValue: "v", Weight: 1.0, Enabled: true, Models: []ModelKey{"*"}, Concurrency: 100},
		},
	}}
	reg.Replace(routes)

	cfg := DefaultConfig()
	cfg.Log = log
	sch := New(cfg, reg, NoopObservability())
	sch.AttachMetrics()
	sch.RegisterProvider("openai", newMockClient())

	sessionID := "sess-abc"

	// 第一次请求：应记录亲和性
	res1, err := sch.Schedule(context.Background(), Invoke{Model: "gpt-4o", SessionID: sessionID, Body: []byte(`{}`)})
	if err != nil {
		t.Fatalf("first call failed: %v", err)
	}
	key1 := res1.Meta.KeyID

	if log.Count("DEBUG", "session affinity recorded") != 1 {
		t.Errorf("expected 1 affinity recorded log, got %d", log.Count("DEBUG", "session affinity recorded"))
	}

	log.Reset()

	// 第二次请求：应命中亲和性
	res2, err := sch.Schedule(context.Background(), Invoke{Model: "gpt-4o", SessionID: sessionID, Body: []byte(`{}`)})
	if err != nil {
		t.Fatalf("second call failed: %v", err)
	}
	key2 := res2.Meta.KeyID

	if key1 != key2 {
		t.Errorf("expected same key for affinity, got %s then %s", key1, key2)
	}
	if log.Count("DEBUG", "session affinity hit") != 1 {
		t.Errorf("expected 1 affinity hit log, got %d", log.Count("DEBUG", "session affinity hit"))
	}
	// 亲和性命中后仍然打印 "endpoint selected" 日志（因为 ep 被赋值后继续执行）
	if log.Count("DEBUG", "endpoint selected") < 1 {
		t.Errorf("expected at least 1 'endpoint selected' log for affinity hit, got %d", log.Count("DEBUG", "endpoint selected"))
	}

	log.Reset()

	// 第三次请求：无 sessionID，走正常选择
	_, err = sch.Schedule(context.Background(), Invoke{Model: "gpt-4o", SessionID: "", Body: []byte(`{}`)})
	if err != nil {
		t.Fatalf("third call failed: %v", err)
	}
	if log.Count("DEBUG", "session affinity hit") != 0 {
		t.Errorf("expected no affinity hit for empty session, got %d", log.Count("DEBUG", "session affinity hit"))
	}
	if log.Count("DEBUG", "endpoint selected") != 1 {
		t.Errorf("expected selector to run, got %d 'endpoint selected' logs", log.Count("DEBUG", "endpoint selected"))
	}
}

// --- 场景 3: 并发硬限制 ---
func TestIntegration_ConcurrencyHardLimit(t *testing.T) {
	log := newTestLogger()
	reg := NewRegistry()
	routes := []*ModelRoute{{
		Model: "gpt-4o", MaxConcurrency: 2, // 硬限制 2
		Endpoints: []*Endpoint{
			{Provider: "openai", KeyID: "key-a", KeyValue: "v", Weight: 1.0, Enabled: true, Models: []ModelKey{"*"}, Concurrency: 100},
		},
	}}
	reg.Replace(routes)

	cfg := DefaultConfig()
	cfg.Log = log
	cfg.SoftConcurrencyRatio = 1.0 // 硬限制
	sch := New(cfg, reg, NoopObservability())
	sch.AttachMetrics()

	// 阻塞 client，让请求挂住
	hold := make(chan struct{})
	client := &blockingClient{hold: hold, hook: func() {}}
	sch.RegisterProvider("openai", client)

	var wg sync.WaitGroup
	for i := 0; i < 3; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _ = sch.Schedule(context.Background(), Invoke{Model: "gpt-4o", Body: []byte(`{}`)})
		}()
	}

	// 等几秒让第三个请求被阻塞
	time.Sleep(300 * time.Millisecond)

	// 第三个请求应该被阻塞（等待队列中）
	// 释放前两个
	close(hold)
	wg.Wait()

	// 应该有 semaphore acquire 的日志（或阻塞日志，但由于是异步的，我们检查 inflight 逻辑）
	if log.Count("DEBUG", "schedule begin") != 3 {
		t.Errorf("expected 3 schedule begin logs, got %d", log.Count("DEBUG", "schedule begin"))
	}
	// 至少 2 个成功（前两个），第三个可能成功也可能超时（取决于等待时间）
	if log.Count("INFO", "endpoint call success") < 2 {
		t.Errorf("expected at least 2 success logs, got %d", log.Count("INFO", "endpoint call success"))
	}
}

// --- 场景 4: 软限制 ---
func TestIntegration_SoftLimit(t *testing.T) {
	log := newTestLogger()
	reg := NewRegistry()
	routes := []*ModelRoute{{
		Model: "gpt-4o", MaxConcurrency: 2, // 硬限制 2，软限制 3
		Endpoints: []*Endpoint{
			{Provider: "openai", KeyID: "key-a", KeyValue: "v", Weight: 1.0, Enabled: true, Models: []ModelKey{"*"}, Concurrency: 100},
		},
	}}
	reg.Replace(routes)

	cfg := DefaultConfig()
	cfg.Log = log
	cfg.SoftConcurrencyRatio = 1.5 // 软限制 = 3
	sch := New(cfg, reg, NoopObservability())
	sch.AttachMetrics()

	hold := make(chan struct{})
	var activeCount atomic.Int32
	client := &blockingClient{hold: hold, hook: func() { activeCount.Add(1) }}
	sch.RegisterProvider("openai", client)

	var wg sync.WaitGroup
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _ = sch.Schedule(context.Background(), Invoke{Model: "gpt-4o", Body: []byte(`{}`)})
		}()
	}

	// 等所有请求到达
	time.Sleep(200 * time.Millisecond)

	// 软限制 3，所以 3 个请求 active，1 个在等待
	got := activeCount.Load()
	if got < 3 {
		t.Errorf("expected at least 3 active with soft limit, got %d", got)
	}

	close(hold)
	wg.Wait()

	if log.Count("INFO", "schedule success") < 3 {
		t.Errorf("expected at least 3 success with soft limit, got %d", log.Count("INFO", "schedule success"))
	}
}

// --- 场景 5: 熔断器 ---
func TestIntegration_CircuitBreaker(t *testing.T) {
	log := newTestLogger()
	reg := NewRegistry()
	routes := []*ModelRoute{{
		Model: "gpt-4o", MaxConcurrency: 10,
		Endpoints: []*Endpoint{
			{Provider: "openai", KeyID: "key-a", KeyValue: "v", Weight: 1.0, Enabled: true, Models: []ModelKey{"*"}, Concurrency: 100},
		},
	}}
	reg.Replace(routes)

	cfg := DefaultConfig()
	cfg.BreakerConfig.MinSampleSize = 5
	cfg.BreakerConfig.FailureRate = 0.5
	cfg.BreakerConfig.OpenDuration = 100 * time.Millisecond
	cfg.Log = log
	sch := New(cfg, reg, NoopObservability())
	sch.AttachMetrics()

	client := newMockClient()
	client.QueueErrors("key-a", &UpstreamError{Status: 503})
	client.QueueErrors("key-a", &UpstreamError{Status: 503})
	client.QueueErrors("key-a", &UpstreamError{Status: 503})
	client.QueueErrors("key-a", &UpstreamError{Status: 503})
	client.QueueErrors("key-a", &UpstreamError{Status: 503})
	client.QueueErrors("key-a", &UpstreamError{Status: 503})
	client.QueueErrors("key-a", &UpstreamError{Status: 503})
	sch.RegisterProvider("openai", client)

	// 发送请求直到熔断器打开（需要手动触发 Tick，因为后台 goroutine 未启动）
	for i := 0; i < 7; i++ {
		_, _ = sch.Schedule(context.Background(), Invoke{Model: "gpt-4o", Body: []byte(`{}`)})
	}
	// 手动触发熔断器 Tick
	br := sch.breakers.GetOrCreate(EndpointKey{Provider: "openai", KeyID: "key-a"})
	ep := reg.LookupEndpoint(EndpointKey{Provider: "openai", KeyID: "key-a"})
	br.Tick(ep.metrics, log)

	// 检查熔断器打开日志
	if log.Count("WARN", "breaker opened") != 1 {
		t.Errorf("expected 1 breaker opened log, got %d", log.Count("WARN", "breaker opened"))
	}

	// 再次请求，应该被熔断器拦截
	log.Reset()
	_, err := sch.Schedule(context.Background(), Invoke{Model: "gpt-4o", Body: []byte(`{}`)})
	if log.Count("DEBUG", "breaker blocked") != 1 {
		t.Errorf("expected 1 breaker blocked log, got %d", log.Count("DEBUG", "breaker blocked"))
	}
	if err == nil {
		t.Fatalf("expected error when breaker is open, got nil")
	}

	log.Reset()

	// 等待半开
	time.Sleep(150 * time.Millisecond)

	// 恢复 key-a
	client = newMockClient()
	sch.RegisterProvider("openai", client)

	// 再次请求，应该触发半开探测并恢复
	_, err = sch.Schedule(context.Background(), Invoke{Model: "gpt-4o", Body: []byte(`{}`)})
	if err != nil {
		t.Fatalf("expected success after breaker half-open, got: %v", err)
	}

	if log.Count("INFO", "breaker closed") != 1 {
		t.Errorf("expected 1 breaker closed log, got %d", log.Count("INFO", "breaker closed"))
	}
}

// --- 场景 6: 重试 + Fallback ---
func TestIntegration_RetryAndFallback(t *testing.T) {
	log := newTestLogger()
	reg := NewRegistry()
	routes := []*ModelRoute{
		{
			Model: "gpt-4o", MaxConcurrency: 10,
			Endpoints: []*Endpoint{
				{Provider: "openai", KeyID: "key-a", KeyValue: "v", Weight: 1.0, Enabled: true, Models: []ModelKey{"*"}, Concurrency: 100},
			},
			FallbackModels: []ModelKey{"claude-3-5"},
		},
		{
			Model: "claude-3-5", MaxConcurrency: 10,
			Endpoints: []*Endpoint{
				{Provider: "anthropic", KeyID: "key-x", KeyValue: "v", Weight: 1.0, Enabled: true, Models: []ModelKey{"*"}, Concurrency: 100},
			},
		},
	}
	reg.Replace(routes)

	cfg := DefaultConfig()
	cfg.Log = log
	sch := New(cfg, reg, NoopObservability())
	sch.AttachMetrics()

	client := newMockClient()
	client.QueueErrors("key-a", &UpstreamError{Status: 503})
	client.QueueErrors("key-a", &UpstreamError{Status: 503})
	client.QueueErrors("key-a", &UpstreamError{Status: 503})
	client.QueueErrors("key-a", &UpstreamError{Status: 503})
	sch.RegisterProvider("openai", client)
	sch.RegisterProvider("anthropic", newMockClient())

	res, err := sch.Schedule(context.Background(), Invoke{Model: "gpt-4o", Body: []byte(`{}`)})
	if err != nil {
		t.Fatalf("expected fallback success, got: %v", err)
	}
	if res.Meta.Provider != "anthropic" {
		t.Errorf("expected anthropic fallback, got: %s", res.Meta.Provider)
	}

	if log.Count("INFO", "fallback triggered") != 1 {
		t.Errorf("expected 1 fallback triggered log, got %d", log.Count("INFO", "fallback triggered"))
	}
	if log.Count("INFO", "transient error, retrying") < 1 {
		t.Errorf("expected at least 1 retry log, got %d", log.Count("INFO", "transient error, retrying"))
	}
	if log.Count("INFO", "schedule success") != 1 {
		t.Errorf("expected 1 schedule success log, got %d", log.Count("INFO", "schedule success"))
	}
}

// --- 场景 7: 429 Rate Limit + 退避 ---
func TestIntegration_RateLimit429(t *testing.T) {
	log := newTestLogger()
	reg := NewRegistry()
	routes := []*ModelRoute{{
		Model: "gpt-4o", MaxConcurrency: 10,
		Endpoints: []*Endpoint{
			{Provider: "openai", KeyID: "key-a", KeyValue: "v", Weight: 1.0, Enabled: true, Models: []ModelKey{"*"}, Concurrency: 100},
			{Provider: "openai", KeyID: "key-b", KeyValue: "v", Weight: 1.0, Enabled: true, Models: []ModelKey{"*"}, Concurrency: 100},
			{Provider: "openai", KeyID: "key-c", KeyValue: "v", Weight: 1.0, Enabled: true, Models: []ModelKey{"*"}, Concurrency: 100},
		},
	}}
	reg.Replace(routes)

	cfg := DefaultConfig()
	cfg.Log = log
	sch := New(cfg, reg, NoopObservability())
	sch.AttachMetrics()

	client := newMockClient()
	client.QueueErrors("key-a", &UpstreamError{Status: 429, Message: "rate limit"})
	client.QueueErrors("key-b", &UpstreamError{Status: 429, Message: "rate limit"})
	sch.RegisterProvider("openai", client)

	res, err := sch.Schedule(context.Background(), Invoke{Model: "gpt-4o", Body: []byte(`{}`)})
	if err != nil {
		t.Fatalf("expected success after rate-limit retry, got: %v", err)
	}
	if res.Meta.Provider != "openai" {
		t.Errorf("expected openai after retry, got: %s", res.Meta.Provider)
	}

	if log.Count("INFO", "rate limit, backing off") < 1 {
		t.Errorf("expected at least 1 rate limit backoff log, got %d", log.Count("INFO", "rate limit, backing off"))
	}
	if log.Count("DEBUG", "backoff") < 1 {
		t.Errorf("expected at least 1 backoff log, got %d", log.Count("DEBUG", "backoff"))
	}
	if log.Count("INFO", "schedule success") != 1 {
		t.Errorf("expected 1 schedule success log, got %d", log.Count("INFO", "schedule success"))
	}
}

// --- 场景 8: 401 Auth Dead Key ---
func TestIntegration_AuthDeadKey(t *testing.T) {
	log := newTestLogger()
	reg := NewRegistry()
	routes := []*ModelRoute{{
		Model: "gpt-4o", MaxConcurrency: 10,
		Endpoints: []*Endpoint{
			{Provider: "openai", KeyID: "key-a", KeyValue: "v", Weight: 1.0, Enabled: true, Models: []ModelKey{"*"}, Concurrency: 100},
			{Provider: "openai", KeyID: "key-b", KeyValue: "v", Weight: 1.0, Enabled: true, Models: []ModelKey{"*"}, Concurrency: 100},
			{Provider: "openai", KeyID: "key-c", KeyValue: "v", Weight: 1.0, Enabled: true, Models: []ModelKey{"*"}, Concurrency: 100},
			{Provider: "openai", KeyID: "key-c", KeyValue: "v", Weight: 1.0, Enabled: true, Models: []ModelKey{"*"}, Concurrency: 100},
		},
	}}
	reg.Replace(routes)

	cfg := DefaultConfig()
	cfg.MaxRetries = 5
	cfg.Log = log
	sch := New(cfg, reg, NoopObservability())
	sch.AttachMetrics()

	client := newMockClient()
	client.QueueErrors("key-a", &UpstreamError{Status: 401})
	client.QueueErrors("key-b", &UpstreamError{Status: 401})
	client.QueueErrors("key-c", &UpstreamError{Status: 401})
	sch.RegisterProvider("openai", client)

	_, err := sch.Schedule(context.Background(), Invoke{Model: "gpt-4o", Body: []byte(`{}`)})
	if !errors.Is(err, ErrAllEndpointsDead) {
		t.Fatalf("expected ErrAllEndpointsDead, got: %v", err)
	}

	if log.Count("INFO", "auth error, marking key dead") < 3 {
		t.Errorf("expected at least 3 dead key logs, got %d", log.Count("INFO", "auth error, marking key dead"))
	}
	if log.Count("ERROR", "all endpoints dead") != 1 {
		t.Errorf("expected 1 'all endpoints dead' error log, got %d", log.Count("ERROR", "all endpoints dead"))
	}
	if log.Count("ERROR", "schedule failed") != 1 {
		t.Errorf("expected 1 'schedule failed' error log, got %d", log.Count("ERROR", "schedule failed"))
	}
}

// --- 场景 9: 日志级别过滤 ---
func TestIntegration_LogLevels(t *testing.T) {
	log := newTestLogger()
	reg := NewRegistry()
	routes := []*ModelRoute{{
		Model: "gpt-4o", MaxConcurrency: 10,
		Endpoints: []*Endpoint{
			{Provider: "openai", KeyID: "key-a", KeyValue: "v", Weight: 1.0, Enabled: true, Models: []ModelKey{"*"}, Concurrency: 100},
		},
	}}
	reg.Replace(routes)

	cfg := DefaultConfig()
	cfg.Log = log
	sch := New(cfg, reg, NoopObservability())
	sch.AttachMetrics()
	sch.RegisterProvider("openai", newMockClient())

	_, err := sch.Schedule(context.Background(), Invoke{Model: "gpt-4o", Body: []byte(`{}`)})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// 验证日志级别分布
	debugCount := len(log.Find("DEBUG", ""))
	infoCount := len(log.Find("INFO", ""))
	warnCount := len(log.Find("WARN", ""))
	errorCount := len(log.Find("ERROR", ""))

	if debugCount < 3 {
		t.Errorf("expected at least 3 debug logs, got %d", debugCount)
	}
	if infoCount < 2 {
		t.Errorf("expected at least 2 info logs, got %d", infoCount)
	}
	if warnCount != 0 {
		t.Errorf("expected 0 warn logs for happy path, got %d", warnCount)
	}
	if errorCount != 0 {
		t.Errorf("expected 0 error logs for happy path, got %d", errorCount)
	}
}
