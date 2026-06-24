package protocol

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/maximhq/bifrost/core/schemas"
)

// StreamAnthropicResponseToOpenAI 把 Anthropic SSE 流实时转换为 OpenAI chat.completion.chunk SSE 流。
func StreamAnthropicResponseToOpenAI(r io.Reader, w io.Writer) (*schemas.BifrostLLMUsage, error) {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, DefaultSSEBufferSize), DefaultSSEBufferSize)

	flusher, ok := w.(httpFlusher)
	if !ok {
		return nil, fmt.Errorf("response writer does not support flushing")
	}

	var usage schemas.BifrostLLMUsage
	var msgID, model string
	created := int(time.Now().Unix())
	roleEmitted := false

	// Anthropic content_block index -> OpenAI tool_calls index；同一 message 中
	// 可能夹杂 text/thinking/tool_use，OpenAI 端要求 tool_calls 数组的 index
	// 紧凑递增，所以单独维护一份计数器。
	blockToToolIndex := make(map[int]int)
	nextToolIndex := 0

	emitRole := func() error {
		if roleEmitted {
			return nil
		}
		chunk := buildOpenAIStreamChunk(msgID, model, created, 0, map[string]any{"role": "assistant"}, nil)
		if err := writeOpenAIStreamChunk(w, flusher, chunk); err != nil {
			return err
		}
		roleEmitted = true
		return nil
	}

	for scanner.Scan() {
		line := strings.TrimRight(scanner.Text(), "\r")
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		data := strings.TrimPrefix(line, "data: ")
		if data == "[DONE]" {
			break
		}

		var evt anthropicStreamEventReverse
		if err := json.Unmarshal([]byte(data), &evt); err != nil {
			return nil, fmt.Errorf("parse anthropic stream event: %w", err)
		}

		switch evt.Type {
		case "message_start":
			if evt.Message == nil {
				return nil, fmt.Errorf("message_start event has nil message")
			}
			msgID = evt.Message.ID
			model = evt.Message.Model
			created = int(time.Now().Unix())
			if evt.Message.Usage != nil {
				// cache 相关 token 也属于输入 token 总量
				usage.PromptTokens = evt.Message.Usage.InputTokens +
					evt.Message.Usage.CacheCreationInputTokens +
					evt.Message.Usage.CacheReadInputTokens
			}
			if err := emitRole(); err != nil {
				return nil, err
			}

		case "content_block_start":
			if evt.Index == nil || evt.ContentBlock == nil {
				continue
			}
			blockIdx := *evt.Index
			if evt.ContentBlock.Type == "tool_use" {
				if _, ok := blockToToolIndex[blockIdx]; !ok {
					blockToToolIndex[blockIdx] = nextToolIndex
					nextToolIndex++
				}
				if err := emitRole(); err != nil {
					return nil, err
				}
				delta := map[string]any{
					"tool_calls": []map[string]any{{
						"index": blockToToolIndex[blockIdx],
						"id":    evt.ContentBlock.ID,
						"type":  "function",
						"function": map[string]any{
							"name":      evt.ContentBlock.Name,
							"arguments": "",
						},
					}},
				}
				chunk := buildOpenAIStreamChunk(msgID, model, created, 0, delta, nil)
				if err := writeOpenAIStreamChunk(w, flusher, chunk); err != nil {
					return nil, err
				}
			} else if !roleEmitted {
				if err := emitRole(); err != nil {
					return nil, err
				}
			}

		case "content_block_delta":
			if evt.Delta == nil {
				continue
			}
			delta := map[string]any{}
			switch evt.Delta.Type {
			case "text_delta":
				if evt.Delta.Text != "" {
					delta["content"] = evt.Delta.Text
				}
			case "thinking_delta":
				if evt.Delta.Thinking != "" {
					delta["reasoning_content"] = evt.Delta.Thinking
				}
			case "input_json_delta":
				toolIdx := 0
				if evt.Index != nil {
					if mapped, ok := blockToToolIndex[*evt.Index]; ok {
						toolIdx = mapped
					} else {
						// 见到 input_json_delta 时必然已经过 content_block_start
						// 注册过 tool index；这里保留 0 作为兜底。
						toolIdx = 0
					}
				}
				delta["tool_calls"] = []map[string]any{{
					"index": toolIdx,
					"function": map[string]any{
						"arguments": evt.Delta.PartialJson,
					},
				}}
			}
			if len(delta) > 0 {
				// role 必须先于任何 content/tool_calls 发出。
				if err := emitRole(); err != nil {
					return nil, err
				}
				chunk := buildOpenAIStreamChunk(msgID, model, created, 0, delta, nil)
				if err := writeOpenAIStreamChunk(w, flusher, chunk); err != nil {
					return nil, err
				}
			} else if !roleEmitted {
				if err := emitRole(); err != nil {
					return nil, err
				}
			}

		case "message_delta":
			var finishReason *string
			if evt.Delta != nil && evt.Delta.StopReason != "" {
				fr := mapAnthropicStopReasonToOpenAI(evt.Delta.StopReason)
				finishReason = &fr
			}
			if finishReason != nil {
				// finish_reason 也必须在 role 之后。
				if err := emitRole(); err != nil {
					return nil, err
				}
				chunk := buildOpenAIStreamChunk(msgID, model, created, 0, map[string]any{}, finishReason)
				if err := writeOpenAIStreamChunk(w, flusher, chunk); err != nil {
					return nil, err
				}
			}
			if evt.Usage != nil {
				usage.CompletionTokens = evt.Usage.OutputTokens
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("read anthropic stream: %w", err)
	}

	if err := writeOpenAIStreamDone(w, flusher); err != nil {
		return nil, err
	}

	usage.TotalTokens = usage.PromptTokens + usage.CompletionTokens
	return &usage, nil
}

type httpFlusher interface {
	Flush()
}

type anthropicStreamEventReverse struct {
	Type    string `json:"type"`
	Index   *int   `json:"index"`
	Message *struct {
		ID    string `json:"id"`
		Model string `json:"model"`
		Usage *struct {
			InputTokens              int `json:"input_tokens"`
			OutputTokens             int `json:"output_tokens"`
			CacheCreationInputTokens int `json:"cache_creation_input_tokens"`
			CacheReadInputTokens     int `json:"cache_read_input_tokens"`
		} `json:"usage"`
	} `json:"message"`
	ContentBlock *struct {
		Type string `json:"type"`
		Text string `json:"text,omitempty"`
		ID   string `json:"id,omitempty"`
		Name string `json:"name,omitempty"`
	} `json:"content_block"`
	Delta *struct {
		Type        string `json:"type"`
		Text        string `json:"text"`
		Thinking    string `json:"thinking"`
		PartialJson string `json:"partial_json"`
		StopReason  string `json:"stop_reason"`
	} `json:"delta"`
	Usage *struct {
		OutputTokens int `json:"output_tokens"`
	} `json:"usage"`
}

func buildOpenAIStreamChunk(id, model string, created, choiceIndex int, delta map[string]any, finishReason *string) map[string]any {
	choice := map[string]any{
		"index":    choiceIndex,
		"delta":    delta,
		"logprobs": nil,
	}
	if finishReason != nil {
		choice["finish_reason"] = *finishReason
	} else {
		choice["finish_reason"] = nil
	}

	return map[string]any{
		"id":      id,
		"object":  "chat.completion.chunk",
		"created": created,
		"model":   model,
		"choices": []map[string]any{choice},
	}
}

func writeOpenAIStreamChunk(w io.Writer, flusher httpFlusher, chunk map[string]any) error {
	b, err := json.Marshal(chunk)
	if err != nil {
		return fmt.Errorf("marshal openai stream chunk: %w", err)
	}
	if _, err := fmt.Fprintf(w, "data: %s\n\n", b); err != nil {
		return fmt.Errorf("write openai stream chunk: %w", err)
	}
	if flusher != nil {
		flusher.Flush()
	}
	return nil
}

func writeOpenAIStreamDone(w io.Writer, flusher httpFlusher) error {
	if _, err := fmt.Fprintf(w, "data: [DONE]\n\n"); err != nil {
		return fmt.Errorf("write openai stream done: %w", err)
	}
	if flusher != nil {
		flusher.Flush()
	}
	return nil
}

func mapAnthropicStopReasonToOpenAI(reason string) string {
	switch reason {
	case "end_turn", "stop_sequence":
		return "stop"
	case "max_tokens":
		return "length"
	case "tool_use":
		return "tool_calls"
	default:
		// Anthropic 未来可能新增 stop_reason，对 OpenAI 客户端统一降级为 stop，避免泄露未知值。
		return "stop"
	}
}
