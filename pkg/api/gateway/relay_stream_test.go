package gateway

import (
	"bytes"
	"net/http"
	"testing"
)

type mockFlusher struct {
	flushed bool
}

func (f *mockFlusher) Flush() {
	f.flushed = true
}

type mockResponseWriter struct{}

func (mockResponseWriter) Header() http.Header { return http.Header{} }
func (mockResponseWriter) Write([]byte) (int, error) { return 0, nil }
func (mockResponseWriter) WriteHeader(int) {}

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
