package protocol

import (
	"encoding/json"
	"fmt"
	"runtime/debug"

	cgateway "github.com/computeflux/tee-provider/tee-chain/contracts/module/gateway"
	"github.com/maximhq/bifrost/core/providers/anthropic"
	"github.com/maximhq/bifrost/core/providers/openai"
	"github.com/maximhq/bifrost/core/schemas"
)

// ConvertAnthropicRequestToOpenAI 把 Anthropic Messages API 请求转换为 OpenAI Chat Completions 请求。
// targetProvider 是 tee-chain 的 ProviderType，用于决定最终 upstream 的兼容处理。
func ConvertAnthropicRequestToOpenAI(body []byte, targetProvider uint16) (out []byte, err error) {
	defer func() {
		if rec := recover(); rec != nil {
			err = fmt.Errorf("panic in ConvertAnthropicRequestToOpenAI: %v\n%s", rec, debug.Stack())
		}
	}()

	var anthropicReq anthropic.AnthropicMessageRequest
	if err = json.Unmarshal(body, &anthropicReq); err != nil {
		return nil, fmt.Errorf("parse anthropic request: %w", err)
	}

	stream := false
	if anthropicReq.Stream != nil {
		stream = *anthropicReq.Stream
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

	openAIReq := openai.ToOpenAIChatRequest(ctx, chatReq)
	if openAIReq == nil {
		return nil, fmt.Errorf("failed to convert to openai request")
	}

	// bifrost 的 Responses->Chat 转换不保留 stream 标志，需要显式透传。
	if stream {
		openAIReq.Stream = schemas.Ptr(true)
	}

	out, err = json.Marshal(openAIReq)
	if err != nil {
		return nil, fmt.Errorf("marshal openai request: %w", err)
	}
	return out, nil
}

// ConvertOpenAIResponseToAnthropic 把 OpenAI Chat Completions 响应转换为 Anthropic Messages API 响应。
func ConvertOpenAIResponseToAnthropic(body []byte) (out []byte, err error) {
	defer func() {
		if rec := recover(); rec != nil {
			err = fmt.Errorf("panic in ConvertOpenAIResponseToAnthropic: %v\n%s", rec, debug.Stack())
		}
	}()

	var chatResp schemas.BifrostChatResponse
	if err = json.Unmarshal(body, &chatResp); err != nil {
		return nil, fmt.Errorf("parse openai response: %w", err)
	}

	ctx := schemas.NewBifrostContext(nil, schemas.NoDeadline)
	responsesResp := chatResp.ToBifrostResponsesResponse()
	if responsesResp == nil {
		return nil, fmt.Errorf("ToBifrostResponsesResponse returned nil")
	}
	anthropicResp := anthropic.ToAnthropicResponsesResponse(ctx, responsesResp)
	if anthropicResp == nil {
		return nil, fmt.Errorf("ToAnthropicResponsesResponse returned nil")
	}

	out, err = json.Marshal(anthropicResp)
	if err != nil {
		return nil, fmt.Errorf("marshal anthropic response: %w", err)
	}
	return out, nil
}

// AnthropicErrorResponse 构造 Anthropic 格式的错误响应体。
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

// providerTypeToBifrost 把 tee-chain 的 ProviderType 数值映射为 bifrost 的 ModelProvider。
func providerTypeToBifrost(providerType uint16) schemas.ModelProvider {
	switch providerType {
	case cgateway.ProviderClaude:
		return schemas.Anthropic
	case cgateway.ProviderGemini:
		return schemas.Gemini
	case cgateway.ProviderOpenAI, cgateway.ProviderLocal, cgateway.ProviderPersonal:
		// 本地模型按 OpenAI 兼容格式处理
		return schemas.OpenAI
	default:
		return schemas.OpenAI
	}
}
