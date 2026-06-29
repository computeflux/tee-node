package scheduler

import (
	"context"
	"errors"
	"fmt"
	"net/http"
)

// ErrorClass 把上游错误分类,决定调度策略。
type ErrorClass int

const (
	// ClassUnknown 兜底,走"不重试"分支。
	ClassUnknown ErrorClass = iota
	// ClassTransient 临时性错误(5xx、网络、DNS),同端点退避后重试。
	ClassTransient
	// ClassRateLimit 限速(429 或 message 含 "rate limit"),换端点 + 退避。
	ClassRateLimit
	// ClassAuth 凭证失效(401/403/402),换端点且本次请求永不回访。
	ClassAuth
	// ClassClient 请求本身错(400/404/422),不重试、不换端点。
	ClassClient
	// ClassCancel 客户端取消。
	ClassCancel
)

func (c ErrorClass) String() string {
	switch c {
	case ClassTransient:
		return "transient"
	case ClassRateLimit:
		return "rate_limit"
	case ClassAuth:
		return "auth"
	case ClassClient:
		return "client"
	case ClassCancel:
		return "cancel"
	default:
		return "unknown"
	}
}

// SchedError 是 Scheduler 暴露给调用方的错误类型,带错误分类。
type SchedError struct {
	Class   ErrorClass
	Status  int // HTTP 状态码(若适用)
	Message string
	Cause   error
}

func (e *SchedError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("%s: %s (status=%d): %v", e.Class, e.Message, e.Status, e.Cause)
	}
	return fmt.Sprintf("%s: %s (status=%d)", e.Class, e.Message, e.Status)
}

func (e *SchedError) Unwrap() error { return e.Cause }

// Is 实现 errors.Is 支持。
func (e *SchedError) Is(target error) bool {
	var t *SchedError
	if errors.As(target, &t) {
		return t.Class == e.Class
	}
	return false
}

// 预定义 sentinel error(方便 errors.Is 判断)。
var (
	// ErrNoHealthyEndpoint 没有可用端点(全部熔断 / 全部打满)。
	ErrNoHealthyEndpoint = &SchedError{Class: ClassUnknown, Message: "no healthy endpoint available"}
	// ErrAllEndpointsDead 所有端点的凭证都死了(401/402/403)。
	ErrAllEndpointsDead = &SchedError{Class: ClassAuth, Message: "all endpoints returned permanent auth/billing errors"}
	// ErrModelNotFound 模型未配置。
	ErrModelNotFound = &SchedError{Class: ClassClient, Message: "model not configured"}
	// ErrCanceled 请求被取消。
	ErrCanceled = &SchedError{Class: ClassCancel, Message: "request cancelled"}
)

// UpstreamError 是 provider 调用层(实现者)需要返回的错误类型,
// 让 Scheduler 能分类调度。
type UpstreamError struct {
	Status  int
	Code    string // 供应商特定错误码
	Message string
}

func (e *UpstreamError) Error() string {
	return fmt.Sprintf("upstream error: status=%d code=%s message=%s", e.Status, e.Code, e.Message)
}

// Classify 把上游错误分类。
func Classify(err error) ErrorClass {
	if err == nil {
		return ClassUnknown
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return ClassCancel
	}

	// 尝试拆出 UpstreamError
	var ue *UpstreamError
	if errors.As(err, &ue) {
		switch {
		case ue.Status == http.StatusTooManyRequests:
			return ClassRateLimit
		case ue.Status == http.StatusUnauthorized,
			ue.Status == http.StatusForbidden,
			ue.Status == http.StatusPaymentRequired:
			return ClassAuth
		case ue.Status >= 500 && ue.Status < 600:
			return ClassTransient
		case ue.Status >= 400 && ue.Status < 500:
			return ClassClient
		}

		// 一些供应商把 429 写到 message 里
		if isRateLimitText(ue.Message) || isRateLimitText(ue.Code) {
			return ClassRateLimit
		}
	}

	// 网络层错误 → transient
	msg := err.Error()
	if isNetworkError(msg) {
		return ClassTransient
	}
	return ClassUnknown
}

func isRateLimitText(s string) bool {
	low := toLower(s)
	return containsAny(low, "rate limit", "rate_limit", "ratelimit", "quota exceeded", "too many requests")
}

func isNetworkError(s string) bool {
	low := toLower(s)
	return containsAny(low, "connection refused", "connection reset", "no such host",
		"timeout", "tls handshake", "eof", "broken pipe", "i/o timeout", "dial tcp")
}

func toLower(s string) string {
	b := []byte(s)
	for i, c := range b {
		if c >= 'A' && c <= 'Z' {
			b[i] = c + 32
		}
	}
	return string(b)
}

func containsAny(s string, subs ...string) bool {
	for _, sub := range subs {
		if indexOf(s, sub) >= 0 {
			return true
		}
	}
	return false
}

// indexOf 极简实现,避免引入 strings 依赖。
func indexOf(s, sub string) int {
	if len(sub) == 0 {
		return 0
	}
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
