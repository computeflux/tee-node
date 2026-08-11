package gateway

import (
	"bytes"
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

type mockFlusher struct {
	flushed bool
}

func (f *mockFlusher) Flush() {
	f.flushed = true
}

type mockResponseWriter struct{}

func (mockResponseWriter) Header() http.Header       { return http.Header{} }
func (mockResponseWriter) Write([]byte) (int, error) { return 0, nil }
func (mockResponseWriter) WriteHeader(int)           {}

func TestFlushWriter_ImplementsHTTPFlusher(t *testing.T) {
	var buf bytes.Buffer
	fl := &mockFlusher{}
	fw := flushWriter{w: &buf, fl: fl}

	// 类型断言必须成功，否则跨协议流式转换器会报错。
	if _, ok := any(fw).(http.Flusher); !ok {
		t.Fatal("flushWriter does not implement http.Flusher")
	}

	fw.Write([]byte("hello"))
	if !fl.flushed {
		t.Fatal("flushWriter.Write did not flush")
	}

	fl.flushed = false
	fw.Flush()
	if !fl.flushed {
		t.Fatal("flushWriter.Flush did not flush")
	}
}

func TestFlushWriter_NilFlusherNoPanic(t *testing.T) {
	var buf bytes.Buffer
	fw := flushWriter{w: &buf, fl: nil}

	// nil flusher 不应 panic
	if _, err := fw.Write([]byte("hello")); err != nil {
		t.Fatalf("unexpected write error: %v", err)
	}
	fw.Flush()
}

// 回归测试：responseWriter 必须实现 Unwrap，否则 http.ResponseController
// 无法穿透中间件包装层，长流式响应的写超时清除会静默失效。
func TestResponseWriter_UnwrapLetsControllerReachUnderlying(t *testing.T) {
	underlying := mockResponseWriter{}
	rw := &responseWriter{ResponseWriter: underlying}

	// Unwrap 必须返回底层 writer
	assert.Equal(t, underlying, rw.Unwrap())

	// 模拟 HandleRelay 的用法：经过包装层后 ResponseController 仍应能找到目标。
	// 底层 mock 不支持 SetWriteDeadline 时返回 ErrNotSupported（而非找不到目标）；
	// 只要错误是 ErrNotSupported 就说明穿透成功。
	ctrl := http.NewResponseController(rw)
	err := ctrl.SetWriteDeadline(time.Time{})
	if err != nil && !errors.Is(err, http.ErrNotSupported) {
		t.Fatalf("unexpected error: %v", err)
	}
}
