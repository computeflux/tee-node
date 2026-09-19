package relay

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ── isOpenCodeURL ──────────────────────────────────────────

func TestIsOpenCodeURL(t *testing.T) {
	cases := []struct {
		url  string
		want bool
	}{
		{"https://opencode.ai/zen/go/v1/chat/completions", true},
		{"https://opencode.ai/zen/go", true},
		{"https://opencode.ai", true},
		{"https://opencode.ai/", true},
		// 同前缀域名不是 OpenCode
		{"https://opencode.ai.evil.com/v1/chat/completions", false},
		{"https://opencode.ai.evil.com", false},
		// 非 https / 其它上游
		{"http://opencode.ai/v1/chat/completions", false},
		{"https://api.openai.com/v1/chat/completions", false},
		{"", false},
	}
	for _, c := range cases {
		assert.Equal(t, c.want, isOpenCodeURL(c.url), "url=%s", c.url)
	}
}

// ── applyOpenCodeSession ───────────────────────────────────

func newTestPostRequest(t *testing.T, url string) *http.Request {
	t.Helper()
	req, err := newPostRequest(context.Background(), url, []byte(`{}`))
	require.NoError(t, err)
	return req
}

func TestApplyOpenCodeSession_InjectsHeader(t *testing.T) {
	req := newTestPostRequest(t, "https://opencode.ai/zen/go/v1/chat/completions")
	applyOpenCodeSession(req, req.URL.String(), "conv-1")

	assert.Equal(t, "conv-1", req.Header.Get(openCodeSessionHeader))
	// User-Agent 不应被改动
	assert.Empty(t, req.Header.Get("User-Agent"))
}

func TestApplyOpenCodeSession_SkipsOtherUpstreams(t *testing.T) {
	req := newTestPostRequest(t, "https://api.openai.com/v1/chat/completions")
	applyOpenCodeSession(req, req.URL.String(), "conv-1")
	assert.Empty(t, req.Header.Get(openCodeSessionHeader))
}

func TestApplyOpenCodeSession_SkipsWithoutSession(t *testing.T) {
	req := newTestPostRequest(t, "https://opencode.ai/zen/go/v1/chat/completions")
	applyOpenCodeSession(req, req.URL.String(), "")
	assert.Empty(t, req.Header.Get(openCodeSessionHeader))
}

func TestApplyOpenCodeSession_KeepsExplicitHeader(t *testing.T) {
	// 端点通过 ExtraHeaders 显式配置时，以显式配置为准
	req := newTestPostRequest(t, "https://opencode.ai/zen/go/v1/chat/completions")
	req.Header.Set(openCodeSessionHeader, "configured")
	applyOpenCodeSession(req, req.URL.String(), "conv-1")
	assert.Equal(t, "configured", req.Header.Get(openCodeSessionHeader))
}

func TestApplyOpenCodeSession_HashesUnsafeSessionID(t *testing.T) {
	// 客户端传入的会话 ID 不可信：换行等非法头值会让请求失败，必须退化为哈希
	req := newTestPostRequest(t, "https://opencode.ai/zen/go/v1/chat/completions")
	applyOpenCodeSession(req, req.URL.String(), "bad\nvalue")

	got := req.Header.Get(openCodeSessionHeader)
	assert.NotEmpty(t, got)
	assert.NotContains(t, got, "\n")
	assert.NotEqual(t, "bad\nvalue", got)

	// 同一输入 → 同一取值（会话 ID 必须稳定）
	req2 := newTestPostRequest(t, "https://opencode.ai/zen/go/v1/chat/completions")
	applyOpenCodeSession(req2, req2.URL.String(), "bad\nvalue")
	assert.Equal(t, got, req2.Header.Get(openCodeSessionHeader))
}

// ── ctx → 上游请求头 全链路 ────────────────────────────────

// recordingTransport 拦截 HTTP 层请求，避免真实网络调用。
type recordingTransport struct {
	req  *http.Request
	body string
}

func (rt *recordingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	rt.req = req
	return &http.Response{
		StatusCode: http.StatusOK,
		Header:     make(http.Header),
		Body:       io.NopCloser(strings.NewReader(rt.body)),
		Request:    req,
	}, nil
}

func TestHttpDo_InjectsOpenCodeSessionFromCtx(t *testing.T) {
	rec := &recordingTransport{body: `{"ok":true}`}
	old := doHTTPClient
	doHTTPClient = &http.Client{Transport: rec}
	defer func() { doHTTPClient = old }()

	ctx := WithUpstreamSession(context.Background(), "conv-ctx")
	body, err := httpDo(ctx, nil, "https://opencode.ai/zen/go/v1/chat/completions", []byte(`{}`), nil)
	require.NoError(t, err)
	assert.Equal(t, `{"ok":true}`, string(body))
	require.NotNil(t, rec.req)
	assert.Equal(t, "conv-ctx", rec.req.Header.Get(openCodeSessionHeader))
}

func TestHttpStream_InjectsOpenCodeSessionFromCtx(t *testing.T) {
	rec := &recordingTransport{body: "data: {\"ok\":true}\n\n"}
	old := streamHTTPClient
	streamHTTPClient = &http.Client{Transport: rec}
	defer func() { streamHTTPClient = old }()

	ctx, cancel := context.WithCancel(WithUpstreamSession(context.Background(), "conv-stream"))
	defer cancel()

	s, err := httpStream(ctx, nil, "https://opencode.ai/zen/go/v1/chat/completions", []byte(`{"stream":true}`), nil)
	require.NoError(t, err)
	defer s.Close()
	require.NotNil(t, rec.req)
	assert.Equal(t, "conv-stream", rec.req.Header.Get(openCodeSessionHeader))

	chunk, err := s.Next(context.Background())
	require.NoError(t, err)
	assert.Contains(t, string(chunk.Data), "ok")
}
