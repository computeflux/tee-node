package relay

import (
	"bytes"
	"context"
	"fmt"
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
		return nil, fmt.Errorf("local adaptor: origin model name is empty")
	}
	sendBody, upstreamPath, err := scheduler.ResolveRequest(ctx, ep, body)
	if err != nil {
		return nil, &scheduler.UpstreamError{Status: http.StatusBadRequest, Message: err.Error()}
	}
	if upstreamPath == "" {
		upstreamPath = "/v1/chat/completions"
	}
	resp, err := localtunnel.ForwardRequest(ctx, ep.ContractProviderID, ep.ModelName, bytes.NewReader(sendBody), upstreamPath, stream, nil)
	if err != nil {
		return nil, fmt.Errorf("local tunnel forward: %w", err)
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
