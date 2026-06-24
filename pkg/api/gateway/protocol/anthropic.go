package protocol

import (
	"bytes"
	"encoding/json"
	"fmt"
	"runtime/debug"
	"strings"

	"github.com/computeflux/tee-provider/pkg/util"
	cgateway "github.com/computeflux/tee-provider/tee-chain/contracts/module/gateway"
	"github.com/maximhq/bifrost/core/providers/anthropic"
	"github.com/maximhq/bifrost/core/providers/openai"
	"github.com/maximhq/bifrost/core/schemas"
)

// ConvertAnthropicRequestToOpenAI 把 Anthropic Messages API 请求转换为 OpenAI Chat Completions 请求。
// targetProvider 是 tee-chain 的 ProviderType，用于决定最终 upstream 的兼容处理。
// 可选传入 tools 覆盖转换后的工具 schema，用于在发向上游前做 schema 加固。
func ConvertAnthropicRequestToOpenAI(body []byte, targetProvider uint16, tools ...schemas.ChatTool) (out []byte, err error) {
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

	if chatReq.Params != nil {
		if len(tools) == 0 {
			// 外部未传入 tools 时（单测或直调场景），从原始请求里重新提取，
			// 包括把 Anthropic 内置 server/client tool 合成为 function tool。
			tools = ExtractBifrostChatTools(body, targetProvider)
		}
		chatReq.Params.Tools = tools
	}

	ctx := schemas.NewBifrostContext(nil, schemas.NoDeadline)
	openAIReq := openai.ToOpenAIChatRequest(ctx, chatReq)
	if openAIReq == nil {
		return nil, fmt.Errorf("failed to convert to openai request")
	}

	// DeepSeek 在 reasoning 模式下工具调用参数经常为空 {}，先关闭 reasoning
	// 参数以恢复工具调用的可靠性。
	if openAIReq.Reasoning != nil && strings.Contains(strings.ToLower(chatReq.Model), "deepseek") {
		// util.DebugLog("[protocol] disabling reasoning params for deepseek to improve tool calling")
		openAIReq.Reasoning = nil
	}

	// DeepSeek 在多工具长上下文中容易输出空 {}，追加一句系统级提醒约束参数生成。
	if strings.Contains(strings.ToLower(chatReq.Model), "deepseek") {
		reminder := "When calling a tool, always include all required parameters with valid non-empty values. Do not return empty JSON objects for tool calls."
		openAIReq.Messages = append(openAIReq.Messages, openai.OpenAIMessage{
			Role:    schemas.ChatMessageRoleSystem,
			Content: &schemas.ChatMessageContent{ContentStr: schemas.Ptr(reminder)},
		})
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
// 走 Bifrost Chat 路径：OpenAI JSON -> schemas.BifrostChatResponse -> 按 schema 修复 tool arguments ->
// anthropic.ToAnthropicChatResponse。
//
// dropThinking 用于在原始 Anthropic 请求没有启用 thinking 时，丢弃上游返回的 reasoning/thinking
// 内容。因为 Claude Code 等客户端如果没请求 thinking，收到没有 signature 的 thinking 块会报错。
func ConvertOpenAIResponseToAnthropic(body []byte, dropThinking bool, toolSchemas ...schemas.ChatTool) (out []byte, err error) {
	defer func() {
		if rec := recover(); rec != nil {
			err = fmt.Errorf("panic in ConvertOpenAIResponseToAnthropic: %v\n%s", rec, debug.Stack())
		}
	}()

	var chatResp schemas.BifrostChatResponse
	if err = json.Unmarshal(body, &chatResp); err != nil {
		// 部分上游会在合法 JSON 后附加非 JSON 字节（如控制字符、编码问题或代理尾巴）。
		// 先尝试用 Decoder 只读取第一个顶层 JSON 值，其余忽略。
		decErr := json.NewDecoder(bytes.NewReader(body)).Decode(&chatResp)
		if decErr != nil {
			previewLen := len(body)
			if previewLen > 500 {
				previewLen = 500
			}
			util.DebugLog("[protocol] ConvertOpenAIResponseToAnthropic parse error: %v, len=%d, preview=%q", err, len(body), string(body[:previewLen]))
			return nil, fmt.Errorf("parse openai response: %w", err)
		}
		util.DebugLog("[protocol] ConvertOpenAIResponseToAnthropic recovered from trailing garbage after first JSON value")
	}

	FilterBifrostToolCalls(&chatResp, toolSchemas)

	if dropThinking {
		stripThinkingFromChatResponse(&chatResp)
	}

	anthropicResp := anthropic.ToAnthropicChatResponse(&chatResp)
	if anthropicResp == nil {
		return nil, fmt.Errorf("ToAnthropicChatResponse returned nil")
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
	case cgateway.ProviderOpenAI, cgateway.ProviderPersonal:
		// 本地模型按 OpenAI 兼容格式处理
		return schemas.OpenAI
	default:
		return schemas.OpenAI
	}
}

// stripThinkingFromChatResponse 移除 BifrostChatResponse 中的 reasoning/thinking 信息，
// 避免把没有 signature 的 thinking 块转发给没启用 thinking 的 Anthropic 客户端。
func stripThinkingFromChatResponse(resp *schemas.BifrostChatResponse) {
	if resp == nil {
		return
	}
	for i := range resp.Choices {
		choice := &resp.Choices[i]
		if choice.ChatNonStreamResponseChoice == nil || choice.ChatNonStreamResponseChoice.Message == nil {
			continue
		}
		msg := choice.ChatNonStreamResponseChoice.Message
		if msg.ChatAssistantMessage == nil {
			continue
		}
		msg.ChatAssistantMessage.Reasoning = nil
		msg.ChatAssistantMessage.ReasoningDetails = nil
	}
}
