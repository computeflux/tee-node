package relay

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/computeflux/tee-provider/pkg/services/scheduler"
	"github.com/computeflux/tee-provider/tee-chain/runtime/module/gateway"
)

// OpenAIResponsesApiAdaptor 以 OpenAI Responses API(OpenAIResponsesApi) 原生格式与上游通信。
// 端点格式为 POST /v1/responses，请求/响应体是 OpenAI Responses 格式；
// 请求体由 scheduler.ResolveRequest 按端点协议转换后透传，relay 层不做格式转换。
type OpenAIResponsesApiAdaptor struct{}

func (a *OpenAIResponsesApiAdaptor) GetProviderName() string { return "openai-responses-api" }

// requestURL 拼接上游 URL，并对 base 末尾与 path 开头重复的路径组件去重。
// 默认上游路径 /v1/responses。
func (a *OpenAIResponsesApiAdaptor) requestURL(baseURL, path string) string {
	base := strings.TrimRight(baseURL, "/")
	if path == "" {
		path = "/v1/responses"
	}
	lastSeg := filepath.Base(base)
	firstSeg := strings.SplitN(strings.TrimPrefix(path, "/"), "/", 2)[0]
	if lastSeg == firstSeg && lastSeg != "" {
		base = strings.TrimSuffix(base, "/"+lastSeg)
	}
	return base + path
}

func (a *OpenAIResponsesApiAdaptor) Do(ctx context.Context, ep *scheduler.Endpoint, body []byte) ([]byte, error) {
	sendBody, upstreamPath, err := scheduler.ResolveRequest(ctx, ep, body)
	if err != nil {
		var ue *scheduler.UpstreamError
		if errors.As(err, &ue) {
			return nil, ue
		}
		return nil, &scheduler.UpstreamError{Status: http.StatusInternalServerError, Message: err.Error()}
	}
	if upstreamPath == "" {
		upstreamPath = "/v1/responses"
	}
	url := a.requestURL(ep.ModelUrls[gateway.OpenAIResponsesApi], upstreamPath)
	return httpDo(ctx, ep, url, sendBody, func(req *http.Request) {
		req.Header.Set("Authorization", "Bearer "+ep.KeyValue)
		for k, v := range ep.ExtraHeaders {
			req.Header.Set(k, v)
		}
	})
}

func (a *OpenAIResponsesApiAdaptor) Stream(ctx context.Context, ep *scheduler.Endpoint, body []byte) (scheduler.Stream, error) {
	sendBody, upstreamPath, err := scheduler.ResolveRequest(ctx, ep, body)
	if err != nil {
		var ue *scheduler.UpstreamError
		if errors.As(err, &ue) {
			return nil, ue
		}
		return nil, &scheduler.UpstreamError{Status: http.StatusInternalServerError, Message: err.Error()}
	}
	if upstreamPath == "" {
		upstreamPath = "/v1/responses"
	}
	url := a.requestURL(ep.ModelUrls[gateway.OpenAIResponsesApi], upstreamPath)
	out := ensureResponsesStream(sendBody)
	return httpStream(ctx, ep, url, out, func(req *http.Request) {
		req.Header.Set("Authorization", "Bearer "+ep.KeyValue)
		for k, v := range ep.ExtraHeaders {
			req.Header.Set(k, v)
		}
	})
}

// ensureResponsesStream 给流式请求注入 stream=true（Responses API 用顶层 stream 字段控制流式），
// 已存在时保持原样不覆盖。
func ensureResponsesStream(body []byte) []byte {
	var reqMap map[string]any
	if json.Unmarshal(body, &reqMap) != nil || reqMap == nil {
		return body
	}
	if _, ok := reqMap["stream"]; ok {
		return body
	}
	reqMap["stream"] = true
	if newRaw, err := json.Marshal(reqMap); err == nil {
		return newRaw
	}
	return body
}
