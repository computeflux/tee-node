package localtunnel

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
)

// ForwarderConfig 转发器配置
type ForwarderConfig struct {
	RequestTimeout time.Duration // 请求总超时
	ReadTimeout    time.Duration // 读取响应超时
	MaxBodySize    int64         // 最大请求体大小
	MaxRetries     int           // 最大重试次数
}

var defaultForwarderConfig = ForwarderConfig{
	RequestTimeout: 120 * time.Second,
	ReadTimeout:    120 * time.Second,
	MaxBodySize:    MaxBodySize,
	MaxRetries:     2,
}

// ForwardMetrics 转发指标
type ForwardMetrics struct {
	TotalRequests  int64
	FailedRequests int64
	TotalLatency   int64 // 累计延迟 ms
}

var forwardMetrics ForwardMetrics

// ForwardRequest 通过 Yamux 反向隧道将 HTTP 请求转发给本地客户端
func ForwardRequest(ctx context.Context, providerID uint64, modelID string, reqBody io.Reader, path string, isStream bool, headers map[string]string) (*http.Response, error) {
	start := time.Now()
	defer func() {
		atomic.AddInt64(&forwardMetrics.TotalLatency, time.Since(start).Milliseconds())
	}()
	atomic.AddInt64(&forwardMetrics.TotalRequests, 1)

	pool := GetGlobalPool()

	// 尝试读取请求体（带大小限制）
	body, err := readBodyLimited(reqBody, defaultForwarderConfig.MaxBodySize)
	if err != nil {
		atomic.AddInt64(&forwardMetrics.FailedRequests, 1)
		pool.IncrRequests(false)
		return nil, fmt.Errorf("read request body: %w", err)
	}

	rk := routeKey(providerID, modelID)

	// 构建请求帧
	reqFrame := RequestFrame{
		ModelID: modelID,
		Path:    path,
		Method:  "POST",
		Headers: headers,
		Body:    body,
	}

	// 重试逻辑
	var lastErr error
	for attempt := 0; attempt <= defaultForwarderConfig.MaxRetries; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				atomic.AddInt64(&forwardMetrics.FailedRequests, 1)
				pool.IncrRequests(false)
				return nil, ctx.Err()
			case <-time.After(time.Duration(attempt) * 500 * time.Millisecond):
			}
		}

		resp, err := forwardOnce(ctx, pool, rk, reqFrame, isStream)
		if err == nil {
			pool.IncrRequests(true)
			return resp, nil
		}
		lastErr = err
	}

	atomic.AddInt64(&forwardMetrics.FailedRequests, 1)
	pool.IncrRequests(false)
	return nil, fmt.Errorf("forward failed after %d attempts: %w", defaultForwarderConfig.MaxRetries+1, lastErr)
}

func forwardOnce(ctx context.Context, pool *Pool, routeKey string, reqFrame RequestFrame, isStream bool) (*http.Response, error) {
	entry := pool.Pick(routeKey)
	if entry == nil {
		return nil, fmt.Errorf("no available local client for route=%s", routeKey)
	}

	atomic.AddInt32(&entry.InFlight, 1)
	defer atomic.AddInt32(&entry.InFlight, -1)

	// 在已有 session 上打开新 stream
	stream, err := entry.Session.OpenStream(ctx)
	if err != nil {
		return nil, fmt.Errorf("open yamux stream: %w", err)
	}

	// 发送请求帧
	if err := WriteJSONFrame(stream, FrameTypeRequest, reqFrame); err != nil {
		stream.Close()
		return nil, fmt.Errorf("write request frame: %w", err)
	}

	// 注册 waiter
	reqID := uuid.New().String()
	waiter := newResponseWaiter(reqID)
	waiterRegistryMu.Lock()
	waiterRegistry[reqID] = waiter
	waiterRegistryMu.Unlock()

	// 启动 goroutine 在 stream 上读取响应帧
	done := make(chan struct{})
	go func() {
		defer close(done)
		readResponseFrames(stream, reqID)
	}()

	// 等待响应头（支持 context 取消）
	select {
	case header := <-waiter.headerDone:
		// 成功：stream 生命周期交给 tunnelResponseBody.Close() 管理
		return buildHTTPResponse(waiter, header, isStream, stream), nil
	case errPayload := <-waiter.errChan:
		stream.Close()
		waiterRegistryMu.Lock()
		delete(waiterRegistry, reqID)
		waiterRegistryMu.Unlock()
		<-done
		return nil, fmt.Errorf("local client error: %s", errPayload.Message)
	case <-ctx.Done():
		stream.Close()
		waiterRegistryMu.Lock()
		delete(waiterRegistry, reqID)
		waiterRegistryMu.Unlock()
		<-done
		return nil, ctx.Err()
	case <-time.After(defaultForwarderConfig.RequestTimeout):
		stream.Close()
		waiterRegistryMu.Lock()
		delete(waiterRegistry, reqID)
		waiterRegistryMu.Unlock()
		<-done
		return nil, fmt.Errorf("tunnel request timeout")
	}
}

func readBodyLimited(r io.Reader, limit int64) ([]byte, error) {
	if r == nil {
		return nil, nil
	}
	data, err := io.ReadAll(io.LimitReader(r, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > limit {
		return nil, fmt.Errorf("request body exceeds max size %d", limit)
	}
	return data, nil
}

// readResponseFrames 在 stream 上循环读取响应帧并分发给 waiter
func readResponseFrames(stream io.Reader, reqID string) {
	for {
		ft, payload, err := ReadTypedFrame(stream)
		if err != nil {
			notifyError(reqID, err.Error())
			return
		}

		waiterRegistryMu.RLock()
		waiter, ok := waiterRegistry[reqID]
		waiterRegistryMu.RUnlock()
		if !ok {
			// waiter 已被清理，忽略后续帧
			if ft == FrameTypeDone {
				return
			}
			continue
		}

		switch ft {
		case FrameTypeHeader:
			var h HeaderFrame
			if json.Unmarshal(payload, &h) == nil {
				select {
				case waiter.headerDone <- &h:
				case <-waiter.closedCh:
					return
				}
			}
		case FrameTypeData:
			// Data 帧直接是原始二进制，无需 JSON 解析
			select {
			case waiter.dataChan <- payload:
			case <-waiter.closedCh:
				return
			}
		case FrameTypeDone:
			select {
			case waiter.doneChan <- struct{}{}:
			case <-waiter.closedCh:
				return
			}
			// 清理 waiter
			waiterRegistryMu.Lock()
			delete(waiterRegistry, reqID)
			waiterRegistryMu.Unlock()
			return
		case FrameTypeError:
			var e ErrorFrame
			if json.Unmarshal(payload, &e) == nil {
				select {
				case waiter.errChan <- &e:
				case <-waiter.closedCh:
					return
				}
			}
			// 清理 waiter
			waiterRegistryMu.Lock()
			delete(waiterRegistry, reqID)
			waiterRegistryMu.Unlock()
			return
		default:
			// 未知帧类型，忽略
		}
	}
}

func notifyError(reqID, message string) {
	waiterRegistryMu.RLock()
	waiter, ok := waiterRegistry[reqID]
	waiterRegistryMu.RUnlock()
	if ok {
		select {
		case waiter.errChan <- &ErrorFrame{Message: message}:
		default:
		}
	}
}

// 全局 waiter 注册表
var (
	waiterRegistryMu sync.RWMutex
	waiterRegistry   = make(map[string]*responseWaiter)
)

// responseWaiter 等待单次请求的响应
type responseWaiter struct {
	id         string
	headerDone chan *HeaderFrame
	dataChan   chan []byte
	doneChan   chan struct{}
	errChan    chan *ErrorFrame
	closedCh   chan struct{} // 当 tunnelResponseBody.Close() 时关闭，通知 readResponseFrames 停止写入
}

func newResponseWaiter(id string) *responseWaiter {
	return &responseWaiter{
		id:         id,
		headerDone: make(chan *HeaderFrame, 1),
		dataChan:   make(chan []byte, 64), // 增大缓冲防止阻塞
		doneChan:   make(chan struct{}, 1),
		errChan:    make(chan *ErrorFrame, 1),
		closedCh:   make(chan struct{}),
	}
}

// tunnelResponseBody 实现了 io.ReadCloser，从 dataChan 中读取数据
type tunnelResponseBody struct {
	waiter      *responseWaiter
	stream      io.Closer // 底层 yamux stream，Close() 时释放
	reqID       string    // 用于从 registry 清理
	buffer      []byte
	closed      bool
	done        bool // doneChan 信号已到达（guarded by mu）
	mu          sync.Mutex
	readTimeout time.Duration
}

func (b *tunnelResponseBody) Read(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.closed {
		return 0, io.EOF
	}

	if len(b.buffer) > 0 {
		n := copy(p, b.buffer)
		b.buffer = b.buffer[n:]
		return n, nil
	}

	// done 信号已到达：只排空 dataChan 中剩余的数据，然后 EOF。
	// 不能在 done 到达时直接返回 EOF：done 与最后一个 Data 帧可能同时就绪，
	// 若 select 随机选中 doneChan，剩余数据会丢失导致响应截断。
	if b.done {
		n, ok, err := b.takeData(p)
		if ok || err != nil {
			return n, err
		}
		b.closed = true
		return 0, io.EOF
	}

	// 优先非阻塞消费 dataChan（如果已有数据，必须先消费，不能跳过最后一个 Data 帧）
	if n, ok, err := b.takeData(p); ok || err != nil {
		return n, err
	}

	// dataChan 为空时，阻塞等待 data / done / 超时
	timer := time.NewTimer(b.readTimeout)
	defer timer.Stop()
	select {
	case data := <-b.waiter.dataChan:
		if len(data) == 0 {
			return 0, io.EOF
		}
		n := copy(p, data)
		if n < len(data) {
			b.buffer = data[n:]
		}
		return n, nil
	case <-b.waiter.doneChan:
		// 用标志记住 done，而不是只消费一次：若此刻 dataChan 还有剩余数据
		// （数据与 done 同时就绪时 select 随机选择），后续 Read 必须继续排空。
		b.done = true
		if n, ok, err := b.takeData(p); ok || err != nil {
			return n, err
		}
		b.closed = true
		return 0, io.EOF
	case <-timer.C:
		b.closed = true
		return 0, fmt.Errorf("tunnel read timeout")
	}
}

// takeData 尝试从 dataChan 非阻塞取一块数据。
// 返回 (n, ok=true) 表示取到数据；ok=false 且 err=nil 表示当前无数据。
func (b *tunnelResponseBody) takeData(p []byte) (int, bool, error) {
	select {
	case data := <-b.waiter.dataChan:
		if len(data) == 0 {
			return 0, false, io.EOF
		}
		n := copy(p, data)
		if n < len(data) {
			b.buffer = data[n:]
		}
		return n, true, nil
	default:
		return 0, false, nil
	}
}

func (b *tunnelResponseBody) Close() error {
	b.mu.Lock()
	if b.closed {
		b.mu.Unlock()
		return nil
	}
	b.closed = true
	b.mu.Unlock()
	// 通知 readResponseFrames 停止向 channel 写入，防止 goroutine 泄漏
	close(b.waiter.closedCh)
	// 关闭底层 stream，触发 readResponseFrames goroutine 退出
	if b.stream != nil {
		b.stream.Close()
	}
	// 清理 waiter registry（防止 goroutine 已退出但 registry 未清理的泄漏）
	if b.reqID != "" {
		waiterRegistryMu.Lock()
		delete(waiterRegistry, b.reqID)
		waiterRegistryMu.Unlock()
	}
	return nil
}

func buildHTTPResponse(waiter *responseWaiter, header *HeaderFrame, isStream bool, stream io.Closer) *http.Response {
	contentType := "application/json"
	if isStream {
		contentType = "text/event-stream"
	}
	if v, ok := header.Headers["Content-Type"]; ok {
		contentType = v
	}

	body := &tunnelResponseBody{
		waiter:      waiter,
		stream:      stream,
		reqID:       waiter.id,
		readTimeout: defaultForwarderConfig.ReadTimeout,
	}
	resp := &http.Response{
		StatusCode: header.Status,
		Header:     make(http.Header),
		Body:       body,
	}
	resp.Header.Set("Content-Type", contentType)
	for k, v := range header.Headers {
		resp.Header.Set(k, v)
	}
	return resp
}
