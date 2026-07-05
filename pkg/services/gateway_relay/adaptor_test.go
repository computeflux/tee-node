package relay

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// ── OpenAIAdaptor ──────────────────────────────────────────

func TestOpenAIAdaptor_GetRequestURL(t *testing.T) {
	a := &OpenAIAdaptor{}

	assert.Equal(t, "https://api.openai.com/v1/chat/completions",
		a.requestURL("https://api.openai.com", "/v1/chat/completions"))

	// Deduplicate overlapping path segment
	assert.Equal(t, "https://api.minimaxi.com/v1/chat/completions",
		a.requestURL("https://api.minimaxi.com/v1", "/v1/chat/completions"))

	// Empty path
	assert.Equal(t, "https://api.openai.com",
		a.requestURL("https://api.openai.com", ""))
}

func TestOpenAIAdaptor_GetProviderName(t *testing.T) {
	a := &OpenAIAdaptor{}
	assert.Equal(t, "openai", a.GetProviderName())
}

// ── AnthropicAdaptor ───────────────────────────────────────

func TestAnthropicAdaptor_GetRequestURL(t *testing.T) {
	a := &AnthropicAdaptor{}
	assert.Equal(t, "https://api.anthropic.com/v1/messages",
		a.requestURL("https://api.anthropic.com", ""))
}

// ── GeminiAdaptor ──────────────────────────────────────────

func TestGeminiAdaptor_GetProviderName(t *testing.T) {
	a := &GeminiAdaptor{}
	assert.Equal(t, "gemini", a.GetProviderName())
}

func TestGeminiAdaptor_GetRequestURL(t *testing.T) {
	a := &GeminiAdaptor{}

	// 标准 base URL（不含 v1beta）
	assert.Equal(t, "https://generativelanguage.googleapis.com/v1beta/models/gemini-1.5-pro:generateContent",
		a.requestURL("https://generativelanguage.googleapis.com", "gemini-1.5-pro", false))

	// base URL 已包含 /v1beta，去重
	assert.Equal(t, "https://generativelanguage.googleapis.com/v1beta/models/gemini-1.5-pro:generateContent",
		a.requestURL("https://generativelanguage.googleapis.com/v1beta", "gemini-1.5-pro", false))

	// model 名带 models/ 前缀，需要剥离
	assert.Equal(t, "https://generativelanguage.googleapis.com/v1beta/models/gemini-1.5-pro:generateContent",
		a.requestURL("https://generativelanguage.googleapis.com", "models/gemini-1.5-pro", false))

	// 流式端点
	assert.Equal(t, "https://generativelanguage.googleapis.com/v1beta/models/gemini-1.5-pro:streamGenerateContent?alt=sse",
		a.requestURL("https://generativelanguage.googleapis.com", "gemini-1.5-pro", true))
}
