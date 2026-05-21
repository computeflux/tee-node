package relay

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
)

// ── OpenAIAdaptor ────────────────────────────────────────

type OpenAIAdaptor struct{}

func (a *OpenAIAdaptor) Init(meta *Meta) {}

func (a *OpenAIAdaptor) GetRequestURL(meta *Meta) (string, error) {
	return meta.BaseURL + meta.RequestURLPath, nil
}

func (a *OpenAIAdaptor) SetupRequestHeader(ctx *RelayContext, req *http.Request, meta *Meta) error {
	req.Header.Set("Authorization", "Bearer "+meta.APIKey)
	req.Header.Set("Content-Type", "application/json")
	return nil
}

func (a *OpenAIAdaptor) DoRequest(ctx *RelayContext, meta *Meta, body io.Reader) (*http.Response, error) {
	url, err := a.GetRequestURL(meta)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx.Context(), "POST", url, body)
	if err != nil {
		return nil, err
	}

	a.SetupRequestHeader(ctx, req, meta)

	return DoHTTP(req)
}

func (a *OpenAIAdaptor) DoResponse(ctx *RelayContext, resp *http.Response, meta *Meta) (*Usage, *ErrorWithStatusCode) {
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

	// 判断是否流式
	contentType := resp.Header.Get("Content-Type")
	if strings.Contains(contentType, "text/event-stream") || meta.IsStream {
		return a.handleStream(ctx, resp)
	}
	return a.handleNormal(ctx, resp)
}

// handleStream 处理 SSE 流式响应
func (a *OpenAIAdaptor) handleStream(ctx *RelayContext, resp *http.Response) (*Usage, *ErrorWithStatusCode) {
	flusher, ok := ctx.W.(http.Flusher)
	if !ok {
		return nil, &ErrorWithStatusCode{StatusCode: 500, Error: ErrorDetail{Message: "streaming not supported"}}
	}

	ctx.W.Header().Set("Content-Type", "text/event-stream")
	ctx.W.Header().Set("Cache-Control", "no-cache")
	ctx.W.Header().Set("Connection", "keep-alive")

	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 0, 64*1024), 64*1024)

	totalTokens := 0
	for scanner.Scan() {
		line := scanner.Text()
		// 写入客户端
		fmt.Fprintf(ctx.W, "%s\n", line)
		flusher.Flush()

		// 解析 usage（OpenAI 在最后一条 data: [DONE] 之前的 chunk 中包含 usage）
		if strings.HasPrefix(line, "data: ") && !strings.Contains(line, "[DONE]") {
			data := strings.TrimPrefix(line, "data: ")
			var chunk struct {
				Usage *Usage `json:"usage,omitempty"`
			}
			if json.Unmarshal([]byte(data), &chunk) == nil && chunk.Usage != nil {
				totalTokens = chunk.Usage.TotalTokens
			}
		}
	}

	return &Usage{TotalTokens: totalTokens}, nil
}

// handleNormal 处理普通（非流式）响应
func (a *OpenAIAdaptor) handleNormal(ctx *RelayContext, resp *http.Response) (*Usage, *ErrorWithStatusCode) {
	// 复制 headers
	for k, vs := range resp.Header {
		for _, v := range vs {
			ctx.W.Header().Add(k, v)
		}
	}
	ctx.W.WriteHeader(resp.StatusCode)

	// 读取完整响应体
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, &ErrorWithStatusCode{StatusCode: 500, Error: ErrorDetail{Message: "failed to read response"}}
	}

	// 解析 usage
	var result struct {
		Usage *Usage `json:"usage,omitempty"`
	}
	var usage *Usage
	if json.Unmarshal(body, &result) == nil {
		usage = result.Usage
	}

	// 写回客户端
	ctx.W.Write(body)

	return usage, nil
}

func (a *OpenAIAdaptor) GetModelList() []string {
	return []string{"gpt-4", "gpt-3.5-turbo"}
}

func (a *OpenAIAdaptor) GetChannelName() string {
	return "openai"
}

func init() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)
}
