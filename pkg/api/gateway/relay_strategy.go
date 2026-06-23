package gateway

import (
	"errors"

	"github.com/computeflux/tee-provider/pkg/api/gateway/protocol"
	cgateway "github.com/computeflux/tee-provider/tee-chain/contracts/module/gateway"
)

var errUnsupportedConversion = errors.New("unsupported endpoint for protocol conversion")

// providerNativeProtocol 根据 tee-chain 的 ProviderType 判断上游 provider 的原生协议。
func providerNativeProtocol(providerType uint16) protocol.Protocol {
	switch providerType {
	case cgateway.ProviderClaude:
		return protocol.ProtocolAnthropic
	case cgateway.ProviderOpenAI, cgateway.ProviderGemini, cgateway.ProviderLocal, cgateway.ProviderPersonal:
		return protocol.ProtocolOpenAI
	default:
		GATEWAYLOG().Printf("[WARN] unknown provider type %d, defaulting to openai protocol", providerType)
		return protocol.ProtocolOpenAI
	}
}

// canonicalAnthropicPath 把入口路由 /anthropic/* 归一化为标准的 /v1/* Anthropic 路径，
// 便于后续做 provider 协议判断和 upstream 路径映射。
func canonicalAnthropicPath(path string) string {
	switch path {
	case "/anthropic/messages":
		return "/v1/messages"
	case "/anthropic/messages/count_tokens":
		return "/v1/messages/count_tokens"
	case "/anthropic/complete":
		return "/v1/complete"
	default:
		GATEWAYLOG().Printf("[WARN] unrecognized anthropic path: %s", path)
	}
	return path
}

// mapAnthropicPathToOpenAI 把标准 Anthropic 端点映射到对等的 OpenAI 端点。
// 当前只支持 /v1/messages 跨协议转换。
func mapAnthropicPathToOpenAI(path string) string {
	switch path {
	case "/v1/messages":
		return "/v1/chat/completions"
	}
	return path
}

// mapOpenAIPathToAnthropic 把标准 OpenAI 端点映射到对等的 Anthropic 端点。
// 当前只支持 /v1/chat/completions 跨协议转换。
func mapOpenAIPathToAnthropic(path string) string {
	switch path {
	case "/v1/chat/completions":
		return "/v1/messages"
	}
	return path
}

// resolveUpstreamPath 根据入口协议、provider 原生协议和请求路径，决定转发到上游的路径。
// 如果跨协议端点不支持，返回 errUnsupportedConversion。
func resolveUpstreamPath(inboundProto, providerProto protocol.Protocol, requestPath string) (string, error) {
	if inboundProto == providerProto {
		return requestPath, nil
	}
	if inboundProto == protocol.ProtocolAnthropic && providerProto == protocol.ProtocolOpenAI {
		if requestPath != "/v1/messages" {
			return "", errUnsupportedConversion
		}
		return mapAnthropicPathToOpenAI(requestPath), nil
	}
	if inboundProto == protocol.ProtocolOpenAI && providerProto == protocol.ProtocolAnthropic {
		if requestPath != "/v1/chat/completions" {
			return "", errUnsupportedConversion
		}
		return mapOpenAIPathToAnthropic(requestPath), nil
	}
	return "", errUnsupportedConversion
}
