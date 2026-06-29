package relay

import (
	"context"
	"net/http"
	"strings"

	"github.com/computeflux/tee-provider/pkg/services/scheduler"
)

// AnthropicAdaptor 以 Anthropic 原生格式与上游通信：请求体由
// scheduler.ResolveRequest 按端点协议转换后透传，relay 层不做 OpenAI ↔ Anthropic 转换。
type AnthropicAdaptor struct{}

func (a *AnthropicAdaptor) GetProviderName() string { return "anthropic" }

// requestURL 拼接上游 URL，默认路径 /v1/messages，并对 base/path 重复段去重。
func (a *AnthropicAdaptor) requestURL(baseURL, path string) string {
	base := strings.TrimRight(baseURL, "/")
	if path == "" {
		path = "/v1/messages"
	}
	pathParts := strings.SplitN(strings.TrimPrefix(path, "/"), "/", 2)
	if len(pathParts) > 0 && pathParts[0] != "" && strings.HasSuffix(base, "/"+pathParts[0]) {
		base = strings.TrimSuffix(base, "/"+pathParts[0])
	}
	return base + path
}

func (a *AnthropicAdaptor) Do(ctx context.Context, ep *scheduler.Endpoint, body []byte) ([]byte, error) {
	sendBody, upstreamPath, err := scheduler.ResolveRequest(ctx, ep, body)
	if err != nil {
		return nil, &scheduler.UpstreamError{Status: http.StatusBadRequest, Message: err.Error()}
	}
	url := a.requestURL(ep.BaseURL, upstreamPath)
	return httpDo(ctx, url, sendBody, func(req *http.Request) {
		req.Header.Set("x-api-key", ep.KeyValue)
		req.Header.Set("anthropic-version", "2023-06-01")
	})
}

func (a *AnthropicAdaptor) Stream(ctx context.Context, ep *scheduler.Endpoint, body []byte) (scheduler.Stream, error) {
	sendBody, upstreamPath, err := scheduler.ResolveRequest(ctx, ep, body)
	if err != nil {
		return nil, &scheduler.UpstreamError{Status: http.StatusBadRequest, Message: err.Error()}
	}
	url := a.requestURL(ep.BaseURL, upstreamPath)
	return httpStream(ctx, url, sendBody, func(req *http.Request) {
		req.Header.Set("x-api-key", ep.KeyValue)
		req.Header.Set("anthropic-version", "2023-06-01")
	})
}
