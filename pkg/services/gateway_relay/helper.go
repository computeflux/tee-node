package relay

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

// ── Common adaptor helpers ───────────────────────────────

// SetupCommonRequestHeader sets common headers for upstream requests
func SetupCommonRequestHeader(ctx *RelayContext, req *http.Request, meta *Meta) {
	req.Header.Set("Content-Type", "application/json")
	if meta.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+meta.APIKey)
	}
}

// SetEventStreamHeaders sets headers for SSE streaming
func SetEventStreamHeaders(ctx *RelayContext) {
	ctx.W.Header().Set("Content-Type", "text/event-stream")
	ctx.W.Header().Set("Cache-Control", "no-cache")
	ctx.W.Header().Set("Connection", "keep-alive")
	ctx.W.Header().Set("X-Accel-Buffering", "no")
}

// WriteSSEChunk writes a Server-Sent Event chunk
func WriteSSEChunk(ctx *RelayContext, data []byte) error {
	_, err := fmt.Fprintf(ctx.W, "data: %s\n\n", data)
	if flusher, ok := ctx.W.(http.Flusher); ok {
		flusher.Flush()
	}
	return err
}

// WriteSSEDone writes the SSE termination signal
func WriteSSEDone(ctx *RelayContext) {
	fmt.Fprintf(ctx.W, "data: [DONE]\n\n")
	if flusher, ok := ctx.W.(http.Flusher); ok {
		flusher.Flush()
	}
}

// ── JSON helpers ─────────────────────────────────────────

func jsonEncode(v interface{}) []byte {
	b, _ := json.Marshal(v)
	return b
}

func jsonDecode(data []byte, v interface{}) error {
	return json.Unmarshal(data, v)
}

// ── Provider detection ──────────────────────────────────

// DetectProvider tries to detect the provider from the base URL
func DetectProvider(baseURL string) string {
	lower := strings.ToLower(baseURL)
	switch {
	case strings.Contains(lower, "openai.com"):
		return "openai"
	case strings.Contains(lower, "anthropic.com"):
		return "anthropic"
	default:
		return "openai" // default to OpenAI-compatible
	}
}

// GetAdaptorForProvider returns the right adaptor for a provider
func GetAdaptorForProviderType(providerType string) Adaptor {
	switch providerType {
	case "anthropic":
		return &AnthropicAdaptor{}
	case "gemini":
		return &GeminiAdaptor{}
	default:
		return &OpenAIAdaptor{}
	}
}
