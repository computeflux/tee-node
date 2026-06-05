package relay

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"sync"
)

// RelayContext 替代 gin.Context，兼容 chi/net/http
type RelayContext struct {
	W      http.ResponseWriter
	R      *http.Request
	values map[string]any
	mu     sync.RWMutex
}

// NewRelayContext 从 http.ResponseWriter 和 *http.Request 创建
func NewRelayContext(w http.ResponseWriter, r *http.Request) *RelayContext {
	return &RelayContext{
		W:      w,
		R:      r,
		values: make(map[string]any),
	}
}

// ── mimic gin.Context methods ────────────────────────────

func (c *RelayContext) Set(key string, value any) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.values[key] = value
}

func (c *RelayContext) Get(key string) (any, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	v, ok := c.values[key]
	return v, ok
}

func (c *RelayContext) GetString(key string) string {
	v, ok := c.Get(key)
	if !ok {
		return ""
	}
	s, _ := v.(string)
	return s
}

func (c *RelayContext) GetInt(key string) int {
	v, ok := c.Get(key)
	if !ok {
		return 0
	}
	i, _ := v.(int)
	return i
}

func (c *RelayContext) GetStringMapString(key string) map[string]string {
	v, ok := c.Get(key)
	if !ok {
		return nil
	}
	m, _ := v.(map[string]string)
	return m
}

// Request 返回原始 *http.Request
func (c *RelayContext) Request() *http.Request {
	return c.R
}

// Writer 返回 http.ResponseWriter
func (c *RelayContext) Writer() http.ResponseWriter {
	return c.W
}

// Context 返回 context.Context
func (c *RelayContext) Context() context.Context {
	return c.R.Context()
}

// ReadBody 读取请求体并重置
func (c *RelayContext) ReadBody() ([]byte, error) {
	body, err := io.ReadAll(c.R.Body)
	if err != nil {
		return nil, err
	}
	c.R.Body = io.NopCloser(bytes.NewBuffer(body))
	return body, nil
}

// Stream 流式写入（SSE）
func (c *RelayContext) Stream(step func(w io.Writer) bool) {
	flusher, ok := c.W.(http.Flusher)
	if !ok {
		http.Error(c.W, "streaming not supported", http.StatusInternalServerError)
		return
	}
	c.W.Header().Set("Content-Type", "text/event-stream")
	c.W.Header().Set("Cache-Control", "no-cache")
	c.W.Header().Set("Connection", "keep-alive")

	for {
		if !step(c.W) {
			break
		}
		flusher.Flush()
	}
}

// ClientIP returns client IP
func (c *RelayContext) ClientIP() string {
	xff := c.R.Header.Get("X-Forwarded-For")
	if xff != "" {
		return xff
	}
	return c.R.RemoteAddr
}
