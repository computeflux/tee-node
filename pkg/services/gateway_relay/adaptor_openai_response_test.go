package relay

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/computeflux/tee-provider/pkg/services/scheduler"
	cgateway "github.com/computeflux/tee-provider/tee-chain/runtime/module/gateway"
)

// ── OpenAIResponsesApiAdaptor ──────────────────────────────

func TestOpenAIResponsesApiAdaptor_GetProviderName(t *testing.T) {
	a := &OpenAIResponsesApiAdaptor{}
	if a.GetProviderName() != "openai-responses-api" {
		t.Errorf("unexpected provider name: %s", a.GetProviderName())
	}
}

func TestOpenAIResponsesApiAdaptor_GetRequestURL(t *testing.T) {
	a := &OpenAIResponsesApiAdaptor{}

	if url := a.requestURL("https://api.openai.com", "/v1/responses"); url != "https://api.openai.com/v1/responses" {
		t.Errorf("unexpected url: %s", url)
	}
	// base 已含 /v1，去重
	if url := a.requestURL("https://api.openai.com/v1", "/v1/responses"); url != "https://api.openai.com/v1/responses" {
		t.Errorf("unexpected dedup url: %s", url)
	}
	// 空 path → 默认 /v1/responses
	if url := a.requestURL("https://api.openai.com", ""); url != "https://api.openai.com/v1/responses" {
		t.Errorf("unexpected default url: %s", url)
	}
}

func TestOpenAIResponsesApiAdaptor_Do_ForwardsBodyAndHeaders(t *testing.T) {
	scheduler.SetRequestTransform(func(inbound, providerProto, reqPath string, providerType uint16, body []byte) ([]byte, string, error) {
		return body, reqPath, nil
	})
	defer scheduler.SetRequestTransform(nil)

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/responses" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer sk-test" {
			t.Errorf("unexpected auth header: %s", r.Header.Get("Authorization"))
		}
		body, _ := io.ReadAll(r.Body)
		if !strings.Contains(string(body), `"model":"gpt-4o"`) {
			t.Errorf("body not forwarded: %s", string(body))
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"id":"resp_1","object":"response","model":"gpt-4o","status":"completed","output":[{"type":"message","role":"assistant","content":[{"type":"output_text","text":"Hi"}]}],"usage":{"input_tokens":5,"output_tokens":2,"total_tokens":7}}`))
	}))
	defer upstream.Close()

	body := []byte(`{"model":"gpt-4o","input":[{"role":"user","content":"hello"}]}`)
	a := &OpenAIResponsesApiAdaptor{}
	ep := &scheduler.Endpoint{
		KeyValue: "sk-test",
		ModelUrls: map[uint16]string{
			cgateway.OpenAIResponsesApi: upstream.URL,
		},
	}
	ctx := scheduler.WithRequest(context.Background(), "openai-responses-api", "/v1/responses")

	resp, err := a.Do(ctx, ep, body)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(string(resp), `"id":"resp_1"`) {
		t.Errorf("unexpected response: %s", string(resp))
	}
}
