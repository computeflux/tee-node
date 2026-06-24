package relay

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/computeflux/tee-provider/pkg/api/gateway/protocol"
)

// ── Anthropic types（仅用于解析 usage / error）────────────────

type AnthropicResponse struct {
	Id      string             `json:"id"`
	Type    string             `json:"type,omitempty"`
	Model   string             `json:"model"`
	Role    string             `json:"role,omitempty"`
	Content []AnthropicContent `json:"content"`
	Usage   struct {
		InputTokens              int `json:"input_tokens"`
		OutputTokens             int `json:"output_tokens"`
		CacheCreationInputTokens int `json:"cache_creation_input_tokens"`
		CacheReadInputTokens     int `json:"cache_read_input_tokens"`
	} `json:"usage"`
	Error *struct {
		Type    string `json:"type"`
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

type AnthropicContent struct {
	Type string `json:"type"`
	Text string `json:"text,omitempty"`
}

// anthropicStreamEvent 表示 Anthropic SSE 流中的各种事件
type anthropicStreamEvent struct {
	Type  string `json:"type"`
	Delta *struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"delta"`
	Message *struct {
		Usage *struct {
			InputTokens              int `json:"input_tokens"`
			OutputTokens             int `json:"output_tokens"`
			CacheCreationInputTokens int `json:"cache_creation_input_tokens"`
			CacheReadInputTokens     int `json:"cache_read_input_tokens"`
		} `json:"usage"`
	} `json:"message"`
	Usage *struct {
		OutputTokens int `json:"output_tokens"`
	} `json:"usage"`
}

// ── AnthropicAdaptor ─────────────────────────────────────
// AnthropicAdaptor 以 Anthropic 原生格式与上游通信：请求体/响应体均直接透传，
// 不再在 relay 层做 OpenAI ↔ Anthropic 转换。协议转换统一在 gateway/protocol 层完成。
type AnthropicAdaptor struct{}

func (a *AnthropicAdaptor) Init(meta *Meta) {}

func (a *AnthropicAdaptor) GetRequestURL(meta *Meta) (string, error) {
	base := strings.TrimRight(meta.BaseURL, "/")
	path := meta.RequestURLPath
	if path == "" {
		path = "/v1/messages"
	}

	// 去重：例如 base 末尾有 /v1 而 path 以 /v1 开头
	base = strings.TrimRight(base, "/")
	pathParts := strings.SplitN(strings.TrimPrefix(path, "/"), "/", 2)
	if len(pathParts) > 0 && pathParts[0] != "" && strings.HasSuffix(base, "/"+pathParts[0]) {
		base = strings.TrimSuffix(base, "/"+pathParts[0])
	}

	return base + path, nil
}

func (a *AnthropicAdaptor) SetupRequestHeader(ctx *RelayContext, req *http.Request, meta *Meta) error {
	forwardClientHeaders(ctx, req)
	req.Header.Set("x-api-key", meta.APIKey)
	req.Header.Set("Content-Type", "application/json")

	// 透传客户端指定的 anthropic-version，缺省用官方默认版本。
	if v := ctx.R.Header.Get("anthropic-version"); v != "" {
		req.Header.Set("anthropic-version", v)
	} else {
		req.Header.Set("anthropic-version", "2023-06-01")
	}

	// 透传 beta header（prompt caching、extended thinking 等能力依赖它）。
	if v := ctx.R.Header.Get("anthropic-beta"); v != "" {
		req.Header.Set("anthropic-beta", v)
	}
	return nil
}

func (a *AnthropicAdaptor) DoRequest(ctx *RelayContext, meta *Meta, body io.Reader) (*http.Response, error) {
	raw, _ := io.ReadAll(body)

	url, err := a.GetRequestURL(meta)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx.Context(), http.MethodPost, url, bytes.NewReader(raw))
	if err != nil {
		return nil, err
	}
	a.SetupRequestHeader(ctx, req, meta)
	return DoHTTP(req)
}

func (a *AnthropicAdaptor) DoResponse(ctx *RelayContext, resp *http.Response, meta *Meta) (*Usage, *ErrorWithStatusCode) {
	contentType := resp.Header.Get("Content-Type")
	if strings.Contains(contentType, "text/event-stream") || meta.IsStream {
		return a.handleStream(ctx, resp)
	}
	return a.handleNormal(ctx, resp)
}

func (a *AnthropicAdaptor) handleNormal(ctx *RelayContext, resp *http.Response) (*Usage, *ErrorWithStatusCode) {
	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		msg := string(body)
		errType := "upstream_error"
		if msg == "" {
			msg = http.StatusText(resp.StatusCode)
		}
		// 尝试解析 Anthropic 错误结构以保留类型信息
		var antResp AnthropicResponse
		if json.Unmarshal(body, &antResp) == nil && antResp.Error != nil {
			if antResp.Error.Message != "" {
				msg = antResp.Error.Message
			}
			if antResp.Error.Type != "" {
				errType = antResp.Error.Type
			}
		}
		return nil, &ErrorWithStatusCode{
			StatusCode: resp.StatusCode,
			Error: ErrorDetail{
				Message: msg,
				Type:    errType,
			},
		}
	}

	// 透传响应头（过滤 hop-by-hop headers）
	for k, vv := range resp.Header {
		if hopByHopHeaders[k] {
			continue
		}
		for _, v := range vv {
			ctx.W.Header().Add(k, v)
		}
	}
	ctx.W.WriteHeader(resp.StatusCode)
	ctx.W.Write(body)

	// 尝试解析 usage
	var antResp AnthropicResponse
	if json.Unmarshal(body, &antResp) == nil && antResp.Usage.InputTokens+antResp.Usage.OutputTokens > 0 {
		return &Usage{
			PromptTokens:     antResp.Usage.InputTokens + antResp.Usage.CacheCreationInputTokens + antResp.Usage.CacheReadInputTokens,
			CompletionTokens: antResp.Usage.OutputTokens,
			TotalTokens:      antResp.Usage.InputTokens + antResp.Usage.CacheCreationInputTokens + antResp.Usage.CacheReadInputTokens + antResp.Usage.OutputTokens,
			PromptTokensDetails: PromptTokensDetails{
				CachedTokens: antResp.Usage.CacheReadInputTokens,
			},
		}, nil
	}
	return nil, nil
}

func (a *AnthropicAdaptor) handleStream(ctx *RelayContext, resp *http.Response) (*Usage, *ErrorWithStatusCode) {
	flusher, ok := ctx.W.(http.Flusher)
	if !ok {
		return nil, &ErrorWithStatusCode{
			StatusCode: http.StatusInternalServerError,
			Error: ErrorDetail{
				Message: "streaming response writer must implement http.Flusher",
				Type:    "server_error",
			},
		}
	}

	ctx.W.Header().Set("Content-Type", "text/event-stream")
	ctx.W.Header().Set("Cache-Control", "no-cache")
	ctx.W.Header().Set("Connection", "keep-alive")

	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 0, protocol.DefaultSSEBufferSize), protocol.DefaultSSEBufferSize)

	usage := &Usage{}
	for scanner.Scan() {
		line := strings.TrimRight(scanner.Text(), "\r")
		fmt.Fprintln(ctx.W, line)
		flusher.Flush()

		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		data := strings.TrimPrefix(line, "data: ")

		var evt anthropicStreamEvent
		if json.Unmarshal([]byte(data), &evt) != nil {
			continue
		}

		if evt.Message != nil && evt.Message.Usage != nil {
			usage.PromptTokens = evt.Message.Usage.InputTokens +
				evt.Message.Usage.CacheCreationInputTokens +
				evt.Message.Usage.CacheReadInputTokens
			usage.PromptTokensDetails.CachedTokens = evt.Message.Usage.CacheReadInputTokens
		}
		if evt.Usage != nil {
			// message_delta 可能修订输入侧的 cache tokens，重新汇总。
			usage.CompletionTokens = evt.Usage.OutputTokens
			if evt.Message != nil && evt.Message.Usage != nil {
				usage.PromptTokens = evt.Message.Usage.InputTokens +
					evt.Message.Usage.CacheCreationInputTokens +
					evt.Message.Usage.CacheReadInputTokens
				usage.PromptTokensDetails.CachedTokens = evt.Message.Usage.CacheReadInputTokens
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, &ErrorWithStatusCode{
			StatusCode: http.StatusBadGateway,
			Error: ErrorDetail{
				Message: fmt.Sprintf("stream read error: %v", err),
				Type:    "upstream_error",
			},
		}
	}

	usage.TotalTokens = usage.PromptTokens + usage.CompletionTokens
	return usage, nil
}

func (a *AnthropicAdaptor) GetProviderName() string {
	return "anthropic"
}
