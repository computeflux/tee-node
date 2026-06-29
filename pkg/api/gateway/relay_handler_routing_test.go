package gateway

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/computeflux/tee-provider/pkg/api/gateway/protocol"
	cgateway "github.com/computeflux/tee-provider/tee-chain/runtime/module/gateway"
)

// 这些用例验证 HandleRelay 经 scheduler 选中端点后的「入口协议 ↔ provider 协议」转换分支:
//   - 同协议:请求体/路径原样透传给上游
//   - 跨协议:请求体被转换,响应体被转换回入口协议

func relayRequest(body, modelName, path string, proto protocol.Protocol) *http.Request {
	req := httptest.NewRequest(http.MethodPost, path, nil)
	req.Header.Set("Authorization", "Bearer "+makeTestToken(true).Key)
	req = req.WithContext(context.WithValue(req.Context(), ctxToken, makeTestToken(true)))
	req = req.WithContext(context.WithValue(req.Context(), ctxBody, []byte(body)))
	req = req.WithContext(context.WithValue(req.Context(), ctxModelName, modelName))
	req.URL.Path = path
	if proto != "" {
		req = protocol.WithProtocol(req, proto)
	}
	return req
}

// 入口 Anthropic + provider Anthropic(同协议):路径归一化为 /v1/messages,body 不改。
func TestHandleRelay_MatchingAnthropicProtocol(t *testing.T) {
	cleanup := setupMockFns(t)
	defer cleanup()

	fake := &fakeAdaptor{
		respBody: `{"id":"msg_1","type":"message","role":"assistant","content":[{"type":"text","text":"Hi"}],"model":"claude-3","stop_reason":"end_turn","usage":{"input_tokens":1,"output_tokens":1}}`,
	}
	installScheduler(t, "anthropic", "claude-3", "claude-3", cgateway.ProviderClaude, fake)

	body := `{"model":"anthropic/claude-3","max_tokens":1024,"messages":[{"role":"user","content":"hello"}]}`
	req := relayRequest(body, "anthropic/claude-3", "/anthropic/v1/messages", protocol.ProtocolAnthropic)

	w := httptest.NewRecorder()
	HandleRelay(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d body=%s", w.Code, w.Body.String())
	}
	if fake.gotPath != "/v1/messages" {
		t.Errorf("path not canonicalized: %s", fake.gotPath)
	}
	if string(fake.gotBody) != body {
		t.Errorf("body modified on matching protocol: %s", string(fake.gotBody))
	}
}

// 入口 OpenAI + provider OpenAI(同协议):路径/ body 原样透传。
func TestHandleRelay_MatchingOpenAIProtocol(t *testing.T) {
	cleanup := setupMockFns(t)
	defer cleanup()

	fake := &fakeAdaptor{
		respBody: `{"id":"chatcmpl-1","object":"chat.completion","model":"gpt-4","choices":[{"index":0,"message":{"role":"assistant","content":"Hello"},"finish_reason":"stop"}],"usage":{"prompt_tokens":1,"completion_tokens":1,"total_tokens":2}}`,
	}
	installScheduler(t, "openai", "gpt-4", "gpt-4", cgateway.ProviderOpenAI, fake)

	body := `{"model":"openai/gpt-4","messages":[{"role":"user","content":"hello"}]}`
	req := relayRequest(body, "openai/gpt-4", "/v1/chat/completions", "")

	w := httptest.NewRecorder()
	HandleRelay(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d body=%s", w.Code, w.Body.String())
	}
	if fake.gotPath != "/v1/chat/completions" {
		t.Errorf("path not preserved: %s", fake.gotPath)
	}
	if string(fake.gotBody) != body {
		t.Errorf("body modified on matching protocol: %s", string(fake.gotBody))
	}
}

// 入口 Anthropic + provider OpenAI(跨协议):请求体被转换为 OpenAI,响应被转回 Anthropic。
func TestHandleRelay_AnthropicToOpenAI(t *testing.T) {
	cleanup := setupMockFns(t)
	defer cleanup()

	fake := &fakeAdaptor{
		respBody: `{"id":"chatcmpl-1","object":"chat.completion","model":"gpt-4","choices":[{"index":0,"message":{"role":"assistant","content":"Hello"},"finish_reason":"stop"}],"usage":{"prompt_tokens":2,"completion_tokens":1,"total_tokens":3}}`,
	}
	installScheduler(t, "openai", "claude-proxy", "gpt-4", cgateway.ProviderOpenAI, fake)

	body := `{"model":"openai/claude-proxy","max_tokens":1024,"messages":[{"role":"user","content":"hello"}]}`
	req := relayRequest(body, "openai/claude-proxy", "/anthropic/v1/messages", protocol.ProtocolAnthropic)

	w := httptest.NewRecorder()
	HandleRelay(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d body=%s", w.Code, w.Body.String())
	}
	// 上游收到的应是被转换后的 OpenAI 路径,且 body 不再等于原始 Anthropic body。
	if fake.gotPath != "/v1/chat/completions" {
		t.Errorf("expected converted upstream path /v1/chat/completions, got %s", fake.gotPath)
	}
	if string(fake.gotBody) == body {
		t.Errorf("body should be converted to OpenAI, got original anthropic body")
	}
	// 回给客户端的应是 Anthropic 协议响应。
	if !contains(w.Body.String(), `"type":"message"`) {
		t.Errorf("response not converted to anthropic: %s", w.Body.String())
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (func() bool {
		for i := 0; i+len(sub) <= len(s); i++ {
			if s[i:i+len(sub)] == sub {
				return true
			}
		}
		return false
	})()
}
