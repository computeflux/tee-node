package relay

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ── GetAdaptor ─────────────────────────────────────────────

func TestGetAdaptor(t *testing.T) {
	assert.IsType(t, &OpenAIAdaptor{}, GetAdaptor(0))
	assert.IsType(t, &AnthropicAdaptor{}, GetAdaptor(1))
	assert.IsType(t, &GeminiAdaptor{}, GetAdaptor(2))
	assert.IsType(t, &OpenAIAdaptor{}, GetAdaptor(99)) // default
}

// ── GetRelayMode ───────────────────────────────────────────

func TestGetRelayMode(t *testing.T) {
	tests := []struct {
		path string
		want int
	}{
		{"/v1/chat/completions", 1},
		{"/v1/chat/completions?model=gpt-4", 1},
		{"/v1/completions", 2},
		{"/v1/embeddings", 3},
		{"/v1/images/generations", 5},
		{"/v1/audio/speech", 8},
		{"/v1/audio/transcriptions", 9},
		{"/v1/models", 0},
		{"/v1/unknown", 0},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			assert.Equal(t, tt.want, GetRelayMode(tt.path))
		})
	}
}

// ── GetAdaptorForProviderType ──────────────────────────────

func TestGetAdaptorForProviderType(t *testing.T) {
	assert.IsType(t, &OpenAIAdaptor{}, GetAdaptorForProviderType("openai"))
	assert.IsType(t, &AnthropicAdaptor{}, GetAdaptorForProviderType("anthropic"))
	assert.IsType(t, &GeminiAdaptor{}, GetAdaptorForProviderType("gemini"))
	assert.IsType(t, &OpenAIAdaptor{}, GetAdaptorForProviderType("unknown"))
}

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
