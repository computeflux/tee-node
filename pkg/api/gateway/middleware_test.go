package gateway

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ── RecoveryMiddleware ─────────────────────────────────────

func TestRecoveryMiddleware_RecoversPanic(t *testing.T) {
	panicHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("something went wrong")
	})

	mw := RecoveryMiddleware(panicHandler)
	w := httptest.NewRecorder()
	mw.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/", nil))

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	assert.Contains(t, w.Body.String(), "internal server error")
	assert.NotContains(t, w.Body.String(), "panic:", "panic 详情不应泄露给客户端")
	assert.Equal(t, "application/json", w.Header().Get("Content-Type"))
}

func TestRecoveryMiddleware_PassesThroughWhenNoPanic(t *testing.T) {
	okHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})

	mw := RecoveryMiddleware(okHandler)
	w := httptest.NewRecorder()
	mw.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/", nil))

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "ok", w.Body.String())
}

// ── RequestIDMiddleware ────────────────────────────────────

func TestRequestIDMiddleware_GeneratesIDWhenMissing(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	mw := RequestIDMiddleware(next)
	w := httptest.NewRecorder()
	mw.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/", nil))

	id := w.Header().Get("X-Request-Id")
	assert.NotEmpty(t, id)
	assert.Contains(t, id, "-")
}

func TestRequestIDMiddleware_PreservesExistingID(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	mw := RequestIDMiddleware(next)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Request-Id", "existing-id-123")
	mw.ServeHTTP(w, req)

	assert.Equal(t, "existing-id-123", w.Header().Get("X-Request-Id"))
}

// ── LoggerMiddleware ───────────────────────────────────────

func TestLoggerMiddleware_CapturesStatusCode(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
		w.Write([]byte("created"))
	})

	mw := LoggerMiddleware(next)
	w := httptest.NewRecorder()
	mw.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/test", nil))

	// responseWriter wrapper should capture the status code
	assert.Equal(t, http.StatusCreated, w.Code)
	assert.Equal(t, "created", w.Body.String())
}

func TestResponseWriter_Flush(t *testing.T) {
	// httptest.ResponseWriter implements http.Flusher
	rec := httptest.NewRecorder()
	rw := &responseWriter{ResponseWriter: rec, statusCode: 200}

	// Should not panic
	rw.Flush()
}

// ── GzipDecodeMiddleware ───────────────────────────────────

func TestGzipDecodeMiddleware_DecompressesBody(t *testing.T) {
	var receivedBody []byte
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedBody, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusOK)
	})

	// Compress payload
	var buf bytes.Buffer
	gw := gzip.NewWriter(&buf)
	gw.Write([]byte(`{"model":"gpt-4"}`))
	gw.Close()

	req := httptest.NewRequest(http.MethodPost, "/", &buf)
	req.Header.Set("Content-Encoding", "gzip")

	mw := GzipDecodeMiddleware(next)
	w := httptest.NewRecorder()
	mw.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, `{"model":"gpt-4"}`, string(receivedBody))
}

func TestGzipDecodeMiddleware_InvalidGzip(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader("not-gzip"))
	req.Header.Set("Content-Encoding", "gzip")

	mw := GzipDecodeMiddleware(next)
	w := httptest.NewRecorder()
	mw.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "invalid gzip body")
}

func TestGzipDecodeMiddleware_PassesPlainBody(t *testing.T) {
	var receivedBody []byte
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedBody, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`plain body`))
	// No Content-Encoding header

	mw := GzipDecodeMiddleware(next)
	w := httptest.NewRecorder()
	mw.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "plain body", string(receivedBody))
}

// ── RateLimitMiddleware ────────────────────────────────────

func TestRateLimitMiddleware_AllowsWithinLimit(t *testing.T) {
	// Reset global limiter for test isolation
	limiter.buckets = make(map[string][]time.Time)

	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	mw := RateLimitMiddleware(3, time.Minute)(next)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "1.2.3.4:1234"

	// 3 requests should pass
	for i := 0; i < 3; i++ {
		mw.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code, "request %d", i)
		w = httptest.NewRecorder() // reset
	}
}

func TestRateLimitMiddleware_BlocksOverLimit(t *testing.T) {
	limiter.buckets = make(map[string][]time.Time)

	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	mw := RateLimitMiddleware(2, time.Minute)(next)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "1.2.3.4:1234"

	// First 2 pass
	mw.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
	w = httptest.NewRecorder()
	mw.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	// 3rd should be rate limited
	w = httptest.NewRecorder()
	mw.ServeHTTP(w, req)
	assert.Equal(t, http.StatusTooManyRequests, w.Code)
	assert.Contains(t, w.Body.String(), "rate limit exceeded")
}

func TestRateLimitMiddleware_DisabledWhenMaxReqZero(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	mw := RateLimitMiddleware(0, time.Minute)(next)
	w := httptest.NewRecorder()
	mw.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/", nil))

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestRateLimitMiddleware_DifferentIPsIndependent(t *testing.T) {
	limiter.buckets = make(map[string][]time.Time)

	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	mw := RateLimitMiddleware(1, time.Minute)(next)

	// IP A uses its only slot
	w1 := httptest.NewRecorder()
	req1 := httptest.NewRequest(http.MethodGet, "/", nil)
	req1.RemoteAddr = "1.1.1.1:1111"
	mw.ServeHTTP(w1, req1)
	assert.Equal(t, http.StatusOK, w1.Code)

	// IP A blocked
	w1 = httptest.NewRecorder()
	mw.ServeHTTP(w1, req1)
	assert.Equal(t, http.StatusTooManyRequests, w1.Code)

	// IP B still allowed
	w2 := httptest.NewRecorder()
	req2 := httptest.NewRequest(http.MethodGet, "/", nil)
	req2.RemoteAddr = "2.2.2.2:2222"
	mw.ServeHTTP(w2, req2)
	assert.Equal(t, http.StatusOK, w2.Code)
}

func TestRateLimitMiddleware_WindowExpiresOldEntries(t *testing.T) {
	limiter.buckets = make(map[string][]time.Time)

	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	mw := RateLimitMiddleware(1, time.Millisecond)(next)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "1.2.3.4:1234"

	// First request passes
	w := httptest.NewRecorder()
	mw.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	// Immediately blocked
	w = httptest.NewRecorder()
	mw.ServeHTTP(w, req)
	assert.Equal(t, http.StatusTooManyRequests, w.Code)

	// Wait for window to expire
	time.Sleep(5 * time.Millisecond)

	w = httptest.NewRecorder()
	mw.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}

// ── CacheMiddleware ────────────────────────────────────────

func TestCacheMiddleware_RootPathNoCache(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	mw := CacheMiddleware(next)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	mw.ServeHTTP(w, req)

	assert.Equal(t, "no-cache", w.Header().Get("Cache-Control"))
}

func TestCacheMiddleware_OtherPathMaxAge(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	mw := CacheMiddleware(next)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/models", nil)
	mw.ServeHTTP(w, req)

	assert.Equal(t, "max-age=604800", w.Header().Get("Cache-Control"))
}

// ── Provider Monitor ───────────────────────────────────────

func TestEmitProviderMetric_TracksSuccessAndFail(t *testing.T) {
	providerMetrics = make(map[uint64]*providerMetric)

	// Emit 50 success + 50 fail = 50% success rate
	for i := 0; i < 50; i++ {
		EmitProviderMetric(1, true)
		EmitProviderMetric(1, false)
	}

	providerMetricsMu.Lock()
	m := providerMetrics[1]
	providerMetricsMu.Unlock()
	require.NotNil(t, m)
	assert.Equal(t, 100, len(m.recent))

	// Success rate is 50%, which is above 30% threshold, so queue is not reset
	assert.Equal(t, 100, len(m.recent))
}

func TestEmitProviderMetric_LowSuccessRateResets(t *testing.T) {
	providerMetrics = make(map[uint64]*providerMetric)

	// Emit 100 failures = 0% success rate → below 30% threshold, queue reset
	for i := 0; i < 100; i++ {
		EmitProviderMetric(2, false)
	}

	providerMetricsMu.Lock()
	m := providerMetrics[2]
	providerMetricsMu.Unlock()
	require.NotNil(t, m)
	// Queue is reset to nil when threshold is breached
	assert.Nil(t, m.recent)
}


func TestParseProviderModel_WithProvider(t *testing.T) {
	p, m := parseProviderModel("openai/gpt-4")
	assert.Equal(t, "openai", p)
	assert.Equal(t, "gpt-4", m)
}

func TestParseProviderModel_BareModel(t *testing.T) {
	p, m := parseProviderModel("gpt-4")
	assert.Equal(t, "", p)
	assert.Equal(t, "gpt-4", m)
}

func TestParseProviderModel_MultipleSlashes(t *testing.T) {
	p, m := parseProviderModel("anthropic/claude-3/opus")
	assert.Equal(t, "anthropic", p)
	assert.Equal(t, "claude-3/opus", m)
}

// ── writeError ─────────────────────────────────────────────

func TestWriteError(t *testing.T) {
	w := httptest.NewRecorder()
	writeError(w, http.StatusBadRequest, "bad input", "invalid_request")

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Equal(t, "application/json", w.Header().Get("Content-Type"))

	var body map[string]any
	json.Unmarshal(w.Body.Bytes(), &body)
	err := body["error"].(map[string]any)
	assert.Equal(t, "bad input", err["message"])
	assert.Equal(t, "invalid_request", err["type"])
}

// ── readRequestBody ────────────────────────────────────────

func TestReadRequestBody(t *testing.T) {
	body := []byte(`{"key":"value"}`)
	r := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(body))

	got, err := readRequestBody(r)
	require.NoError(t, err)
	assert.Equal(t, body, got)

	// Body should be reset and readable again
	got2, _ := io.ReadAll(r.Body)
	assert.Equal(t, body, got2)
}

func TestReadRequestBody_ReadError(t *testing.T) {
	r := httptest.NewRequest(http.MethodPost, "/", &errorReader{})
	_, err := readRequestBody(r)
	assert.Error(t, err)
}

type errorReader struct{}

func (e *errorReader) Read(p []byte) (n int, err error) {
	return 0, fmt.Errorf("read error")
}

func TestMin(t *testing.T) {
	assert.Equal(t, 3, min(3, 5))
	assert.Equal(t, 5, min(10, 5))
	assert.Equal(t, 0, min(0, 100))
}

// ── DistributeMiddleware ───────────────────────────────────

func TestDistributeMiddleware_ModelNumeric_400(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	mw := DistributeMiddleware(next)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"model":123}`))
	req.Header.Set("Content-Type", "application/json")

	mw.ServeHTTP(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "model must be a string")
}

func TestDistributeMiddleware_ModelObject_400(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	mw := DistributeMiddleware(next)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"model":{"name":"gpt-4"}}`))
	req.Header.Set("Content-Type", "application/json")

	mw.ServeHTTP(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "model must be a string")
}

func TestIsRouteName_CaseInsensitive(t *testing.T) {
	cases := []struct {
		input string
		want  bool
	}{
		{"MiningPool/foo", true},
		{"miningpool/foo", true},
		{"MININGPOOL/foo", true},
		{"Edge/foo", true},
		{"edge/foo", true},
		{"TEE/foo", true},
		{"tee/foo", true},
		{"openai/foo", false},
		{"foo", false},
	}
	for _, c := range cases {
		t.Run(c.input, func(t *testing.T) {
			assert.Equal(t, c.want, isRouteName(c.input))
		})
	}
}
