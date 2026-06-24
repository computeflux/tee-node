package protocol

import (
	"context"
	"net/http"

	cgateway "github.com/computeflux/tee-provider/tee-chain/contracts/module/gateway"
	"github.com/maximhq/bifrost/core/schemas"
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

// ProviderNativeProtocol 根据 tee-chain 的 ProviderType 返回上游 provider 的原生协议。
func ProviderNativeProtocol(providerType uint16) Protocol {
	switch providerType {
	case cgateway.ProviderClaude:
		return ProtocolAnthropic
	case cgateway.ProviderGemini:
		return ProtocolGemini
	case cgateway.ProviderOpenAI, cgateway.ProviderPersonal:
		return ProtocolOpenAI
	default:
		return ProtocolOpenAI
	}
}

// providerTypeToBifrost 把 tee-chain 的 ProviderType 数值映射为 bifrost 的 ModelProvider。
func providerTypeToBifrost(providerType uint16) schemas.ModelProvider {
	switch providerType {
	case cgateway.ProviderClaude:
		return schemas.Anthropic
	case cgateway.ProviderGemini:
		return schemas.Gemini
	case cgateway.ProviderOpenAI, cgateway.ProviderPersonal:
		// 本地模型按 OpenAI 兼容格式处理
		return schemas.OpenAI
	default:
		return schemas.OpenAI
	}
}
