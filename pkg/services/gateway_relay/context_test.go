package relay

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRelayContext_SetGet(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	ctx := NewRelayContext(rec, req)

	ctx.Set("k1", "v1")
	v, ok := ctx.Get("k1")
	assert.True(t, ok)
	assert.Equal(t, "v1", v)
	assert.Equal(t, "v1", ctx.GetString("k1"))
	assert.Equal(t, 0, ctx.GetInt("k1")) // string can't convert to int directly

	ctx.Set("k2", 42)
	assert.Equal(t, 42, ctx.GetInt("k2"))
}

func TestRelayContext_GetStringMapString(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	ctx := NewRelayContext(rec, req)

	ctx.Set("headers", map[string]string{"x-key": "x-val"})
	m := ctx.GetStringMapString("headers")
	assert.Equal(t, "x-val", m["x-key"])
}

func TestRelayContext_RequestWriter(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	ctx := NewRelayContext(rec, req)

	assert.NotNil(t, ctx.Request())
	assert.NotNil(t, ctx.Writer())
}

func TestRelayContext_Context(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	ctx := NewRelayContext(rec, req)

	c := ctx.Context()
	assert.Equal(t, context.Background(), c)
}

func TestRelayContext_ReadBody(t *testing.T) {
	body := `{"message":"hello"}`
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
	ctx := NewRelayContext(rec, req)

	b, err := ctx.ReadBody()
	require.NoError(t, err)
	assert.Equal(t, body, string(b))
}

func TestRelayContext_ClientIP(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "192.168.1.100:12345"
	ctx := NewRelayContext(rec, req)

	assert.Equal(t, "192.168.1.100:12345", ctx.ClientIP())
}

func TestRelayContext_JSONBinding(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"model":"gpt-4"}`))
	req.Header.Set("Content-Type", "application/json")
	ctx := NewRelayContext(rec, req)

	body, err := ctx.ReadBody()
	require.NoError(t, err)

	var payload struct {
		Model string `json:"model"`
	}
	err = json.Unmarshal(body, &payload)
	require.NoError(t, err)
	assert.Equal(t, "gpt-4", payload.Model)
}
