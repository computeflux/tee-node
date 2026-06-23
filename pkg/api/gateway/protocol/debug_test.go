package protocol

import (
	"testing"
)

func TestDebugAnthropicToOpenAIConversion(t *testing.T) {
	body := []byte(`{"model":"Bail/MiniMax-M3","max_tokens":1024,"messages":[{"role":"user","content":"Hello"}]}`)
	t.Logf("body size=%d", len(body))

	out, err := ConvertAnthropicRequestToOpenAI(body, 1)
	if err != nil {
		t.Fatalf("conversion error: %v", err)
	}
	t.Logf("output size=%d", len(out))
	t.Logf("output=%s", string(out))
}
