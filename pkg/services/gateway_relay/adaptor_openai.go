package relay

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// ── OpenAIAdaptor ────────────────────────────────────────

type OpenAIAdaptor struct{}

func (a *OpenAIAdaptor) Init(meta *Meta) {}

func (a *OpenAIAdaptor) GetRequestURL(meta *Meta) (string, error) {
	base := strings.TrimRight(meta.BaseURL, "/")
	path := meta.RequestURLPath

	// 去重：如果 base 末尾路径组件与 path 开头重复，去掉 base 中的重复部分
	// 例如 https://api.minimaxi.com/v1 + /v1/chat/completions
	//      → https://api.minimaxi.com/v1/chat/completions
	if path != "" {
		lastSeg := filepath.Base(base)
		firstSeg := strings.SplitN(strings.TrimPrefix(path, "/"), "/", 2)[0]
		if lastSeg == firstSeg && lastSeg != "" {
			base = strings.TrimSuffix(base, "/"+lastSeg)
		}
	}

	return base + path, nil
}

func (a *OpenAIAdaptor) SetupRequestHeader(ctx *RelayContext, req *http.Request, meta *Meta) error {
	forwardClientHeaders(ctx, req)
	req.Header.Set("Authorization", "Bearer "+meta.APIKey)
	req.Header.Set("Content-Type", "application/json")
	return nil
}

func (a *OpenAIAdaptor) DoRequest(ctx *RelayContext, meta *Meta, body io.Reader) (*http.Response, error) {
	url, err := a.GetRequestURL(meta)
	if err != nil {
		return nil, err
	}
	// util.DebugLog("[openai] do request url=%s", url)

	rawBody, _ := io.ReadAll(body)
	// util.DebugLog("[openai] request body len=%d", len(rawBody))
	previewLen := len(rawBody)
	if previewLen > 500 {
		previewLen = 500
	}
	// util.DebugLog("[openai] request body preview=%s", string(rawBody[:previewLen]))

	finalBody := rawBody
	requestBody := bytes.NewReader(rawBody)
	if meta.IsStream {
		var reqMap map[string]any
		if json.Unmarshal(rawBody, &reqMap) == nil {
			if _, ok := reqMap["stream_options"]; !ok {
				reqMap["stream_options"] = map[string]any{"include_usage": true}
				if newRaw, err := json.Marshal(reqMap); err == nil {
					finalBody = newRaw
					requestBody = bytes.NewReader(newRaw)
				}
				// util.DebugLog("[openai] added stream_options include_usage")
			}
		}
	}

	// 落盘最终实际发向上游请求体（仅带 tools 时），便于与模型原始返回做对比。
	if strings.Contains(string(finalBody), `"tools"`) {
		if dumpPath := dumpOpenAIRequest(finalBody, "json"); dumpPath != "" {
			log.Printf("[openai] dumped final request body to %s", dumpPath)
		}
	}
	log.Printf("[openai] request url=%s body=%d stream=%v", url, len(finalBody), meta.IsStream)

	req, err := http.NewRequestWithContext(ctx.Context(), "POST", url, requestBody)
	if err != nil {
		return nil, err
	}

	a.SetupRequestHeader(ctx, req, meta)
	// util.DebugLog("[openai] request headers=%v", redactAuthHeader(req.Header))

	return DoHTTP(req)
}

func (a *OpenAIAdaptor) DoResponse(ctx *RelayContext, resp *http.Response, meta *Meta) (*Usage, *ErrorWithStatusCode) {
	log.Printf("[openai] response status=%d content-type=%s stream=%v", resp.StatusCode, resp.Header.Get("Content-Type"), meta.IsStream)
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

	// 落盘原始 SSE 响应，便于排查模型实际返回的 tool_calls 参数。
	dumpFile, dumpPath := dumpOpenAIStreamResponse()
	if dumpFile != nil {
		defer dumpFile.Close()
		log.Printf("[openai] dumping stream response to %s", dumpPath)
	}

	usage := &Usage{}
	for scanner.Scan() {
		line := scanner.Text()
		// 写入客户端
		fmt.Fprintf(ctx.W, "%s\n", line)
		flusher.Flush()

		// 同步写入调试文件
		if dumpFile != nil {
			fmt.Fprintf(dumpFile, "%s\n", line)
		}

		// 解析 usage（OpenAI 在最后一条 data: [DONE] 之前的 chunk 中包含 usage）
		if strings.HasPrefix(line, "data: ") && !strings.Contains(line, "[DONE]") {
			data := strings.TrimPrefix(line, "data: ")
			var chunk struct {
				Usage *Usage `json:"usage,omitempty"`
			}
			if json.Unmarshal([]byte(data), &chunk) == nil && chunk.Usage != nil {
				usage = chunk.Usage
			}
		}
	}

	return usage, nil
}

// handleNormal 处理普通（非流式）响应
func (a *OpenAIAdaptor) handleNormal(ctx *RelayContext, resp *http.Response) (*Usage, *ErrorWithStatusCode) {
	// 复制 headers，跳过 hop-by-hop 及内容长度相关头部（后续可能进行协议转换，body 长度会变化）。
	for k, vs := range resp.Header {
		if hopByHopHeaders[k] {
			continue
		}
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

	// 落盘原始响应体，便于排查模型实际返回的 tool_calls 参数。
	if dumpPath := dumpOpenAIResponse(body, "json"); dumpPath != "" {
		log.Printf("[openai] dumped response body to %s", dumpPath)
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

// dumpOpenAIRequest 把最终发向上游请求体写入带时间戳的文件，便于与响应做对比。
func dumpOpenAIRequest(body []byte, ext string) string {
	candidates := []string{"/token_chain_data", "."}
	for _, dir := range candidates {
		if err := os.MkdirAll(dir, 0755); err != nil {
			continue
		}
		path := filepath.Join(dir, fmt.Sprintf("gateway_openai_request_final_%d.%s", time.Now().UnixMilli(), ext))
		if err := os.WriteFile(path, body, 0644); err == nil {
			return path
		}
	}
	return ""
}

// dumpOpenAIResponse 把非流式响应体写入带时间戳的文件，便于对比模型原始输出。
func dumpOpenAIResponse(body []byte, ext string) string {
	candidates := []string{"/token_chain_data", "."}
	for _, dir := range candidates {
		if err := os.MkdirAll(dir, 0755); err != nil {
			continue
		}
		path := filepath.Join(dir, fmt.Sprintf("gateway_openai_response_%d.%s", time.Now().UnixMilli(), ext))
		if err := os.WriteFile(path, body, 0644); err == nil {
			return path
		}
	}
	return ""
}

// dumpOpenAIStreamResponse 为流式响应创建落盘文件，返回打开的文件句柄和路径。
func dumpOpenAIStreamResponse() (*os.File, string) {
	candidates := []string{"/token_chain_data", "."}
	for _, dir := range candidates {
		if err := os.MkdirAll(dir, 0755); err != nil {
			continue
		}
		path := filepath.Join(dir, fmt.Sprintf("gateway_openai_response_%d.sse", time.Now().UnixMilli()))
		f, err := os.Create(path)
		if err == nil {
			return f, path
		}
	}
	return nil, ""
}

func (a *OpenAIAdaptor) GetProviderName() string {
	return "openai"
}

func init() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)
}
