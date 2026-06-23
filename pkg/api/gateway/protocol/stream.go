package protocol

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/maximhq/bifrost/core/schemas"
)

// DefaultSSEBufferSize 是 SSE 流扫描器的默认缓冲区大小。
const DefaultSSEBufferSize = 64 * 1024

// StreamOpenAIResponseToAnthropic 读取 OpenAI SSE 流并实时转换为 Anthropic SSE 事件写入 w。
// 返回从流最后一个 chunk 解析出的 usage（如果有）。
func StreamOpenAIResponseToAnthropic(r io.Reader, w io.Writer) (*schemas.BifrostLLMUsage, error) {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, DefaultSSEBufferSize), DefaultSSEBufferSize)

	state := &streamState{currentBlockIndex: -1}

	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		data := strings.TrimPrefix(line, "data: ")
		if data == "[DONE]" {
			break
		}

		var chunk openAIStreamChunk
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			return nil, fmt.Errorf("parse openai stream chunk: %w", err)
		}

		if err := state.handleChunk(w, chunk); err != nil {
			return nil, err
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("read openai stream: %w", err)
	}

	if err := state.finish(w); err != nil {
		return nil, err
	}

	return state.usage, nil
}

type blockType string

const (
	blockTypeNone     blockType = ""
	blockTypeText     blockType = "text"
	blockTypeThinking blockType = "thinking"
	blockTypeToolUse  blockType = "tool_use"
)

type streamState struct {
	started      bool
	id           string
	model        string
	usage        *schemas.BifrostLLMUsage
	finishReason *string

	currentBlockIndex int
	currentBlockType  blockType
	currentToolCall   *toolCallState
	toolCallStates    map[int]*toolCallState // key: tool call index from OpenAI
}

type toolCallState struct {
	id        string
	name      string
	arguments string
	started   bool
	stopped   bool
	blockIdx  int // Anthropic content block index
}

func (s *streamState) handleChunk(w io.Writer, chunk openAIStreamChunk) error {
	if !s.started {
		s.started = true
		s.id = chunk.ID
		s.model = chunk.Model
		if err := writeEvent(w, "message_start", map[string]any{
			"type": "message_start",
			"message": map[string]any{
				"type":    "message",
				"id":      chunk.ID,
				"role":    "assistant",
				"model":   chunk.Model,
				"content": []any{},
				"usage": map[string]any{
					"input_tokens":  0,
					"output_tokens": 0,
				},
			},
		}); err != nil {
			return err
		}
	}

	if chunk.Usage != nil {
		s.usage = chunk.Usage
	}

	// 当前转换只支持单条非空 choice；多条 choice 会共享同一组 content block，导致内容交错。
	nonEmptyChoices := 0
	for _, choice := range chunk.Choices {
		if choice.Delta == nil {
			continue
		}
		delta := choice.Delta
		if delta.Content != "" || delta.Reasoning != "" || delta.ReasoningContent != "" || len(delta.ToolCalls) > 0 {
			nonEmptyChoices++
		}
	}
	if nonEmptyChoices > 1 {
		return fmt.Errorf("multiple non-empty choices not supported")
	}

	for _, choice := range chunk.Choices {
		if choice.FinishReason != nil && *choice.FinishReason != "" {
			s.finishReason = choice.FinishReason
		}
		if choice.Delta == nil {
			continue
		}
		delta := choice.Delta

		// thinking / reasoning
		reasoning := delta.Reasoning
		if reasoning == "" && delta.ReasoningContent != "" {
			reasoning = delta.ReasoningContent
		}
		if reasoning != "" {
			if err := s.startBlock(w, blockTypeThinking); err != nil {
				return err
			}
			if err := writeEvent(w, "thinking_delta", map[string]any{
				"type":  "thinking_delta",
				"index": s.currentBlockIndex,
				"delta": map[string]any{"type": "thinking_delta", "thinking": reasoning},
			}); err != nil {
				return err
			}
		}

		// text content
		if delta.Content != "" {
			if err := s.startBlock(w, blockTypeText); err != nil {
				return err
			}
			if err := writeEvent(w, "content_block_delta", map[string]any{
				"type":  "content_block_delta",
				"index": s.currentBlockIndex,
				"delta": map[string]any{"type": "text_delta", "text": delta.Content},
			}); err != nil {
				return err
			}
		}

		// tool_calls
		for _, tc := range delta.ToolCalls {
			if s.toolCallStates == nil {
				s.toolCallStates = make(map[int]*toolCallState)
			}
			state, ok := s.toolCallStates[tc.Index]
			if !ok {
				// 切换到一个新的 tool_use block
				if err := s.closeCurrentBlock(w); err != nil {
					return err
				}
				state = &toolCallState{blockIdx: s.currentBlockIndex}
				s.toolCallStates[tc.Index] = state
				s.currentToolCall = state
			}
			if tc.ID != "" {
				state.id = tc.ID
			}
			if tc.Function.Name != "" {
				state.name = tc.Function.Name
			}

			if tc.Function.Arguments != "" {
				if !state.started {
					// 在 content_block_start 之前先缓存参数片段
					state.arguments += tc.Function.Arguments
				} else {
					if err := writeEvent(w, "input_json_delta", map[string]any{
						"type":  "input_json_delta",
						"index": state.blockIdx,
						"delta": map[string]any{"type": "input_json_delta", "partial_json": tc.Function.Arguments},
					}); err != nil {
						return err
					}
					state.arguments += tc.Function.Arguments
				}
			}

			// 等 id 和 name 都到齐后再 emit content_block_start，避免 name 为空。
			if !state.started && state.id != "" && state.name != "" {
				state.started = true
				s.currentBlockType = blockTypeToolUse
				s.currentBlockIndex++
				state.blockIdx = s.currentBlockIndex
				s.currentToolCall = state
				if err := writeEvent(w, "content_block_start", map[string]any{
					"type":  "content_block_start",
					"index": state.blockIdx,
					"content_block": map[string]any{
						"type":  "tool_use",
						"id":    state.id,
						"name":  state.name,
						"input": map[string]any{},
					},
				}); err != nil {
					return err
				}
				// 补发缓存的参数增量
				if state.arguments != "" {
					if err := writeEvent(w, "input_json_delta", map[string]any{
						"type":  "input_json_delta",
						"index": state.blockIdx,
						"delta": map[string]any{"type": "input_json_delta", "partial_json": state.arguments},
					}); err != nil {
						return err
					}
				}
			}
		}
	}

	return nil
}

func (s *streamState) startBlock(w io.Writer, t blockType) error {
	if s.currentBlockType == t {
		return nil
	}
	if err := s.closeCurrentBlock(w); err != nil {
		return err
	}
	s.currentBlockType = t
	s.currentBlockIndex++
	switch t {
	case blockTypeText:
		return writeEvent(w, "content_block_start", map[string]any{
			"type":          "content_block_start",
			"index":         s.currentBlockIndex,
			"content_block": map[string]any{"type": "text", "text": ""},
		})
	case blockTypeThinking:
		return writeEvent(w, "content_block_start", map[string]any{
			"type":          "content_block_start",
			"index":         s.currentBlockIndex,
			"content_block": map[string]any{"type": "thinking", "thinking": ""},
		})
	}
	return nil
}

func (s *streamState) closeCurrentBlock(w io.Writer) error {
	switch s.currentBlockType {
	case blockTypeText:
		if err := writeEvent(w, "content_block_stop", map[string]any{
			"type":          "content_block_stop",
			"index":         s.currentBlockIndex,
			"content_block": map[string]any{"type": "text", "text": ""},
		}); err != nil {
			return err
		}
	case blockTypeThinking:
		if err := writeEvent(w, "content_block_stop", map[string]any{
			"type":          "content_block_stop",
			"index":         s.currentBlockIndex,
			"content_block": map[string]any{"type": "thinking", "thinking": ""},
		}); err != nil {
			return err
		}
	case blockTypeToolUse:
		tc := s.currentToolCall
		contentBlock := map[string]any{"type": "tool_use"}
		var input any = map[string]any{}
		if tc != nil {
			contentBlock["id"] = tc.id
			contentBlock["name"] = tc.name
			if tc.arguments != "" {
				if err := json.Unmarshal([]byte(tc.arguments), &input); err != nil {
					// JSON 不合法时回退为空对象，避免破坏下游 Anthropic SDK 解析
					input = map[string]any{}
				}
			}
		}
		contentBlock["input"] = input
		if err := writeEvent(w, "content_block_stop", map[string]any{
			"type":          "content_block_stop",
			"index":         s.currentBlockIndex,
			"content_block": contentBlock,
		}); err != nil {
			return err
		}
		if tc != nil {
			tc.stopped = true
		}
	}
	s.currentBlockType = blockTypeNone
	s.currentToolCall = nil
	return nil
}

func (s *streamState) finish(w io.Writer) error {
	if !s.started {
		return nil
	}

	if err := s.closeCurrentBlock(w); err != nil {
		return err
	}

	for _, tc := range s.toolCallStates {
		if !tc.started || tc.stopped {
			continue
		}
		var input any = map[string]any{}
		if tc.arguments != "" {
			if err := json.Unmarshal([]byte(tc.arguments), &input); err != nil {
				input = map[string]any{}
			}
		}
		if err := writeEvent(w, "content_block_stop", map[string]any{
			"type":  "content_block_stop",
			"index": tc.blockIdx,
			"content_block": map[string]any{
				"type":  "tool_use",
				"id":    tc.id,
				"name":  tc.name,
				"input": input,
			},
		}); err != nil {
			return err
		}
	}

	stopReason := "end_turn"
	if s.finishReason != nil {
		stopReason = mapOpenAIFinishReasonToAnthropic(*s.finishReason)
	}

	usageOut := 0
	if s.usage != nil {
		usageOut = s.usage.CompletionTokens
	}
	if err := writeEvent(w, "message_delta", map[string]any{
		"type": "message_delta",
		"delta": map[string]any{
			"stop_reason":   stopReason,
			"stop_sequence": nil,
		},
		"usage": map[string]any{
			"output_tokens": usageOut,
		},
	}); err != nil {
		return err
	}

	return writeEvent(w, "message_stop", map[string]any{
		"type": "message_stop",
	})
}

func writeEvent(w io.Writer, event string, data any) error {
	b, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("marshal sse event: %w", err)
	}
	if _, err := fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event, string(b)); err != nil {
		return fmt.Errorf("write sse event: %w", err)
	}
	if f, ok := w.(http.Flusher); ok {
		f.Flush()
	}
	return nil
}

func mapOpenAIFinishReasonToAnthropic(reason string) string {
	switch reason {
	case "stop":
		return "end_turn"
	case "length":
		return "max_tokens"
	case "tool_calls":
		return "tool_use"
	case "content_filter":
		return "content_filter"
	default:
		return reason
	}
}

type openAIStreamChunk struct {
	ID      string                   `json:"id"`
	Object  string                   `json:"object"`
	Created int                      `json:"created"`
	Model   string                   `json:"model"`
	Choices []openAIStreamChoice     `json:"choices"`
	Usage   *schemas.BifrostLLMUsage `json:"usage,omitempty"`
}

type openAIStreamChoice struct {
	Index        int                `json:"index"`
	Delta        *openAIStreamDelta `json:"delta"`
	FinishReason *string            `json:"finish_reason"`
}

type openAIStreamDelta struct {
	Role             string                 `json:"role,omitempty"`
	Content          string                 `json:"content,omitempty"`
	Reasoning        string                 `json:"reasoning,omitempty"`
	ReasoningContent string                 `json:"reasoning_content,omitempty"`
	ToolCalls        []openAIStreamToolCall `json:"tool_calls,omitempty"`
}

type openAIStreamToolCall struct {
	Index    int                      `json:"index"`
	ID       string                   `json:"id"`
	Type     string                   `json:"type"`
	Function openAIStreamToolFunction `json:"function"`
}

type openAIStreamToolFunction struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}
