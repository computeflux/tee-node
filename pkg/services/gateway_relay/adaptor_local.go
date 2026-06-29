package relay

import (
	"bytes"
	"context"
	"net/http"

	localtunnel "github.com/computeflux/tee-provider/pkg/services/local_tunnel"
	"github.com/computeflux/tee-provider/pkg/services/scheduler"
)

// ── LocalAdaptor ─────────────────────────────────────────
// LocalAdaptor 把请求经 local tunnel 转发到 provider 自建节点。
type LocalAdaptor struct{}

func (a *LocalAdaptor) GetProviderName() string { return "local" }

func (a *LocalAdaptor) forward(ctx context.Context, ep *scheduler.Endpoint, body []byte, stream bool) (*http.Response, error) {
	if ep.ModelName == "" {
		return nil, &scheduler.UpstreamError{Status: http.StatusBadRequest, Message: "local adaptor: missing ModelName"}
	}
	if ep.ContractProviderID == 0 {
		return nil, &scheduler.UpstreamError{Status: http.StatusBadGateway, Message: "local adaptor: missing ContractProviderID"}
	}
	sendBody, upstreamPath, err := scheduler.ResolveRequest(ctx, ep, body)
	if err != nil {
		// 本地模型的「客户端协议不被本地 provider 支持」属于服务端配置问题，用 502 让上层正确分类。
		return nil, &scheduler.UpstreamError{Status: http.StatusBadGateway, Message: err.Error()}
	}
	// 仅在 transform 未注入且 ctx 无入口路径时触发:本地 provider 默认按 OpenAI 兼容格式访问。
	if upstreamPath == "" {
		upstreamPath = "/v1/chat/completions"
	}

	headers := map[string]string{
		"Content-Type": "application/json",
		"Accept":       "application/json",
	}
	if stream {
		headers["Accept"] = "text/event-stream"
	}

	resp, err := localtunnel.ForwardRequest(ctx, ep.ContractProviderID, ep.ModelName, bytes.NewReader(sendBody), upstreamPath, stream, headers)
	if err != nil {
		// local tunnel 故障 = provider 离线/路由不存在，应作为可重试的 5xx 上游错误。
		return nil, &scheduler.UpstreamError{Status: http.StatusBadGateway, Message: err.Error()}
	}
	return resp, nil
}

func (a *LocalAdaptor) Do(ctx context.Context, ep *scheduler.Endpoint, body []byte) ([]byte, error) {
	resp, err := a.forward(ctx, ep, body, false)
	if err != nil {
		return nil, err
	}
	return readResponse(resp)
}

func (a *LocalAdaptor) Stream(ctx context.Context, ep *scheduler.Endpoint, body []byte) (scheduler.Stream, error) {
	resp, err := a.forward(ctx, ep, body, true)
	if err != nil {
		return nil, err
	}
	return openStream(ctx, resp)
}
