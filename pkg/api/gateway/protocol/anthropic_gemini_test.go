package protocol

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConvertAnthropicRequestToGemini_Simple(t *testing.T) {
	anthropicBody := []byte(`{
		"model": "gemini/gemini-2.5-flash",
		"max_tokens": 1024,
		"messages": [
			{"role": "user", "content": "Hello"}
		]
	}`)

	out, err := ConvertAnthropicRequestToGemini(anthropicBody, 3)
	require.NoError(t, err)

	var req map[string]any
	require.NoError(t, json.Unmarshal(out, &req))

	assert.Nil(t, req["model"], "model should be stripped from body")

	contents, ok := req["contents"].([]any)
	require.True(t, ok)
	require.Len(t, contents, 1)
	parts := contents[0].(map[string]any)["parts"].([]any)
	require.Len(t, parts, 1)
	assert.Equal(t, "Hello", parts[0].(map[string]any)["text"])

	genConfig := req["generationConfig"].(map[string]any)
	assert.Equal(t, float64(1024), genConfig["maxOutputTokens"])
}

func TestConvertAnthropicRequestToGemini_System(t *testing.T) {
	anthropicBody := []byte(`{
		"model": "gemini/gemini-2.5-flash",
		"max_tokens": 1024,
		"system": "You are a helpful assistant.",
		"messages": [
			{"role": "user", "content": "Hello"}
		]
	}`)

	out, err := ConvertAnthropicRequestToGemini(anthropicBody, 3)
	require.NoError(t, err)

	var req map[string]any
	require.NoError(t, json.Unmarshal(out, &req))

	si, ok := req["systemInstruction"].(map[string]any)
	require.True(t, ok)
	parts := si["parts"].([]any)
	require.Len(t, parts, 1)
	assert.Equal(t, "You are a helpful assistant.", parts[0].(map[string]any)["text"])
}

func TestConvertGeminiResponseToAnthropic_Simple(t *testing.T) {
	geminiBody := []byte(`{
		"candidates": [{
			"content": {"parts": [{"text": "Hi there"}], "role": "model"},
			"finishReason": "STOP"
		}],
		"usageMetadata": {
			"promptTokenCount": 10,
			"candidatesTokenCount": 5,
			"totalTokenCount": 15
		}
	}`)

	out, usage, err := ConvertGeminiResponseToAnthropic(geminiBody)
	require.NoError(t, err)
	require.NotNil(t, usage)
	assert.Equal(t, 10, usage.PromptTokens)
	assert.Equal(t, 5, usage.CompletionTokens)

	var resp map[string]any
	require.NoError(t, json.Unmarshal(out, &resp))

	assert.Equal(t, "message", resp["type"])
	assert.Equal(t, "assistant", resp["role"])
	assert.Equal(t, "end_turn", resp["stop_reason"])

	content, ok := resp["content"].([]any)
	require.True(t, ok)
	require.Len(t, content, 1)
	block := content[0].(map[string]any)
	assert.Equal(t, "text", block["type"])
	assert.Equal(t, "Hi there", block["text"])

	respUsage, ok := resp["usage"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, float64(10), respUsage["input_tokens"])
	assert.Equal(t, float64(5), respUsage["output_tokens"])
}

func TestStreamGeminiResponseToAnthropic_Text(t *testing.T) {
	input := "data: {\"candidates\":[{\"content\":{\"parts\":[{\"text\":\"Hello\"}],\"role\":\"model\"},\"finishReason\":\"\"}]}\n\n" +
		"data: {\"candidates\":[{\"content\":{\"parts\":[{\"text\":\" world\"}],\"role\":\"model\"},\"finishReason\":\"STOP\"}],\"usageMetadata\":{\"promptTokenCount\":10,\"candidatesTokenCount\":2,\"totalTokenCount\":12}}\n\n" +
		"data: [DONE]\n\n"

	var buf bytes.Buffer
	usage, err := StreamGeminiResponseToAnthropic(strings.NewReader(input), &buf)
	require.NoError(t, err)
	require.NotNil(t, usage)
	assert.Equal(t, 10, usage.PromptTokens)
	assert.Equal(t, 2, usage.CompletionTokens)

	out := buf.String()
	assert.Contains(t, out, "event: message_start")
	assert.Contains(t, out, "event: content_block_start")
	assert.Contains(t, out, "event: content_block_delta")
	assert.Contains(t, out, "event: content_block_stop")
	assert.Contains(t, out, "event: message_delta")
	assert.Contains(t, out, "event: message_stop")
	assert.Contains(t, out, `"text":"Hello"`)
	assert.Contains(t, out, `"text":" world"`)
}

func TestStreamGeminiResponseToAnthropic_ToolCall(t *testing.T) {
	input := "data: {\"candidates\":[{\"content\":{\"parts\":[{\"functionCall\":{\"name\":\"get_weather\",\"args\":{\"city\":\"Beijing\"}}}],\"role\":\"model\"},\"finishReason\":\"STOP\"}],\"usageMetadata\":{\"promptTokenCount\":15,\"candidatesTokenCount\":8,\"totalTokenCount\":23}}\n\n" +
		"data: [DONE]\n\n"

	var buf bytes.Buffer
	usage, err := StreamGeminiResponseToAnthropic(strings.NewReader(input), &buf)
	require.NoError(t, err)
	require.NotNil(t, usage)
	assert.Equal(t, 15, usage.PromptTokens)
	assert.Equal(t, 8, usage.CompletionTokens)

	out := buf.String()
	assert.Contains(t, out, `"type":"tool_use"`)
	assert.Contains(t, out, `"name":"get_weather"`)
	assert.Contains(t, out, `"stop_reason":"tool_use"`)
}
