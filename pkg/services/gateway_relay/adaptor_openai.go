package relay

import (
	"context"
	"encoding/json"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/computeflux/tee-provider/pkg/services/scheduler"
)

// OpenAIAdaptor 直接实现 scheduler.ProviderClient：以 OpenAI 兼容格式与上游通信。
type OpenAIAdaptor struct{}

func (a *OpenAIAdaptor) GetProviderName() string { return "openai" }

// requestURL 拼接上游 URL，并对 base 末尾与 path 开头重复的路径组件去重。
// 例如 https://api.minimaxi.com/v1 + /v1/chat/completions
//
//	→ https://api.minimaxi.com/v1/chat/completions
func (a *OpenAIAdaptor) requestURL(baseURL, path string) string {
	base := strings.TrimRight(baseURL, "/")
	if path != "" {
		lastSeg := filepath.Base(base)
		firstSeg := strings.SplitN(strings.TrimPrefix(path, "/"), "/", 2)[0]
		if lastSeg == firstSeg && lastSeg != "" {
			base = strings.TrimSuffix(base, "/"+lastSeg)
		}
	}
	return base + path
}

func (a *OpenAIAdaptor) Do(ctx context.Context, ep *scheduler.Endpoint, body []byte) ([]byte, error) {
	sendBody, upstreamPath, err := scheduler.ResolveRequest(ctx, ep, body)
	if err != nil {
		return nil, &scheduler.UpstreamError{Status: http.StatusBadRequest, Message: err.Error()}
	}
	url := a.requestURL(ep.BaseURL, upstreamPath)
	return httpDo(ctx, url, sendBody, func(req *http.Request) {
		req.Header.Set("Authorization", "Bearer "+ep.KeyValue)
	})
}

func (a *OpenAIAdaptor) Stream(ctx context.Context, ep *scheduler.Endpoint, body []byte) (scheduler.Stream, error) {
	sendBody, upstreamPath, err := scheduler.ResolveRequest(ctx, ep, body)
	if err != nil {
		return nil, &scheduler.UpstreamError{Status: http.StatusBadRequest, Message: err.Error()}
	}
	url := a.requestURL(ep.BaseURL, upstreamPath)
	return httpStream(ctx, url, injectStreamUsage(sendBody), func(req *http.Request) {
		req.Header.Set("Authorization", "Bearer "+ep.KeyValue)
	})
}

// injectStreamUsage 给流式请求注入 stream_options.include_usage=true，
// 以便上游在流末尾返回 usage。已存在 stream_options 时不覆盖。
func injectStreamUsage(body []byte) []byte {
	var reqMap map[string]any
	if json.Unmarshal(body, &reqMap) != nil {
		return body
	}
	if _, ok := reqMap["stream_options"]; ok {
		return body
	}
	reqMap["stream_options"] = map[string]any{"include_usage": true}
	if newRaw, err := json.Marshal(reqMap); err == nil {
		return newRaw
	}
	return body
}
