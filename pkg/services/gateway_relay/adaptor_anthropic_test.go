package relay

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/computeflux/tee-provider/pkg/services/scheduler"
)

func TestAnthropicAdaptor_GetRequestURL_Dedup(t *testing.T) {
	a := &AnthropicAdaptor{}

	if url := a.requestURL("https://api.anthropic.com", "/v1/messages"); url != "https://api.anthropic.com/v1/messages" {
		t.Errorf("unexpected url: %s", url)
	}

	if url := a.requestURL("https://api.anthropic.com/v1", "/v1/messages"); url != "https://api.anthropic.com/v1/messages" {
		t.Errorf("unexpected dedup url: %s", url)
	}
}

func TestAnthropicAdaptor_Do_ForwardsBodyAndHeaders(t *testing.T) {
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
	a := &AnthropicAdaptor{}
	ep := &scheduler.Endpoint{BaseURL: upstream.URL, KeyValue: "sk-test"}

	resp, err := a.Do(context.Background(), ep, body)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(string(resp), `"id":"msg_1"`) {
		t.Errorf("unexpected response: %s", string(resp))
	}
}
