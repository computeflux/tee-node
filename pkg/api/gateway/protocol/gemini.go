package protocol

import (
	"encoding/json"
	"fmt"

	cgateway "github.com/computeflux/tee-provider/tee-chain/contracts/module/gateway"
	"github.com/maximhq/bifrost/core/providers/gemini"
	bifrostOpenAI "github.com/maximhq/bifrost/core/providers/openai"
	"github.com/maximhq/bifrost/core/schemas"
)

// ConvertOpenAIRequestToGemini 把 OpenAI Chat Completions 请求转换为 Gemini 原生请求。
// 复用 Bifrost 的 Gemini 转换器，自动覆盖 tools、多模态内容块、response_format 等。
func ConvertOpenAIRequestToGemini(body []byte, targetProvider uint16) ([]byte, error) {
	if targetProvider != cgateway.ProviderGemini {
		return nil, fmt.Errorf("ConvertOpenAIRequestToGemini called with provider type %d", targetProvider)
	}

	var oaiReq bifrostOpenAI.OpenAIChatRequest
	if err := json.Unmarshal(body, &oaiReq); err != nil {
		return nil, fmt.Errorf("parse openai request: %w", err)
	}

	ctx := schemas.NewBifrostContext(nil, schemas.NoDeadline)
	chatReq := oaiReq.ToBifrostChatRequest(ctx)
	if chatReq == nil {
		return nil, fmt.Errorf("ToBifrostChatRequest returned nil")
	}
	chatReq.Provider = schemas.Gemini

	// OpenAI 的 max_tokens 等价于 max_completion_tokens
	if oaiReq.MaxTokens != nil {
		if chatReq.Params == nil {
			chatReq.Params = &schemas.ChatParameters{}
		}
		if chatReq.Params.MaxCompletionTokens == nil {
			chatReq.Params.MaxCompletionTokens = oaiReq.MaxTokens
		}
	}

	gemReq, err := gemini.ToGeminiChatCompletionRequest(ctx, chatReq)
	if err != nil {
		return nil, fmt.Errorf("convert to gemini request: %w", err)
	}
	if gemReq == nil {
		return nil, fmt.Errorf("ToGeminiChatCompletionRequest returned nil")
	}

	// 保持旧实现行为：默认放行所有安全类别，避免上游意外拦截。
	if len(gemReq.SafetySettings) == 0 {
		gemReq.SafetySettings = []gemini.SafetySetting{
			{Category: "HARM_CATEGORY_HARASSMENT", Threshold: "BLOCK_NONE"},
			{Category: "HARM_CATEGORY_HATE_SPEECH", Threshold: "BLOCK_NONE"},
			{Category: "HARM_CATEGORY_SEXUALLY_EXPLICIT", Threshold: "BLOCK_NONE"},
			{Category: "HARM_CATEGORY_DANGEROUS_CONTENT", Threshold: "BLOCK_NONE"},
		}
	}

	raw, err := json.Marshal(gemReq)
	if err != nil {
		return nil, fmt.Errorf("marshal gemini request: %w", err)
	}

	// Gemini 请求体不需要顶层 model 字段，删除它以保持与旧实现一致。
	return stripTopLevelJSONField(raw, "model")
}

// ConvertGeminiResponseToOpenAI 把 Gemini 原生响应转换为 OpenAI Chat Completions 响应。
func ConvertGeminiResponseToOpenAI(body []byte) ([]byte, *schemas.BifrostLLMUsage, error) {
	var gemResp gemini.GenerateContentResponse
	if err := json.Unmarshal(body, &gemResp); err != nil {
		return nil, nil, fmt.Errorf("parse gemini response: %w", err)
	}

	chatResp := gemResp.ToBifrostChatResponse()
	if chatResp == nil {
		return nil, nil, fmt.Errorf("ToBifrostChatResponse returned nil")
	}

	resp, err := buildOpenAIChatCompletionResponse(chatResp)
	if err != nil {
		return nil, nil, err
	}
	return resp, chatResp.Usage, nil
}

// stripTopLevelJSONField 从 JSON 对象中删除指定顶层字段。
func stripTopLevelJSONField(raw []byte, field string) ([]byte, error) {
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		return nil, err
	}
	delete(m, field)
	return json.Marshal(m)
}
