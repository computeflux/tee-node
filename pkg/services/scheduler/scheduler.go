package scheduler

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"sync"
	"time"

	"go.opentelemetry.io/otel/attribute"
)

// Config 是调度器的核心配置。
type Config struct {
	MaxRetries           int
	BackoffInitial       time.Duration
	BackoffMax           time.Duration
	BackoffJitterMin     float64 // 默认 0.8
	BackoffJitterMax     float64 // 默认 1.2
	ColdStartSamples     int     // 端点冷启动样本阈值,默认 20
	EMADecay             float64 // 延迟 EMA 衰减,默认 0.3
	BreakerConfig        BreakerConfig
	BreakerTickInterval  time.Duration // 默认 5s
	Log                  Logger        // 可选日志接口，nil 时静默
	SoftConcurrencyRatio float64       // 软限制倍率，默认 1.0（硬限制）
	ModelRouteCacheTTL   time.Duration // 解析后 ModelRoute 缓存 TTL，默认 5s；0 禁用
}

// DefaultConfig 返回合理默认配置。
func DefaultConfig() Config {
	return Config{
		MaxRetries:           3,
		BackoffInitial:       200 * time.Millisecond,
		BackoffMax:           5 * time.Second,
		BackoffJitterMin:     0.8,
		BackoffJitterMax:     1.2,
		ColdStartSamples:     20,
		EMADecay:             0.3,
		BreakerConfig:        DefaultBreakerConfig(),
		BreakerTickInterval:  5 * time.Second,
		Log:                  noopLogger{},
		SoftConcurrencyRatio: 1.0,
		ModelRouteCacheTTL:   5 * time.Second,
	}
}

// Invoke 描述一次调用的入参。
type Invoke struct {
	Model     ModelKey
	Provider  ProviderID // 显式指定的 provider（如 "openai/BailLocal"）；空表示按 route/模型自动解析
	SessionID string     // 可选，同一 session 的请求优先复用上次端点以命中缓存
	Body      []byte     // 已序列化好的 HTTP body(JSON)
	Stream    bool       // 是否流式
}

// Result 描述一次调用的出参。
type Result struct {
	Body   []byte   // 非流式响应体
	Stream Stream   // 流式响应(非流时为 nil)
	Meta   CallMeta // 哪个端点被选中
}

// CallMeta 是结果元数据。
type CallMeta struct {
	Provider ProviderID
	KeyID    string
	Attempts int
	Score    float64
	// Endpoint 是被选中的完整端点(包括 KeyValue、BaseURL、ExtraHeaders)。
	// 调用方用这个来真正发起 HTTP 请求。Score 和 Provider/KeyID 都从 Endpoint 派生。
	Endpoint *Endpoint
}

// ProviderClient 是 provider 的抽象,由用户实现。
// Scheduler 通过这个接口调用真正的 LLM API。
type ProviderClient interface {
	// Do 非流式调用。
	Do(ctx context.Context, ep *Endpoint, body []byte) ([]byte, error)
	// Stream 流式调用。返回的 Stream 必须实现 ctx 取消时立即退出 Next。
	Stream(ctx context.Context, ep *Endpoint, body []byte) (Stream, error)
}

// Scheduler 是主入口。
type Scheduler struct {
	cfg        Config
	resolver   ModelResolver
	routeCache *ModelRouteCache
	sems       *SemaphoreSet
	breakers   *BreakerSet
	selector   *Selector
	obs        *Observability
	log        Logger
	affinity   *SessionAffinity // session 亲和性：同一 session 复用上次端点

	metricsMu sync.RWMutex
	metrics   map[EndpointKey]*EndpointMetrics

	clientsMu sync.RWMutex
	clients   map[ProviderID]ProviderClient
}

// New 创建 Scheduler(不启动后台 goroutine)。
func New(cfg Config, resolver ModelResolver, obs *Observability) *Scheduler {
	if cfg.ColdStartSamples == 0 {
		cfg.ColdStartSamples = 20
	}
	if cfg.EMADecay == 0 {
		cfg.EMADecay = 0.3
	}
	if cfg.BreakerTickInterval == 0 {
		cfg.BreakerTickInterval = 5 * time.Second
	}
	if cfg.BackoffJitterMin == 0 {
		cfg.BackoffJitterMin = 0.8
	}
	if cfg.BackoffJitterMax == 0 {
		cfg.BackoffJitterMax = 1.2
	}

	if obs == nil {
		obs = NoopObservability()
	}
	logger := cfg.Log
	if logger == nil {
		logger = noopLogger{}
	}

	s := &Scheduler{
		cfg:        cfg,
		resolver:   resolver,
		routeCache: NewModelRouteCache(cfg.ModelRouteCacheTTL, logger),
		sems:       NewSemaphoreSet(cfg.SoftConcurrencyRatio),
		breakers:   NewBreakerSet(cfg.BreakerConfig, logger),
		selector:   NewSelector(),
		obs:        obs,
		log:        logger,
		affinity:   NewSessionAffinity(0), // 默认 TTL 5 分钟
		metrics:    make(map[EndpointKey]*EndpointMetrics),
		clients:    make(map[ProviderID]ProviderClient),
	}
	s.selector.SetLog(logger)
	return s
}

// RegisterClients 批量注册/更新 provider clients（不清除旧项，用于启动时）。
func (s *Scheduler) RegisterClients(providers map[ProviderID]ProviderClient) {
	s.clientsMu.Lock()
	defer s.clientsMu.Unlock()
	for p, c := range providers {
		s.clients[p] = c
	}
	s.log.Info("providers registered in batch", "count", len(providers))
}

// Start 启动后台熔断评估。
func (s *Scheduler) Start(ctx context.Context) {
	s.log.Info("scheduler started", "breaker_tick_interval", s.cfg.BreakerTickInterval)
	go s.breakers.RunTicker(ctx, s, s.cfg.BreakerTickInterval)
}

// GetOrCreate 实现 EndpointMetricsSource，供 breaker ticker 使用。
func (s *Scheduler) GetOrCreate(k EndpointKey) *EndpointMetrics {
	return s.getMetrics(k)
}

func (s *Scheduler) getMetrics(k EndpointKey) *EndpointMetrics {
	s.metricsMu.RLock()
	m, ok := s.metrics[k]
	s.metricsMu.RUnlock()
	if ok {
		return m
	}

	s.metricsMu.Lock()
	defer s.metricsMu.Unlock()
	if m, ok := s.metrics[k]; ok {
		return m
	}
	m = &EndpointMetrics{}
	s.metrics[k] = m
	return m
}

func (s *Scheduler) attachMetricsToRoute(route *ModelRoute) {
	if route == nil {
		return
	}
	for _, ep := range route.Endpoints {
		if ep == nil {
			continue
		}
		if ep.metrics == nil {
			ep.metrics = s.getMetrics(EndpointKey{Provider: ep.Provider, KeyID: ep.KeyID})
		}
	}
}

// Schedule 同步非流式入口。
func (s *Scheduler) Schedule(ctx context.Context, inv Invoke) (*Result, error) {
	return s.schedule(ctx, inv, false)
}

// ScheduleStream 流式入口。返回的 Result.Stream 在 err==nil 时非 nil。
func (s *Scheduler) ScheduleStream(ctx context.Context, inv Invoke) (*Result, error) {
	return s.schedule(ctx, inv, true)
}

func (s *Scheduler) resolveRoute(ctx context.Context, model ModelKey, provider ProviderID) (*ModelRoute, error) {
	if s.routeCache != nil {
		if r := s.routeCache.Get(model, provider); r != nil {
			s.attachMetricsToRoute(r)
			return r, nil
		}
	}
	r, err := s.resolver.Resolve(ctx, model, provider)
	if err != nil {
		return nil, err
	}
	s.attachMetricsToRoute(r)
	if s.routeCache != nil {
		s.routeCache.Set(model, provider, r)
	}
	return r, nil
}

func (s *Scheduler) schedule(ctx context.Context, inv Invoke, stream bool) (*Result, error) {
	main, err := s.resolveRoute(ctx, inv.Model, inv.Provider)
	if err != nil {
		fmt.Println("[schedule] RESOLVE FAILED", "model", inv.Model, "err", err)
		if errors.Is(err, ErrModelNotFound) {
			s.log.Warn("model not found", "model", inv.Model, "error", err)
			return nil, fmt.Errorf("%w: %s", ErrModelNotFound, inv.Model)
		}
		s.log.Warn("resolve model failed", "model", inv.Model, "error", err)
		return nil, err
	}
	s.attachMetricsToRoute(main)

	s.log.Debug("schedule begin",
		"model", inv.Model,
		"provider", inv.Provider,
		"stream", stream,
		"endpoint_count", len(main.Endpoints),
		"max_concurrency", main.MaxConcurrency,
	)
	if len(main.Endpoints) == 0 {
		s.log.Warn("resolved model has no endpoints", "model", inv.Model, "provider", inv.Provider)
		return nil, fmt.Errorf("%w: %s", ErrNoHealthyEndpoint, inv.Model)
	}

	// per-model 并发信号量
	sem := s.sems.GetOrCreate(inv.Model, main.MaxConcurrency)
	release, err := sem.Acquire(ctx)
	if err != nil {
		s.log.Warn("semaphore acquire failed", "model", inv.Model, "error", err)
		return nil, err
	}
	defer release()

	// 模型链:主 + 回退
	models := append([]ModelKey{inv.Model}, main.FallbackModels...)
	var lastErr error
	for fbIdx, m := range models {
		if fbIdx > 0 {
			s.log.Info("fallback triggered", "from_model", inv.Model, "to_model", m, "fallback_index", fbIdx)
			s.obs.RecordFallback(inv.Model, fbIdx)
		}
		r, err := s.resolveRoute(ctx, m, inv.Provider)
		if err != nil {
			s.log.Warn("fallback model not found", "model", m, "error", err)
			continue
		}
		res, err := s.tryOneModel(ctx, m, r, inv, stream, fbIdx > 0)
		if err == nil {
			s.log.Info("schedule success",
				"model", inv.Model,
				"selected_provider", res.Meta.Provider,
				"selected_key", res.Meta.KeyID,
				"attempts", res.Meta.Attempts,
				"score", res.Meta.Score,
				"is_fallback", fbIdx > 0,
				"stream", stream,
			)
			return res, nil
		}
		lastErr = err
		s.log.Warn("model failed, trying next",
			"model", m,
			"error", err,
			"is_fallback", fbIdx > 0,
		)
		// 不再短路 ErrAllEndpointsDead，让 fallback model 有机会接管
	}
	if lastErr == nil {
		lastErr = ErrNoHealthyEndpoint
	}
	s.log.Error("schedule failed", "model", inv.Model, "error", lastErr, "stream", stream)
	return nil, lastErr
}

// tryOneModel 在单个 model 内做"选端点 + 重试"。
func (s *Scheduler) tryOneModel(
	ctx context.Context,
	model ModelKey,
	route *ModelRoute,
	inv Invoke,
	stream bool,
	isFallback bool,
) (*Result, error) {
	var deadKeys, usedKeys, failKeys map[string]bool
	lastWasRateLimit := false
	var lastUpstreamErr error // 最近一次端点调用的真实错误,用于耗尽时回传给调用方
	lastClass := ClassUnknown // 最近一次失败时的错误分类

	s.log.Debug("tryOneModel begin",
		"model", model,
		"endpoint_count", len(route.Endpoints),
		"max_retries", s.cfg.MaxRetries,
		"stream", stream,
	)

	for attempt := 0; attempt <= s.cfg.MaxRetries; attempt++ {
		var ep *Endpoint
		var noEpReason string // Select 返回 nil 时的简明原因,用于日志和耗尽错误透传

		// Session affinity: 同一 session 优先复用上次端点
		if inv.SessionID != "" && attempt == 0 {
			if cached := s.affinity.Get(inv.SessionID); cached != nil {
				// 检查该端点是否仍在当前 route 中且健康
				for _, candidate := range route.Endpoints {
					if candidate.KeyID == cached.KeyID && candidate.Provider == cached.Provider {
						if candidate.Enabled && modelAllowed(candidate.Models, route.Model) {
							if candidate.Concurrency == 0 || candidate.InFlight() < int64(candidate.Concurrency) {
								score := computeScore(candidate.Weight, candidate.metrics, s.cfg.ColdStartSamples)
								if score > 0 {
									br := s.breakers.GetOrCreate(EndpointKey{Provider: candidate.Provider, KeyID: candidate.KeyID})
									if br.Allow(candidate.metrics) {
										ep = candidate
										s.log.Debug("session affinity hit", "session", inv.SessionID, "model", model, "provider", ep.Provider, "key_id", ep.KeyID)
									}
								}
							}
						}
						break
					}
				}
				if ep == nil {
					s.affinity.Delete(inv.SessionID)
					s.log.Debug("session affinity stale, cleared", "session", inv.SessionID)
				} else if deadKeys[ep.KeyID] || usedKeys[ep.KeyID] {
					// 缓存的端点本轮已被排除(401/429),直接清缓存走 Select,避免一次注定失败的请求
					s.affinity.Delete(inv.SessionID)
					s.log.Debug("session affinity key dead/used, cleared", "session", inv.SessionID, "key_id", ep.KeyID)
					ep = nil
				}
			}
		}

		if ep == nil {
			// 排除已 dead(401)和本轮已被限流(429)的 key,这样所有候选都被排除时
			// Select 返回 nil,触发下面的「第二次机会」清空逻辑,而不是空转重试已用 key。
			ep, noEpReason = s.selector.Select(ctx, route, s.cfg.ColdStartSamples, skipSet(deadKeys, usedKeys, failKeys))
		}
		if ep == nil {
			// 池子空了:若是 429 周期耗尽,清掉 usedKeys 给滑过窗口的 key 第二次机会
			if lastWasRateLimit && len(usedKeys) > 0 {
				s.log.Debug("clearing usedKeys for rate-limit retry", "model", model)
				usedKeys = nil
				ep, noEpReason = s.selector.Select(ctx, route, s.cfg.ColdStartSamples, skipSet(deadKeys, usedKeys, failKeys))
			}
			if ep == nil {
				s.log.Warn("no available endpoint", "model", model, "attempt", attempt, "reason", noEpReason)
				// 让最终耗尽错误带上原因，避免上层只看到 "retries exhausted" 无从排查。
				lastUpstreamErr = fmt.Errorf("no available endpoint for model %s at attempt %d: %s", model, attempt, noEpReason)
				break
			}
		}
		if deadKeys[ep.KeyID] || usedKeys[ep.KeyID] || failKeys[ep.KeyID] {
			s.log.Debug("skipping dead/used/failed key", "model", model, "key_id", ep.KeyID, "attempt", attempt)
			continue
		}

		breaker := s.breakers.GetOrCreate(EndpointKey{Provider: ep.Provider, KeyID: ep.KeyID})
		if !breaker.Allow(ep.metrics) {
			s.log.Debug("breaker blocked", "model", model, "provider", ep.Provider, "key_id", ep.KeyID, "state", breaker.State())
			continue
		}

		ctx, span := s.obs.StartEndpointCall(ctx, ep, model, attempt)
		ep.metrics.InFlight.Add(1)
		ep.metrics.TotalRequests.Add(1)
		start := time.Now()

		clientProto := ProviderID(ClientTypeName(ep.NativeProtocol()))

		s.log.Debug("endpoint selected",
			"model", model,
			"provider", ep.Provider,
			"key_id", ep.KeyID,
			"protocol", clientProto,
			"score", ep.EffectiveScore(s.cfg.ColdStartSamples),
			"attempt", attempt,
			"inflight", ep.InFlight(),
		)

		client := s.getClient(clientProto)
		if client == nil {
			ep.metrics.InFlight.Add(-1)
			span.End()
			s.log.Error("no client for protocol", "model", model, "protocol", clientProto, "provider", ep.Provider)
			return nil, fmt.Errorf("no client for protocol %s", clientProto)
		}

		s.log.Debug("calling provider client",
			"model", model,
			"provider", ep.Provider,
			"key_id", ep.KeyID,
			"protocol", clientProto,
			"provider_type", ep.NativeProtocol(),
			"model_name", ep.ModelName,
			"stream", stream,
			"attempt", attempt,
		)

		// route 请求需要把 body 里的 model 字段改写成上游真实模型名。
		body := inv.Body
		if ep.ModelName != "" && string(model) != ep.ModelName {
			if rewritten, err := rewriteBodyModel(inv.Body, ep.ModelName); err == nil {
				body = rewritten
			}
		}

		var (
			outBody []byte
			strm    Stream
			err     error
		)
		streamSucceeded := false
		if stream {
			strm, err = client.Stream(ctx, ep, body)
			if err == nil && strm != nil {
				streamSucceeded = true
				strm = newInstrumentedStream(strm, ep, model, s.obs)
			}
		} else {
			outBody, err = client.Do(ctx, ep, body)
		}

		latency := time.Since(start)
		ep.metrics.InFlight.Add(-1)

		// 成功路径
		if err == nil {
			ep.metrics.SuccessCount.Add(1)
			ep.metrics.LastSuccessAtUnix.Store(time.Now().Unix())
			updateEMA(ep.metrics, latency.Microseconds(), s.cfg.EMADecay)
			breaker.RecordSuccess(ep.metrics, s.log)
			s.obs.RecordRequest(ep, model, ClassUnknown, latency, isFallback)
			span.End()
			s.log.Info("endpoint call success",
				"model", model,
				"provider", ep.Provider,
				"key_id", ep.KeyID,
				"latency", latency,
				"attempt", attempt,
				"score", ep.EffectiveScore(s.cfg.ColdStartSamples),
			)
			if streamSucceeded {
				if inv.SessionID != "" {
					s.affinity.Set(inv.SessionID, EndpointKey{Provider: ep.Provider, KeyID: ep.KeyID})
					s.log.Debug("session affinity recorded", "session", inv.SessionID, "provider", ep.Provider, "key_id", ep.KeyID)
				}
				return &Result{Stream: strm, Meta: s.makeMeta(ep, attempt)}, nil
			}
			if inv.SessionID != "" {
				s.affinity.Set(inv.SessionID, EndpointKey{Provider: ep.Provider, KeyID: ep.KeyID})
				s.log.Debug("session affinity recorded", "session", inv.SessionID, "provider", ep.Provider, "key_id", ep.KeyID)
			}
			return &Result{Body: outBody, Meta: s.makeMeta(ep, attempt)}, nil
		}

		// 失败路径
		ep.metrics.LastErrorAtUnix.Store(time.Now().Unix())
		class := Classify(err)
		lastClass = class
		lastUpstreamErr = err
		s.recordFailureClass(ep.metrics, class)
		updateEMA(ep.metrics, latency.Microseconds(), s.cfg.EMADecay)

		span.RecordError(err)
		span.SetAttributes(attribute.String(AttrSchedErrorClass, class.String()))
		span.End()

		var status int
		if ue, ok := err.(*UpstreamError); ok {
			status = ue.Status
		}
		s.log.Warn("endpoint call failed",
			"model", model,
			"provider", ep.Provider,
			"key_id", ep.KeyID,
			"error_class", class,
			"status", status,
			"error", err,
			"latency", latency,
			"attempt", attempt,
		)

		if attempt < s.cfg.MaxRetries {
			s.obs.RecordRetry(ep.Provider)
		}

		switch class {
		case ClassClient, ClassCancel:
			s.log.Info("client/cancel error, aborting", "model", model, "error_class", class)
			return nil, &SchedError{Class: class, Status: statusFromErr(err), Message: "client error", Cause: err}

		case ClassAuth:
			breaker.RecordFailure(ep.metrics, s.log)
			if deadKeys == nil {
				deadKeys = map[string]bool{}
			}
			deadKeys[ep.KeyID] = true
			s.log.Info("auth error, marking key dead",
				"model", model,
				"provider", ep.Provider,
				"key_id", ep.KeyID,
				"dead_count", len(deadKeys),
			)
			continue // 不退避

		case ClassRateLimit:
			breaker.RecordFailure(ep.metrics, s.log)
			lastWasRateLimit = true
			if usedKeys == nil {
				usedKeys = map[string]bool{}
			}
			usedKeys[ep.KeyID] = true
			s.log.Info("rate limit, backing off",
				"model", model,
				"provider", ep.Provider,
				"key_id", ep.KeyID,
				"used_count", len(usedKeys),
			)
			s.backoff(ctx, attempt)
			continue

		case ClassTransient:
			breaker.RecordFailure(ep.metrics, s.log)
			if failKeys == nil {
				failKeys = map[string]bool{}
			}
			failKeys[ep.KeyID] = true
			// 清除 session affinity，避免下一次请求又绑到这个故障端点
			if inv.SessionID != "" {
				s.affinity.Delete(inv.SessionID)
			}
			s.log.Info("transient error, retrying after backoff",
				"model", model,
				"provider", ep.Provider,
				"key_id", ep.KeyID,
			)
			s.backoff(ctx, attempt)
			continue

		default:
			s.log.Error("unknown error, aborting", "model", model, "error", err)
			return nil, &SchedError{Class: class, Status: statusFromErr(err), Message: "unknown error", Cause: err}
		}
	}

	// 耗尽
	allDead := len(route.Endpoints) > 0
	for _, ep := range route.Endpoints {
		if !deadKeys[ep.KeyID] {
			allDead = false
			break
		}
	}
	if allDead {
		s.log.Error("all endpoints dead", "model", model, "dead_count", len(deadKeys))
		return nil, ErrAllEndpointsDead
	}
	s.log.Error("retries exhausted", "model", model, "attempts", s.cfg.MaxRetries+1, "dead_count", len(deadKeys), "last_error", lastUpstreamErr)
	// 把最近一次的真实上游错误回传,避免 502 只剩 "retries exhausted" 无从排查。
	if lastUpstreamErr != nil {
		// 若上游错误带 HTTP status,透传它(含 status + message),让 mapScheduleError 给出正确状态码。
		var ue *UpstreamError
		if errors.As(lastUpstreamErr, &ue) {
			return nil, &SchedError{
				Class:   lastClass, // 复用循环里上一次失败时已经算好的 class,避免重复分类
				Status:  ue.Status,
				Message: fmt.Sprintf("retries exhausted for model %s: %s", model, ue.Message),
				Cause:   lastUpstreamErr,
			}
		}
		return nil, fmt.Errorf("retries exhausted for model %s: %w", model, lastUpstreamErr)
	}
	return nil, fmt.Errorf("retries exhausted for model %s", model)
}

// skipSet 合并 dead/used 两个 key 集合给 selector.Select 排除。
// 都为空时返回 nil(Select 视为不排除)。
func skipSet(deadKeys, usedKeys, failKeys map[string]bool) map[string]bool {
	if len(deadKeys) == 0 && len(usedKeys) == 0 && len(failKeys) == 0 {
		return nil
	}
	skip := make(map[string]bool, len(deadKeys)+len(usedKeys)+len(failKeys))
	for k := range deadKeys {
		skip[k] = true
	}
	for k := range usedKeys {
		skip[k] = true
	}
	for k := range failKeys {
		skip[k] = true
	}
	return skip
}

// makeMeta 构造 CallMeta。
func (s *Scheduler) makeMeta(ep *Endpoint, attempts int) CallMeta {
	return CallMeta{
		Provider: ep.Provider,
		KeyID:    ep.KeyID,
		Attempts: attempts + 1,
		Score:    ep.EffectiveScore(s.cfg.ColdStartSamples),
		Endpoint: ep,
	}
}

// backoff 指数退避 + jitter,响应 ctx 取消。
func (s *Scheduler) backoff(ctx context.Context, attempt int) {
	d := s.cfg.BackoffInitial << attempt
	if d > s.cfg.BackoffMax {
		d = s.cfg.BackoffMax
	}
	jitter := s.cfg.BackoffJitterMin + rand.Float64()*(s.cfg.BackoffJitterMax-s.cfg.BackoffJitterMin)
	sleep := time.Duration(float64(d) * jitter)
	s.log.Debug("backoff", "attempt", attempt, "duration", sleep, "jitter", jitter)
	t := time.NewTimer(sleep)
	defer t.Stop()
	select {
	case <-t.C:
	case <-ctx.Done():
		s.log.Debug("backoff interrupted by context cancellation")
	}
}

// getClient 取出 provider client(线程安全)。
func (s *Scheduler) getClient(p ProviderID) ProviderClient {
	s.clientsMu.RLock()
	defer s.clientsMu.RUnlock()
	return s.clients[p]
}

// recordFailureClass 把错误分类记入 metrics。
func (s *Scheduler) recordFailureClass(m *EndpointMetrics, class ErrorClass) {
	switch class {
	case ClassRateLimit:
		m.ErrorCount429.Add(1)
	case ClassTransient:
		m.ErrorCount5xx.Add(1)
	case ClassAuth:
		m.ErrorCountAuth.Add(1)
	case ClassClient:
		m.ErrorCountClient.Add(1)
	}
}

// statusFromErr 提取 HTTP 状态码。
func statusFromErr(err error) int {
	var ue *UpstreamError
	if errors.As(err, &ue) {
		return ue.Status
	}
	return 0
}
