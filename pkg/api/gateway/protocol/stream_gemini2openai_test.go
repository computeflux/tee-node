package protocol

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type geminiFlushWriter struct {
	strings.Builder
}

func (f *geminiFlushWriter) Flush() {}

func TestStreamGeminiResponseToOpenAI_Text(t *testing.T) {
	input := "data: {\"candidates\":[{\"content\":{\"parts\":[{\"text\":\"Hello\"}],\"role\":\"model\"},\"finishReason\":\"\"}]}\n\n" +
		"data: {\"candidates\":[{\"content\":{\"parts\":[{\"text\":\" world\"}],\"role\":\"model\"},\"finishReason\":\"STOP\"}],\"usageMetadata\":{\"promptTokenCount\":10,\"candidatesTokenCount\":2,\"totalTokenCount\":12}}\n\n" +
		"data: [DONE]\n\n"

	var buf geminiFlushWriter
	usage, err := StreamGeminiResponseToOpenAI(strings.NewReader(input), &buf)
	require.NoError(t, err)
	require.NotNil(t, usage)
	assert.Equal(t, 10, usage.PromptTokens)
	assert.Equal(t, 2, usage.CompletionTokens)

	out := buf.String()
	assert.Contains(t, out, `"object":"chat.completion.chunk"`)
	assert.Contains(t, out, `"role":"assistant"`)
	assert.Contains(t, out, `"content":"Hello"`)
	assert.Contains(t, out, `"content":" world"`)
	assert.Contains(t, out, `"finish_reason":"stop"`)
	assert.Contains(t, out, "data: [DONE]")
}

func TestStreamGeminiResponseToOpenAI_ToolCall(t *testing.T) {
	input := "data: {\"candidates\":[{\"content\":{\"parts\":[{\"functionCall\":{\"name\":\"get_weather\",\"args\":{\"city\":\"Beijing\"}}}],\"role\":\"model\"},\"finishReason\":\"STOP\"}],\"usageMetadata\":{\"promptTokenCount\":15,\"candidatesTokenCount\":8,\"totalTokenCount\":23}}\n\n" +
		"data: [DONE]\n\n"

	var buf geminiFlushWriter
	usage, err := StreamGeminiResponseToOpenAI(strings.NewReader(input), &buf)
	require.NoError(t, err)
	require.NotNil(t, usage)
	assert.Equal(t, 15, usage.PromptTokens)
	assert.Equal(t, 8, usage.CompletionTokens)

	out := buf.String()
	assert.Contains(t, out, `"tool_calls"`)
	assert.Contains(t, out, `"name":"get_weather"`)
	assert.Contains(t, out, `"arguments":"{\"city\":\"Beijing\"}"`)
	assert.Contains(t, out, `"finish_reason":"tool_calls"`)
}

func TestStreamGeminiResponseToOpenAI_Reasoning(t *testing.T) {
	input := "data: {\"candidates\":[{\"content\":{\"parts\":[{\"text\":\"Let me think\",\"thought\":true},{\"text\":\"Done\"}],\"role\":\"model\"},\"finishReason\":\"STOP\"}],\"usageMetadata\":{\"promptTokenCount\":5,\"candidatesTokenCount\":3,\"totalTokenCount\":8}}\n\n" +
		"data: [DONE]\n\n"

	var buf geminiFlushWriter
	_, err := StreamGeminiResponseToOpenAI(strings.NewReader(input), &buf)
	require.NoError(t, err)

	out := buf.String()
	assert.Contains(t, out, `"reasoning_content":"Let me think"`)
	assert.Contains(t, out, `"content":"Done"`)
}
