package protocol

import (
	"context"
	"net/http"
)

// Protocol 标识客户端使用的入口协议格式。
type Protocol string

const (
	ProtocolOpenAI    Protocol = "openai"
	ProtocolAnthropic Protocol = "anthropic"
	ProtocolGemini    Protocol = "gemini"
)

type protocolKey struct{}

// WithProtocol 把入口协议写入请求上下文。
func WithProtocol(r *http.Request, p Protocol) *http.Request {
	return r.WithContext(context.WithValue(r.Context(), protocolKey{}, p))
}

// FromContext 从请求上下文读取入口协议，缺省为 OpenAI。
func FromContext(r *http.Request) Protocol {
	p, ok := r.Context().Value(protocolKey{}).(Protocol)
	if !ok || p == "" {
		return ProtocolOpenAI
	}
	return p
}

// IsAnthropic 返回该协议是否属于 Anthropic 响应格式。
func IsAnthropic(p Protocol) bool {
	return p == ProtocolAnthropic
}

// AnthropicMiddleware 设置入口协议为 Anthropic，供后续中间件和 Handler 识别。
func AnthropicMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		next.ServeHTTP(w, WithProtocol(r, ProtocolAnthropic))
	})
}
