package relay

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"

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
	default:
		return &OpenAIAdaptor{}
	}
}

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
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
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
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
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
func readResponse(resp *http.Response) ([]byte, error) {
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, &scheduler.UpstreamError{
			Status:  resp.StatusCode,
			Message: string(respBody),
		}
	}
	return io.ReadAll(resp.Body)
}

// openStream 把响应封装成 scheduler.Stream。4xx/5xx 转成 *scheduler.UpstreamError。
func openStream(ctx context.Context, resp *http.Response) (scheduler.Stream, error) {
	if resp.StatusCode >= 400 {
		defer resp.Body.Close()
		respBody, _ := io.ReadAll(resp.Body)
		return nil, &scheduler.UpstreamError{
			Status:  resp.StatusCode,
			Message: string(respBody),
		}
	}
	return &sseStream{
		reader: bufio.NewReader(resp.Body),
		body:   resp.Body,
		ctx:    ctx,
	}, nil
}

// sseStream 把 HTTP response body 的 SSE 流逐行透传成 scheduler.Stream。
//
// 设计要点:Data 携带「原始行字节(含换行)」而非解析后的 payload。
// 原因:上游可能是 Anthropic 风格 SSE(依赖 `event:` 行)或 OpenAI 风格(`data:` + `[DONE]`)。
// 若在这里剥掉前缀只留 data,同协议 Anthropic 透传会丢失 `event:` 行导致客户端协议损坏;
// 跨协议转换器(StreamXxx)也需要原始字节流作为输入。因此这里只做"逐行切分 + ctx 取消",
// 不做任何协议语义解析,把 framing 留给上层(直接透传或喂给转换器)。
type sseStream struct {
	reader *bufio.Reader
	body   io.ReadCloser
	ctx    context.Context
	idx    int
	closed bool
}

func (s *sseStream) Next(ctx context.Context) (*scheduler.StreamChunk, error) {
	if s.closed {
		return nil, scheduler.ErrStreamClosed
	}
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
			// 末尾可能有不带换行的残留字节,先发出再结束。
			if len(line) > 0 {
				idx := s.idx
				s.idx++
				return &scheduler.StreamChunk{Data: line, Index: idx}, nil
			}
			return &scheduler.StreamChunk{Index: s.idx, IsFinal: true}, nil
		}
		return nil, fmt.Errorf("read sse: %w", err)
	}
	idx := s.idx
	s.idx++
	return &scheduler.StreamChunk{Data: line, Index: idx}, nil
}

func (s *sseStream) Close() error {
	if s.closed {
		return nil
	}
	s.closed = true
	if s.body != nil {
		return s.body.Close()
	}
	return nil
}
