package protocol

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConvertOpenAIRequestToGemini_Text(t *testing.T) {
	body := []byte(`{
		"model": "gemini-2.5-flash",
		"messages": [
			{"role": "system", "content": "You are a helpful assistant."},
			{"role": "user", "content": "Hello"}
		],
		"temperature": 0.5,
		"max_tokens": 256
	}`)

	out, err := ConvertOpenAIRequestToGemini(body, 3)
	require.NoError(t, err)

	var req map[string]any
	require.NoError(t, json.Unmarshal(out, &req))

	assert.Nil(t, req["model"], "model should be stripped from body")

	si, ok := req["systemInstruction"].(map[string]any)
	require.True(t, ok, "system instruction should be present")
	parts := si["parts"].([]any)
	require.Len(t, parts, 1)
	assert.Equal(t, "You are a helpful assistant.", parts[0].(map[string]any)["text"])

	contents, ok := req["contents"].([]any)
	require.True(t, ok)
	require.Len(t, contents, 1)
	userParts := contents[0].(map[string]any)["parts"].([]any)
	require.Len(t, userParts, 1)
	assert.Equal(t, "Hello", userParts[0].(map[string]any)["text"])

	genConfig := req["generationConfig"].(map[string]any)
	assert.Equal(t, float64(0.5), genConfig["temperature"])
	assert.Equal(t, float64(256), genConfig["maxOutputTokens"])

	safety, ok := req["safetySettings"].([]any)
	require.True(t, ok)
	assert.Len(t, safety, 4)
}

func TestConvertOpenAIRequestToGemini_Image(t *testing.T) {
	body := []byte(`{
		"model": "gemini-2.5-flash",
		"messages": [{
			"role": "user",
			"content": [
				{"type": "image_url", "image_url": {"url": "data:image/png;base64,iVBORw0KGgo="}},
				{"type": "text", "text": "describe"}
			]
		}]
	}`)

	out, err := ConvertOpenAIRequestToGemini(body, 3)
	require.NoError(t, err)

	var req map[string]any
	require.NoError(t, json.Unmarshal(out, &req))

	contents := req["contents"].([]any)
	parts := contents[0].(map[string]any)["parts"].([]any)
	require.Len(t, parts, 2)

	inline := parts[0].(map[string]any)["inlineData"].(map[string]any)
	assert.Equal(t, "image/png", inline["mimeType"])
	assert.NotEmpty(t, inline["data"])
	assert.Equal(t, "describe", parts[1].(map[string]any)["text"])
}

func TestConvertOpenAIRequestToGemini_Audio(t *testing.T) {
	body := []byte(`{
		"model": "gemini-2.5-flash",
		"messages": [{
			"role": "user",
			"content": [
				{"type": "input_audio", "input_audio": {"data": "aGVsbG8=", "format": "mp3"}}
			]
		}]
	}`)

	out, err := ConvertOpenAIRequestToGemini(body, 3)
	require.NoError(t, err)

	var req map[string]any
	require.NoError(t, json.Unmarshal(out, &req))

	parts := req["contents"].([]any)[0].(map[string]any)["parts"].([]any)
	require.Len(t, parts, 1)
	inline := parts[0].(map[string]any)["inlineData"].(map[string]any)
	assert.Equal(t, "audio/mp3", inline["mimeType"])
	assert.NotEmpty(t, inline["data"])
}

func TestConvertOpenAIRequestToGemini_File(t *testing.T) {
	body := []byte(`{
		"model": "gemini-2.5-flash",
		"messages": [{
			"role": "user",
			"content": [
				{"type": "file", "file": {"file_data": "JVBERi0xLjQKJeLjz9MKMyAwIG9iago8PC9UeXBlL1BhZ2UvUGFyZW50IDIgMCBSP", "file_type": "application/pdf"}}
			]
		}]
	}`)

	out, err := ConvertOpenAIRequestToGemini(body, 3)
	require.NoError(t, err)

	var req map[string]any
	require.NoError(t, json.Unmarshal(out, &req))

	parts := req["contents"].([]any)[0].(map[string]any)["parts"].([]any)
	require.Len(t, parts, 1)
	inline := parts[0].(map[string]any)["inlineData"].(map[string]any)
	assert.Equal(t, "application/pdf", inline["mimeType"])
	assert.NotEmpty(t, inline["data"])
}

func TestConvertOpenAIRequestToGemini_Tool(t *testing.T) {
	body := []byte(`{
		"model": "gemini-2.5-flash",
		"messages": [{"role": "user", "content": "What's the weather?"}],
		"tools": [{
			"type": "function",
			"function": {
				"name": "get_weather",
				"description": "Get weather",
				"parameters": {"type": "object", "properties": {"city": {"type": "string"}}, "required": ["city"]}
			}
		}],
		"tool_choice": "required"
	}`)

	out, err := ConvertOpenAIRequestToGemini(body, 3)
	require.NoError(t, err)

	var req map[string]any
	require.NoError(t, json.Unmarshal(out, &req))

	tools := req["tools"].([]any)
	require.Len(t, tools, 1)
	decls := tools[0].(map[string]any)["functionDeclarations"].([]any)
	require.Len(t, decls, 1)
	assert.Equal(t, "get_weather", decls[0].(map[string]any)["name"])

	toolConfig := req["toolConfig"].(map[string]any)
	assert.Equal(t, "ANY", toolConfig["functionCallingConfig"].(map[string]any)["mode"])
}

func TestConvertOpenAIRequestToGemini_ResponseFormatJSONSchema(t *testing.T) {
	body := []byte(`{
		"model": "gemini-2.5-flash",
		"messages": [{"role": "user", "content": "Give me a JSON object"}],
		"response_format": {
			"type": "json_schema",
			"json_schema": {
				"name": "answer",
				"schema": {"type": "object", "properties": {"value": {"type": "string"}}, "required": ["value"]}
			}
		}
	}`)

	out, err := ConvertOpenAIRequestToGemini(body, 3)
	require.NoError(t, err)

	var req map[string]any
	require.NoError(t, json.Unmarshal(out, &req))

	genConfig := req["generationConfig"].(map[string]any)
	assert.Equal(t, "application/json", genConfig["responseMimeType"])
	assert.NotNil(t, genConfig["responseJsonSchema"])
}

func TestConvertOpenAIRequestToGemini_MaxTokensFallback(t *testing.T) {
	body := []byte(`{
		"model": "gemini-2.5-flash",
		"messages": [{"role": "user", "content": "Hi"}],
		"max_tokens": 128
	}`)

	out, err := ConvertOpenAIRequestToGemini(body, 3)
	require.NoError(t, err)

	var req map[string]any
	require.NoError(t, json.Unmarshal(out, &req))
	genConfig := req["generationConfig"].(map[string]any)
	assert.Equal(t, float64(128), genConfig["maxOutputTokens"])
}

func TestConvertGeminiResponseToOpenAI_Text(t *testing.T) {
	body := []byte(`{
		"candidates": [{
			"content": {"parts": [{"text": "Hello there"}], "role": "model"},
			"finishReason": "STOP"
		}],
		"usageMetadata": {
			"promptTokenCount": 10,
			"candidatesTokenCount": 2,
			"totalTokenCount": 12
		}
	}`)

	out, err := ConvertGeminiResponseToOpenAI(body)
	require.NoError(t, err)

	var resp map[string]any
	require.NoError(t, json.Unmarshal(out, &resp))
	assert.Equal(t, "chat.completion", resp["object"])

	choices := resp["choices"].([]any)
	require.Len(t, choices, 1)
	msg := choices[0].(map[string]any)["message"].(map[string]any)
	assert.Equal(t, "assistant", msg["role"])
	assert.Equal(t, "Hello there", msg["content"])
	assert.Equal(t, "stop", choices[0].(map[string]any)["finish_reason"])

	usage := resp["usage"].(map[string]any)
	assert.Equal(t, float64(10), usage["prompt_tokens"])
	assert.Equal(t, float64(2), usage["completion_tokens"])
}

func TestConvertGeminiResponseToOpenAI_ToolCall(t *testing.T) {
	body := []byte(`{
		"candidates": [{
			"content": {
				"parts": [{
					"functionCall": {"name": "get_weather", "args": {"city": "Beijing"}}
				}],
				"role": "model"
			},
			"finishReason": "STOP"
		}],
		"usageMetadata": {"promptTokenCount": 15, "candidatesTokenCount": 8, "totalTokenCount": 23}
	}`)

	out, err := ConvertGeminiResponseToOpenAI(body)
	require.NoError(t, err)

	var resp map[string]any
	require.NoError(t, json.Unmarshal(out, &resp))

	choices := resp["choices"].([]any)
	require.Len(t, choices, 1)
	msg := choices[0].(map[string]any)["message"].(map[string]any)
	toolCalls := msg["tool_calls"].([]any)
	require.Len(t, toolCalls, 1)
	fn := toolCalls[0].(map[string]any)["function"].(map[string]any)
	assert.Equal(t, "get_weather", fn["name"])
	assert.Contains(t, fn["arguments"], "Beijing")
}

func TestConvertGeminiResponseToOpenAI_Reasoning(t *testing.T) {
	body := []byte(`{
		"candidates": [{
			"content": {
				"parts": [
					{"text": "Let me think", "thought": true},
					{"text": "Answer"}
				],
				"role": "model"
			},
			"finishReason": "STOP"
		}],
		"usageMetadata": {"promptTokenCount": 5, "candidatesTokenCount": 3, "totalTokenCount": 8}
	}`)

	out, err := ConvertGeminiResponseToOpenAI(body)
	require.NoError(t, err)

	var resp map[string]any
	require.NoError(t, json.Unmarshal(out, &resp))

	choices := resp["choices"].([]any)
	msg := choices[0].(map[string]any)["message"].(map[string]any)
	assert.Equal(t, "Answer", msg["content"])
	assert.Equal(t, "Let me think", msg["reasoning_content"])
}
