package gateway

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/computeflux/tee-provider/pkg/api/gateway/protocol"
	relay "github.com/computeflux/tee-provider/pkg/services/gateway_relay"
	"github.com/computeflux/tee-provider/pkg/types"
	cgateway "github.com/computeflux/tee-provider/tee-chain/runtime/module/gateway"
)

func makeTestEndpoint(url string) *types.GatewayEndpoint {
	return &types.GatewayEndpoint{ID: 1, URL: url, APIKey: "sk-test", ProviderID: 1}
}

func makeTestModelWithProviderType(name string, providerType uint16) *cgateway.ModelInfoWithID {
	m := makeTestModel(name, true, []string{"sk-test"})
	m.Data.ProviderType = providerType
	return m
}

func requestWithBody(token *types.GatewayTokenInfo, modelData *cgateway.ModelInfoWithID, endpoint *types.GatewayEndpoint, body []byte, path string, proto protocol.Protocol) *http.Request {
	req := requestWithContext(token, modelData, endpoint, string(body))
	req = req.WithContext(context.WithValue(req.Context(), ctxBody, body))
	req.URL.Path = path
	if proto != "" {
		req = protocol.WithProtocol(req, proto)
	}
	return req
}

func TestHandleRelay_MatchingAnthropicProtocol(t *testing.T) {
	cleanup := setupMockFns(t)
	defer cleanup()

	var gotBody []byte
	var gotPath string
	relayToUpstreamFn = func(w http.ResponseWriter, r *http.Request, ep *types.GatewayEndpoint, modelName string, body []byte, isStream bool, providerType uint16) (*relay.Usage, *relay.ErrorWithStatusCode, error) {
		gotBody = body
		gotPath = r.URL.Path
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"id":"msg_1","type":"message","role":"assistant","content":[{"type":"text","text":"Hi"}],"model":"claude-3","stop_reason":"end_turn","usage":{"input_tokens":1,"output_tokens":1}}`))
		return &relay.Usage{PromptTokens: 1, CompletionTokens: 1, TotalTokens: 2}, nil, nil
	}

	body := []byte(`{"model":"anthropic/claude-3","max_tokens":1024,"messages":[{"role":"user","content":"hello"}]}`)
	req := requestWithBody(makeTestToken(true), makeTestModelWithProviderType("claude-3", 2), makeTestEndpoint("https://api.anthropic.com"), body, "/anthropic/v1/messages", protocol.ProtocolAnthropic)

	w := httptest.NewRecorder()
	HandleRelay(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d body=%s", w.Code, w.Body.String())
	}
	if gotPath != "/v1/messages" {
		t.Errorf("path not preserved: %s", gotPath)
	}
	if !bytes.Equal(gotBody, body) {
		t.Errorf("body modified on matching protocol: %s", string(gotBody))
	}
}

func TestHandleRelay_AnthropicToOpenAI(t *testing.T) {
	cleanup := setupMockFns(t)
	defer cleanup()

	streamRelayFn = func(w http.ResponseWriter, r *http.Request, ep *types.GatewayEndpoint, modelName string, body []byte, providerType uint16, isStream bool, inbound, provider protocol.Protocol, origBody []byte) (*relay.Usage, bool) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"id":"msg_1","type":"message","role":"assistant","content":[{"type":"text","text":"Hello"}],"model":"claude-3","stop_reason":"end_turn","usage":{"input_tokens":2,"output_tokens":1}}`))
		return nil, true
	}

	body := []byte(`{"model":"openai/gpt-4","max_tokens":1024,"messages":[{"role":"user","content":"hello"}]}`)
	req := requestWithBody(makeTestToken(true), makeTestModelWithProviderType("gpt-4", 1), makeTestEndpoint("https://api.openai.com"), body, "/anthropic/v1/messages", protocol.ProtocolAnthropic)

	w := httptest.NewRecorder()
	HandleRelay(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d body=%s", w.Code, w.Body.String())
	}
}

func TestHandleRelay_OpenAIToAnthropic(t *testing.T) {
	cleanup := setupMockFns(t)
	defer cleanup()

	streamRelayFn = func(w http.ResponseWriter, r *http.Request, ep *types.GatewayEndpoint, modelName string, body []byte, providerType uint16, isStream bool, inbound, provider protocol.Protocol, origBody []byte) (*relay.Usage, bool) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"id":"chatcmpl-1","object":"chat.completion","model":"gpt-4","choices":[{"index":0,"message":{"role":"assistant","content":"Hi"},"finish_reason":"stop"}],"usage":{"prompt_tokens":2,"completion_tokens":1,"total_tokens":3}}`))
		return nil, true
	}

	body := []byte(`{"model":"anthropic/claude-3","messages":[{"role":"user","content":"hello"}],"max_tokens":256}`)
	req := requestWithBody(makeTestToken(true), makeTestModelWithProviderType("claude-3", 2), makeTestEndpoint("https://api.anthropic.com"), body, "/v1/chat/completions", "")

	w := httptest.NewRecorder()
	HandleRelay(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d body=%s", w.Code, w.Body.String())
	}
}

func TestHandleRelay_MatchingOpenAIProtocol(t *testing.T) {
	cleanup := setupMockFns(t)
	defer cleanup()

	var gotBody []byte
	var gotPath string
	relayToUpstreamFn = func(w http.ResponseWriter, r *http.Request, ep *types.GatewayEndpoint, modelName string, body []byte, isStream bool, providerType uint16) (*relay.Usage, *relay.ErrorWithStatusCode, error) {
		gotBody = body
		gotPath = r.URL.Path
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"id":"chatcmpl-1","object":"chat.completion","model":"gpt-4","choices":[{"index":0,"message":{"role":"assistant","content":"Hello"},"finish_reason":"stop"}],"usage":{"prompt_tokens":1,"completion_tokens":1,"total_tokens":2}}`))
		return &relay.Usage{PromptTokens: 1, CompletionTokens: 1, TotalTokens: 2}, nil, nil
	}

	body := []byte(`{"model":"openai/gpt-4","messages":[{"role":"user","content":"hello"}]}`)
	req := requestWithBody(makeTestToken(true), makeTestModelWithProviderType("gpt-4", 1), makeTestEndpoint("https://api.openai.com"), body, "/v1/chat/completions", "")

	w := httptest.NewRecorder()
	HandleRelay(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d body=%s", w.Code, w.Body.String())
	}
	if gotPath != "/v1/chat/completions" {
		t.Errorf("path not preserved: %s", gotPath)
	}
	if !bytes.Equal(gotBody, body) {
		t.Errorf("body modified on matching protocol: %s", string(gotBody))
	}
}
