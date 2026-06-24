package relay

import (
	"context"
	"net/http"
)

// RelayContext wraps http.ResponseWriter and *http.Request.
type RelayContext struct {
	W http.ResponseWriter
	R *http.Request
}

// NewRelayContext 从 http.ResponseWriter 和 *http.Request 创建
func NewRelayContext(w http.ResponseWriter, r *http.Request) *RelayContext {
	return &RelayContext{W: w, R: r}
}

// Context 返回 context.Context
func (c *RelayContext) Context() context.Context {
	return c.R.Context()
}
