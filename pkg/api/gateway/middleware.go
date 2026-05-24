package gateway

import (
	"bytes"
	"compress/gzip"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"runtime/debug"
	"strings"
	"sync"
	"time"
)

// ── Panic Recovery ───────────────────────────────────────

func RecoveryMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				GATEWAYLOG().Printf("[PANIC] %v\n%s", err, debug.Stack())
				writeError(w, http.StatusInternalServerError,
					fmt.Sprintf("panic: %v", err), "server_error")
			}
		}()
		next.ServeHTTP(w, r)
	})
}

// ── Request ID ───────────────────────────────────────────

func RequestIDMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.Header.Get("X-Request-Id")
		if id == "" {
			b := make([]byte, 4)
			rand.Read(b)
			id = fmt.Sprintf("%d-%x", time.Now().UnixNano(), b)
		}
		w.Header().Set("X-Request-Id", id)
		next.ServeHTTP(w, r)
	})
}

// ── Logger ────────────────────────────────────────────────

func LoggerMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		ww := &responseWriter{ResponseWriter: w, statusCode: 200}
		next.ServeHTTP(ww, r)
		elapsed := time.Since(start)
		GATEWAYLOG().Printf("[HTTP] %s %s | %d | %v | %s",
			r.Method, r.URL.Path, ww.statusCode, elapsed, r.RemoteAddr)
	})
}

type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

// Flush 实现 http.Flusher，支持 SSE 流式响应。
func (rw *responseWriter) Flush() {
	if f, ok := rw.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

// ── GZip Decode ──────────────────────────────────────────

func GzipDecodeMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Content-Encoding") == "gzip" {
			gr, err := gzip.NewReader(r.Body)
			if err != nil {
				http.Error(w, "invalid gzip body", http.StatusBadRequest)
				return
			}
			defer gr.Close()
			r.Body = io.NopCloser(gr)
		}
		next.ServeHTTP(w, r)
	})
}

// ── In-Memory Rate Limiter ───────────────────────────────

type rateLimiter struct {
	mu      sync.Mutex
	buckets map[string][]time.Time
}

var limiter = &rateLimiter{buckets: make(map[string][]time.Time)}

func (rl *rateLimiter) allow(key string, maxReq int, duration time.Duration) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	now := time.Now()
	window := now.Add(-duration)
	bucket := rl.buckets[key]

	// 清理过期
	filtered := make([]time.Time, 0, len(bucket))
	for _, t := range bucket {
		if t.After(window) {
			filtered = append(filtered, t)
		}
	}

	if len(filtered) >= maxReq {
		rl.buckets[key] = filtered
		return false
	}

	filtered = append(filtered, now)
	rl.buckets[key] = filtered
	return true
}

func RateLimitMiddleware(maxReq int, duration time.Duration) func(http.Handler) http.Handler {
	if maxReq <= 0 {
		return func(next http.Handler) http.Handler { return next }
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			key := r.RemoteAddr
			if !limiter.allow(key, maxReq, duration) {
				writeError(w, http.StatusTooManyRequests, "rate limit exceeded", "rate_limit")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// ── Cache Control ────────────────────────────────────────

func CacheMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			w.Header().Set("Cache-Control", "no-cache")
		} else {
			w.Header().Set("Cache-Control", "max-age=604800")
		}
		next.ServeHTTP(w, r)
	})
}

// ── Provider Monitor ──────────────────────────────────────

type providerMetric struct {
	recent []bool // true=success, false=fail
}

var (
	providerMetrics   = make(map[uint64]*providerMetric)
	providerMetricsMu sync.Mutex
)

func EmitProviderMetric(providerID uint64, success bool) {
	providerMetricsMu.Lock()
	defer providerMetricsMu.Unlock()

	m, ok := providerMetrics[providerID]
	if !ok {
		m = &providerMetric{}
		providerMetrics[providerID] = m
	}

	queueSize := 100
	if len(m.recent) >= queueSize {
		m.recent = m.recent[1:]
	}
	m.recent = append(m.recent, success)

	// 监控低成功率 provider（只记录日志，不修改本地 DB；Provider 状态由合约管理）
	threshold := 0.3
	if len(m.recent) >= queueSize {
		successCount := 0
		for _, s := range m.recent {
			if s {
				successCount++
			}
		}
		rate := float64(successCount) / float64(len(m.recent))
		if rate < threshold {
			GATEWAYLOG().Printf("[MONITOR] provider %d success rate %.2f < %.2f, alerting",
				providerID, rate, threshold)
			m.recent = nil
		}
	}
}

func ShouldDisableProvider(errMsg string, statusCode int) bool {
	if statusCode == http.StatusUnauthorized {
		return true
	}
	lower := strings.ToLower(errMsg)
	triggers := []string{
		"insufficient_quota", "invalid_api_key", "account_deactivated",
		"credit", "balance", "已欠费", "permission denied",
	}
	for _, t := range triggers {
		if strings.Contains(lower, t) {
			return true
		}
	}
	return false
}

// ── JSON helpers ─────────────────────────────────────────

func writeError(w http.ResponseWriter, code int, msg, typ string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"error": map[string]string{"message": msg, "type": typ},
	})
}

func readRequestBody(r *http.Request) ([]byte, error) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		return nil, err
	}
	r.Body = io.NopCloser(bytes.NewBuffer(body))
	return body, nil
}
