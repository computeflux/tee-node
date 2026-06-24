package relay

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// ── Gemini response types（仅用于解析 usage / error）────────────────

type geminiResponseLite struct {
	Candidates []struct {
		Content struct {
			Parts []struct {
				Text string `json:"text"`
			} `json:"parts"`
		} `json:"content"`
		FinishReason string `json:"finishReason"`
	} `json:"candidates"`
	UsageMetadata *struct {
		PromptTokenCount        int `json:"promptTokenCount"`
		CandidatesTokenCount    int `json:"candidatesTokenCount"`
		TotalTokenCount         int `json:"totalTokenCount"`
		CachedContentTokenCount int `json:"cachedContentTokenCount"`
	} `json:"usageMetadata"`
	Error *struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

// geminiStreamEvent 解析 Gemini SSE 流中的事件
type geminiStreamEventLite struct {
	Candidates []struct {
		Content struct {
			Parts []struct {
				Text string `json:"text"`
			} `json:"parts"`
		} `json:"content"`
	} `json:"candidates"`
	UsageMetadata *struct {
		PromptTokenCount        int `json:"promptTokenCount"`
		CandidatesTokenCount    int `json:"candidatesTokenCount"`
		TotalTokenCount         int `json:"totalTokenCount"`
		CachedContentTokenCount int `json:"cachedContentTokenCount"`
	} `json:"usageMetadata"`
}

// ── GeminiAdaptor ────────────────────────────────────────────────
// GeminiAdaptor 以 Gemini 原生格式与上游通信：请求体/响应体均直接透传，
// 协议转换统一在 gateway/protocol 层完成。
type GeminiAdaptor struct{}

func (a *GeminiAdaptor) Init(meta *Meta) {}

func (a *GeminiAdaptor) GetRequestURL(meta *Meta) (string, error) {
	action := "generateContent"
	if meta.IsStream {
		action = "streamGenerateContent?alt=sse"
	}
	base := strings.TrimRight(meta.BaseURL, "/")
	return fmt.Sprintf("%s/v1beta/models/%s:%s", base, meta.OriginModelName, action), nil
}

func (a *GeminiAdaptor) SetupRequestHeader(ctx *RelayContext, req *http.Request, meta *Meta) error {
	forwardClientHeaders(ctx, req)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-goog-api-key", meta.APIKey)
	return nil
}

func (a *GeminiAdaptor) DoRequest(ctx *RelayContext, meta *Meta, body io.Reader) (*http.Response, error) {
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

func (a *GeminiAdaptor) DoResponse(ctx *RelayContext, resp *http.Response, meta *Meta) (*Usage, *ErrorWithStatusCode) {
	if meta.IsStream {
		return a.handleStream(ctx, resp)
	}
	return a.handleNormal(ctx, resp)
}

func (a *GeminiAdaptor) handleNormal(ctx *RelayContext, resp *http.Response) (*Usage, *ErrorWithStatusCode) {
	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		msg := string(body)
		errType := "upstream_error"
		if msg == "" {
			msg = http.StatusText(resp.StatusCode)
		}
		var gemResp geminiResponseLite
		if json.Unmarshal(body, &gemResp) == nil && gemResp.Error != nil {
			if gemResp.Error.Message != "" {
				msg = gemResp.Error.Message
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
	var gemResp geminiResponseLite
	if json.Unmarshal(body, &gemResp) == nil && gemResp.UsageMetadata != nil {
		return &Usage{
			PromptTokens:     gemResp.UsageMetadata.PromptTokenCount,
			CompletionTokens: gemResp.UsageMetadata.CandidatesTokenCount,
			TotalTokens:      gemResp.UsageMetadata.TotalTokenCount,
			PromptTokensDetails: PromptTokensDetails{
				CachedTokens: gemResp.UsageMetadata.CachedContentTokenCount,
			},
		}, nil
	}
	return nil, nil
}

func (a *GeminiAdaptor) handleStream(ctx *RelayContext, resp *http.Response) (*Usage, *ErrorWithStatusCode) {
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
	scanner.Buffer(make([]byte, 0, 64*1024), 64*1024)

	usage := &Usage{}
	for scanner.Scan() {
		line := strings.TrimRight(scanner.Text(), "\r")
		fmt.Fprintln(ctx.W, line)
		flusher.Flush()

		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		data := strings.TrimPrefix(line, "data: ")

		var evt geminiStreamEventLite
		if json.Unmarshal([]byte(data), &evt) != nil {
			continue
		}
		if evt.UsageMetadata != nil {
			usage.PromptTokens = evt.UsageMetadata.PromptTokenCount
			usage.CompletionTokens = evt.UsageMetadata.CandidatesTokenCount
			usage.TotalTokens = evt.UsageMetadata.TotalTokenCount
			usage.PromptTokensDetails = PromptTokensDetails{
				CachedTokens: evt.UsageMetadata.CachedContentTokenCount,
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

	return usage, nil
}

func (a *GeminiAdaptor) GetProviderName() string {
	return "gemini"
}
