package protocol

import (
	"context"
	"net/http"

	cgateway "github.com/computeflux/tee-provider/tee-chain/runtime/module/gateway"
	"github.com/maximhq/bifrost/core/schemas"
)

// Protocol 标识客户端使用的入口协议格式。
type Protocol string

const (
	ProtocolOpenAI          Protocol = "openai"
	ProtocolAnthropic       Protocol = "anthropic"
	ProtocolGemini          Protocol = "gemini"
	ProtocolOpenAIResponses Protocol = "openai-responses-api"
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

// OpenAIResponsesMiddleware 设置入口协议为 OpenAI Responses API，供后续中间件和 Handler 识别。
func OpenAIResponsesMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		next.ServeHTTP(w, WithProtocol(r, ProtocolOpenAIResponses))
	})
}

// ProviderNativeProtocol 根据 tee-chain 的 ProviderType 返回上游 provider 的原生协议。
func ProviderNativeProtocol(providerType uint16) Protocol {
	switch providerType {
	case cgateway.Anthropic:
		return ProtocolAnthropic
	case cgateway.Gemini:
		return ProtocolGemini
	case cgateway.OpenAIResponsesApi:
		return ProtocolOpenAIResponses
	case cgateway.OpenAI:
		return ProtocolOpenAI
	default:
		return ProtocolOpenAI
	}
}

// ToType 把入口协议转换为 tee-chain 的协议类型(uint16),用于判断 endpoint 是否原生支持。
// 未知协议按 OpenAI 处理(OpenAI 兼容是默认格式)。
func ToType(p Protocol) uint16 {
	switch p {
	case ProtocolAnthropic:
		return cgateway.Anthropic
	case ProtocolGemini:
		return cgateway.Gemini
	case ProtocolOpenAIResponses:
		return cgateway.OpenAIResponsesApi
	default:
		return cgateway.OpenAI
	}
}

// providerTypeToBifrost 把 tee-chain 的 ProviderType 数值映射为 bifrost 的 ModelProvider。
func providerTypeToBifrost(providerType uint16) schemas.ModelProvider {
	switch providerType {
	case cgateway.Anthropic:
		return schemas.Anthropic
	case cgateway.Gemini:
		return schemas.Gemini
	case cgateway.OpenAI:
		// 本地模型按 OpenAI 兼容格式处理
		return schemas.OpenAI
	default:
		return schemas.OpenAI
	}
}
