package relay

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/computeflux/tee-provider/pkg/services/scheduler"
)

// GeminiAdaptor 以 Gemini 原生格式与上游通信：请求体由 scheduler.ResolveRequest
// 按端点协议转换后透传，协议转换统一在 gateway/protocol 层完成。
type GeminiAdaptor struct{}

func (a *GeminiAdaptor) GetProviderName() string { return "gemini" }

// requestURL 按模型名与是否流式拼接 Gemini 端点 URL。
func (a *GeminiAdaptor) requestURL(baseURL, model string, stream bool) string {
	action := "generateContent"
	if stream {
		action = "streamGenerateContent?alt=sse"
	}
	base := strings.TrimRight(baseURL, "/")
	const versionPrefix = "/v1beta"
	base = strings.TrimSuffix(base, versionPrefix)
	model = strings.TrimPrefix(model, "models/")
	return fmt.Sprintf("%s%s/models/%s:%s", base, versionPrefix, model, action)
}

func (a *GeminiAdaptor) Do(ctx context.Context, ep *scheduler.Endpoint, body []byte) ([]byte, error) {
	sendBody, _, err := scheduler.ResolveRequest(ctx, ep, body)
	if err != nil {
		return nil, &scheduler.UpstreamError{Status: http.StatusBadRequest, Message: err.Error()}
	}
	url := a.requestURL(ep.BaseURL, ep.ModelName, false)
	return httpDo(ctx, ep, url, sendBody, func(req *http.Request) {
		req.Header.Set("x-goog-api-key", ep.KeyValue)
	})
}

func (a *GeminiAdaptor) Stream(ctx context.Context, ep *scheduler.Endpoint, body []byte) (scheduler.Stream, error) {
	sendBody, _, err := scheduler.ResolveRequest(ctx, ep, body)
	if err != nil {
		return nil, &scheduler.UpstreamError{Status: http.StatusBadRequest, Message: err.Error()}
	}
	url := a.requestURL(ep.BaseURL, ep.ModelName, true)
	return httpStream(ctx, ep, url, sendBody, func(req *http.Request) {
		req.Header.Set("x-goog-api-key", ep.KeyValue)
	})
}
