package scheduler

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"

	relay "github.com/computeflux/tee-provider/pkg/services/gateway_relay"
)

// AdaptorClient 把现有的 relay.Adaptor 包装成 scheduler.ProviderClient。
// 复用 OpenAI/Anthropic/Gemini/Local 的 HTTP 实现,不需要重新写 HTTP 层。
type AdaptorClient struct {
	adaptor relay.Adaptor
}

// NewAdaptorClient 创建 AdaptorClient。
func NewAdaptorClient(a relay.Adaptor) *AdaptorClient {
	return &AdaptorClient{adaptor: a}
}

// Do 非流式调用。
func (c *AdaptorClient) Do(ctx context.Context, ep *Endpoint, body []byte) ([]byte, error) {
	meta := &relay.Meta{
		BaseURL:  ep.BaseURL,
		APIKey:   ep.KeyValue,
		IsStream: false,
	}

	// Adaptor 用 RelayContext 传 ctx + response writer。
	// 这里我们跳过 ResponseWriter——Adaptor 的 DoResponse 才会写。
	// 用一个空 ResponseWriter 即可,DoRequest 阶段不会写。
	relayCtx := &relay.RelayContext{
		W: nopResponseWriter{},
		R: (&http.Request{}).WithContext(ctx),
	}

	resp, err := c.adaptor.DoRequest(relayCtx, meta, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("upstream: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, &UpstreamError{
			Status:  resp.StatusCode,
			Message: string(respBody),
		}
	}

	// Adaptor 的 DoResponse 会写 W(我们这里是 nop),所以我们不能调用它。
	// 直接读 body 即可。
	return io.ReadAll(resp.Body)
}

// Stream 流式调用。返回 scheduler.Stream 让上层按 chunk 读。
func (c *AdaptorClient) Stream(ctx context.Context, ep *Endpoint, body []byte) (Stream, error) {
	meta := &relay.Meta{
		BaseURL:  ep.BaseURL,
		APIKey:   ep.KeyValue,
		IsStream: true,
	}

	relayCtx := &relay.RelayContext{
		W: nopResponseWriter{},
		R: (&http.Request{}).WithContext(ctx),
	}

	resp, err := c.adaptor.DoRequest(relayCtx, meta, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("upstream: %w", err)
	}

	if resp.StatusCode >= 400 {
		defer resp.Body.Close()
		respBody, _ := io.ReadAll(resp.Body)
		return nil, &UpstreamError{
			Status:  resp.StatusCode,
			Message: string(respBody),
		}
	}

	// Adaptor 的 handleStream 会写 W,我们直接读 resp.Body 就行。
	return &sseStream{
		reader: bufio.NewReader(resp.Body),
		body:   resp.Body,
		ctx:    ctx,
	}, nil
}

// sseStream 把 HTTP response body 的 SSE 流解析成 scheduler.Stream。
type sseStream struct {
	reader *bufio.Reader
	body   io.ReadCloser
	ctx    context.Context
	idx    int
	closed bool
}

func (s *sseStream) Next(ctx context.Context) (*StreamChunk, error) {
	if s.closed {
		return nil, ErrStreamClosed
	}
	for {
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
				return &StreamChunk{Index: s.idx, IsFinal: true}, nil
			}
			return nil, fmt.Errorf("read sse: %w", err)
		}
		line = bytes.TrimSpace(line)
		if len(line) == 0 || !bytes.HasPrefix(line, []byte("data:")) {
			continue
		}
		payload := bytes.TrimSpace(line[5:])
		if bytes.Equal(payload, []byte("[DONE]")) {
			s.Close()
			return &StreamChunk{Index: s.idx, IsFinal: true}, nil
		}

		idx := s.idx
		s.idx++
		return &StreamChunk{
			Data:  payload,
			Index: idx,
		}, nil
	}
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

// nopResponseWriter 是一个丢弃所有写入的 http.ResponseWriter。
// 用来满足 relay.RelayContext 的 W 字段,但不真正输出(由 HandleRelay 负责)。
type nopResponseWriter struct{}

func (nopResponseWriter) Header() http.Header         { return http.Header{} }
func (nopResponseWriter) Write(b []byte) (int, error) { return len(b), nil }
func (nopResponseWriter) WriteHeader(int)             {}
