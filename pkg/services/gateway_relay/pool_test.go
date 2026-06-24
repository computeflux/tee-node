package relay

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestApplyPoolConfig(t *testing.T) {
	cfg := PoolConfig{
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 10,
		MaxConnsPerHost:     50,
		DialTimeout:         5 * time.Second,
		KeepAlive:           30 * time.Second,
		IdleTimeout:         60 * time.Second,
		RequestTimeout:      120 * time.Second,
	}
	ApplyPoolConfig(cfg)

	got := defaultPoolConfig
	assert.Equal(t, 100, got.MaxIdleConns)
	assert.Equal(t, 10, got.MaxIdleConnsPerHost)
	assert.Equal(t, 50, got.MaxConnsPerHost)

	// restore default
	ApplyPoolConfig(defaultPoolConfig)
}

func TestDoHTTP_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"result":"ok"}`))
	}))
	defer server.Close()

	req, err := http.NewRequest(http.MethodGet, server.URL, nil)
	require.NoError(t, err)

	resp, err := DoHTTP(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	body, _ := io.ReadAll(resp.Body)
	assert.Contains(t, string(body), "ok")
}

func TestDoHTTP_Error(t *testing.T) {
	req, err := http.NewRequest(http.MethodGet, "http://127.0.0.1:1/invalid", nil)
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	req = req.WithContext(ctx)

	resp, err := DoHTTP(req)
	assert.Error(t, err)
	assert.Nil(t, resp)
}

func TestDoHTTP_PostJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
		body, _ := io.ReadAll(r.Body)
		assert.Equal(t, `{"key":"val"}`, string(body))
		w.WriteHeader(http.StatusCreated)
	}))
	defer server.Close()

	req, err := http.NewRequest(http.MethodPost, server.URL, strings.NewReader(`{"key":"val"}`))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")

	resp, err := DoHTTP(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusCreated, resp.StatusCode)
}
