package relay

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	localtunnel "github.com/computeflux/tee-provider/pkg/services/local_tunnel"
)

// ── LocalAdaptor ─────────────────────────────────────────
//
// LocalAdaptor 通过 Yamux 反向隧道将请求转发到本地私有化部署的 AI 模型。
// 本地客户端（local-client）主动通过 TCP + Yamux 连接到 TEE-Node，
// TEE-Node 在已有 session 上 OpenStream 实现请求转发。

type LocalAdaptor struct{}

func (a *LocalAdaptor) Init(meta *Meta) {}

func (a *LocalAdaptor) GetRequestURL(meta *Meta) (string, error) {
	// 隧道模式下不需要构造 URL
	return "", nil
}

func (a *LocalAdaptor) SetupRequestHeader(ctx *RelayContext, req *http.Request, meta *Meta) error {
	// 隧道模式下 header 在 RequestFrame 中传递
	return nil
}

func (a *LocalAdaptor) DoRequest(ctx *RelayContext, meta *Meta, body io.Reader) (*http.Response, error) {
	modelID := meta.OriginModelName
	if modelID == "" {
		return nil, fmt.Errorf("local adaptor: origin model name is empty")
	}

	path := meta.RequestURLPath
	if path == "" {
		path = "/v1/chat/completions"
	}

	// 收集原始 HTTP headers（透传 Authorization、Content-Type 等）
	headers := make(map[string]string)
	if ctx.R != nil {
		for k, vs := range ctx.R.Header {
			if len(vs) > 0 {
				headers[k] = vs[0]
			}
		}
	}

	resp, err := localtunnel.ForwardRequest(ctx.Context(), modelID, body, path, meta.IsStream, headers)
	if err != nil {
		return nil, fmt.Errorf("local tunnel forward: %w", err)
	}
	return resp, nil
}

func (a *LocalAdaptor) DoResponse(ctx *RelayContext, resp *http.Response, meta *Meta) (*Usage, *ErrorWithStatusCode) {
	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return nil, &ErrorWithStatusCode{
			StatusCode: resp.StatusCode,
			Error: ErrorDetail{
				Message: string(body),
				Type:    "upstream_error",
			},
		}
	}

	// 注入本地模型标识头
	ctx.W.Header().Set("X-Provider-Type", "local")

	contentType := resp.Header.Get("Content-Type")
	if strings.Contains(contentType, "text/event-stream") || meta.IsStream {
		return a.handleLocalStream(ctx, resp)
	}
	return a.handleLocalNormal(ctx, resp)
}

func (a *LocalAdaptor) GetProviderName() string {
	return "local"
}

// handleLocalStream 处理 SSE 流式响应，兼容本地模型不规范输出
func (a *LocalAdaptor) handleLocalStream(ctx *RelayContext, resp *http.Response) (*Usage, *ErrorWithStatusCode) {
	flusher, ok := ctx.W.(http.Flusher)
	if !ok {
		return nil, &ErrorWithStatusCode{StatusCode: 500, Error: ErrorDetail{Message: "streaming not supported"}}
	}

	SetEventStreamHeaders(ctx)

	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 0, 64*1024), 64*1024)

	usage := &Usage{}
	for scanner.Scan() {
		line := scanner.Text()

		// 兼容本地模型不规范 SSE：补全缺少的 data: 前缀
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			fmt.Fprintln(ctx.W)
			flusher.Flush()
			continue
		}
		if !strings.HasPrefix(trimmed, "data: ") && (strings.HasPrefix(trimmed, "{") || strings.HasPrefix(trimmed, "[")) {
			trimmed = "data: " + trimmed
		}

		fmt.Fprintf(ctx.W, "%s\n", trimmed)
		flusher.Flush()

		// 解析 usage
		if strings.HasPrefix(trimmed, "data: ") && !strings.Contains(trimmed, "[DONE]") {
			data := strings.TrimPrefix(trimmed, "data: ")
			var chunk struct {
				Usage *Usage `json:"usage,omitempty"`
			}
			if json.Unmarshal([]byte(data), &chunk) == nil && chunk.Usage != nil {
				usage = chunk.Usage
			}
		}
	}

	WriteSSEDone(ctx)
	return usage, nil
}

// handleLocalNormal 处理普通（非流式）响应
func (a *LocalAdaptor) handleLocalNormal(ctx *RelayContext, resp *http.Response) (*Usage, *ErrorWithStatusCode) {
	for k, vs := range resp.Header {
		for _, v := range vs {
			ctx.W.Header().Add(k, v)
		}
	}
	ctx.W.WriteHeader(resp.StatusCode)

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, &ErrorWithStatusCode{StatusCode: 500, Error: ErrorDetail{Message: "failed to read response"}}
	}

	var result struct {
		Usage *Usage `json:"usage,omitempty"`
	}
	var usage *Usage
	if json.Unmarshal(body, &result) == nil {
		usage = result.Usage
	}

	ctx.W.Write(body)
	return usage, nil
}
