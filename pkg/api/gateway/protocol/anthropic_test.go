package protocol

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConvertAnthropicRequestToOpenAI_Simple(t *testing.T) {
	anthropicBody := []byte(`{
		"model": "openai/gpt-4o-mini",
		"max_tokens": 1024,
		"messages": [
			{"role": "user", "content": "Hello"}
		]
	}`)

	openAIBody, err := ConvertAnthropicRequestToOpenAI(anthropicBody, 1) // ProviderType OpenAI
	require.NoError(t, err)

	var openAIReq map[string]any
	require.NoError(t, json.Unmarshal(openAIBody, &openAIReq))

	assert.Equal(t, "gpt-4o-mini", openAIReq["model"])
	assert.NotNil(t, openAIReq["messages"])
	messages, ok := openAIReq["messages"].([]any)
	require.True(t, ok)
	require.Len(t, messages, 1)
	msg := messages[0].(map[string]any)
	assert.Equal(t, "user", msg["role"])
	assert.Equal(t, "Hello", msg["content"])
}

func TestConvertAnthropicRequestToOpenAI_Stream(t *testing.T) {
	anthropicBody := []byte(`{
		"model": "openai/gpt-4o-mini",
		"max_tokens": 1024,
		"stream": true,
		"messages": [{"role": "user", "content": "Hello"}]
	}`)

	openAIBody, err := ConvertAnthropicRequestToOpenAI(anthropicBody, 1)
	require.NoError(t, err)

	var openAIReq map[string]any
	require.NoError(t, json.Unmarshal(openAIBody, &openAIReq))
	assert.Equal(t, true, openAIReq["stream"])
}

func TestConvertAnthropicRequestToOpenAI_System(t *testing.T) {
	anthropicBody := []byte(`{
		"model": "openai/gpt-4o-mini",
		"max_tokens": 1024,
		"system": "You are a helpful assistant.",
		"messages": [{"role": "user", "content": "Hello"}]
	}`)

	openAIBody, err := ConvertAnthropicRequestToOpenAI(anthropicBody, 1)
	require.NoError(t, err)

	var openAIReq map[string]any
	require.NoError(t, json.Unmarshal(openAIBody, &openAIReq))
	messages, ok := openAIReq["messages"].([]any)
	require.True(t, ok)
	require.GreaterOrEqual(t, len(messages), 2)
	firstMsg := messages[0].(map[string]any)
	assert.Equal(t, "system", firstMsg["role"])
	assert.Equal(t, "You are a helpful assistant.", firstMsg["content"])
}

func TestConvertAnthropicRequestToOpenAI_Image(t *testing.T) {
	anthropicBody := []byte(`{
		"model": "openai/gpt-4o-mini",
		"max_tokens": 1024,
		"messages": [{
			"role": "user",
			"content": [
				{"type": "image", "source": {"type": "base64", "media_type": "image/png", "data": "iVBORw0KGgo="}},
				{"type": "text", "text": "What's in this image?"}
			]
		}]
	}`)

	openAIBody, err := ConvertAnthropicRequestToOpenAI(anthropicBody, 1)
	require.NoError(t, err)

	var openAIReq map[string]any
	require.NoError(t, json.Unmarshal(openAIBody, &openAIReq))
	messages, ok := openAIReq["messages"].([]any)
	require.True(t, ok)
	// bifrost 可能把 image + text 拆成两条 user message，也可能合并；这里只验证 image 被转成 image_url
	foundImage := false
	for _, m := range messages {
		msg := m.(map[string]any)
		if msg["role"] != "user" {
			continue
		}
		content, ok := msg["content"].([]any)
		if !ok {
			continue
		}
		for _, c := range content {
			block := c.(map[string]any)
			if block["type"] == "image_url" {
				foundImage = true
				url := block["image_url"].(map[string]any)["url"].(string)
				assert.Contains(t, url, "data:image/png;base64,")
			}
		}
	}
	assert.True(t, foundImage, "image block should be converted to image_url")
}

func TestConvertAnthropicRequestToOpenAI_WithTool(t *testing.T) {
	anthropicBody := []byte(`{
		"model": "openai/gpt-4o-mini",
		"max_tokens": 1024,
		"messages": [{"role": "user", "content": "What's the weather?"}],
		"tools": [
			{"name": "get_weather", "description": "Get weather", "input_schema": {"type": "object", "properties": {"city": {"type": "string"}}}}
		]
	}`)

	openAIBody, err := ConvertAnthropicRequestToOpenAI(anthropicBody, 1)
	require.NoError(t, err)

	var openAIReq map[string]any
	require.NoError(t, json.Unmarshal(openAIBody, &openAIReq))
	tools, ok := openAIReq["tools"].([]any)
	require.True(t, ok)
	require.Len(t, tools, 1)
	tool := tools[0].(map[string]any)
	assert.Equal(t, "function", tool["type"])
	fn := tool["function"].(map[string]any)
	assert.Equal(t, "get_weather", fn["name"])
}

func TestConvertOpenAIResponseToAnthropic_Simple(t *testing.T) {
	openAIBody := []byte(`{
		"id": "chatcmpl-123",
		"object": "chat.completion",
		"created": 1677652288,
		"model": "gpt-4o-mini",
		"choices": [{
			"index": 0,
			"message": {"role": "assistant", "content": "Hi there"},
			"finish_reason": "stop"
		}],
		"usage": {
			"prompt_tokens": 10,
			"completion_tokens": 5,
			"total_tokens": 15
		}
	}`)

	anthropicBody, usage, err := ConvertOpenAIResponseToAnthropic(openAIBody)
	require.NoError(t, err)
	require.NotNil(t, usage)
	assert.Equal(t, 10, usage.PromptTokens)
	assert.Equal(t, 5, usage.CompletionTokens)
	assert.Equal(t, 15, usage.TotalTokens)

	var anthropicResp map[string]any
	require.NoError(t, json.Unmarshal(anthropicBody, &anthropicResp))

	assert.Equal(t, "message", anthropicResp["type"])
	assert.Equal(t, "chatcmpl-123", anthropicResp["id"])
	assert.Equal(t, "assistant", anthropicResp["role"])
	assert.Equal(t, "gpt-4o-mini", anthropicResp["model"])
	assert.Equal(t, "end_turn", anthropicResp["stop_reason"])

	content, ok := anthropicResp["content"].([]any)
	require.True(t, ok)
	require.Len(t, content, 1)
	block := content[0].(map[string]any)
	assert.Equal(t, "text", block["type"])
	assert.Equal(t, "Hi there", block["text"])

	respUsage, ok := anthropicResp["usage"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, float64(10), respUsage["input_tokens"])
	assert.Equal(t, float64(5), respUsage["output_tokens"])
}

func TestStreamOpenAIResponseToAnthropic_Text(t *testing.T) {
	openAIStream := `data: {"id":"chatcmpl-123","object":"chat.completion.chunk","created":1677652288,"model":"gpt-4o-mini","choices":[{"index":0,"delta":{"role":"assistant"},"finish_reason":null}]}

data: {"id":"chatcmpl-123","object":"chat.completion.chunk","created":1677652288,"model":"gpt-4o-mini","choices":[{"index":0,"delta":{"content":"Hello"},"finish_reason":null}]}

data: {"id":"chatcmpl-123","object":"chat.completion.chunk","created":1677652288,"model":"gpt-4o-mini","choices":[{"index":0,"delta":{"content":" world"},"finish_reason":null}]}

data: {"id":"chatcmpl-123","object":"chat.completion.chunk","created":1677652288,"model":"gpt-4o-mini","choices":[{"index":0,"delta":{},"finish_reason":"stop"}],"usage":{"prompt_tokens":10,"completion_tokens":2,"total_tokens":12}}

data: [DONE]
`

	var buf bytes.Buffer
	usage, err := StreamOpenAIResponseToAnthropic(strings.NewReader(openAIStream), &buf)
	require.NoError(t, err)
	require.NotNil(t, usage)
	assert.Equal(t, 10, usage.PromptTokens)
	assert.Equal(t, 2, usage.CompletionTokens)

	output := buf.String()
	assert.Contains(t, output, "event: message_start")
	assert.Contains(t, output, "event: content_block_start")
	assert.Contains(t, output, "event: content_block_delta")
	assert.Contains(t, output, "event: content_block_stop")
	assert.Contains(t, output, "event: message_delta")
	assert.Contains(t, output, "event: message_stop")
	assert.Contains(t, output, `"text":"Hello"`)
	assert.Contains(t, output, `"text":" world"`)
}

func TestStreamOpenAIResponseToAnthropic_ToolCall(t *testing.T) {
	openAIStream := `data: {"id":"chatcmpl-tool","object":"chat.completion.chunk","created":1677652288,"model":"gpt-4o-mini","choices":[{"index":0,"delta":{"role":"assistant","content":null},"finish_reason":null}]}

data: {"id":"chatcmpl-tool","object":"chat.completion.chunk","created":1677652288,"model":"gpt-4o-mini","choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"id":"call_123","type":"function","function":{"name":"get_weather","arguments":""}}]},"finish_reason":null}]}

data: {"id":"chatcmpl-tool","object":"chat.completion.chunk","created":1677652288,"model":"gpt-4o-mini","choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"function":{"arguments":"{\""}}]},"finish_reason":null}]}

data: {"id":"chatcmpl-tool","object":"chat.completion.chunk","created":1677652288,"model":"gpt-4o-mini","choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"function":{"arguments":"city\":\""}}]},"finish_reason":null}]}

data: {"id":"chatcmpl-tool","object":"chat.completion.chunk","created":1677652288,"model":"gpt-4o-mini","choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"function":{"arguments":"Beijing\"}"}}]},"finish_reason":null}]}

data: {"id":"chatcmpl-tool","object":"chat.completion.chunk","created":1677652288,"model":"gpt-4o-mini","choices":[{"index":0,"delta":{},"finish_reason":"tool_calls"}],"usage":{"prompt_tokens":20,"completion_tokens":10,"total_tokens":30}}

data: [DONE]
`

	var buf bytes.Buffer
	usage, err := StreamOpenAIResponseToAnthropic(strings.NewReader(openAIStream), &buf)
	require.NoError(t, err)
	require.NotNil(t, usage)
	assert.Equal(t, 20, usage.PromptTokens)
	assert.Equal(t, 10, usage.CompletionTokens)

	output := buf.String()
	assert.Contains(t, output, "event: content_block_start")
	assert.Contains(t, output, "event: content_block_delta")
	assert.Contains(t, output, "event: content_block_stop")
	assert.Contains(t, output, `"name":"get_weather"`)
	assert.Contains(t, output, `"stop_reason":"tool_use"`)

	// 解析出所有 input_json_delta 的 partial_json，验证增量拼接正确
	var deltas []string
	for _, line := range strings.Split(output, "\n") {
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		data := strings.TrimPrefix(line, "data: ")
		var event struct {
			Type  string `json:"type"`
			Delta struct {
				Type        string `json:"type"`
				PartialJSON string `json:"partial_json"`
			} `json:"delta"`
		}
		if err := json.Unmarshal([]byte(data), &event); err != nil {
			continue
		}
		if event.Type == "content_block_delta" && event.Delta.Type == "input_json_delta" {
			deltas = append(deltas, event.Delta.PartialJSON)
		}
	}
	require.Len(t, deltas, 3)
	assert.Equal(t, `{"`, deltas[0])
	assert.Equal(t, `city":"`, deltas[1])
	assert.Equal(t, `Beijing"}`, deltas[2])
}

func TestStreamOpenAIResponseToAnthropic_ToolCallDeferredStart(t *testing.T) {
	// name 在参数片段之后才到达，验证 content_block_start 推迟到 name 可用，
	// 并补发已缓存的参数增量。
	openAIStream := `data: {"id":"chatcmpl-deferred","object":"chat.completion.chunk","created":1,"model":"gpt-4","choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"id":"call_d"}]},"finish_reason":null}]}

data: {"id":"chatcmpl-deferred","object":"chat.completion.chunk","created":1,"model":"gpt-4","choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"function":{"arguments":"{\"loc"}}]},"finish_reason":null}]}

data: {"id":"chatcmpl-deferred","object":"chat.completion.chunk","created":1,"model":"gpt-4","choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"function":{"name":"get_weather"}}]},"finish_reason":null}]}

data: {"id":"chatcmpl-deferred","object":"chat.completion.chunk","created":1,"model":"gpt-4","choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"function":{"arguments":"ation\":\"NYC\"}"}}]},"finish_reason":null}]}

data: {"id":"chatcmpl-deferred","object":"chat.completion.chunk","created":1,"model":"gpt-4","choices":[{"index":0,"delta":{},"finish_reason":"tool_calls"}],"usage":{"prompt_tokens":1,"completion_tokens":1,"total_tokens":2}}

data: [DONE]
`

	var buf bytes.Buffer
	_, err := StreamOpenAIResponseToAnthropic(strings.NewReader(openAIStream), &buf)
	require.NoError(t, err)

	output := buf.String()
	require.Contains(t, output, `"name":"get_weather"`)
	require.Contains(t, output, `"id":"call_d"`)

	// content_block_start 之前缓存的参数应在 start 之后以 input_json_delta 形式补发。
	startIdx := strings.Index(output, `"type":"tool_use"`)
	require.GreaterOrEqual(t, startIdx, 0)
	require.Contains(t, output[startIdx:], `\"loc`)
}

func TestStreamOpenAIResponseToAnthropic_InvalidToolArgsRawString(t *testing.T) {
	openAIStream := `data: {"id":"chatcmpl-invalid","object":"chat.completion.chunk","created":1,"model":"gpt-4","choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"id":"call_i","type":"function","function":{"name":"fn"}}]},"finish_reason":null}]}

data: {"id":"chatcmpl-invalid","object":"chat.completion.chunk","created":1,"model":"gpt-4","choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"function":{"arguments":"not-json"}}]},"finish_reason":null}]}

data: {"id":"chatcmpl-invalid","object":"chat.completion.chunk","created":1,"model":"gpt-4","choices":[{"index":0,"delta":{},"finish_reason":"stop"}]}

data: [DONE]
`

	var buf bytes.Buffer
	_, err := StreamOpenAIResponseToAnthropic(strings.NewReader(openAIStream), &buf)
	require.NoError(t, err)

	output := buf.String()
	// tool_use 块的 start 事件必须存在；input 字段在 start 时为空（json.RawMessage("")）
	// 序列化时被省略，delta 自身承载有效 JSON 片段。
	assert.Contains(t, output, `"type":"tool_use","id":"call_i","name":"fn"`)
	assert.Contains(t, output, `"partial_json":"not-json"`)
}

func TestStreamOpenAIResponseToAnthropic_MultipleChoices(t *testing.T) {
	openAIStream := `data: {"id":"chatcmpl-multi","object":"chat.completion.chunk","created":1,"model":"gpt-4","choices":[{"index":0,"delta":{"content":"A"}},{"index":1,"delta":{"content":"B"}}]}

data: [DONE]
`

	var buf bytes.Buffer
	_, err := StreamOpenAIResponseToAnthropic(strings.NewReader(openAIStream), &buf)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "multiple non-empty choices")
}

func TestStreamOpenAIResponseToAnthropic_Reasoning(t *testing.T) {
	openAIStream := `data: {"id":"chatcmpl-reason","object":"chat.completion.chunk","created":1677652288,"model":"gpt-4o-mini","choices":[{"index":0,"delta":{"role":"assistant"},"finish_reason":null}]}

data: {"id":"chatcmpl-reason","object":"chat.completion.chunk","created":1677652288,"model":"gpt-4o-mini","choices":[{"index":0,"delta":{"reasoning":"Let me think"},"finish_reason":null}]}

data: {"id":"chatcmpl-reason","object":"chat.completion.chunk","created":1677652288,"model":"gpt-4o-mini","choices":[{"index":0,"delta":{"content":"Done"},"finish_reason":null}]}

data: {"id":"chatcmpl-reason","object":"chat.completion.chunk","created":1677652288,"model":"gpt-4o-mini","choices":[{"index":0,"delta":{},"finish_reason":"stop"}],"usage":{"prompt_tokens":5,"completion_tokens":3,"total_tokens":8}}

data: [DONE]
`

	var buf bytes.Buffer
	_, err := StreamOpenAIResponseToAnthropic(strings.NewReader(openAIStream), &buf)
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "event: content_block_delta")
	assert.Contains(t, output, `"thinking":"Let me think"`)
	assert.Contains(t, output, `"text":"Done"`)
}

func TestStreamOpenAIResponseToAnthropic_Thinking(t *testing.T) {
	openAIStream := `data: {"id":"chatcmpl-thinking","object":"chat.completion.chunk","created":1,"model":"deepseek-v4-pro","choices":[{"index":0,"delta":{"role":"assistant"},"finish_reason":null}]}

data: {"id":"chatcmpl-thinking","object":"chat.completion.chunk","created":1,"model":"deepseek-v4-pro","choices":[{"index":0,"delta":{"content":null,"reasoning_content":"some reasoning"},"finish_reason":null}]}

data: {"id":"chatcmpl-thinking","object":"chat.completion.chunk","created":1,"model":"deepseek-v4-pro","choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"id":"call_1","type":"function","function":{"name":"bash","arguments":""}}]},"finish_reason":null}]}

data: {"id":"chatcmpl-thinking","object":"chat.completion.chunk","created":1,"model":"deepseek-v4-pro","choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"function":{"arguments":"{\"command\":\"ls\"}"}}]},"finish_reason":null}]}

data: {"id":"chatcmpl-thinking","object":"chat.completion.chunk","created":1,"model":"deepseek-v4-pro","choices":[{"index":0,"delta":{},"finish_reason":"tool_calls"}],"usage":{"prompt_tokens":10,"completion_tokens":5,"total_tokens":15}}

data: [DONE]
`

	var buf bytes.Buffer
	_, err := StreamOpenAIResponseToAnthropic(strings.NewReader(openAIStream), &buf)
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "\"type\":\"thinking\"")
	assert.Contains(t, output, "thinking_delta")
	assert.Contains(t, output, "\"thinking\":\"some reasoning\"")
	assert.Contains(t, output, "\"type\":\"tool_use\"")
	// 验证 Anthropic 标准事件类型
	assert.Contains(t, output, "event: content_block_delta")
	assert.Contains(t, output, "event: content_block_stop")
}

func TestStreamOpenAIResponseToAnthropic_SequentialToolCalls(t *testing.T) {
	openAIStream := `data: {"id":"chatcmpl-seq","object":"chat.completion.chunk","created":1,"model":"gpt-4","choices":[{"index":0,"delta":{"role":"assistant"},"finish_reason":null}]}
data: {"id":"chatcmpl-seq","object":"chat.completion.chunk","created":1,"model":"gpt-4","choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"id":"call_0","type":"function","function":{"name":"get_weather","arguments":""}}]},"finish_reason":null}]}
data: {"id":"chatcmpl-seq","object":"chat.completion.chunk","created":1,"model":"gpt-4","choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"function":{"arguments":"{\"city\""}}]},"finish_reason":null}]}
data: {"id":"chatcmpl-seq","object":"chat.completion.chunk","created":1,"model":"gpt-4","choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"function":{"arguments":":\"Beijing\"}"}}]},"finish_reason":null}]}
data: {"id":"chatcmpl-seq","object":"chat.completion.chunk","created":1,"model":"gpt-4","choices":[{"index":0,"delta":{"tool_calls":[{"index":1,"id":"call_1","type":"function","function":{"name":"get_time","arguments":""}}]},"finish_reason":null}]}
data: {"id":"chatcmpl-seq","object":"chat.completion.chunk","created":1,"model":"gpt-4","choices":[{"index":0,"delta":{"tool_calls":[{"index":1,"function":{"arguments":"{}"}}]},"finish_reason":null}]}
data: {"id":"chatcmpl-seq","object":"chat.completion.chunk","created":1,"model":"gpt-4","choices":[{"index":0,"delta":{},"finish_reason":"tool_calls"}]}
data: [DONE]
`

	var buf bytes.Buffer
	_, err := StreamOpenAIResponseToAnthropic(strings.NewReader(openAIStream), &buf)
	require.NoError(t, err)

	output := buf.String()
	// Should start tool 0
	assert.Contains(t, output, `"name":"get_weather"`)
	// Should stop tool 0
	// Should start tool 1
	assert.Contains(t, output, `"name":"get_time"`)
	// Should stop tool 1
}

func TestStreamOpenAIResponseToAnthropic_InterleavedToolCalls(t *testing.T) {
	openAIStream := `data: {"id":"chatcmpl-int","object":"chat.completion.chunk","created":1,"model":"gpt-4","choices":[{"index":0,"delta":{"role":"assistant"},"finish_reason":null}]}
data: {"id":"chatcmpl-int","object":"chat.completion.chunk","created":1,"model":"gpt-4","choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"id":"call_0","type":"function","function":{"name":"get_weather","arguments":""}},{"index":1,"id":"call_1","type":"function","function":{"name":"get_time","arguments":""}}]},"finish_reason":null}]}
data: {"id":"chatcmpl-int","object":"chat.completion.chunk","created":1,"model":"gpt-4","choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"function":{"arguments":"{\"city\":\"Beijing\"}"}},{"index":1,"function":{"arguments":"{}"}}]},"finish_reason":null}]}
data: {"id":"chatcmpl-int","object":"chat.completion.chunk","created":1,"model":"gpt-4","choices":[{"index":0,"delta":{},"finish_reason":"tool_calls"}]}
data: [DONE]
`

	var buf bytes.Buffer
	_, err := StreamOpenAIResponseToAnthropic(strings.NewReader(openAIStream), &buf)
	require.NoError(t, err)

	output := buf.String()
	// Let's see what output we get
	t.Logf("Interleaved tool call output:\n%s", output)
}
