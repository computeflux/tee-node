package relay

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestAnthropicAdaptor_GetRequestURL_Dedup(t *testing.T) {
	a := &AnthropicAdaptor{}

	url, err := a.GetRequestURL(&Meta{BaseURL: "https://api.anthropic.com", RequestURLPath: "/v1/messages"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if url != "https://api.anthropic.com/v1/messages" {
		t.Errorf("unexpected url: %s", url)
	}

	url, err = a.GetRequestURL(&Meta{BaseURL: "https://api.anthropic.com/v1", RequestURLPath: "/v1/messages"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if url != "https://api.anthropic.com/v1/messages" {
		t.Errorf("unexpected dedup url: %s", url)
	}
}

func TestAnthropicAdaptor_DoRequest_ForwardsBodyAndHeaders(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/messages" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.Header.Get("x-api-key") != "sk-test" {
			t.Errorf("unexpected api key: %s", r.Header.Get("x-api-key"))
		}
		if r.Header.Get("anthropic-version") == "" {
			t.Error("missing anthropic-version header")
		}

		body, _ := io.ReadAll(r.Body)
		if !strings.Contains(string(body), `"model":"claude-3"`) {
			t.Errorf("body not forwarded: %s", string(body))
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"id":"msg_1","type":"message","role":"assistant","model":"claude-3","content":[{"type":"text","text":"Hi"}],"stop_reason":"end_turn","usage":{"input_tokens":5,"output_tokens":2}}`))
	}))
	defer upstream.Close()

	body := []byte(`{"model":"claude-3","max_tokens":1024,"messages":[{"role":"user","content":"hello"}]}`)
	req := httptest.NewRequest(http.MethodPost, "/anthropic/messages", bytes.NewReader(body))
	req.Header.Set("anthropic-version", "2023-06-01")
	rec := httptest.NewRecorder()
	ctx := NewRelayContext(rec, req)

	a := &AnthropicAdaptor{}
	meta := &Meta{BaseURL: upstream.URL, APIKey: "sk-test", RequestURLPath: "/v1/messages"}

	resp, err := a.DoRequest(ctx, meta, bytes.NewReader(body))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("unexpected status: %d", resp.StatusCode)
	}
}

func TestAnthropicAdaptor_DoResponse_NonStream(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"id":"msg_1","type":"message","role":"assistant","model":"claude-3","content":[{"type":"text","text":"Hello"}],"stop_reason":"end_turn","usage":{"input_tokens":10,"output_tokens":3}}`))
	}))
	defer upstream.Close()

	body := []byte(`{"model":"claude-3","max_tokens":1024,"messages":[]}`)
	req := httptest.NewRequest(http.MethodPost, "/anthropic/messages", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	ctx := NewRelayContext(rec, req)

	a := &AnthropicAdaptor{}
	meta := &Meta{BaseURL: upstream.URL, APIKey: "sk-test", RequestURLPath: "/v1/messages"}

	resp, err := a.DoRequest(ctx, meta, bytes.NewReader(body))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer resp.Body.Close()

	outRec := httptest.NewRecorder()
	outCtx := NewRelayContext(outRec, req)
	usage, bizErr := a.DoResponse(outCtx, resp, &Meta{IsStream: false})
	if bizErr != nil {
		t.Fatalf("unexpected bizErr: %v", bizErr)
	}
	if usage == nil || usage.PromptTokens != 10 || usage.CompletionTokens != 3 {
		t.Fatalf("unexpected usage: %+v", usage)
	}
	if !strings.Contains(outRec.Body.String(), `"text":"Hello"`) {
		t.Errorf("response not copied: %s", outRec.Body.String())
	}
}

func TestAnthropicAdaptor_DoResponse_NonStream_CacheTokens(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"id":"msg_1","type":"message","role":"assistant","model":"claude-3","content":[{"type":"text","text":"Hello"}],"stop_reason":"end_turn","usage":{"input_tokens":10,"cache_creation_input_tokens":4,"cache_read_input_tokens":6,"output_tokens":3}}`))
	}))
	defer upstream.Close()

	body := []byte(`{"model":"claude-3","max_tokens":1024,"messages":[]}`)
	req := httptest.NewRequest(http.MethodPost, "/anthropic/messages", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	ctx := NewRelayContext(rec, req)

	a := &AnthropicAdaptor{}
	meta := &Meta{BaseURL: upstream.URL, APIKey: "sk-test", RequestURLPath: "/v1/messages"}

	resp, err := a.DoRequest(ctx, meta, bytes.NewReader(body))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer resp.Body.Close()

	outRec := httptest.NewRecorder()
	outCtx := NewRelayContext(outRec, req)
	usage, bizErr := a.DoResponse(outCtx, resp, &Meta{IsStream: false})
	if bizErr != nil {
		t.Fatalf("unexpected bizErr: %v", bizErr)
	}
	wantPrompt := 10 + 4 + 6
	if usage == nil || usage.PromptTokens != wantPrompt || usage.PromptTokensDetails.CachedTokens != 6 {
		t.Fatalf("unexpected usage: %+v", usage)
	}
}

func TestAnthropicAdaptor_DoResponse_NonStream_UpstreamError(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"type":"error","error":{"type":"authentication_error","message":"invalid key"}}`))
	}))
	defer upstream.Close()

	body := []byte(`{"model":"claude-3","max_tokens":1024,"messages":[]}`)
	req := httptest.NewRequest(http.MethodPost, "/anthropic/messages", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	ctx := NewRelayContext(rec, req)

	a := &AnthropicAdaptor{}
	meta := &Meta{BaseURL: upstream.URL, APIKey: "sk-test", RequestURLPath: "/v1/messages"}

	resp, err := a.DoRequest(ctx, meta, bytes.NewReader(body))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer resp.Body.Close()

	outRec := httptest.NewRecorder()
	outCtx := NewRelayContext(outRec, req)
	usage, bizErr := a.DoResponse(outCtx, resp, &Meta{IsStream: false})
	if usage != nil {
		t.Fatalf("expected no usage on error, got %+v", usage)
	}
	if bizErr == nil {
		t.Fatal("expected upstream error")
	}
	if bizErr.StatusCode != http.StatusUnauthorized {
		t.Errorf("unexpected status: %d", bizErr.StatusCode)
	}
	if outRec.Code != 0 && outRec.Body.Len() > 0 {
		t.Errorf("error response should not be written by adaptor, got: %s", outRec.Body.String())
	}
}

func TestAnthropicAdaptor_DoResponse_Stream(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("data: {\"type\":\"message_start\",\"message\":{\"usage\":{\"input_tokens\":7}}}\n\n"))
		w.Write([]byte("data: {\"type\":\"content_block_delta\",\"delta\":{\"type\":\"text_delta\",\"text\":\"Hi\"}}\n\n"))
		w.Write([]byte("data: {\"type\":\"message_delta\",\"delta\":{\"stop_reason\":\"end_turn\"},\"usage\":{\"output_tokens\":2}}\n\n"))
	}))
	defer upstream.Close()

	body := []byte(`{"model":"claude-3","max_tokens":1024,"stream":true,"messages":[]}`)
	req := httptest.NewRequest(http.MethodPost, "/anthropic/messages", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	ctx := NewRelayContext(rec, req)

	a := &AnthropicAdaptor{}
	meta := &Meta{BaseURL: upstream.URL, APIKey: "sk-test", RequestURLPath: "/v1/messages"}

	resp, err := a.DoRequest(ctx, meta, bytes.NewReader(body))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer resp.Body.Close()

	outRec := httptest.NewRecorder()
	outCtx := NewRelayContext(outRec, req)
	usage, bizErr := a.DoResponse(outCtx, resp, &Meta{IsStream: true})
	if bizErr != nil {
		t.Fatalf("unexpected bizErr: %v", bizErr)
	}
	if usage == nil || usage.PromptTokens != 7 || usage.CompletionTokens != 2 {
		t.Fatalf("unexpected usage: %+v", usage)
	}
	if !strings.Contains(outRec.Body.String(), `"type":"content_block_delta"`) {
		t.Errorf("stream not copied: %s", outRec.Body.String())
	}
}

func TestAnthropicAdaptor_DoResponse_Stream_RequiresFlusher(t *testing.T) {
	a := &AnthropicAdaptor{}
	req := httptest.NewRequest(http.MethodPost, "/anthropic/messages", bytes.NewReader([]byte(`{}`)))

	// 用一个没有 Flush 方法的 ResponseWriter 触发错误。
	noFlush := &noopResponseWriter{}
	ctx := NewRelayContext(noFlush, req)

	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"text/event-stream"}},
		Body:       io.NopCloser(bytes.NewReader([]byte("data: {}\n\n"))),
	}

	usage, bizErr := a.DoResponse(ctx, resp, &Meta{IsStream: true})
	if usage != nil {
		t.Fatalf("expected no usage without flusher, got %+v", usage)
	}
	if bizErr == nil {
		t.Fatal("expected error when response writer does not implement http.Flusher")
	}
}

type noopResponseWriter struct {
	bytes.Buffer
}

func (n *noopResponseWriter) Header() http.Header { return http.Header{} }
func (n *noopResponseWriter) WriteHeader(int)     {}
func (n *noopResponseWriter) Write(p []byte) (int, error) {
	return n.Buffer.Write(p)
}

func TestAnthropicAdaptor_DoResponse_Stream_CacheRevision(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("data: {\"type\":\"message_start\",\"message\":{\"usage\":{\"input_tokens\":5}}}\n\n"))
		w.Write([]byte("data: {\"type\":\"content_block_delta\",\"delta\":{\"type\":\"text_delta\",\"text\":\"Hi\"}}\n\n"))
		w.Write([]byte("data: {\"type\":\"message_delta\",\"delta\":{\"stop_reason\":\"end_turn\"},\"usage\":{\"output_tokens\":2},\"message\":{\"usage\":{\"input_tokens\":5,\"cache_creation_input_tokens\":3,\"cache_read_input_tokens\":2}}}\n\n"))
	}))
	defer upstream.Close()

	body := []byte(`{"model":"claude-3","max_tokens":1024,"stream":true,"messages":[]}`)
	req := httptest.NewRequest(http.MethodPost, "/anthropic/messages", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	ctx := NewRelayContext(rec, req)

	a := &AnthropicAdaptor{}
	meta := &Meta{BaseURL: upstream.URL, APIKey: "sk-test", RequestURLPath: "/v1/messages"}

	resp, err := a.DoRequest(ctx, meta, bytes.NewReader(body))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer resp.Body.Close()

	outRec := httptest.NewRecorder()
	outCtx := NewRelayContext(outRec, req)
	usage, bizErr := a.DoResponse(outCtx, resp, &Meta{IsStream: true})
	if bizErr != nil {
		t.Fatalf("unexpected bizErr: %v", bizErr)
	}
	if usage == nil || usage.PromptTokens != 10 || usage.PromptTokensDetails.CachedTokens != 2 || usage.CompletionTokens != 2 {
		t.Fatalf("unexpected usage: %+v", usage)
	}
}
