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

// ── Model name helpers ───────────────────────────────────

// ProviderBaseURLs maps provider type to default base URL
var ProviderBaseURLs = map[string]string{
	"openai":    "https://api.openai.com",
	"anthropic": "https://api.anthropic.com",
	"gemini":    "https://generativelanguage.googleapis.com",
	"azure":     "",
	"baidu":     "https://aip.baidubce.com",
	"zhipu":     "https://open.bigmodel.cn",
	"ali":       "https://dashscope.aliyuncs.com",
	"deepseek":  "https://api.deepseek.com",
	"groq":      "https://api.groq.com/openai",
	"ollama":    "http://localhost:11434",
	"mistral":   "https://api.mistral.ai",
	"cohere":    "https://api.cohere.ai",
	"xai":       "https://api.x.ai",
}

// GetBaseURL returns the default base URL for a provider
func GetBaseURL(provider string) string {
	if url, ok := ProviderBaseURLs[provider]; ok && url != "" {
		return url
	}
	return ""
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
	case strings.Contains(lower, "googleapis.com"):
		return "gemini"
	case strings.Contains(lower, "deepseek.com"):
		return "deepseek"
	case strings.Contains(lower, "groq.com"):
		return "groq"
	case strings.Contains(lower, "mistral.ai"):
		return "mistral"
	case strings.Contains(lower, "cohere.ai"):
		return "cohere"
	case strings.Contains(lower, "x.ai"):
		return "xai"
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
