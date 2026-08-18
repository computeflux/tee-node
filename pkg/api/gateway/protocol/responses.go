package protocol

import (
	"context"
	"encoding/json"
	"fmt"

	cgateway "github.com/computeflux/tee-provider/tee-chain/runtime/module/gateway"
	bifrostOpenAI "github.com/maximhq/bifrost/core/providers/openai"
	"github.com/maximhq/bifrost/core/schemas"
)

// ── 其他协议格式 → OpenAI Responses API ────────────────────
//
// 方向约束：仅支持「其他格式 → Responses API」，不支持「Responses API → 其他格式」。
// 统一经 Bifrost 的 ChatRequest 中转：其他格式 → BifrostChatRequest →
// BifrostResponsesRequest → OpenAIResponsesRequest(Responses wire 格式)。

// ToResponsesRequest 把 BifrostChatRequest 转换为 OpenAI Responses API wire 格式的请求体。
func ToResponsesRequest(chatReq *schemas.BifrostChatRequest) ([]byte, error) {
	if chatReq == nil {
		return nil, fmt.Errorf("ToResponsesType: nil chat request")
	}
	ctx := schemas.NewBifrostContext(context.Background(), schemas.NoDeadline)
	respReq := chatReq.ToResponsesRequest()
	if respReq == nil {
		return nil, fmt.Errorf("ToResponsesRequest returned nil")
	}
	oaiRespReq := bifrostOpenAI.ToOpenAIResponsesRequest(ctx, respReq)
	if oaiRespReq == nil {
		return nil, fmt.Errorf("ToOpenAIResponsesRequest returned nil")
	}
	raw, err := json.Marshal(oaiRespReq)
	if err != nil {
		return nil, fmt.Errorf("marshal responses request: %w", err)
	}
	return raw, nil
}

// ConvertOpenAIRequestToResponses 把 OpenAI Chat Completions 请求转换为 Responses API 请求。
func ConvertOpenAIRequestToResponses(body []byte, targetProvider uint16) ([]byte, error) {
	if targetProvider != cgateway.OpenAIResponsesApi {
		return nil, fmt.Errorf("ConvertOpenAIRequestToResponses called with provider type %d", targetProvider)
	}
	var oaiReq bifrostOpenAI.OpenAIChatRequest
	if err := json.Unmarshal(body, &oaiReq); err != nil {
		return nil, fmt.Errorf("parse openai request: %w", err)
	}
	ctx := schemas.NewBifrostContext(context.Background(), schemas.NoDeadline)
	chatReq := oaiReq.ToBifrostChatRequest(ctx)
	if chatReq == nil {
		return nil, fmt.Errorf("ToBifrostChatRequest returned nil")
	}
	chatReq.Provider = schemas.OpenAI
	return ToResponsesRequest(chatReq)
}

// ConvertAnthropicRequestToResponses 把 Anthropic Messages 请求转换为 Responses API 请求。
func ConvertAnthropicRequestToResponses(body []byte, targetProvider uint16) ([]byte, error) {
	if targetProvider != cgateway.OpenAIResponsesApi {
		return nil, fmt.Errorf("ConvertAnthropicRequestToResponses called with provider type %d", targetProvider)
	}
	// 复用以有的 Anthropic → OpenAI Chat 转换，得到 ChatRequest JSON 后再转 Responses。
	chatJSON, err := ConvertAnthropicRequestToOpenAI(body, targetProvider)
	if err != nil {
		return nil, err
	}
	return ConvertOpenAIRequestToResponses(chatJSON, targetProvider)
}

// ConvertGeminiRequestToResponses 把 Gemini 原生请求转换为 Responses API 请求。
// 注意：bifrost 仅提供 ChatRequest→Gemini 转换，无 Gemini→ChatRequest 反解码，故暂不支持。
func ConvertGeminiRequestToResponses(body []byte, targetProvider uint16) ([]byte, error) {
	return nil, fmt.Errorf("ConvertGeminiRequestToResponses: unsupported conversion from gemini to responses api")
}

// ── Responses API → 其他协议格式（响应转换）─────────────────

// parseResponsesResponseBody 解析 Responses API 响应体为 BifrostChatResponse。
func parseResponsesResponseBody(body []byte) (*schemas.BifrostChatResponse, error) {
	var resp schemas.BifrostResponsesResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("parse responses response: %w", err)
	}
	chatResp := resp.ToBifrostChatResponse()
	if chatResp == nil {
		return nil, fmt.Errorf("ToBifrostChatResponse returned nil")
	}
	return chatResp, nil
}

// ConvertResponsesResponseToOpenAI 把 Responses API 响应转换为 OpenAI Chat Completions 响应。
func ConvertResponsesResponseToOpenAI(body []byte) ([]byte, *schemas.BifrostLLMUsage, error) {
	chatResp, err := parseResponsesResponseBody(body)
	if err != nil {
		return nil, nil, err
	}
	raw, err := json.Marshal(chatResp)
	if err != nil {
		return nil, nil, fmt.Errorf("marshal openai response: %w", err)
	}
	return raw, chatResp.Usage, nil
}

// ConvertResponsesResponseToAnthropic 把 Responses API 响应转换为 Anthropic Messages 响应。
func ConvertResponsesResponseToAnthropic(body []byte) ([]byte, *schemas.BifrostLLMUsage, error) {
	openAIJSON, _, err := ConvertResponsesResponseToOpenAI(body)
	if err != nil {
		return nil, nil, err
	}
	return ConvertOpenAIResponseToAnthropic(openAIJSON)
}

// ConvertResponsesResponseToGemini 把 Responses API 响应转换为 Gemini 原生响应。
// 注意：bifrost 无 Chat→Gemini 响应转换器，故暂不支持。
func ConvertResponsesResponseToGemini(body []byte) ([]byte, *schemas.BifrostLLMUsage, error) {
	return nil, nil, fmt.Errorf("ConvertResponsesResponseToGemini: unsupported conversion from responses api to gemini")
}
