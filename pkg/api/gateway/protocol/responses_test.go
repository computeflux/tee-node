package protocol_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/computeflux/tee-provider/pkg/api/gateway/protocol"
	cgateway "github.com/computeflux/tee-provider/tee-chain/runtime/module/gateway"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// 其他格式 → Responses API：OpenAI Chat 请求转 Responses。
func TestConvertOpenAIRequestToResponses(t *testing.T) {
	body := []byte(`{"model":"gpt-4o","messages":[{"role":"user","content":"hello"},{"role":"assistant","content":"hi"}],"stream":false}`)
	out, err := protocol.ConvertOpenAIRequestToResponses(body, cgateway.OpenAIResponsesApi)
	require.NoError(t, err)

	var m map[string]any
	require.NoError(t, json.Unmarshal(out, &m))
	// Responses API 用顶层 input 而非 messages
	_, hasMessages := m["messages"]
	assert.False(t, hasMessages, "responses request must not contain messages")
	require.NotNil(t, m["input"])
	assert.Equal(t, "gpt-4o", m["model"])
}

// 其他格式 → Responses API：Anthropic Messages 请求转 Responses。
func TestConvertAnthropicRequestToResponses(t *testing.T) {
	body := []byte(`{"model":"claude-3","max_tokens":1024,"messages":[{"role":"user","content":"hello"}]}`)
	out, err := protocol.ConvertAnthropicRequestToResponses(body, cgateway.OpenAIResponsesApi)
	require.NoError(t, err)

	var m map[string]any
	require.NoError(t, json.Unmarshal(out, &m))
	require.NotNil(t, m["input"])
}

// Responses API → 其他格式：Responses 响应转 OpenAI Chat 响应。
func TestConvertResponsesResponseToOpenAI(t *testing.T) {
	body := []byte(`{
		"id":"resp_1","object":"response","created_at":1700000000,"model":"gpt-4o","status":"completed",
		"output":[{"type":"message","role":"assistant","content":[{"type":"output_text","text":"Hi"}]}],
		"usage":{"input_tokens":5,"output_tokens":2,"total_tokens":7}
	}`)
	out, usage, err := protocol.ConvertResponsesResponseToOpenAI(body)
	require.NoError(t, err)
	require.NotNil(t, usage)
	assert.Equal(t, 5, usage.PromptTokens)

	var m map[string]any
	require.NoError(t, json.Unmarshal(out, &m))
	assert.Equal(t, "chat.completion", m["object"])
	choices := m["choices"].([]any)
	require.Len(t, choices, 1)
}

// Responses API → Anthropic 响应（经 OpenAI 中转）。
func TestConvertResponsesResponseToAnthropic(t *testing.T) {
	body := []byte(`{
		"id":"resp_1","object":"response","created_at":1700000000,"model":"claude-3","status":"completed",
		"output":[{"type":"message","role":"assistant","content":[{"type":"output_text","text":"Hi"}]}],
		"usage":{"input_tokens":5,"output_tokens":2,"total_tokens":7}
	}`)
	out, _, err := protocol.ConvertResponsesResponseToAnthropic(body)
	require.NoError(t, err)
	assert.True(t, strings.Contains(string(out), "content"), "anthropic response must have content field")
}

// 方向约束：Responses API → 其他格式不支持（此方向在 convertRequest/convertResponse 中已拦截）。
// 这里验证 Gemin→Responses 请求明确返回不支持（bifrost 无 gemini 反解码）。
func TestConvertGeminiRequestToResponses_Unsupported(t *testing.T) {
	_, err := protocol.ConvertGeminiRequestToResponses([]byte(`{}`), cgateway.OpenAIResponsesApi)
	assert.Error(t, err)
}

// ResolveUpstreamPath：入口 openai → provider responses 应落到 /v1/responses。
func TestResolveUpstreamPath_ToResponses(t *testing.T) {
	p, err := protocol.ResolveUpstreamPath(protocol.ProtocolOpenAI, protocol.ProtocolOpenAIResponses, "/v1/chat/completions")
	require.NoError(t, err)
	assert.Equal(t, "/v1/responses", p)

	// Anthropic → Responses
	p, err = protocol.ResolveUpstreamPath(protocol.ProtocolAnthropic, protocol.ProtocolOpenAIResponses, "/v1/messages")
	require.NoError(t, err)
	assert.Equal(t, "/v1/responses", p)

	// 相同协议透传
	p, err = protocol.ResolveUpstreamPath(protocol.ProtocolOpenAIResponses, protocol.ProtocolOpenAIResponses, "/v1/responses")
	require.NoError(t, err)
	assert.Equal(t, "/v1/responses", p)
}

// 方向约束：Responses API → 其他格式不支持。
func TestResolveUpstreamPath_ResponsesToOther_Unsupported(t *testing.T) {
	_, err := protocol.ResolveUpstreamPath(protocol.ProtocolOpenAIResponses, protocol.ProtocolOpenAI, "/v1/responses")
	assert.Error(t, err)
}

// OpenAIResponsesMiddleware 把入口协议标记为 Responses API，供 HandleRelay 识别。
func TestOpenAIResponsesMiddleware(t *testing.T) {
	mw := protocol.OpenAIResponsesMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, protocol.ProtocolOpenAIResponses, protocol.FromContext(r))
	}))
	req := httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
	mw.ServeHTTP(httptest.NewRecorder(), req)
}
