package protocol

import (
	"encoding/json"
	"testing"

	cgateway "github.com/computeflux/tee-provider/tee-chain/contracts/module/gateway"
	"github.com/maximhq/bifrost/core/schemas"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSynthesizeAnthropicServerTools_Bash(t *testing.T) {
	body := []byte(`{
		"model": "claude-3",
		"messages": [{"role": "user", "content": "hi"}],
		"tools": [
			{"type": "bash_20250124", "name": "Bash"}
		]
	}`)

	tools := ExtractBifrostChatTools(body, 0)
	require.Len(t, tools, 1)
	assert.Equal(t, schemas.ChatToolTypeFunction, tools[0].Type)
	assert.Equal(t, "Bash", tools[0].Function.Name)
	require.NotNil(t, tools[0].Function.Parameters)

	props := tools[0].Function.Parameters.Properties.ToMap()
	assert.Contains(t, props, "command")
	assert.Contains(t, props, "restart")
	assert.Equal(t, []string{"command"}, tools[0].Function.Parameters.Required)
}

func TestSynthesizeAnthropicServerTools_TextEditor(t *testing.T) {
	body := []byte(`{
		"model": "claude-3",
		"messages": [{"role": "user", "content": "hi"}],
		"tools": [
			{"type": "text_editor_20250124", "name": "str_replace_editor"}
		]
	}`)

	tools := ExtractBifrostChatTools(body, 0)
	require.Len(t, tools, 1)
	assert.Equal(t, "str_replace_editor", tools[0].Function.Name)

	props := tools[0].Function.Parameters.Properties.ToMap()
	assert.Contains(t, props, "command")
	assert.Contains(t, props, "path")
	assert.Contains(t, props, "old_str")
	assert.Equal(t, []string{"command", "path"}, tools[0].Function.Parameters.Required)
}

func TestSynthesizeAnthropicServerTools_DeduplicatesWithCustomTool(t *testing.T) {
	body := []byte(`{
		"model": "claude-3",
		"messages": [{"role": "user", "content": "hi"}],
		"tools": [
			{"type": "bash_20250124", "name": "Bash"},
			{
				"name": "Bash",
				"description": "custom bash",
				"input_schema": {"type": "object", "properties": {"cmd": {"type": "string"}}, "required": ["cmd"]}
			}
		]
	}`)

	tools := ExtractBifrostChatTools(body, 0)
	require.Len(t, tools, 1)
}

func TestConvertAnthropicRequestToOpenAI_IncludesBuiltinTool(t *testing.T) {
	body := []byte(`{
		"model": "MiniMax-M3",
		"messages": [{"role": "user", "content": "list files"}],
		"stream": false,
		"tools": [
			{"type": "bash_20250124", "name": "Bash"}
		]
	}`)

	out, err := ConvertAnthropicRequestToOpenAI(body, 0)
	require.NoError(t, err)

	var req struct {
		Tools []struct {
			Type     string `json:"type"`
			Function struct {
				Name       string          `json:"name"`
				Parameters json.RawMessage `json:"parameters"`
			} `json:"function"`
		} `json:"tools"`
	}
	require.NoError(t, json.Unmarshal(out, &req))
	require.Len(t, req.Tools, 1)
	assert.Equal(t, "function", req.Tools[0].Type)
	assert.Equal(t, "Bash", req.Tools[0].Function.Name)
	assert.Contains(t, string(req.Tools[0].Function.Parameters), "command")
}

func TestExtractBifrostChatTools_BuiltinReplacesNilParameters(t *testing.T) {
	body := []byte(`{
		"model": "claude-3",
		"messages": [{"role": "user", "content": "hi"}],
		"tools": [
			{"type": "bash_20250124", "name": "bash"}
		]
	}`)

	// 用 ProviderOpenAI 走 Bifrost 的 Chat 转换路径，该路径可能保留 name 但丢失 parameters。
	// 此测试确保合成出的 schema 会替换掉空 parameters。
	tools := ExtractBifrostChatTools(body, cgateway.ProviderOpenAI)
	require.Len(t, tools, 1)
	assert.Equal(t, "bash", tools[0].Function.Name)
	require.NotNil(t, tools[0].Function.Parameters)
	props := tools[0].Function.Parameters.Properties.ToMap()
	assert.Contains(t, props, "command")
	assert.Equal(t, []string{"command"}, tools[0].Function.Parameters.Required)
}
