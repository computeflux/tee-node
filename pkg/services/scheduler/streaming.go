package scheduler

import (
	"context"
	"errors"
	"sync"
	"time"
)

// StreamChunk 是流式响应的单个数据片。
// Provider 实现者负责把 SSE / chunked HTTP response 转成这个统一结构。
type StreamChunk struct {
	Data    []byte        // 原始字节(通常是 SSE data: 字段去掉前缀)
	Index   int           // 序号(0 开始)
	IsFinal bool          // 最后一个 chunk
	Latency time.Duration // 自请求开始到本 chunk 到达的时间
}

// Stream 是 provider 实现的流接口。
// Scheduler 通过它做统一的 streaming 编排。
type Stream interface {
	// Next 读取下一块。ctx 取消时应立即返回。
	Next(ctx context.Context) (*StreamChunk, error)
	// Close 释放资源(provider 关闭连接等)。
	Close() error
}

// instrumentedStream 给 Stream 加一层 instrumentation:
//   - 记录首个 chunk 的 TTFT
//   - 提供可靠的 ctx 取消传播
type instrumentedStream struct {
	inner        Stream
	startTime    time.Time
	firstChunk   bool
	ttftReported bool
	ep           *Endpoint
	model        ModelKey
	obs          *Observability
	mu           sync.Mutex
	closed       bool
}

func newInstrumentedStream(inner Stream, ep *Endpoint, model ModelKey, obs *Observability) *instrumentedStream {
	return &instrumentedStream{
		inner:      inner,
		startTime:  time.Now(),
		firstChunk: true,
		ep:         ep,
		model:      model,
		obs:        obs,
	}
}

func (s *instrumentedStream) Next(ctx context.Context) (*StreamChunk, error) {
	chunk, err := s.inner.Next(ctx)
	if err != nil {
		return chunk, err
	}
	if chunk == nil {
		return nil, nil
	}

	s.mu.Lock()
	if s.firstChunk && !s.ttftReported {
		s.firstChunk = false
		s.ttftReported = true
		ttft := time.Since(s.startTime)
		s.mu.Unlock()
		if s.obs != nil {
			s.obs.RecordTTFT(s.ep.Provider, s.model, ttft)
		}
	} else {
		s.mu.Unlock()
	}
	return chunk, nil
}

func (s *instrumentedStream) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return nil
	}
	s.closed = true
	return s.inner.Close()
}

// ErrStreamClosed 在 stream 已关闭后调用 Next 时返回。
var ErrStreamClosed = errors.New("stream closed")
