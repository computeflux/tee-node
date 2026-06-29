package scheduler

import (
	"context"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
)

// 标准的 OTel semantic conventions + 我们的自定义属性。
const (
	// 复用 OTel GenAI 语义约定
	AttrGenAIProvider      = "gen_ai.provider.name"
	AttrGenAIRequestModel  = "gen_ai.request.model"
	AttrGenAIResponseModel = "gen_ai.response.model"
	AttrGenAIOperation     = "gen_ai.operation.name"

	// 自定义命名空间
	AttrSchedEndpointID  = "sched.endpoint.id"
	AttrSchedEndpointKey = "sched.endpoint.key_id"
	AttrSchedAttempt     = "sched.attempt"
	AttrSchedFallback    = "sched.is_fallback"
	AttrSchedScore       = "sched.score"
	AttrSchedErrorClass  = "sched.error.class"
	AttrSchedInFlight    = "sched.in_flight"

	// Span 命名
	SpanKeySelection = "sched.key.selection"
	SpanEndpointCall = "sched.endpoint.call"
)

// Observability 集中管理 tracer + meter。
// 依赖全局 OTel TracerProvider / MeterProvider。
type Observability struct {
	tracer trace.Tracer
	meter  metric.Meter

	// Counters
	requestTotal   metric.Int64Counter
	successTotal   metric.Int64Counter
	errorTotal     metric.Int64Counter
	retryTotal     metric.Int64Counter
	fallbackTotal  metric.Int64Counter
	circuitOpenCnt metric.Int64Counter

	// Histograms
	latencyMs metric.Float64Histogram
	ttftMs    metric.Float64Histogram
}

// NewObservability 从全局 OTel provider 创建 Observability。
// serviceName 是 instrumentation scope 的名字。
func NewObservability(serviceName string) (*Observability, error) {
	if serviceName == "" {
		serviceName = "scheduler"
	}
	tp := otel.GetTracerProvider()
	mp := otel.GetMeterProvider()
	obs := &Observability{
		tracer: tp.Tracer(serviceName),
		meter:  mp.Meter(serviceName),
	}
	var err error
	obs.requestTotal, err = obs.meter.Int64Counter("sched.requests.total",
		metric.WithDescription("Total requests dispatched by scheduler"))
	if err != nil {
		return nil, err
	}
	obs.successTotal, err = obs.meter.Int64Counter("sched.requests.success")
	if err != nil {
		return nil, err
	}
	obs.errorTotal, err = obs.meter.Int64Counter("sched.requests.error",
		metric.WithDescription("Failed requests partitioned by error class"))
	if err != nil {
		return nil, err
	}
	obs.retryTotal, err = obs.meter.Int64Counter("sched.retries.total")
	if err != nil {
		return nil, err
	}
	obs.fallbackTotal, err = obs.meter.Int64Counter("sched.fallbacks.total")
	if err != nil {
		return nil, err
	}
	obs.circuitOpenCnt, err = obs.meter.Int64Counter("sched.circuit.opens.total")
	if err != nil {
		return nil, err
	}
	obs.latencyMs, err = obs.meter.Float64Histogram("sched.request.duration",
		metric.WithUnit("ms"),
		metric.WithDescription("End-to-end request duration"))
	if err != nil {
		return nil, err
	}
	obs.ttftMs, err = obs.meter.Float64Histogram("sched.stream.ttft",
		metric.WithUnit("ms"),
		metric.WithDescription("Time to first streaming chunk"))
	if err != nil {
		return nil, err
	}
	return obs, nil
}

// NoopObservability 返回一个不依赖 OTel 的实现(用于测试或禁用 tracing)。
func NoopObservability() *Observability {
	obs, _ := NewObservability("noop")
	return obs
}

// --- Span helpers ---

// StartKeySelection 开始一个"key selection"span(可选,通常不需要)。
func (o *Observability) StartKeySelection(ctx context.Context, model ModelKey) (context.Context, trace.Span) {
	ctx, span := o.tracer.Start(ctx, SpanKeySelection, trace.WithSpanKind(trace.SpanKindInternal))
	span.SetAttributes(attribute.String(AttrGenAIRequestModel, string(model)))
	return ctx, span
}

// StartEndpointCall 开始一个"endpoint call"span。
func (o *Observability) StartEndpointCall(ctx context.Context, ep *Endpoint, model ModelKey, attempt int) (context.Context, trace.Span) {
	ctx, span := o.tracer.Start(ctx, SpanEndpointCall, trace.WithSpanKind(trace.SpanKindClient))
	span.SetAttributes(
		attribute.String(AttrGenAIProvider, string(ep.Provider)),
		attribute.String(AttrGenAIRequestModel, string(model)),
		attribute.String(AttrSchedEndpointKey, ep.KeyID),
		attribute.Int(AttrSchedAttempt, attempt),
	)
	return ctx, span
}

// EndSpanError 在 span 上记录错误。
func (o *Observability) EndSpanError(span trace.Span, err error, class ErrorClass) {
	if err == nil {
		return
	}
	span.RecordError(err)
	span.SetStatus(codes.Error, err.Error())
	span.SetAttributes(attribute.String(AttrSchedErrorClass, class.String()))
}

// --- Metric helpers ---

// RecordRequest 记录一次请求的结果。
func (o *Observability) RecordRequest(ep *Endpoint, model ModelKey, class ErrorClass, latency time.Duration, isFallback bool) {
	attrs := metric.WithAttributes(
		attribute.String(AttrGenAIProvider, string(ep.Provider)),
		attribute.String(AttrGenAIRequestModel, string(model)),
		attribute.String(AttrSchedEndpointKey, ep.KeyID),
		attribute.String(AttrSchedErrorClass, class.String()),
		attribute.Bool(AttrSchedFallback, isFallback),
	)
	o.requestTotal.Add(context.Background(), 1, attrs)
	if class == ClassUnknown {
		o.successTotal.Add(context.Background(), 1, attrs)
	} else {
		o.errorTotal.Add(context.Background(), 1, attrs)
	}
	o.latencyMs.Record(context.Background(), float64(latency.Milliseconds()), attrs)
}

// RecordRetry 记录一次重试。
func (o *Observability) RecordRetry(provider ProviderID) {
	o.retryTotal.Add(context.Background(), 1, metric.WithAttributes(
		attribute.String(AttrGenAIProvider, string(provider)),
	))
}

// RecordFallback 记录一次跨模型回退。
func (o *Observability) RecordFallback(model ModelKey, attempt int) {
	o.fallbackTotal.Add(context.Background(), 1, metric.WithAttributes(
		attribute.String(AttrGenAIRequestModel, string(model)),
		attribute.Int(AttrSchedAttempt, attempt),
	))
}

// RecordTTFT 记录流式首字节延迟。
func (o *Observability) RecordTTFT(provider ProviderID, model ModelKey, dur time.Duration) {
	o.ttftMs.Record(context.Background(), float64(dur.Milliseconds()), metric.WithAttributes(
		attribute.String(AttrGenAIProvider, string(provider)),
		attribute.String(AttrGenAIRequestModel, string(model)),
	))
}

// RecordCircuitOpen 记录一次熔断触发。
func (o *Observability) RecordCircuitOpen(provider ProviderID, keyID string) {
	o.circuitOpenCnt.Add(context.Background(), 1, metric.WithAttributes(
		attribute.String(AttrGenAIProvider, string(provider)),
		attribute.String(AttrSchedEndpointKey, keyID),
	))
}
