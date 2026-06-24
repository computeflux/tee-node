package relay

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ── OpenAIAdaptor ──────────────────────────────────────────

func TestOpenAIAdaptor_GetRequestURL(t *testing.T) {
	a := &OpenAIAdaptor{}

	url, err := a.GetRequestURL(&Meta{BaseURL: "https://api.openai.com", RequestURLPath: "/v1/chat/completions"})
	require.NoError(t, err)
	assert.Equal(t, "https://api.openai.com/v1/chat/completions", url)

	// Deduplicate overlapping path segment
	url, err = a.GetRequestURL(&Meta{BaseURL: "https://api.minimaxi.com/v1", RequestURLPath: "/v1/chat/completions"})
	require.NoError(t, err)
	assert.Equal(t, "https://api.minimaxi.com/v1/chat/completions", url)

	// Empty path
	url, err = a.GetRequestURL(&Meta{BaseURL: "https://api.openai.com", RequestURLPath: ""})
	require.NoError(t, err)
	assert.Equal(t, "https://api.openai.com", url)
}

func TestOpenAIAdaptor_GetProviderName(t *testing.T) {
	a := &OpenAIAdaptor{}
	assert.Equal(t, "openai", a.GetProviderName())
}

// ── AnthropicAdaptor ───────────────────────────────────────

func TestAnthropicAdaptor_GetRequestURL(t *testing.T) {
	a := &AnthropicAdaptor{}
	url, err := a.GetRequestURL(&Meta{BaseURL: "https://api.anthropic.com"})
	require.NoError(t, err)
	assert.Equal(t, "https://api.anthropic.com/v1/messages", url)
}

// ── GeminiAdaptor ──────────────────────────────────────────

func TestGeminiAdaptor_GetProviderName(t *testing.T) {
	a := &GeminiAdaptor{}
	assert.Equal(t, "gemini", a.GetProviderName())
}
