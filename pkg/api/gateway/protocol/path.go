package protocol

import (
	"errors"
	"strings"
)

var errUnsupportedConversion = errors.New("unsupported endpoint for protocol conversion")

// CanonicalAnthropicPath 把入口路由 /anthropic/v1/* 归一化为标准 /v1/* Anthropic 路径。
func CanonicalAnthropicPath(path string) string {
	if strings.HasPrefix(path, "/anthropic/v1/") {
		return "/v1/" + strings.TrimPrefix(path, "/anthropic/v1/")
	}
	return path
}

// ResolveUpstreamPath 根据入口协议、provider 原生协议和请求路径，决定转发到上游的路径。
func ResolveUpstreamPath(inboundProto, providerProto Protocol, requestPath string) (string, error) {
	if inboundProto == providerProto {
		return requestPath, nil
	}
	if inboundProto == ProtocolAnthropic && providerProto == ProtocolOpenAI {
		if requestPath != "/v1/messages" {
			return "", errUnsupportedConversion
		}
		return "/v1/chat/completions", nil
	}
	if inboundProto == ProtocolOpenAI && providerProto == ProtocolAnthropic {
		if requestPath != "/v1/chat/completions" {
			return "", errUnsupportedConversion
		}
		return "/v1/messages", nil
	}
	if inboundProto == ProtocolOpenAI && providerProto == ProtocolGemini {
		if requestPath != "/v1/chat/completions" {
			return "", errUnsupportedConversion
		}
		return requestPath, nil
	}
	if inboundProto == ProtocolAnthropic && providerProto == ProtocolGemini {
		if requestPath != "/v1/messages" {
			return "", errUnsupportedConversion
		}
		return "/v1/chat/completions", nil
	}
	return "", errUnsupportedConversion
}
