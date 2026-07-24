package protocol

import (
	"context"
	"encoding/json"
	"fmt"
	"runtime/debug"
	"strings"
	"time"

	"github.com/computeflux/tee-provider/pkg/util"
	"github.com/maximhq/bifrost/core/providers/anthropic"
	bifrostOpenAI "github.com/maximhq/bifrost/core/providers/openai"
	"github.com/maximhq/bifrost/core/schemas"
)

// ConvertOpenAIRequestToAnthropic 把 OpenAI Chat Completions 请求转换为 Anthropic Messages API 请求。
func ConvertOpenAIRequestToAnthropic(body []byte, targetProvider uint16) ([]byte, error) {
	var oaiReq bifrostOpenAI.OpenAIChatRequest
	if err := json.Unmarshal(body, &oaiReq); err != nil {
		return nil, fmt.Errorf("parse openai request: %w", err)
	}

	ctx := schemas.NewBifrostContext(context.Background(), schemas.NoDeadline)
	chatReq := oaiReq.ToBifrostChatRequest(ctx)
	if chatReq == nil {
		return nil, fmt.Errorf("ToBifrostChatRequest returned nil")
	}
	chatReq.Provider = providerTypeToBifrost(targetProvider)

	// OpenAI 的 max_tokens 等价于 max_completion_tokens
	if oaiReq.MaxTokens != nil {
		if chatReq.Params == nil {
			chatReq.Params = &schemas.ChatParameters{}
		}
		if chatReq.Params.MaxCompletionTokens == nil {
			chatReq.Params.MaxCompletionTokens = oaiReq.MaxTokens
		}
	}

	antReq, err := anthropic.ToAnthropicChatRequest(ctx, chatReq)
	if antReq == nil && err == nil {
		return nil, fmt.Errorf("ToAnthropicChatRequest returned nil")
	}
	if err != nil {
		return nil, fmt.Errorf("convert to anthropic request: %w", err)
	}

	if oaiReq.Stream != nil && *oaiReq.Stream {
		antReq.Stream = schemas.Ptr(true)
	}

	return json.Marshal(antReq)
}

// ConvertAnthropicResponseToOpenAI 把 Anthropic Messages API 响应转换为 OpenAI Chat Completions 响应。
func ConvertAnthropicResponseToOpenAI(body []byte) (out []byte, usage *schemas.BifrostLLMUsage, err error) {
	defer func() {
		if rec := recover(); rec != nil {
			util.LogWithRed("[protocol] panic in ConvertAnthropicResponseToOpenAI: %v\n%s", rec, debug.Stack())
			err = fmt.Errorf("panic in ConvertAnthropicResponseToOpenAI: %v", rec)
			out = nil
			usage = nil
		}
	}()

	var antResp anthropic.AnthropicMessageResponse
	if err = json.Unmarshal(body, &antResp); err != nil {
		return nil, nil, fmt.Errorf("parse anthropic response: %w", err)
	}
	ctx := schemas.NewBifrostContext(context.Background(), schemas.NoDeadline)
	responsesResp := antResp.ToBifrostResponsesResponse(ctx)
	if responsesResp == nil {
		return nil, nil, fmt.Errorf("ToBifrostResponsesResponse returned nil")
	}
	chatResp := responsesResp.ToBifrostChatResponse()
	if chatResp == nil {
		return nil, nil, fmt.Errorf("ToBifrostChatResponse returned nil")
	}

	out, err = buildOpenAIChatCompletionResponse(chatResp)
	if err != nil {
		return nil, nil, err
	}
	return out, chatResp.Usage, nil
}

// buildOpenAIChatCompletionResponse 从 BifrostChatResponse 构建 OpenAI 标准响应 JSON。
func buildOpenAIChatCompletionResponse(resp *schemas.BifrostChatResponse) ([]byte, error) {
	if resp == nil {
		return nil, fmt.Errorf("empty response: cannot build openai chat completion from nil")
	}
	created := resp.Created
	if created == 0 {
		created = int(time.Now().Unix())
	}

	choices := make([]map[string]any, 0, len(resp.Choices))
	for i, choice := range resp.Choices {
		choiceMap := map[string]any{
			"index": i,
		}
		if choice.FinishReason != nil {
			choiceMap["finish_reason"] = *choice.FinishReason
		} else {
			choiceMap["finish_reason"] = nil
		}

		if choice.ChatNonStreamResponseChoice != nil && choice.ChatNonStreamResponseChoice.Message != nil {
			msg := choice.ChatNonStreamResponseChoice.Message
			msgMap := map[string]any{
				"role": "assistant",
			}
			if msg.Content != nil {
				msgMap["content"] = chatMessageContentToOpenAI(msg.Content)
			}
			if msg.ChatAssistantMessage != nil {
				if reasoning := reasoningContentFromAssistantMessage(msg.ChatAssistantMessage); reasoning != "" {
					msgMap["reasoning_content"] = reasoning
				}
				if len(msg.ChatAssistantMessage.ToolCalls) > 0 {
					toolCalls := make([]map[string]any, 0, len(msg.ChatAssistantMessage.ToolCalls))
					for _, tc := range msg.ChatAssistantMessage.ToolCalls {
						tcMap := map[string]any{
							"id":   derefString(tc.ID),
							"type": "function",
							"function": map[string]string{
								"name":      derefString(tc.Function.Name),
								"arguments": tc.Function.Arguments,
							},
						}
						toolCalls = append(toolCalls, tcMap)
					}
					msgMap["tool_calls"] = toolCalls
				}
				if msg.ChatAssistantMessage.Refusal != nil {
					msgMap["refusal"] = *msg.ChatAssistantMessage.Refusal
				}
			}
			choiceMap["message"] = msgMap
		}
		choices = append(choices, choiceMap)
	}

	usageMap := map[string]any{
		"prompt_tokens":     0,
		"completion_tokens": 0,
		"total_tokens":      0,
	}
	if resp.Usage != nil {
		usageMap["prompt_tokens"] = resp.Usage.PromptTokens
		usageMap["completion_tokens"] = resp.Usage.CompletionTokens
		usageMap["total_tokens"] = resp.Usage.TotalTokens
		cachedTokens := 0
		if resp.Usage.PromptTokensDetails != nil {
			cachedTokens = resp.Usage.PromptTokensDetails.CachedReadTokens
		}
		usageMap["prompt_tokens_details"] = map[string]int{
			"cached_tokens": cachedTokens,
		}
	}

	oaiResp := map[string]any{
		"id":      resp.ID,
		"object":  "chat.completion",
		"created": created,
		"model":   resp.Model,
		"choices": choices,
		"usage":   usageMap,
	}

	b, err := json.Marshal(oaiResp)
	if err != nil {
		return nil, fmt.Errorf("marshal openai response: %w", err)
	}
	return b, nil
}

func derefString(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// reasoningContentFromAssistantMessage 从 Bifrost assistant message 中提取 reasoning 文本。
func reasoningContentFromAssistantMessage(msg *schemas.ChatAssistantMessage) string {
	if msg == nil {
		return ""
	}
	if msg.Reasoning != nil && *msg.Reasoning != "" {
		return *msg.Reasoning
	}
	var buf strings.Builder
	for _, rd := range msg.ReasoningDetails {
		if rd.Type == schemas.BifrostReasoningDetailsTypeText && rd.Text != nil {
			buf.WriteString(*rd.Text)
		}
	}
	return buf.String()
}

// chatMessageContentToOpenAI 把 Bifrost 的 ChatMessageContent 转为 OpenAI 的 content（string 或 content blocks）。
func chatMessageContentToOpenAI(content *schemas.ChatMessageContent) any {
	if content == nil {
		return nil
	}
	if content.ContentStr != nil {
		return *content.ContentStr
	}
	if len(content.ContentBlocks) == 0 {
		return nil
	}

	// 如果全是 text block，直接拼成字符串；否则返回 blocks 数组。
	allText := true
	for _, block := range content.ContentBlocks {
		if block.Type != "text" {
			allText = false
			break
		}
	}

	if allText {
		var textBuf strings.Builder
		for _, block := range content.ContentBlocks {
			if block.Text != nil {
				textBuf.WriteString(*block.Text)
			}
		}
		return textBuf.String()
	}

	blocks := make([]map[string]any, 0, len(content.ContentBlocks))
	for _, block := range content.ContentBlocks {
		b := map[string]any{"type": string(block.Type)}
		switch block.Type {
		case "text":
			if block.Text != nil {
				b["text"] = *block.Text
			}
		case "image_url":
			if block.ImageURLStruct != nil {
				b["image_url"] = block.ImageURLStruct
			}
		case "input_audio":
			if block.InputAudio != nil {
				b["input_audio"] = block.InputAudio
			}
		case "file":
			if block.File != nil {
				b["file"] = block.File
			}
		case "refusal":
			if block.Refusal != nil {
				b["refusal"] = *block.Refusal
			}
		default:
			// 保留未知 block 类型已有的字段，避免静默丢数据
			if block.Text != nil {
				b["text"] = *block.Text
			}
		}
		blocks = append(blocks, b)
	}
	return blocks
}
