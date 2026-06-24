package protocol

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConvertOpenAIRequestToAnthropic(t *testing.T) {
	body := []byte(`{
		"model": "claude-3-5-sonnet",
		"messages": [
			{"role": "system", "content": "you are a bot"},
			{"role": "user", "content": "hello"}
		],
		"max_tokens": 256,
		"stream": true
	}`)

	out, err := ConvertOpenAIRequestToAnthropic(body, 2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var antReq map[string]any
	if err := json.Unmarshal(out, &antReq); err != nil {
		t.Fatalf("invalid output json: %v", err)
	}

	if antReq["model"] != "claude-3-5-sonnet" {
		t.Errorf("model mismatch: %v", antReq["model"])
	}
	if antReq["max_tokens"] != float64(256) {
		t.Errorf("max_tokens mismatch: %v", antReq["max_tokens"])
	}
	if antReq["stream"] != true {
		t.Errorf("stream mismatch: %v", antReq["stream"])
	}

	system := antReq["system"]
	systemText := ""
	switch s := system.(type) {
	case string:
		systemText = s
	case map[string]any:
		systemText, _ = s["text"].(string)
	}
	if systemText != "you are a bot" {
		t.Errorf("system message not extracted: %v", system)
	}

	messages, ok := antReq["messages"].([]any)
	if !ok || len(messages) != 1 {
		t.Fatalf("messages mismatch: %v", antReq["messages"])
	}
}

func TestConvertAnthropicResponseToOpenAI(t *testing.T) {
	body := []byte(`{
		"id": "msg_1",
		"type": "message",
		"role": "assistant",
		"model": "claude-3-5-sonnet",
		"content": [{"type": "text", "text": "Hi there"}],
		"stop_reason": "end_turn",
		"usage": {
			"input_tokens": 10,
			"output_tokens": 2
		}
	}`)

	out, err := ConvertAnthropicResponseToOpenAI(body)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var oaiResp map[string]any
	if err := json.Unmarshal(out, &oaiResp); err != nil {
		t.Fatalf("invalid output json: %v", err)
	}

	if oaiResp["object"] != "chat.completion" {
		t.Errorf("object mismatch: %v", oaiResp["object"])
	}
	if oaiResp["id"] != "msg_1" {
		t.Errorf("id mismatch: %v", oaiResp["id"])
	}

	choices, ok := oaiResp["choices"].([]any)
	if !ok || len(choices) != 1 {
		t.Fatalf("choices mismatch: %v", oaiResp["choices"])
	}
	choice := choices[0].(map[string]any)
	msg := choice["message"].(map[string]any)
	if msg["role"] != "assistant" || msg["content"] != "Hi there" {
		t.Errorf("message mismatch: %v", msg)
	}

	usage := oaiResp["usage"].(map[string]any)
	if usage["prompt_tokens"] != float64(10) || usage["completion_tokens"] != float64(2) {
		t.Errorf("usage mismatch: %v", usage)
	}
}

type flushWriter struct {
	strings.Builder
}

func (f *flushWriter) Flush() {}

func TestStreamAnthropicResponseToOpenAI(t *testing.T) {
	input := "data: {\"type\":\"message_start\",\"message\":{\"id\":\"msg_1\",\"model\":\"claude-3-5-sonnet\",\"usage\":{\"input_tokens\":7}}}\n\n" +
		"data: {\"type\":\"content_block_delta\",\"delta\":{\"type\":\"text_delta\",\"text\":\"Hello\"}}\n\n" +
		"data: {\"type\":\"message_delta\",\"delta\":{\"stop_reason\":\"end_turn\"},\"usage\":{\"output_tokens\":3}}\n\n"

	var buf flushWriter
	usage, err := StreamAnthropicResponseToOpenAI(strings.NewReader(input), &buf)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if usage == nil || usage.PromptTokens != 7 || usage.CompletionTokens != 3 {
		t.Fatalf("unexpected usage: %+v", usage)
	}

	out := buf.String()
	if !strings.Contains(out, `"object":"chat.completion.chunk"`) {
		t.Errorf("missing openai chunk object: %s", out)
	}
	if !strings.Contains(out, `"role":"assistant"`) {
		t.Errorf("missing role delta: %s", out)
	}
	if !strings.Contains(out, `"content":"Hello"`) {
		t.Errorf("missing content delta: %s", out)
	}
	if !strings.Contains(out, `"finish_reason":"stop"`) {
		t.Errorf("missing finish_reason: %s", out)
	}
	if !strings.Contains(out, "data: [DONE]") {
		t.Errorf("missing done marker: %s", out)
	}
}

func TestConvertAnthropicResponseToOpenAI_WithReasoning(t *testing.T) {
	body := []byte(`{
		"id": "msg_2",
		"type": "message",
		"role": "assistant",
		"model": "claude-opus-4",
		"content": [
			{"type": "thinking", "thinking": "Let me think"},
			{"type": "text", "text": "Done"}
		],
		"stop_reason": "end_turn",
		"usage": {"input_tokens": 5, "output_tokens": 3}
	}`)

	out, err := ConvertAnthropicResponseToOpenAI(body)
	require.NoError(t, err)

	var oaiResp map[string]any
	require.NoError(t, json.Unmarshal(out, &oaiResp))

	choices := oaiResp["choices"].([]any)
	msg := choices[0].(map[string]any)["message"].(map[string]any)
	assert.Equal(t, "Done", msg["content"])
	assert.Equal(t, "Let me think", msg["reasoning_content"])
}
