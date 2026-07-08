package protocol

import (
	"bytes"
	"encoding/json"
	"fmt"
	"runtime/debug"

	"github.com/maximhq/bifrost/core/providers/anthropic"
	"github.com/maximhq/bifrost/core/providers/openai"
	"github.com/maximhq/bifrost/core/schemas"
)

// ConvertAnthropicRequestToOpenAI 把 Anthropic Messages API 请求转换为 OpenAI Chat Completions 请求。
func ConvertAnthropicRequestToOpenAI(body []byte, targetProvider uint16) (out []byte, err error) {
	defer func() {
		if rec := recover(); rec != nil {
			err = fmt.Errorf("panic in ConvertAnthropicRequestToOpenAI: %v\n%s", rec, debug.Stack())
		}
	}()

	chatReq, err := convertAnthropicBodyToChatReq(body, targetProvider)
	if err != nil {
		return nil, err
	}

	stream := false
	var anthropicReq anthropic.AnthropicMessageRequest
	if err := json.Unmarshal(body, &anthropicReq); err == nil && anthropicReq.Stream != nil {
		stream = *anthropicReq.Stream
	}

	ctx := schemas.NewBifrostContext(nil, schemas.NoDeadline)
	openAIReq := openai.ToOpenAIChatRequest(ctx, chatReq)
	if openAIReq == nil {
		return nil, fmt.Errorf("failed to convert to openai request")
	}

	if stream {
		openAIReq.Stream = schemas.Ptr(true)
	}

	out, err = json.Marshal(openAIReq)
	if err != nil {
		return nil, fmt.Errorf("marshal openai request: %w", err)
	}
	return out, nil
}

// convertAnthropicBodyToChatReq 把 Anthropic 请求体转成 BifrostChatRequest。
func convertAnthropicBodyToChatReq(body []byte, targetProvider uint16) (*schemas.BifrostChatRequest, error) {
	var anthropicReq anthropic.AnthropicMessageRequest
	if err := json.Unmarshal(body, &anthropicReq); err != nil {
		return nil, fmt.Errorf("parse anthropic request: %w", err)
	}

	ctx := schemas.NewBifrostContext(nil, schemas.NoDeadline)
	responsesReq := anthropicReq.ToBifrostResponsesRequest(ctx)
	if responsesReq == nil {
		return nil, fmt.Errorf("ToBifrostResponsesRequest returned nil")
	}
	chatReq := responsesReq.ToChatRequest()
	if chatReq == nil {
		return nil, fmt.Errorf("ToChatRequest returned nil")
	}
	chatReq.Provider = providerTypeToBifrost(targetProvider)
	return chatReq, nil
}

// ConvertOpenAIResponseToAnthropic 把 OpenAI Chat Completions 响应转换为 Anthropic Messages API 响应。
func ConvertOpenAIResponseToAnthropic(body []byte) (out []byte, usage *schemas.BifrostLLMUsage, err error) {
	defer func() {
		if rec := recover(); rec != nil {
			err = fmt.Errorf("panic in ConvertOpenAIResponseToAnthropic: %v\n%s", rec, debug.Stack())
			out = nil
			usage = nil
		}
	}()

	var chatResp schemas.BifrostChatResponse
	if err = json.Unmarshal(body, &chatResp); err != nil {
		decErr := json.NewDecoder(bytes.NewReader(body)).Decode(&chatResp)
		if decErr != nil {
			previewLen := len(body)
			if previewLen > 500 {
				previewLen = 500
			}

			return nil, nil, fmt.Errorf("parse openai response: %w (body preview: %q)", err, string(body[:previewLen]))
		}
	}

	anthropicResp := anthropic.ToAnthropicChatResponse(&chatResp)
	if anthropicResp == nil {
		return nil, nil, fmt.Errorf("ToAnthropicChatResponse returned nil")
	}

	out, err = json.Marshal(anthropicResp)
	if err != nil {
		return nil, nil, fmt.Errorf("marshal anthropic response: %w", err)
	}
	return out, chatResp.Usage, nil
}

func AnthropicErrorResponse(message string, typ string) []byte {
	resp := map[string]any{
		"type": "error",
		"error": map[string]any{
			"type":    typ,
			"message": message,
		},
	}
	b, err := json.Marshal(resp)
	if err != nil {
		return []byte(`{"type":"error","error":{"type":"server_error","message":"failed to encode error response"}}`)
	}
	return b
}
