package relay

import (
	"bufio"
	"bytes"
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/computeflux/tee-provider/pkg/services/scheduler"
	cgateway "github.com/computeflux/tee-provider/tee-chain/runtime/module/gateway"
)

// GetProviderClient 按合约 ProviderType 返回对应的 scheduler.ProviderClient。
// 命名与 reloader.providerTypeName 输出保持一致(由调用方注册到 scheduler)。
func GetProviderClient(providerType uint16) scheduler.ProviderClient {
	switch providerType {
	case cgateway.ProviderClaude:
		return &AnthropicAdaptor{}
	case cgateway.ProviderGemini:
		return &GeminiAdaptor{}
	case cgateway.ProviderPersonal:
		return &LocalAdaptor{}
	case cgateway.ProviderOpenAI:
		return &OpenAIAdaptor{}
	default:
		// 未知 ProviderType 不再静默兜底到 OpenAI，返回 nil 让调用方/调度器拒绝该端点，
		// 避免协议错配导致数据污染或错误计费。
		return nil
	}
}

// 包级专用 client，集中配置超时(避免依赖 http.DefaultClient 的零超时默认值)。
var defaultHTTPClient = &http.Client{
	Timeout: 120 * time.Second,
	Transport: &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			Timeout:   10 * time.Second,
			KeepAlive: 120 * time.Second,
		}).DialContext,
		TLSHandshakeTimeout:   10 * time.Second,
		ResponseHeaderTimeout: 90 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
		IdleConnTimeout:       90 * time.Second,
		MaxIdleConns:          100,
		// 部分后端对 HTTP/2 支持不佳(本地模型、某些代理)，全局禁用 h2，强制 HTTP/1.1。
		// TLSNextProto 为空 map 可彻底阻止 ALPN 协商到 h2；ForceAttemptHTTP2 防显式回退。
		ForceAttemptHTTP2: false,
		TLSNextProto:      map[string]func(authority string, c *tls.Conn) http.RoundTripper{},
	},
}

// 错误响应 body 截断上限，避免 OOM 与日志爆炸。
const upstreamErrorBodyLimit = 1 << 20 // 1 MiB

// SSE 单行最大字节数，防御恶意/异常上游发送超长单行撑爆 bufio。
const sseMaxLineBytes = 1 << 20 // 1 MiB

// httpDo 构造 POST 请求、发送、读取非流式响应。
// >=400 的响应转成 *scheduler.UpstreamError 供调度器分类重试。
func httpDo(ctx context.Context, url string, body []byte, setHeaders func(*http.Request)) ([]byte, error) {
	req, err := newPostRequest(ctx, url, body)
	if err != nil {
		return nil, err
	}
	if setHeaders != nil {
		setHeaders(req)
	}

	resp, err := defaultHTTPClient.Do(req)
	if err != nil {
		log.Printf("[relay] httpDo transport error url=%s err=%v", url, err)
		return nil, err
	}
	return readResponse(resp)
}

// httpStream 构造 POST 请求、发送、打开 SSE 流。
// >=400 的响应转成 *scheduler.UpstreamError。
func httpStream(ctx context.Context, url string, body []byte, setHeaders func(*http.Request)) (scheduler.Stream, error) {
	req, err := newPostRequest(ctx, url, body)
	if err != nil {
		return nil, err
	}
	if setHeaders != nil {
		setHeaders(req)
	}

	resp, err := defaultHTTPClient.Do(req)
	if err != nil {
		log.Printf("[relay] httpStream transport error url=%s err=%v", url, err)
		return nil, err
	}
	return openStream(ctx, resp)
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
func readResponse(resp *http.Response) ([]byte, error) {
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
		return nil, &scheduler.UpstreamError{
			Status:  resp.StatusCode,
			Message: msg,
		}
	}
	return io.ReadAll(io.LimitReader(resp.Body, upstreamErrorBodyLimit))
}

// openStream 把响应封装成 scheduler.Stream。4xx/5xx 转成 *scheduler.UpstreamError。
func openStream(ctx context.Context, resp *http.Response) (scheduler.Stream, error) {
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

	line, err := s.reader.ReadBytes('\n')
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
