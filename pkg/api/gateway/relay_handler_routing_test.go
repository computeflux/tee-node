package gateway

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/computeflux/tee-provider/pkg/api/gateway/protocol"
	relay "github.com/computeflux/tee-provider/pkg/services/gateway_relay"
	"github.com/computeflux/tee-provider/pkg/store"
	cgateway "github.com/computeflux/tee-provider/tee-chain/contracts/module/gateway"
)

func makeTestEndpoint(url string) *store.GatewayEndpoint {
	return &store.GatewayEndpoint{
		ID:         1,
		URL:        url,
		APIKey:     "sk-test",
		ProviderID: 1,
	}
}

func makeTestModelWithProviderType(name string, providerType uint16) *cgateway.ModelInfoWithID {
	m := makeTestModel(name, true, []string{"sk-test"})
	m.Data.ProviderType = providerType
	return m
}

func requestWithBody(token *store.GatewayTokenInfo, modelData *cgateway.ModelInfoWithID, endpoint *store.GatewayEndpoint, body []byte, path string, proto protocol.Protocol) *http.Request {
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
	relayToUpstreamFn = func(w http.ResponseWriter, r *http.Request, ep *store.GatewayEndpoint, modelName string, body []byte, isStream bool, providerType uint16) (*relay.Usage, *relay.ErrorWithStatusCode, error) {
		gotBody = body
		gotPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"id":"msg_1","type":"message","role":"assistant","content":[{"type":"text","text":"Hi"}],"model":"claude-3","stop_reason":"end_turn","usage":{"input_tokens":1,"output_tokens":1}}`))
		return &relay.Usage{PromptTokens: 1, CompletionTokens: 1, TotalTokens: 2}, nil, nil
	}

	body := []byte(`{"model":"anthropic/claude-3","max_tokens":1024,"messages":[{"role":"user","content":"hello"}]}`)
	req := requestWithBody(makeTestToken(true), makeTestModelWithProviderType("claude-3", 2), makeTestEndpoint("https://api.anthropic.com"), body, "/anthropic/messages", protocol.ProtocolAnthropic)

	w := httptest.NewRecorder()
	HandleRelay(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d body=%s", w.Code, w.Body.String())
	}
	if gotPath != "/v1/messages" {
		t.Errorf("path not preserved: %s", gotPath)
	}
	if !bytes.Equal(gotBody, body) {
		t.Errorf("body was modified on matching protocol: %s", string(gotBody))
	}
	if !strings.Contains(w.Body.String(), `"text":"Hi"`) {
		t.Errorf("response not passed through: %s", w.Body.String())
	}
}

func TestHandleRelay_AnthropicToOpenAI(t *testing.T) {
	cleanup := setupMockFns(t)
	defer cleanup()

	var gotBody []byte
	var gotPath string
	relayToUpstreamFn = func(w http.ResponseWriter, r *http.Request, ep *store.GatewayEndpoint, modelName string, body []byte, isStream bool, providerType uint16) (*relay.Usage, *relay.ErrorWithStatusCode, error) {
		gotBody = body
		gotPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"id":"chatcmpl-1","object":"chat.completion","model":"gpt-4","choices":[{"index":0,"message":{"role":"assistant","content":"Hello"},"finish_reason":"stop"}],"usage":{"prompt_tokens":2,"completion_tokens":1,"total_tokens":3}}`))
		return &relay.Usage{PromptTokens: 2, CompletionTokens: 1, TotalTokens: 3}, nil, nil
	}

	body := []byte(`{"model":"openai/gpt-4","max_tokens":1024,"messages":[{"role":"user","content":"hello"}]}`)
	req := requestWithBody(makeTestToken(true), makeTestModelWithProviderType("gpt-4", 1), makeTestEndpoint("https://api.openai.com"), body, "/anthropic/messages", protocol.ProtocolAnthropic)

	w := httptest.NewRecorder()
	HandleRelay(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d body=%s", w.Code, w.Body.String())
	}
	if gotPath != "/v1/chat/completions" {
		t.Errorf("path not mapped to openai: %s", gotPath)
	}

	var oaiReq map[string]any
	if err := json.Unmarshal(gotBody, &oaiReq); err != nil {
		t.Fatalf("upstream body is not valid json: %v", err)
	}
	if oaiReq["model"] != "gpt-4" {
		t.Errorf("model not converted: %v", oaiReq["model"])
	}
	if oaiReq["max_completion_tokens"] != float64(1024) {
		t.Errorf("max_completion_tokens not converted: %v", oaiReq["max_completion_tokens"])
	}
	if !strings.Contains(w.Body.String(), `"type":"message"`) {
		t.Errorf("response not converted to anthropic: %s", w.Body.String())
	}
}

func TestHandleRelay_OpenAIToAnthropic(t *testing.T) {
	cleanup := setupMockFns(t)
	defer cleanup()

	var gotBody []byte
	var gotPath string
	relayToUpstreamFn = func(w http.ResponseWriter, r *http.Request, ep *store.GatewayEndpoint, modelName string, body []byte, isStream bool, providerType uint16) (*relay.Usage, *relay.ErrorWithStatusCode, error) {
		gotBody = body
		gotPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"id":"msg_1","type":"message","role":"assistant","model":"claude-3","content":[{"type":"text","text":"Hi"}],"stop_reason":"end_turn","usage":{"input_tokens":2,"output_tokens":1}}`))
		return &relay.Usage{PromptTokens: 2, CompletionTokens: 1, TotalTokens: 3}, nil, nil
	}

	body := []byte(`{"model":"anthropic/claude-3","messages":[{"role":"user","content":"hello"}],"max_tokens":256}`)
	req := requestWithBody(makeTestToken(true), makeTestModelWithProviderType("claude-3", 2), makeTestEndpoint("https://api.anthropic.com"), body, "/v1/chat/completions", "")

	w := httptest.NewRecorder()
	HandleRelay(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d body=%s", w.Code, w.Body.String())
	}
	if gotPath != "/v1/messages" {
		t.Errorf("path not mapped to anthropic: %s", gotPath)
	}

	var antReq map[string]any
	if err := json.Unmarshal(gotBody, &antReq); err != nil {
		t.Fatalf("upstream body is not valid json: %v", err)
	}
	if antReq["model"] != "claude-3" {
		t.Errorf("model not converted: %v", antReq["model"])
	}
	if antReq["max_tokens"] != float64(256) {
		t.Errorf("max_tokens not converted: %v", antReq["max_tokens"])
	}
	if !strings.Contains(w.Body.String(), `"object":"chat.completion"`) {
		t.Errorf("response not converted to openai: %s", w.Body.String())
	}
}

func TestHandleRelay_MatchingOpenAIProtocol(t *testing.T) {
	cleanup := setupMockFns(t)
	defer cleanup()

	var gotBody []byte
	var gotPath string
	relayToUpstreamFn = func(w http.ResponseWriter, r *http.Request, ep *store.GatewayEndpoint, modelName string, body []byte, isStream bool, providerType uint16) (*relay.Usage, *relay.ErrorWithStatusCode, error) {
		gotBody = body
		gotPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
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
		t.Errorf("body was modified on matching protocol: %s", string(gotBody))
	}
}
