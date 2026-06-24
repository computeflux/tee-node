package protocol

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/maximhq/bifrost/core/schemas"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExtractBifrostChatTools(t *testing.T) {
	body := []byte(`{
		"model": "claude-3",
		"messages": [{"role": "user", "content": "hi"}],
		"tools": [
			{
				"name": "Bash",
				"description": "Run shell commands",
				"input_schema": {
					"type": "object",
					"properties": {
						"command": {"type": "string"},
						"timeout": {"type": "integer"}
					},
					"required": ["command"]
				}
			}
		]
	}`)

	tools := ExtractBifrostChatTools(body, 0) // ProviderType OpenAI
	require.Len(t, tools, 1)
	assert.Equal(t, schemas.ChatToolTypeFunction, tools[0].Type)
	assert.Equal(t, "Bash", tools[0].Function.Name)
	require.NotNil(t, tools[0].Function.Parameters)
	assert.Equal(t, "object", tools[0].Function.Parameters.Type)
	assert.Equal(t, []string{"command"}, tools[0].Function.Parameters.Required)
	props := tools[0].Function.Parameters.Properties.ToMap()
	assert.Contains(t, props, "command")
	assert.Contains(t, props, "timeout")
}

func TestExtractBifrostChatTools_InvalidBody(t *testing.T) {
	tools := ExtractBifrostChatTools([]byte(`not json`), 0)
	assert.Nil(t, tools)
}

func TestHardenToolParameters_DeepSeek(t *testing.T) {
	tool := makeBashToolSchema()
	hardened := HardenToolParameters([]schemas.ChatTool{tool}, "deepseek-v4-pro")
	require.Len(t, hardened, 1)
	params := hardened[0].Function.Parameters
	cmdProp := propMap(params.Properties, "command")
	assert.Equal(t, float64(1), cmdProp["minLength"])
}

func TestHardenToolParameters_ObjectAdditionalProperties(t *testing.T) {
	tool := schemas.ChatTool{
		Type: schemas.ChatToolTypeFunction,
		Function: &schemas.ChatToolFunction{
			Name: "wrapper",
			Parameters: &schemas.ToolFunctionParameters{
				Type: "object",
				Properties: schemas.OrderedMapFromMap(map[string]any{
					"config": map[string]any{
						"type":       "object",
						"properties": map[string]any{"key": map[string]any{"type": "string"}},
						"required":   []string{"key"},
					},
				}),
				Required: []string{"config"},
			},
		},
	}
	hardened := HardenToolParameters([]schemas.ChatTool{tool}, "minimax-m3")
	params := hardened[0].Function.Parameters
	configProp := propMap(params.Properties, "config")
	assert.Equal(t, false, configProp["additionalProperties"])
}

func TestHardenToolParameters_SkipsNonTargetModels(t *testing.T) {
	tool := makeBashToolSchema()
	hardened := HardenToolParameters([]schemas.ChatTool{tool}, "gpt-4o")
	require.Len(t, hardened, 1)
	params := hardened[0].Function.Parameters
	cmdProp := propMap(params.Properties, "command")
	assert.Nil(t, cmdProp["minLength"])
}

func TestFilterBifrostToolCalls_RemovesUnknownFields(t *testing.T) {
	resp := makeChatResponseWithToolCall("Bash", `{"command":"ls","description":"list files"}`)
	FilterBifrostToolCalls(resp, []schemas.ChatTool{makeBashToolSchema()})
	args := resp.Choices[0].ChatNonStreamResponseChoice.Message.ChatAssistantMessage.ToolCalls[0].Function.Arguments
	var input map[string]any
	require.NoError(t, json.Unmarshal([]byte(args), &input))
	assert.Equal(t, "ls", input["command"])
	assert.NotContains(t, input, "description")
}

func TestFilterBifrostToolCalls_FillsRequiredFields(t *testing.T) {
	resp := makeChatResponseWithToolCall("Bash", `{}`)
	FilterBifrostToolCalls(resp, []schemas.ChatTool{makeBashToolSchema()})
	args := resp.Choices[0].ChatNonStreamResponseChoice.Message.ChatAssistantMessage.ToolCalls[0].Function.Arguments
	var input map[string]any
	require.NoError(t, json.Unmarshal([]byte(args), &input))
	assert.Equal(t, "", input["command"])
}

func TestFilterBifrostToolCalls_NestedObject(t *testing.T) {
	tool := schemas.ChatTool{
		Type: schemas.ChatToolTypeFunction,
		Function: &schemas.ChatToolFunction{
			Name: "wrapper",
			Parameters: &schemas.ToolFunctionParameters{
				Type: "object",
				Properties: schemas.OrderedMapFromMap(map[string]any{
					"config": map[string]any{
						"type":       "object",
						"properties": map[string]any{"key": map[string]any{"type": "string"}},
						"required":   []string{"key"},
					},
				}),
				Required: []string{"config"},
			},
		},
	}
	resp := makeChatResponseWithToolCall("wrapper", `{"config":{},"extra":"drop"}`)
	FilterBifrostToolCalls(resp, []schemas.ChatTool{tool})
	args := resp.Choices[0].ChatNonStreamResponseChoice.Message.ChatAssistantMessage.ToolCalls[0].Function.Arguments
	var input map[string]any
	require.NoError(t, json.Unmarshal([]byte(args), &input))
	config := input["config"].(map[string]any)
	assert.Equal(t, "", config["key"])
	assert.NotContains(t, input, "extra")
}

func TestHardenToolParameters_DeepSeekStripsAdditionalProperties(t *testing.T) {
	tool := schemas.ChatTool{
		Type: schemas.ChatToolTypeFunction,
		Function: &schemas.ChatToolFunction{
			Name: "Read",
			Parameters: &schemas.ToolFunctionParameters{
				Type: "object",
				Properties: schemas.OrderedMapFromMap(map[string]any{
					"file_path": map[string]any{"type": "string"},
				}),
				Required: []string{"file_path"},
				AdditionalProperties: &schemas.AdditionalPropertiesStruct{
					AdditionalPropertiesBool: schemas.Ptr(false),
				},
			},
		},
	}
	hardened := HardenToolParameters([]schemas.ChatTool{tool}, "deepseek-v4-pro")
	params := hardened[0].Function.Parameters
	assert.Nil(t, params.AdditionalProperties)
}

func TestHardenToolParameters_CompressesLongDescriptions(t *testing.T) {
	tool := schemas.ChatTool{
		Type: schemas.ChatToolTypeFunction,
		Function: &schemas.ChatToolFunction{
			Name:        "Agent",
			Description: schemas.Ptr(strings.Repeat("a", 600)),
			Parameters: &schemas.ToolFunctionParameters{
				Type:       "object",
				Properties: schemas.OrderedMapFromMap(map[string]any{}),
			},
		},
	}
	hardened := HardenToolParameters([]schemas.ChatTool{tool}, "deepseek-v4-pro")
	desc := *hardened[0].Function.Description
	assert.LessOrEqual(t, len([]rune(desc)), 510)
	assert.True(t, strings.HasSuffix(desc, " ..."))
}

func TestRepairJSONArguments(t *testing.T) {
	cases := []struct {
		name     string
		input    string
		expected string
	}{
		{"valid", `{"command":"ls"}`, `{"command":"ls"}`},
		{"missing closing brace", `{"command":"ls"`, `{"command":"ls"}`},
		{"missing closing quote", `{"command":"ls`, `{"command":"ls"}`},
		{"empty", `{}`, `{}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := repairJSONArguments(tc.input)
			assert.JSONEq(t, tc.expected, got)
		})
	}
}

// propMap 从 OrderedMap 的 properties 中读取某个 property 的 map 表示，
// 兼容 map[string]any 和 *schemas.OrderedMap 两种值类型。
func propMap(om *schemas.OrderedMap, key string) map[string]any {
	v, _ := om.Get(key)
	switch x := v.(type) {
	case map[string]any:
		return x
	case *schemas.OrderedMap:
		return x.ToMap()
	case schemas.OrderedMap:
		return x.ToMap()
	}
	return nil
}

func makeChatResponseWithToolCall(name, args string) *schemas.BifrostChatResponse {
	id := "call_1"
	return &schemas.BifrostChatResponse{
		Choices: []schemas.BifrostResponseChoice{
			{
				ChatNonStreamResponseChoice: &schemas.ChatNonStreamResponseChoice{
					Message: &schemas.ChatMessage{
						ChatAssistantMessage: &schemas.ChatAssistantMessage{
							ToolCalls: []schemas.ChatAssistantMessageToolCall{
								{
									ID:   &id,
									Type: schemas.Ptr(string(schemas.ChatToolTypeFunction)),
									Function: schemas.ChatAssistantMessageToolCallFunction{
										Name:      &name,
										Arguments: args,
									},
								},
							},
						},
					},
				},
			},
		},
	}
}
