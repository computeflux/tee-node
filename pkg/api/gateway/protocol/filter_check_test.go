package protocol

import (
	"encoding/json"
	"testing"

	"github.com/maximhq/bifrost/core/schemas"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFilterCheck_BashHardenedFiltersDescription(t *testing.T) {
	openAIBody := []byte(`{
		"id": "chatcmpl-bash",
		"object": "chat.completion",
		"created": 1,
		"model": "deepseek-v4-pro",
		"choices": [{
			"index": 0,
			"message": {
				"role": "assistant",
				"content": "",
				"tool_calls": [{
					"id": "call_1",
					"type": "function",
					"function": {"name": "Bash", "arguments": "{\"command\":\"echo hello\",\"description\":\"test\"}"}
				}]
			},
			"finish_reason": "tool_calls"
		}],
		"usage": {"prompt_tokens": 10, "completion_tokens": 5, "total_tokens": 15}
	}`)

	tools := HardenToolParameters([]schemas.ChatTool{makeBashToolSchema()}, "deepseek-v4-pro")
	out, err := ConvertOpenAIResponseToAnthropic(openAIBody, true, tools...)
	require.NoError(t, err)

	var resp map[string]any
	require.NoError(t, json.Unmarshal(out, &resp))
	content := resp["content"].([]any)
	require.Len(t, content, 1)
	input := content[0].(map[string]any)["input"].(map[string]any)
	assert.Equal(t, "echo hello", input["command"])
	assert.NotContains(t, input, "description")
}
