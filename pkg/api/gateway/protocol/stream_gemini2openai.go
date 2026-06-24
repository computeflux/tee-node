package protocol

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/computeflux/tee-provider/pkg/util"
	"github.com/maximhq/bifrost/core/providers/gemini"
	"github.com/maximhq/bifrost/core/schemas"
)

// StreamGeminiResponseToOpenAI 读取 Gemini SSE 流，对每个事件调用 Bifrost 流式转换，
// 然后实时写成 OpenAI chat.completion.chunk SSE 流。
func StreamGeminiResponseToOpenAI(r io.Reader, w io.Writer) (*schemas.BifrostLLMUsage, error) {
	// util.DebugLog("[stream] StreamGeminiResponseToOpenAI started")
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, DefaultSSEBufferSize), DefaultSSEBufferSize)

	flusher, ok := w.(http.Flusher)
	if !ok {
		return nil, fmt.Errorf("response writer does not support flushing")
	}

	state := gemini.NewGeminiStreamState()
	usage := &schemas.BifrostLLMUsage{}

	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		data := strings.TrimPrefix(line, "data: ")
		if data == "[DONE]" {
			break
		}

		var evt gemini.GenerateContentResponse
		if err := json.Unmarshal([]byte(data), &evt); err != nil {
			return nil, fmt.Errorf("parse gemini stream event: %w", err)
		}

		chunk, bifrostErr, isLastChunk := evt.ToBifrostChatCompletionStream(state)
		if bifrostErr != nil {
			msg := ""
			if bifrostErr.Error != nil {
				msg = bifrostErr.Error.Message
			}
			util.DebugLog("[stream] gemini stream conversion error: %s", msg)
			// 流已启动，无法返回 HTTP 错误，静默结束。
			break
		}
		if chunk == nil && !isLastChunk {
			continue
		}
		if chunk == nil {
			continue
		}

		if chunk.Usage != nil {
			usage = chunk.Usage
		}

		if err := writeBifrostStreamChunkToOpenAI(w, flusher, chunk); err != nil {
			return nil, err
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("read gemini stream: %w", err)
	}

	if _, err := fmt.Fprint(w, "data: [DONE]\n\n"); err != nil {
		return nil, fmt.Errorf("write done marker: %w", err)
	}
	flusher.Flush()

	// util.DebugLog("[stream] StreamGeminiResponseToOpenAI done usage=%+v", usage)
	return usage, nil
}

// writeBifrostStreamChunkToOpenAI 把一个 Bifrost 流式 chunk 转成 OpenAI SSE 事件写入 w。
func writeBifrostStreamChunkToOpenAI(w io.Writer, flusher http.Flusher, chunk *schemas.BifrostChatResponse) error {
	if chunk == nil || len(chunk.Choices) == 0 {
		return nil
	}

	choice := chunk.Choices[0]
	delta := map[string]any{}

	if choice.ChatStreamResponseChoice != nil && choice.ChatStreamResponseChoice.Delta != nil {
		d := choice.ChatStreamResponseChoice.Delta
		if d.Role != nil && *d.Role != "" {
			delta["role"] = *d.Role
		}
		if d.Content != nil && *d.Content != "" {
			delta["content"] = *d.Content
		}
		if reasoning := reasoningContentFromDelta(d); reasoning != "" {
			delta["reasoning_content"] = reasoning
		}
		if len(d.ToolCalls) > 0 {
			toolCalls := make([]map[string]any, 0, len(d.ToolCalls))
			for _, tc := range d.ToolCalls {
				toolCalls = append(toolCalls, bifrostToolCallToOpenAIStream(tc))
			}
			delta["tool_calls"] = toolCalls
		}
	}

	// 没有实际增量且没有 finish_reason 时，跳过该 chunk。
	if len(delta) == 0 && choice.FinishReason == nil {
		return nil
	}

	out := map[string]any{
		"id":      chunk.ID,
		"object":  "chat.completion.chunk",
		"created": chunk.Created,
		"model":   chunk.Model,
		"choices": []map[string]any{{
			"index":    choice.Index,
			"delta":    delta,
			"logprobs": nil,
		}},
	}
	if choice.FinishReason != nil {
		outChoices := out["choices"].([]map[string]any)
		outChoices[0]["finish_reason"] = *choice.FinishReason
	}
	if chunk.Usage != nil {
		out["usage"] = map[string]any{
			"prompt_tokens":     chunk.Usage.PromptTokens,
			"completion_tokens": chunk.Usage.CompletionTokens,
			"total_tokens":      chunk.Usage.TotalTokens,
		}
	}

	b, err := json.Marshal(out)
	if err != nil {
		return fmt.Errorf("marshal openai stream chunk: %w", err)
	}
	if _, err := fmt.Fprintf(w, "data: %s\n\n", string(b)); err != nil {
		return fmt.Errorf("write stream chunk: %w", err)
	}
	flusher.Flush()
	return nil
}

// reasoningContentFromDelta 从 Bifrost 流式 delta 中提取 reasoning 文本。
func reasoningContentFromDelta(d *schemas.ChatStreamResponseChoiceDelta) string {
	if d == nil {
		return ""
	}
	if d.Reasoning != nil && *d.Reasoning != "" {
		return *d.Reasoning
	}
	var buf strings.Builder
	for _, rd := range d.ReasoningDetails {
		if rd.Type == schemas.BifrostReasoningDetailsTypeText && rd.Text != nil {
			buf.WriteString(*rd.Text)
		}
	}
	return buf.String()
}

// bifrostToolCallToOpenAIStream 把一个 Bifrost tool call 增量转成 OpenAI 流式 tool_calls 元素。
func bifrostToolCallToOpenAIStream(tc schemas.ChatAssistantMessageToolCall) map[string]any {
	name := ""
	if tc.Function.Name != nil {
		name = *tc.Function.Name
	}
	id := ""
	if tc.ID != nil {
		id = *tc.ID
	}
	typeStr := "function"
	if tc.Type != nil {
		typeStr = *tc.Type
	}
	return map[string]any{
		"index": tc.Index,
		"id":    id,
		"type":  typeStr,
		"function": map[string]any{
			"name":      name,
			"arguments": tc.Function.Arguments,
		},
	}
}
