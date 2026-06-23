package protocol

import (
	"bytes"
	"net/http"
)

// ResponseRecorder 包装 http.ResponseWriter，缓冲响应体与响应头，
// 以便在协议转换完成后再统一刷新到底层 writer。
type ResponseRecorder struct {
	rw          http.ResponseWriter
	statusCode  int
	header      http.Header
	body        bytes.Buffer
	wroteHeader bool
}

// NewResponseRecorder 创建一个响应记录器。
func NewResponseRecorder(w http.ResponseWriter) *ResponseRecorder {
	return &ResponseRecorder{
		rw:         w,
		statusCode: http.StatusOK,
		header:     http.Header{},
	}
}

// WriteHeader 记录状态码，但不立即转发给底层 ResponseWriter。
func (r *ResponseRecorder) WriteHeader(statusCode int) {
	if r.wroteHeader {
		return
	}
	r.statusCode = statusCode
	r.wroteHeader = true
}

// Write 缓冲响应体。如果 WriteHeader 尚未调用，会隐式记录 200。
func (r *ResponseRecorder) Write(p []byte) (int, error) {
	if !r.wroteHeader {
		r.WriteHeader(http.StatusOK)
	}
	return r.body.Write(p)
}

// Header 返回本地缓冲的 Header，供转换完成后再刷新。
func (r *ResponseRecorder) Header() http.Header {
	return r.header
}

// StatusCode 返回记录的状态码。
func (r *ResponseRecorder) StatusCode() int {
	return r.statusCode
}

// Body 返回缓冲的响应体。
func (r *ResponseRecorder) Body() []byte {
	return r.body.Bytes()
}

// ResponseWriter 返回底层 ResponseWriter。
func (r *ResponseRecorder) ResponseWriter() http.ResponseWriter {
	return r.rw
}

// FlushStatus 把缓冲的响应头与状态码写入底层 ResponseWriter。
// 应在协议转换完成后调用。
func (r *ResponseRecorder) FlushStatus() {
	if !r.wroteHeader {
		r.WriteHeader(http.StatusOK)
	}
	for k, vv := range r.header {
		for _, v := range vv {
			r.rw.Header().Set(k, v)
		}
	}
	r.rw.WriteHeader(r.statusCode)
}

// Flush 满足 http.Flusher 接口。
// ResponseRecorder 用于非流式场景（完整缓冲后转换），此处为 no-op。
func (r *ResponseRecorder) Flush() {
	// no-op: streaming is not supported through this recorder.
}
