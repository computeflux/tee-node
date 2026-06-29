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
