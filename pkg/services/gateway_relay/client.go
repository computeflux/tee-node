package relay

import (
	"bufio"
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/computeflux/tee-provider/pkg/services/scheduler"
	"github.com/computeflux/tee-provider/pkg/util"
	cgateway "github.com/computeflux/tee-provider/tee-chain/runtime/module/gateway"
)

// GetProviderClient 按合约 ProviderType 返回对应的 scheduler.ProviderClient。
// 命名与 reloader.providerTypeName 输出保持一致(由调用方注册到 scheduler)。
func GetProviderClient(providerType uint16) scheduler.ProviderClient {
	switch providerType {
	case cgateway.Anthropic:
		return &AnthropicAdaptor{}
	case cgateway.Gemini:
		return &GeminiAdaptor{}
	case cgateway.OpenAIResponsesApi:
		return &OpenAIResponsesApiAdaptor{}
	case scheduler.EdgeProviderType:
		return &LocalAdaptor{}
	case cgateway.OpenAI:
		return &OpenAIAdaptor{}
	default:
		// 未知 ProviderType 不再静默兜底到 OpenAI，返回 nil 让调用方/调度器拒绝该端点，
		// 避免协议错配导致数据污染或错误计费。
		return nil
	}
}

// 包级专用 clients，按「是否流式」分叉超时策略(避免依赖 http.DefaultClient 的零超时默认值)。
//
//   - doHTTPClient（非流式 httpDo 用）：整体 Timeout 放宽到 5min，禁用 ResponseHeaderTimeout，
//     容纳长 prompt/长 output 的整体响应生成时间。LLM 在生成完整响应前通常不发任何 header，
//     启用 ResponseHeaderTimeout 会被 "timeout awaiting response headers" 误杀。
//
//   - streamHTTPClient（流式 httpStream 用）：不设 client-level Timeout（流可能跑很久，
//     生命周期由调用方 ctx 决定）；保留 90s ResponseHeaderTimeout 作为 TTFT 兜底，
//     防止上游永远不发第一个字节。
//
// 两份 Transport 共享同一组网络层超时(Dial/TLS/Idle/KeepAlive)；HTTP/2 仍全局禁用。
func newTransport(headerTimeout time.Duration) *http.Transport {
	return &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			Timeout:   10 * time.Second,
			KeepAlive: 120 * time.Second,
		}).DialContext,
		TLSHandshakeTimeout:   10 * time.Second,
		ResponseHeaderTimeout: headerTimeout,
		ExpectContinueTimeout: 1 * time.Second,
		IdleConnTimeout:       90 * time.Second,
		MaxIdleConns:          100,
		// 部分后端对 HTTP/2 支持不佳(本地模型、某些代理)，全局禁用 h2，强制 HTTP/1.1。
		// TLSNextProto 为空 map 可彻底阻止 ALPN 协商到 h2；ForceAttemptHTTP2 防显式回退。
		ForceAttemptHTTP2: false,
		TLSNextProto:      map[string]func(authority string, c *tls.Conn) http.RoundTripper{},
	}
}

var doHTTPClient = &http.Client{
	Timeout:   300 * time.Second,
	Transport: newTransport(0), // 0 = disable, 由整体 Timeout 兜底
}

var streamHTTPClient = &http.Client{
	// 不设 Timeout：流式响应可能持续数分钟，由 ctx 控制生命周期。
	Transport: newTransport(90 * time.Second),
}

// 错误响应 body 截断上限，避免 OOM 与日志爆炸。
const upstreamErrorBodyLimit = 1 << 20 // 1 MiB

// SSE 单行最大字节数，防御恶意/异常上游发送超长单行撑爆 bufio。
const sseMaxLineBytes = 1 << 20 // 1 MiB

// httpDo 构造 POST 请求、发送、读取非流式响应。
// >=400 的响应转成 *scheduler.UpstreamError 供调度器分类重试。
func httpDo(ctx context.Context, ep *scheduler.Endpoint, url string, body []byte, setHeaders func(*http.Request)) ([]byte, error) {
	req, err := newPostRequest(ctx, url, body)
	if err != nil {
		return nil, err
	}
	if setHeaders != nil {
		setHeaders(req)
	}

	resp, err := doHTTPClient.Do(req)
	if err != nil {
		util.LogErrorf("[relay] httpDo transport error endpoint=%s url=%s err=%v", endpointLabel(ep), url, err)
		return nil, err
	}
	return readResponse(resp, ep)
}

// httpStream 构造 POST 请求、发送、打开 SSE 流。
// >=400 的响应转成 *scheduler.UpstreamError。
func httpStream(ctx context.Context, ep *scheduler.Endpoint, url string, body []byte, setHeaders func(*http.Request)) (scheduler.Stream, error) {
	req, err := newPostRequest(ctx, url, body)
	if err != nil {
		return nil, err
	}
	if setHeaders != nil {
		setHeaders(req)
	}

	resp, err := streamHTTPClient.Do(req)
	if err != nil {
		util.LogErrorf("[relay] httpStream transport error endpoint=%s url=%s err=%v", endpointLabel(ep), url, err)
		return nil, err
	}
	return openStream(ctx, resp, ep)
}

// newPostRequest 构造一个 POST 请求(JSON body)。
func newPostRequest(ctx context.Context, url string, body []byte) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	return req, nil
}

// readResponse 读非流式响应。4xx/5xx 转成 *scheduler.UpstreamError 让调度器分类重试。
// 错误响应 body 截断至 upstreamErrorBodyLimit 以避免恶意上游或异常 body 撑爆内存。
func readResponse(resp *http.Response, ep *scheduler.Endpoint) ([]byte, error) {
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		respBody, readErr := io.ReadAll(io.LimitReader(resp.Body, upstreamErrorBodyLimit+1))
		truncated := ""
		if len(respBody) > upstreamErrorBodyLimit {
			respBody = respBody[:upstreamErrorBodyLimit]
			truncated = "...[truncated]"
		}
		msg := string(respBody) + truncated
		if readErr != nil {
			msg += "; read_error=" + readErr.Error()
		}
		logUpstreamError(resp, ep, msg)
		return nil, &scheduler.UpstreamError{
			Status:  resp.StatusCode,
			Message: msg,
		}
	}
	respBody, err := io.ReadAll(io.LimitReader(resp.Body, upstreamErrorBodyLimit+1))
	if err != nil {
		return nil, fmt.Errorf("read upstream response: %w", err)
	}
	if len(respBody) > upstreamErrorBodyLimit {
		// 成功响应也超限：静默截断会让调用方拿到损坏的 JSON 且错误难以定位，
		// 这里显式报错而不是返回截断数据。
		return nil, &scheduler.UpstreamError{
			Status:  resp.StatusCode,
			Message: fmt.Sprintf("upstream success response exceeds %d bytes limit", upstreamErrorBodyLimit),
		}
	}
	return respBody, nil
}

// openStream 把响应封装成 scheduler.Stream。4xx/5xx 转成 *scheduler.UpstreamError。
func openStream(ctx context.Context, resp *http.Response, ep *scheduler.Endpoint) (scheduler.Stream, error) {
	if resp.StatusCode >= 400 {
		defer resp.Body.Close()
		respBody, readErr := io.ReadAll(io.LimitReader(resp.Body, upstreamErrorBodyLimit+1))
		truncated := ""
		if len(respBody) > upstreamErrorBodyLimit {
			respBody = respBody[:upstreamErrorBodyLimit]
			truncated = "...[truncated]"
		}
		msg := string(respBody) + truncated
		if readErr != nil {
			msg += "; read_error=" + readErr.Error()
		}
		logUpstreamError(resp, ep, msg)
		return nil, &scheduler.UpstreamError{
			Status:  resp.StatusCode,
			Message: msg,
		}
	}
	s := &sseStream{
		reader: bufio.NewReaderSize(resp.Body, 64*1024),
		body:   resp.Body,
		ctx:    ctx,
	}
	// ctx 取消时主动关闭 body，使 Next 中阻塞在 ReadBytes 的 goroutine 立即被唤醒并返回。
	if ctx != nil {
		go func() {
			<-ctx.Done()
			s.Close()
		}()
	}
	return s, nil
}

// logUpstreamError 记录上游错误详情，格式分三段：
//
//	endpoint 身份 | 实际发出的请求 | 上游响应
//
// 只提取 model 字段而非整个请求体，避免把用户 prompt 等敏感内容打进日志；
// KeyValue(API Key 明文)绝不打印，只打印 KeyID。
func logUpstreamError(resp *http.Response, ep *scheduler.Endpoint, msg string) {
	url, bodyModel := "", ""
	if resp.Request != nil {
		url = resp.Request.URL.String()
		if resp.Request.GetBody != nil {
			rb, err := resp.Request.GetBody()
			if err == nil {
				defer rb.Close()
				if b, rerr := io.ReadAll(io.LimitReader(rb, 64<<10)); rerr == nil {
					bodyModel = modelFromBody(b)
				}
			}
		}
	}

	req := fmt.Sprintf("url=%s", url)
	if ep != nil && ep.ModelName != "" && ep.ModelName != bodyModel {
		req += fmt.Sprintf(" model=%q body_model=%q", ep.ModelName, bodyModel)
	} else if bodyModel != "" {
		req += fmt.Sprintf(" model=%q", bodyModel)
	}

	util.LogErrorf("[relay] upstream error: %s | %s | status=%d resp=%s",
		endpointLabel(ep), req, resp.StatusCode, truncateLog(msg, 1024))
}

// endpointLabel 生成日志用的 endpoint 身份标识（不含 API Key 明文）。
func endpointLabel(ep *scheduler.Endpoint) string {
	if ep == nil {
		return "endpoint=nil"
	}
	return fmt.Sprintf("provider=%s provider_id=%d model_id=%d key=%s route=%d proto=%s urls=%v",
		ep.Provider, ep.ContractProviderID, ep.ModelID, ep.KeyID, ep.RouteID, scheduler.ClientTypeName(ep.NativeProtocol()), ep.ModelUrls)
}

// truncateLog 截断过长的日志字段，防止上游异常响应撑爆单行日志。
func truncateLog(s string, limit int) string {
	if len(s) <= limit {
		return s
	}
	return s[:limit] + fmt.Sprintf("...[truncated %d bytes]", len(s))
}

// modelFromBody 从 JSON 请求体提取 model 字段（缺失或非 JSON 时返回空串）。
func modelFromBody(b []byte) string {
	var doc struct {
		Model string `json:"model"`
	}
	if err := json.Unmarshal(b, &doc); err != nil {
		return ""
	}
	return doc.Model
}

// sseStream 把 HTTP response body 的 SSE 流逐行透传成 scheduler.Stream。
//
// 设计要点:Data 携带「原始行字节(含换行)」而非解析后的 payload。
// 原因:上游可能是 Anthropic 风格 SSE(依赖 `event:` 行)或 OpenAI 风格(`data:` + `[DONE]`)。
// 若在这里剥掉前缀只留 data，同协议 Anthropic 透传会丢失 `event:` 行导致客户端协议损坏;
// 跨协议转换器(StreamXxx)也需要原始字节流作为输入。因此这里只做"逐行切分 + ctx 取消"，
// 不做任何协议语义解析，把 framing 留给上层(直接透传或喂给转换器)。
type sseStream struct {
	reader *bufio.Reader
	body   io.ReadCloser
	ctx    context.Context
	mu     sync.Mutex
	idx    int
	closed bool
}

func (s *sseStream) Next(ctx context.Context) (*scheduler.StreamChunk, error) {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return nil, scheduler.ErrStreamClosed
	}
	s.mu.Unlock()

	// 优先用 caller 的 ctx(覆盖 stream ctx)
	select {
	case <-ctx.Done():
		s.Close()
		return nil, ctx.Err()
	default:
	}
	if s.ctx != nil {
		select {
		case <-s.ctx.Done():
			s.Close()
			return nil, s.ctx.Err()
		default:
		}
	}

	line, err := s.readLine()
	if err != nil {
		if errors.Is(err, io.EOF) {
			s.Close()
			// 末尾可能有不带换行的残留字节，先发出再结束。
			if len(line) > 0 {
				idx := s.idx
				s.idx++
				return &scheduler.StreamChunk{Data: line, Index: idx}, nil
			}
			idx := s.idx
			s.idx++
			return &scheduler.StreamChunk{Index: idx, IsFinal: true}, nil
		}
		return nil, fmt.Errorf("read sse: %w", err)
	}
	idx := s.idx
	s.idx++
	return &scheduler.StreamChunk{Data: line, Index: idx}, nil
}

// readLine 读取一行 SSE 数据（含换行符）。
//
// 用 ReadSlice 按 64KB 缓冲分片累积，每片后检查总长度：超过 sseMaxLineBytes 的
// 畸形单行直接报错，避免恶意/异常上游发送超长单行导致内存无限增长（bufio.ReadBytes
// 会把整行累积到内存里，无上限）。
func (s *sseStream) readLine() ([]byte, error) {
	var line []byte
	for {
		frag, err := s.reader.ReadSlice('\n')
		line = append(line, frag...)
		if len(line) > sseMaxLineBytes {
			return nil, fmt.Errorf("sse line exceeds %d bytes limit", sseMaxLineBytes)
		}
		if err == nil {
			return line, nil
		}
		if errors.Is(err, bufio.ErrBufferFull) {
			continue
		}
		// EOF 或底层错误：返回已累积的残留字节
		return line, err
	}
}

func (s *sseStream) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return nil
	}
	s.closed = true
	if s.body != nil {
		return s.body.Close()
	}
	return nil
}

// ── Types ─────────────────────────────────────────────────
type Usage struct {
	PromptTokens        int                 `json:"prompt_tokens"`
	CompletionTokens    int                 `json:"completion_tokens"`
	TotalTokens         int                 `json:"total_tokens"`
	PromptTokensDetails PromptTokensDetails `json:"prompt_tokens_details,omitempty"`
}

type PromptTokensDetails struct {
	CachedTokens int `json:"cached_tokens"`
}
