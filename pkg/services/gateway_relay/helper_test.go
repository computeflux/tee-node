package relay

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ── DetectProvider ─────────────────────────────────────────

func TestDetectProvider(t *testing.T) {
	tests := []struct {
		url  string
		want string
	}{
		{"https://api.openai.com/v1", "openai"},
		{"https://api.anthropic.com/v1", "anthropic"},
		{"https://generativelanguage.googleapis.com", "gemini"},
		{"https://api.deepseek.com", "deepseek"},
		{"https://api.groq.com/openai", "groq"},
		{"https://api.mistral.ai", "mistral"},
		{"https://api.cohere.ai", "cohere"},
		{"https://api.x.ai", "xai"},
		{"https://unknown.com", "openai"}, // default
	}

	for _, tt := range tests {
		t.Run(tt.url, func(t *testing.T) {
			assert.Equal(t, tt.want, DetectProvider(tt.url))
		})
	}
}

// ── jsonEncode / jsonDecode ────────────────────────────────

func TestJsonEncode(t *testing.T) {
	data := map[string]string{"key": "value"}
	b := jsonEncode(data)
	assert.Contains(t, string(b), "key")
	assert.Contains(t, string(b), "value")
}

func TestJsonDecode(t *testing.T) {
	var result map[string]string
	err := jsonDecode([]byte(`{"key":"value"}`), &result)
	require.NoError(t, err)
	assert.Equal(t, "value", result["key"])
}

func TestJsonDecode_Invalid(t *testing.T) {
	var result map[string]string
	err := jsonDecode([]byte(`not json`), &result)
	assert.Error(t, err)
}

// ── SetupCommonRequestHeader ───────────────────────────────

func TestSetupCommonRequestHeader(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/", nil)
	ctx := NewRelayContext(rec, req)
	meta := &Meta{APIKey: "sk-test"}

	SetupCommonRequestHeader(ctx, req, meta)

	assert.Equal(t, "application/json", req.Header.Get("Content-Type"))
	assert.Equal(t, "Bearer sk-test", req.Header.Get("Authorization"))
}

func TestSetupCommonRequestHeader_NoAPIKey(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/", nil)
	ctx := NewRelayContext(rec, req)
	meta := &Meta{APIKey: ""}

	SetupCommonRequestHeader(ctx, req, meta)

	assert.Equal(t, "application/json", req.Header.Get("Content-Type"))
	assert.Equal(t, "", req.Header.Get("Authorization"))
}

// ── SetEventStreamHeaders ──────────────────────────────────

func TestSetEventStreamHeaders(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	ctx := NewRelayContext(rec, req)

	SetEventStreamHeaders(ctx)

	assert.Equal(t, "text/event-stream", rec.Header().Get("Content-Type"))
	assert.Equal(t, "no-cache", rec.Header().Get("Cache-Control"))
	assert.Equal(t, "keep-alive", rec.Header().Get("Connection"))
	assert.Equal(t, "no", rec.Header().Get("X-Accel-Buffering"))
}

// ── WriteSSEChunk / WriteSSEDone ───────────────────────────

func TestWriteSSEChunk(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	ctx := NewRelayContext(rec, req)

	err := WriteSSEChunk(ctx, []byte(`{"data":"hello"}`))
	require.NoError(t, err)
	assert.Contains(t, rec.Body.String(), `data: {"data":"hello"}`)
}

func TestWriteSSEDone(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	ctx := NewRelayContext(rec, req)

	WriteSSEDone(ctx)
	assert.Contains(t, rec.Body.String(), "data: [DONE]")
}
