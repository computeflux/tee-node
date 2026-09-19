package gateway

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	cgateway "github.com/computeflux/tee-provider/tee-chain/runtime/module/gateway"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// upstreamSessionID 是发往 OpenCode 的 x-opencode-session 取值来源。

func TestUpstreamSessionID(t *testing.T) {
	// 请求体解析出了真实会话 ID → 原样使用
	assert.Equal(t, "conv-1", upstreamSessionID("owner@x.com", "conv-1"))

	// 解析不到 → owner 的不可逆哈希：稳定、且不包含身份信息
	a1 := upstreamSessionID("owner@x.com", "")
	a2 := upstreamSessionID("owner@x.com", "")
	assert.Equal(t, a1, a2, "同一 owner 必须稳定")
	assert.True(t, strings.HasPrefix(a1, "acct-"))
	assert.NotContains(t, a1, "owner")
	assert.NotEqual(t, a1, upstreamSessionID("another@x.com", ""), "不同 owner 必须不同")
}

// 请求体带 session_id：HandleRelay 应把它透传给 adaptor 的上游会话标识。
func TestHandleRelay_PropagatesExplicitSessionID(t *testing.T) {
	cleanup := setupMockFns(t)
	defer cleanup()

	fake := &fakeAdaptor{
		respBody: `{"id":"chatcmpl-1","object":"chat.completion","model":"zen","choices":[{"index":0,"message":{"role":"assistant","content":"Hello"},"finish_reason":"stop"}],"usage":{"prompt_tokens":1,"completion_tokens":1,"total_tokens":2}}`,
	}
	installScheduler(t, "openai", "zen", "zen-model", cgateway.OpenAI, fake)

	body := `{"model":"openai/zen","session_id":"conv-42","messages":[{"role":"user","content":"hi"}]}`
	req := relayRequest(body, "openai/zen", "/v1/chat/completions", "")

	w := httptest.NewRecorder()
	HandleRelay(w, req)
	require.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "conv-42", fake.gotUpstreamSession)
}

// 请求体无法解析出会话信息：退化为 owner 的稳定哈希，保证上游会话标识始终非空。
func TestHandleRelay_PropagatesFallbackSessionID(t *testing.T) {
	cleanup := setupMockFns(t)
	defer cleanup()

	fake := &fakeAdaptor{
		respBody: `{"id":"chatcmpl-1","object":"chat.completion","model":"zen","choices":[{"index":0,"message":{"role":"assistant","content":"Hello"},"finish_reason":"stop"}],"usage":{"prompt_tokens":1,"completion_tokens":1,"total_tokens":2}}`,
	}
	installScheduler(t, "openai", "zen", "zen-model", cgateway.OpenAI, fake)

	body := `{"model":"openai/zen"}`
	req := relayRequest(body, "openai/zen", "/v1/chat/completions", "")

	w := httptest.NewRecorder()
	HandleRelay(w, req)
	require.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, upstreamSessionID("test-owner", ""), fake.gotUpstreamSession)
	assert.True(t, strings.HasPrefix(fake.gotUpstreamSession, "acct-"))
	assert.NotContains(t, fake.gotUpstreamSession, "test-owner")
}
