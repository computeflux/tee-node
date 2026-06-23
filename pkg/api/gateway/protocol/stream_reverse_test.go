package protocol

import (
	"strings"
	"testing"
)

func TestStreamAnthropicResponseToOpenAI_CacheTokens(t *testing.T) {
	input := "data: {\"type\":\"message_start\",\"message\":{\"id\":\"msg_1\",\"model\":\"claude-3-5-sonnet\",\"usage\":{\"input_tokens\":10,\"cache_creation_input_tokens\":4,\"cache_read_input_tokens\":6}}}\n\n" +
		"data: {\"type\":\"content_block_delta\",\"delta\":{\"type\":\"text_delta\",\"text\":\"Hello\"}}\n\n" +
		"data: {\"type\":\"message_delta\",\"delta\":{\"stop_reason\":\"end_turn\"},\"usage\":{\"output_tokens\":3}}\n\n"

	var buf flushWriter
	usage, err := StreamAnthropicResponseToOpenAI(strings.NewReader(input), &buf)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if usage == nil || usage.PromptTokens != 20 || usage.CompletionTokens != 3 || usage.TotalTokens != 23 {
		t.Fatalf("unexpected usage: %+v", usage)
	}
}

func TestStreamAnthropicResponseToOpenAI_ToolUse(t *testing.T) {
	input := "data: {\"type\":\"message_start\",\"message\":{\"id\":\"msg_1\",\"model\":\"claude-3-5-sonnet\",\"usage\":{\"input_tokens\":7}}}\n\n" +
		"data: {\"type\":\"content_block_start\",\"index\":0,\"content_block\":{\"type\":\"tool_use\",\"id\":\"call_1\",\"name\":\"get_weather\"}}\n\n" +
		"data: {\"type\":\"content_block_delta\",\"index\":0,\"delta\":{\"type\":\"input_json_delta\",\"partial_json\":\"{\\\"city\\\":\\\"NYC\\\"}\"}}\n\n" +
		"data: {\"type\":\"message_delta\",\"delta\":{\"stop_reason\":\"tool_use\"},\"usage\":{\"output_tokens\":5}}\n\n"

	var buf flushWriter
	_, err := StreamAnthropicResponseToOpenAI(strings.NewReader(input), &buf)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()
	for _, want := range []string{
		`"role":"assistant"`,
		`"tool_calls"`,
		`"id":"call_1"`,
		`"name":"get_weather"`,
		`"arguments":"{\"city\":\"NYC\"}"`,
		`"finish_reason":"tool_calls"`,
		"data: [DONE]",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in output:\n%s", want, out)
		}
	}
}

func TestStreamAnthropicResponseToOpenAI_UnrecognizedStopReason(t *testing.T) {
	input := "data: {\"type\":\"message_start\",\"message\":{\"id\":\"msg_1\",\"model\":\"claude-3-5-sonnet\",\"usage\":{\"input_tokens\":7}}}\n\n" +
		"data: {\"type\":\"message_delta\",\"delta\":{\"stop_reason\":\"unknown_future_reason\"},\"usage\":{\"output_tokens\":3}}\n\n"

	var buf flushWriter
	_, err := StreamAnthropicResponseToOpenAI(strings.NewReader(input), &buf)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, `"finish_reason":"stop"`) {
		t.Errorf("expected unrecognized stop reason to default to stop, got:\n%s", out)
	}
}

func TestStreamAnthropicResponseToOpenAI_RequiresFlusher(t *testing.T) {
	input := "data: {\"type\":\"message_start\",\"message\":{}}\n\n"

	var buf strings.Builder // no Flush method
	_, err := StreamAnthropicResponseToOpenAI(strings.NewReader(input), &buf)
	if err == nil {
		t.Fatal("expected error when response writer does not implement http.Flusher")
	}
}
