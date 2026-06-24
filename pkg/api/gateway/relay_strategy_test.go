package gateway

import (
	"testing"

	"github.com/computeflux/tee-provider/pkg/api/gateway/protocol"
	cgateway "github.com/computeflux/tee-provider/tee-chain/contracts/module/gateway"
)

func TestProviderNativeProtocol(t *testing.T) {
	tests := []struct {
		providerType uint16
		want         protocol.Protocol
	}{
		{cgateway.ProviderOpenAI, protocol.ProtocolOpenAI},
		{cgateway.ProviderClaude, protocol.ProtocolAnthropic},
		{cgateway.ProviderGemini, protocol.ProtocolGemini},
		{cgateway.ProviderPersonal, protocol.ProtocolOpenAI},
	}
	for _, tt := range tests {
		if got := providerNativeProtocol(tt.providerType); got != tt.want {
			t.Errorf("providerNativeProtocol(%d) = %v, want %v", tt.providerType, got, tt.want)
		}
	}
}

func TestCanonicalAnthropicPath(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{"/anthropic/v1/messages", "/v1/messages"},
		{"/anthropic/v1/messages/count_tokens", "/v1/messages/count_tokens"},
		{"/anthropic/v1/complete", "/v1/complete"},
		{"/v1/messages", "/v1/messages"},
		{"/unknown", "/unknown"},
	}
	for _, tt := range tests {
		if got := canonicalAnthropicPath(tt.in); got != tt.want {
			t.Errorf("canonicalAnthropicPath(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestMapAnthropicPathToOpenAI(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{"/v1/messages", "/v1/chat/completions"},
		{"/v1/complete", "/v1/complete"},
		{"/v1/unknown", "/v1/unknown"},
	}
	for _, tt := range tests {
		if got := mapAnthropicPathToOpenAI(tt.in); got != tt.want {
			t.Errorf("mapAnthropicPathToOpenAI(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestMapOpenAIPathToAnthropic(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{"/v1/chat/completions", "/v1/messages"},
		{"/v1/completions", "/v1/completions"},
		{"/v1/unknown", "/v1/unknown"},
	}
	for _, tt := range tests {
		if got := mapOpenAIPathToAnthropic(tt.in); got != tt.want {
			t.Errorf("mapOpenAIPathToAnthropic(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestResolveUpstreamPath(t *testing.T) {
	tests := []struct {
		name     string
		inbound  protocol.Protocol
		provider protocol.Protocol
		path     string
		wantPath string
		wantErr  bool
	}{
		{"matching openai", protocol.ProtocolOpenAI, protocol.ProtocolOpenAI, "/v1/chat/completions", "/v1/chat/completions", false},
		{"matching anthropic", protocol.ProtocolAnthropic, protocol.ProtocolAnthropic, "/v1/messages", "/v1/messages", false},
		{"anthropic to openai", protocol.ProtocolAnthropic, protocol.ProtocolOpenAI, "/v1/messages", "/v1/chat/completions", false},
		{"openai to anthropic", protocol.ProtocolOpenAI, protocol.ProtocolAnthropic, "/v1/chat/completions", "/v1/messages", false},
		{"unsupported anthropic endpoint", protocol.ProtocolAnthropic, protocol.ProtocolOpenAI, "/v1/complete", "", true},
		{"unsupported openai endpoint", protocol.ProtocolOpenAI, protocol.ProtocolAnthropic, "/v1/embeddings", "", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := resolveUpstreamPath(tt.inbound, tt.provider, tt.path)
			if (err != nil) != tt.wantErr {
				t.Fatalf("resolveUpstreamPath() error = %v, wantErr %v", err, tt.wantErr)
			}
			if got != tt.wantPath {
				t.Errorf("resolveUpstreamPath() = %q, want %q", got, tt.wantPath)
			}
		})
	}
}
