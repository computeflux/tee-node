package protocol

import "testing"

func TestCanonicalAnthropicPath(t *testing.T) {
	tests := []struct {
		in, want string
	}{
		{"/anthropic/v1/messages", "/v1/messages"},
		{"/anthropic/v1/messages/count_tokens", "/v1/messages/count_tokens"},
		{"/anthropic/v1/complete", "/v1/complete"},
		{"/v1/messages", "/v1/messages"},
		{"/unknown", "/unknown"},
	}
	for _, tt := range tests {
		if got := CanonicalAnthropicPath(tt.in); got != tt.want {
			t.Errorf("CanonicalAnthropicPath(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestResolveUpstreamPath(t *testing.T) {
	tests := []struct {
		name     string
		inbound  Protocol
		provider Protocol
		path     string
		wantPath string
		wantErr  bool
	}{
		{"matching openai", ProtocolOpenAI, ProtocolOpenAI, "/v1/chat/completions", "/v1/chat/completions", false},
		{"matching anthropic", ProtocolAnthropic, ProtocolAnthropic, "/v1/messages", "/v1/messages", false},
		{"anthropic to openai", ProtocolAnthropic, ProtocolOpenAI, "/v1/messages", "/v1/chat/completions", false},
		{"openai to anthropic", ProtocolOpenAI, ProtocolAnthropic, "/v1/chat/completions", "/v1/messages", false},
		{"openai to gemini", ProtocolOpenAI, ProtocolGemini, "/v1/chat/completions", "/v1/chat/completions", false},
		{"anthropic to gemini", ProtocolAnthropic, ProtocolGemini, "/v1/messages", "/v1/chat/completions", false},
		{"unsupported anthropic endpoint", ProtocolAnthropic, ProtocolOpenAI, "/v1/complete", "", true},
		{"unsupported openai endpoint", ProtocolOpenAI, ProtocolAnthropic, "/v1/embeddings", "", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ResolveUpstreamPath(tt.inbound, tt.provider, tt.path)
			if (err != nil) != tt.wantErr {
				t.Fatalf("ResolveUpstreamPath() error = %v, wantErr %v", err, tt.wantErr)
			}
			if got != tt.wantPath {
				t.Errorf("ResolveUpstreamPath() = %q, want %q", got, tt.wantPath)
			}
		})
	}
}

func TestProviderNativeProtocol(t *testing.T) {
	tests := []struct {
		providerType uint16
		want         Protocol
	}{
		{1, ProtocolOpenAI},
		{2, ProtocolAnthropic},
		{3, ProtocolGemini},
		{99, ProtocolOpenAI},
	}
	for _, tt := range tests {
		if got := ProviderNativeProtocol(tt.providerType); got != tt.want {
			t.Errorf("ProviderNativeProtocol(%d) = %v, want %v", tt.providerType, got, tt.want)
		}
	}
}
