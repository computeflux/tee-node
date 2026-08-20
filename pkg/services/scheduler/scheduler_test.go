package scheduler

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// mockClient 是测试用的 provider client。
type mockClient struct {
	mu          sync.Mutex
	callsByKey  map[string]int
	behaviors   map[string][]error
	respByKey   map[string][]byte
	defaultBody []byte
}

func newMockClient() *mockClient {
	return &mockClient{
		callsByKey:  map[string]int{},
		behaviors:   map[string][]error{},
		respByKey:   map[string][]byte{},
		defaultBody: []byte(`{"ok":true}`),
	}
}

func (m *mockClient) QueueErrors(keyID string, errs ...error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.behaviors[keyID] = append(m.behaviors[keyID], errs...)
}

func (m *mockClient) nextError(keyID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	q := m.behaviors[keyID]
	if len(q) == 0 {
		return nil
	}
	err := q[0]
	m.behaviors[keyID] = q[1:]
	return err
}

func (m *mockClient) Do(ctx context.Context, ep *Endpoint, body []byte) ([]byte, error) {
	m.mu.Lock()
	m.callsByKey[ep.KeyID]++
	m.mu.Unlock()
	if err := m.nextError(ep.KeyID); err != nil {
		return nil, err
	}
	return m.defaultBody, nil
}

func (m *mockClient) Stream(ctx context.Context, ep *Endpoint, body []byte) (Stream, error) {
	m.mu.Lock()
	m.callsByKey[ep.KeyID]++
	m.mu.Unlock()
	if err := m.nextError(ep.KeyID); err != nil {
		return nil, err
	}
	return &mockStream{chunks: []*StreamChunk{
		{Data: []byte("chunk1"), Index: 0},
		{Data: []byte("chunk2"), Index: 1, IsFinal: true},
	}}, nil
}

func (m *mockClient) callCount(keyID string) int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.callsByKey[keyID]
}

func (m *mockClient) totalCalls() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	total := 0
	for _, n := range m.callsByKey {
		total += n
	}
	return total
}

type mockStream struct {
	chunks []*StreamChunk
	idx    int
	mu     sync.Mutex
}

func (s *mockStream) Next(ctx context.Context) (*StreamChunk, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.idx >= len(s.chunks) {
		return nil, nil
	}
	c := s.chunks[s.idx]
	s.idx++
	return c, nil
}

func (s *mockStream) Close() error { return nil }

func newTestScheduler(t *testing.T, maxConc int) (*Scheduler, *Registry, *mockClient) {
	reg := NewRegistry()
	routes := []*ModelRoute{
		{
			Model:          "gpt-4o",
			MaxConcurrency: maxConc,
			Endpoints: []*Endpoint{
				{Provider: "openai", ModelUrls: map[uint16]string{1: "https://u.test"}, KeyID: "key-a", KeyValue: "v-a", Weight: 1.0, Enabled: true, Models: []ModelKey{"*"}, Concurrency: 100},
				{Provider: "openai", ModelUrls: map[uint16]string{1: "https://u.test"}, KeyID: "key-b", KeyValue: "v-b", Weight: 1.0, Enabled: true, Models: []ModelKey{"*"}, Concurrency: 100},
				{Provider: "openai", ModelUrls: map[uint16]string{1: "https://u.test"}, KeyID: "key-c", KeyValue: "v-c", Weight: 1.0, Enabled: true, Models: []ModelKey{"*"}, Concurrency: 100},
			},
			FallbackModels: []ModelKey{"claude-3-5"},
		},
		{
			Model:          "claude-3-5",
			MaxConcurrency: maxConc,
			Endpoints: []*Endpoint{
				{Provider: "anthropic", ModelUrls: map[uint16]string{2: "https://u.test"}, KeyID: "key-x", KeyValue: "v-x", Weight: 1.0, Enabled: true, Models: []ModelKey{"*"}, Concurrency: 100},
			},
		},
	}
	reg.Replace(routes)
	obs := NoopObservability()
	sch := New(DefaultConfig(), reg, obs)
	client := newMockClient()
	sch.RegisterClients(map[ProviderID]ProviderClient{
		"openai":    client,
		"anthropic": client,
	})
	return sch, reg, client
}

// --- 基础测试 ---

func TestSchedule_HappyPath(t *testing.T) {
	sch, _, _ := newTestScheduler(t, 100)
	res, err := sch.Schedule(context.Background(), Invoke{Model: "gpt-4o", Body: []byte(`{}`)})
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if string(res.Body) != `{"ok":true}` {
		t.Errorf("bad body: %s", res.Body)
	}
	if res.Stream != nil {
		t.Errorf("expected nil stream")
	}
}

func TestSchedule_RetryOn429(t *testing.T) {
	sch, _, client := newTestScheduler(t, 100)
	// 给所有端点都排队 429，确保无论 selector 先选哪个都会触发重试
	for _, k := range []string{"key-a", "key-b", "key-c"} {
		client.QueueErrors(k, &UpstreamError{Status: 429, Message: "rate limit"})
	}

	res, err := sch.Schedule(context.Background(), Invoke{Model: "gpt-4o", Body: []byte(`{}`)})
	if err != nil {
		t.Fatalf("expected success after retry, got: %v", err)
	}
	if total := client.totalCalls(); total < 2 {
		t.Errorf("expected at least 2 calls, got %d", total)
	}
	t.Logf("success after %d calls, used key=%s", client.totalCalls(), res.Meta.KeyID)
}

func TestSchedule_AuthError_DeadKey(t *testing.T) {
	sch, reg, client := newTestScheduler(t, 100)
	// 增加重试次数确保所有 3 个 endpoint 都有机会被尝试（加权随机选择下概率极低）
	sch.cfg.MaxRetries = 20
	for _, k := range []string{"key-a", "key-b", "key-c"} {
		for i := 0; i < 5; i++ {
			client.QueueErrors(k, &UpstreamError{Status: 401})
		}
	}
	// claude-3-5 fallback 也会成功，所以测试一个无 fallback 的模型
	reg.Replace([]*ModelRoute{{
		Model: "no-fallback",
		Endpoints: []*Endpoint{
			{Provider: "openai", ModelUrls: map[uint16]string{1: "https://u.test"}, KeyID: "key-a", KeyValue: "v-a", Weight: 1.0, Enabled: true, Models: []ModelKey{"*"}, Concurrency: 100},
			{Provider: "openai", ModelUrls: map[uint16]string{1: "https://u.test"}, KeyID: "key-b", KeyValue: "v-b", Weight: 1.0, Enabled: true, Models: []ModelKey{"*"}, Concurrency: 100},
			{Provider: "openai", ModelUrls: map[uint16]string{1: "https://u.test"}, KeyID: "key-c", KeyValue: "v-c", Weight: 1.0, Enabled: true, Models: []ModelKey{"*"}, Concurrency: 100},
		},
	}})
	_, err := sch.Schedule(context.Background(), Invoke{Model: "no-fallback", Body: []byte(`{}`)})
	if !errors.Is(err, ErrAllEndpointsDead) {
		t.Fatalf("expected ErrAllEndpointsDead, got: %v", err)
	}
}

func TestSchedule_FallbackModel(t *testing.T) {
	sch, _, client := newTestScheduler(t, 100)
	for _, k := range []string{"key-a", "key-b", "key-c"} {
		for i := 0; i < 5; i++ {
			client.QueueErrors(k, &UpstreamError{Status: 503})
		}
	}
	res, err := sch.Schedule(context.Background(), Invoke{Model: "gpt-4o", Body: []byte(`{}`)})
	if err != nil {
		t.Fatalf("expected fallback success, got: %v", err)
	}
	if res.Meta.Provider != "anthropic" {
		t.Errorf("expected anthropic fallback, got: %s", res.Meta.Provider)
	}
}

// --- Streaming 测试 ---

func TestSchedule_Stream(t *testing.T) {
	sch, _, _ := newTestScheduler(t, 100)
	res, err := sch.ScheduleStream(context.Background(), Invoke{Model: "gpt-4o", Body: []byte(`{}`), Stream: true})
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if res.Stream == nil {
		t.Fatal("expected non-nil stream")
	}
	defer res.Stream.Close()

	c1, err := res.Stream.Next(context.Background())
	if err != nil || c1 == nil {
		t.Fatalf("expected chunk 1, got err=%v chunk=%v", err, c1)
	}
	if string(c1.Data) != "chunk1" {
		t.Errorf("bad chunk 1: %s", c1.Data)
	}
	c2, err := res.Stream.Next(context.Background())
	if err != nil || c2 == nil {
		t.Fatalf("expected chunk 2, got err=%v chunk=%v", err, c2)
	}
	if !c2.IsFinal {
		t.Error("expected final chunk")
	}
}

func TestSchedule_Stream_HandshakeFailRetry(t *testing.T) {
	sch, _, client := newTestScheduler(t, 100)
	client.QueueErrors("key-a", &UpstreamError{Status: 503})
	res, err := sch.ScheduleStream(context.Background(), Invoke{Model: "gpt-4o", Body: []byte(`{}`), Stream: true})
	if err != nil {
		t.Fatalf("expected success, got: %v", err)
	}
	if res.Stream == nil {
		t.Fatal("expected non-nil stream after retry")
	}
	defer res.Stream.Close()
}

// --- 并发信号量测试 ---

func TestSemaphore_ConcurrencyLimit(t *testing.T) {
	sch, _, _ := newTestScheduler(t, 2)

	blocking := make(chan struct{})
	gotCount := atomic.Int32{}
	slowClient := &blockingClient{
		hold: blocking,
		hook: func() { gotCount.Add(1) },
	}
	sch.RegisterClients(map[ProviderID]ProviderClient{
		"openai":    slowClient,
		"anthropic": slowClient,
	})

	var wg sync.WaitGroup
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _ = sch.Schedule(context.Background(), Invoke{Model: "gpt-4o", Body: []byte(`{}`)})
		}()
	}
	time.Sleep(100 * time.Millisecond)
	if got := gotCount.Load(); got != 2 {
		t.Errorf("expected 2 concurrent calls, got %d", got)
	}
	close(blocking)
	wg.Wait()
}

type blockingClient struct {
	hold chan struct{}
	hook func()
}

func (c *blockingClient) Do(ctx context.Context, ep *Endpoint, body []byte) ([]byte, error) {
	if c.hook != nil {
		c.hook()
	}
	<-c.hold
	return []byte(`{}`), nil
}

func (c *blockingClient) Stream(ctx context.Context, ep *Endpoint, body []byte) (Stream, error) {
	return nil, errors.New("not used")
}

// --- 熔断器测试 ---

func TestCircuitBreaker_OpensOnHighFailureRate(t *testing.T) {
	cfg := DefaultConfig()
	cfg.BreakerConfig.MinSampleSize = 10
	cfg.BreakerConfig.FailureRate = 0.5
	cfg.BreakerConfig.OpenDuration = 100 * time.Millisecond

	reg := NewRegistry()
	reg.Replace([]*ModelRoute{{
		Model: "m",
		Endpoints: []*Endpoint{
			{Provider: "p", ModelUrls: map[uint16]string{1: "https://u.test"}, KeyID: "k", Enabled: true, Models: []ModelKey{"*"}, Concurrency: 100, Weight: 1.0},
		},
	}})
	obs := NoopObservability()
	sch := New(cfg, reg, obs)

	br := sch.breakers.GetOrCreate(EndpointKey{Provider: "p", KeyID: "k"})
	route := reg.LookupModel("m")
	if route == nil || len(route.Endpoints) == 0 {
		t.Fatal("expected route m with endpoints")
	}
	ep := route.Endpoints[0]

	for i := 0; i < 15; i++ {
		ep.metrics.TotalRequests.Add(1)
		ep.metrics.ErrorCount5xx.Add(1)
	}
	br.Tick(ep.metrics, noopLogger{})
	if !ep.metrics.CircuitOpen.Load() {
		t.Errorf("expected CircuitOpen flag set")
	}

	time.Sleep(150 * time.Millisecond)
	if !br.Allow(ep.metrics) {
		t.Errorf("expected breaker to allow after timeout (half-open)")
	}
}

// --- Registry 测试 ---

func TestRegistry_HotReplace(t *testing.T) {
	reg := NewRegistry()
	reg.Replace([]*ModelRoute{{Model: "m1", Endpoints: []*Endpoint{
		{Provider: "p", ModelUrls: map[uint16]string{1: "https://u.test"}, KeyID: "k1", Enabled: true, Models: []ModelKey{"*"}, Weight: 1.0},
	}}})
	if reg.LookupModel("m1") == nil {
		t.Fatal("expected m1")
	}
	if reg.LookupModel("m2") != nil {
		t.Fatal("expected nil for m2")
	}

	reg.Replace([]*ModelRoute{{Model: "m2", Endpoints: []*Endpoint{
		{Provider: "p", ModelUrls: map[uint16]string{1: "https://u.test"}, KeyID: "k2", Enabled: true, Models: []ModelKey{"*"}, Weight: 1.0},
	}}})
	if reg.LookupModel("m1") != nil {
		t.Fatal("expected m1 to be gone after replace")
	}
	if reg.LookupModel("m2") == nil {
		t.Fatal("expected m2 after replace")
	}
}

// --- 评分算法测试 ---

func TestComputeScore(t *testing.T) {
	m := &EndpointMetrics{}
	if score := computeScore(2.0, m, 20); score != 2.0 {
		t.Errorf("cold start should return weight, got %v", score)
	}
	for i := 0; i < 50; i++ {
		m.TotalRequests.Add(1)
		m.SuccessCount.Add(1)
	}
	if score := computeScore(2.0, m, 20); score < 1.99 || score > 2.01 {
		t.Errorf("expected ~2.0 with all-success no-load, got %v", score)
	}
	m.InFlight.Store(10)
	if score := computeScore(2.0, m, 20); score < 0.99 || score > 1.01 {
		t.Errorf("expected ~1.0 with in_flight=10, got %v", score)
	}
	for i := 0; i < 50; i++ {
		m.TotalRequests.Add(1)
		m.ErrorCount5xx.Add(1)
	}
	if score := computeScore(2.0, m, 20); score < 0.49 || score > 0.51 {
		t.Errorf("expected ~0.5, got %v", score)
	}
	m.CircuitOpen.Store(true)
	// 熔断标记不再过滤 score,熔断由 Breaker.Allow 在调用阶段拦截。
	if score := computeScore(2.0, m, 20); score < 0.49 || score > 0.51 {
		t.Errorf("circuit open should not change score (breaker gates it instead), got %v", score)
	}
}

// --- 错误分类测试 ---

func TestClassify(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want ErrorClass
	}{
		{"429", &UpstreamError{Status: 429}, ClassRateLimit},
		{"401", &UpstreamError{Status: 401}, ClassAuth},
		{"402", &UpstreamError{Status: 402}, ClassAuth},
		{"403", &UpstreamError{Status: 403}, ClassAuth},
		{"500", &UpstreamError{Status: 500}, ClassTransient},
		{"503", &UpstreamError{Status: 503}, ClassTransient},
		{"400", &UpstreamError{Status: 400}, ClassClient},
		{"422", &UpstreamError{Status: 422}, ClassClient},
		{"rate_limit_text", &UpstreamError{Status: 200, Message: "rate limit exceeded"}, ClassRateLimit},
		{"network_error", errors.New("dial tcp: connection refused"), ClassTransient},
		{"ctx_canceled", context.Canceled, ClassCancel},
		{"ctx_deadline", context.DeadlineExceeded, ClassCancel},
		{"unknown", errors.New("foo bar"), ClassUnknown},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := Classify(c.err)
			if got != c.want {
				t.Errorf("got %v, want %v", got, c.want)
			}
		})
	}
}

// --- 大规模压测 ---

func TestScale_ThousandsOfEndpoints(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping scale test in short mode")
	}
	reg := NewRegistry()
	const numModels = 100
	const endpointsPerModel = 50
	var routes []*ModelRoute
	for i := 0; i < numModels; i++ {
		m := ModelKey(fmt.Sprintf("model-%d", i))
		eps := make([]*Endpoint, 0, endpointsPerModel)
		for j := 0; j < endpointsPerModel; j++ {
			providerName := ProviderID(fmt.Sprintf("provider-%d", j%5))
			eps = append(eps, &Endpoint{
				Provider:  ProviderID(providerName),
				KeyID:     fmt.Sprintf("key-%d-%d", i, j),
				KeyValue:  "v",
				Weight:    1.0,
				Enabled:   true,
				Models:    []ModelKey{"*"},
				ModelUrls: map[uint16]string{uint16(j%5) + 1: "https://u.test"},
			})
		}
		routes = append(routes, &ModelRoute{
			Model:     m,
			Endpoints: eps,
		})
	}
	t0 := time.Now()
	reg.Replace(routes)
	t.Logf("replaced %d models × %d endpoints in %v",
		numModels, endpointsPerModel, time.Since(t0))

	obs := NoopObservability()
	sch := New(DefaultConfig(), reg, obs)
	for _, p := range []ProviderID{"provider-0", "provider-1", "provider-2", "provider-3", "provider-4"} {
		sch.RegisterClients(map[ProviderID]ProviderClient{p: newMockClient()})
	}

	t0 = time.Now()
	for i := 0; i < 1000; i++ {
		_, _ = sch.Schedule(context.Background(), Invoke{
			Model: ModelKey(fmt.Sprintf("model-%d", i%numModels)),
			Body:  []byte(`{}`),
		})
	}
	elapsed := time.Since(t0)
	t.Logf("1000 schedules took %v, avg %v/op", elapsed, elapsed/1000)
	if avg := elapsed / 1000; avg > 100*time.Microsecond {
		t.Errorf("average selection took %v, expected < 100us", avg)
	}
}

// --- 死 key 不复用 ---

func TestDeadKeysNeverReset(t *testing.T) {
	sch, _, client := newTestScheduler(t, 100)
	for i := 0; i < 10; i++ {
		client.QueueErrors("key-a", &UpstreamError{Status: 401})
	}
	for i := 0; i < 3; i++ {
		_, _ = sch.Schedule(context.Background(), Invoke{Model: "gpt-4o", Body: []byte(`{}`)})
	}
	callsA := client.callCount("key-a")
	if callsA > 3 {
		t.Errorf("key-a called %d times, expected at most 3 (dead key should be skipped within a single request)", callsA)
	}
}

// --- 429 周期复用 ---

func TestRateLimitKeysCanReuse(t *testing.T) {
	sch, _, client := newTestScheduler(t, 100)
	for i := 0; i < 20; i++ {
		client.QueueErrors("key-a", &UpstreamError{Status: 429})
	}
	for i := 0; i < 3; i++ {
		_, err := sch.Schedule(context.Background(), Invoke{Model: "gpt-4o", Body: []byte(`{}`)})
		if err != nil {
			t.Fatalf("schedule #%d unexpected error: %v", i, err)
		}
	}
	callsA := client.callCount("key-a")
	if callsA > 3 {
		t.Errorf("key-a called %d times, expected at most 3 (rate-limited key should be reusable per request)", callsA)
	}
}

// --- 多协议 endpoint: 按入口协议选 client ---

func TestEndpoint_ProtocolForClient(t *testing.T) {
	// 同时支持 OpenAI(1) 与 ResponsesApi(4)
	ep := &Endpoint{ModelUrls: map[uint16]string{1: "https://oai.test", 4: "https://resp.test"}}
	if got := ep.ProtocolForClient("openai"); got != 1 {
		t.Errorf("ProtocolForClient(openai) = %d, want 1", got)
	}
	if got := ep.ProtocolForClient("openai-responses-api"); got != 4 {
		t.Errorf("ProtocolForClient(openai-responses-api) = %d, want 4", got)
	}
	// endpoint 不支持 anthropic → 回退到最小 key(NativeProtocol)
	if got := ep.ProtocolForClient("anthropic"); got != 1 {
		t.Errorf("ProtocolForClient(anthropic) = %d, want fallback 1", got)
	}
}

func TestIntegration_MultiProtocol_ResponsesInbound(t *testing.T) {
	reg := NewRegistry()
	reg.Replace([]*ModelRoute{{ //nolint:staticcheck // 单个路由即可
		Model: "gpt-5.6-terra",
		Endpoints: []*Endpoint{{
			Provider: "MetaMaskBail", KeyID: "26-0", KeyValue: "v",
			Weight: 1, Enabled: true, Models: []ModelKey{"*"},
			ModelName: "gpt-5.6-terra",
			// endpoint 同时支持 OpenAI 与 Responses
			ModelUrls: map[uint16]string{1: "https://oai.test", 4: "https://resp.test"},
		}},
	}})

	cfg := DefaultConfig()
	sch := New(cfg, reg, NoopObservability())

	oaiClient := newMockClient()
	respClient := newMockClient()
	sch.RegisterClients(map[ProviderID]ProviderClient{
		"openai":               oaiClient,
		"openai-responses-api": respClient,
	})

	// Responses 入口请求：ctx 注入 inbound=openai-responses-api
	ctx := WithRequest(context.Background(), "openai-responses-api", "/v1/responses")
	res, err := sch.Schedule(ctx, Invoke{Model: "gpt-5.6-terra", Body: []byte(`{}`)})
	if err != nil {
		t.Fatalf("schedule: %v", err)
	}
	if res.Meta.Endpoint.KeyID != "26-0" {
		t.Fatalf("unexpected endpoint: %s", res.Meta.Endpoint.KeyID)
	}
	// Responses inbound 应命中 responses client, 而不是 openai client
	if respClient.callCount("26-0") != 1 {
		t.Errorf("openai-responses-api client not used, calls=%d", respClient.callCount("26-0"))
	}
	if oaiClient.callCount("26-0") != 0 {
		t.Errorf("openai client wrongly used, calls=%d", oaiClient.callCount("26-0"))
	}
}
