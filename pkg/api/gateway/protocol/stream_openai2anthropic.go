package protocol

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/maximhq/bifrost/core/providers/anthropic"
	providerUtils "github.com/maximhq/bifrost/core/providers/utils"
	"github.com/maximhq/bifrost/core/schemas"
)

const DefaultSSEBufferSize = 64 * 1024

// StreamOpenAIResponseToAnthropic 读取 OpenAI SSE 流并实时转换为 Anthropic SSE 事件写入 w。
func StreamOpenAIResponseToAnthropic(r io.Reader, w io.Writer, dropThinking bool) (*schemas.BifrostLLMUsage, error) {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, DefaultSSEBufferSize), DefaultSSEBufferSize)

	s := &streamState{currentBlockIndex: -1, dropThinking: dropThinking}

	for scanner.Scan() {
		line := strings.TrimRight(scanner.Text(), "\r")
		if line == "" || !strings.HasPrefix(line, "data: ") {
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
		if err := s.handleChunk(w, chunk); err != nil {
			return nil, err
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("read openai stream: %w", err)
	}
	if err := s.finish(w); err != nil {
		return nil, err
	}
	return s.usage, nil
}

// ============== 状态机 ==============

type blockType int

const (
	blockNone blockType = iota
	blockText
	blockThinking
	blockToolUse
)

type streamState struct {
	started           bool
	id, model         string
	usage             *schemas.BifrostLLMUsage
	finishReason      *string
	dropThinking      bool
	currentBlockIndex int
	currentBlockType  blockType
	currentToolCall   *toolCallState
	toolCallStates    map[int]*toolCallState
}

type toolCallState struct {
	id, name, arguments string
	started             bool
	blockIdx            int
	done                bool
}

func (s *streamState) handleChunk(w io.Writer, chunk openAIStreamChunk) error {
	if !s.started {
		s.started = true
		s.id = chunk.ID
		s.model = chunk.Model
		if err := writeAnthropicSSE(w, &anthropic.AnthropicStreamEvent{
			Type: anthropic.AnthropicStreamEventTypeMessageStart,
			Message: &anthropic.AnthropicMessageResponse{
				ID:      chunk.ID,
				Type:    "message",
				Role:    "assistant",
				Model:   chunk.Model,
				Content: []anthropic.AnthropicContentBlock{},
				Usage:   &anthropic.AnthropicUsage{},
			},
		}); err != nil {
			return err
		}
	}
	if chunk.Usage != nil {
		s.usage = chunk.Usage
	}

	// 仅支持单 choice；tool_calls 增量不计入"非空"判定，避免与其它 choice 的
	// finish_reason 同时到达时误报为多个非空 choice。
	nonEmpty := 0
	for _, c := range chunk.Choices {
		if c.Delta == nil {
			continue
		}
		hasContent := c.Delta.Content != "" || c.Delta.Reasoning != "" || c.Delta.ReasoningContent != ""
		if hasContent {
			nonEmpty++
		}
	}
	if nonEmpty > 1 {
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

		reasoning := delta.Reasoning
		if reasoning == "" {
			reasoning = delta.ReasoningContent
		}
		if reasoning != "" && !s.dropThinking {
			if err := s.startBlock(w, blockThinking); err != nil {
				return err
			}
			if err := writeAnthropicSSE(w, &anthropic.AnthropicStreamEvent{
				Type:  anthropic.AnthropicStreamEventTypeContentBlockDelta,
				Index: &s.currentBlockIndex,
				Delta: &anthropic.AnthropicStreamDelta{
					Type:     anthropic.AnthropicStreamDeltaTypeThinking,
					Thinking: &reasoning,
				},
			}); err != nil {
				return err
			}
		}

		if delta.Content != "" {
			if err := s.startBlock(w, blockText); err != nil {
				return err
			}
			if err := writeAnthropicSSE(w, &anthropic.AnthropicStreamEvent{
				Type:  anthropic.AnthropicStreamEventTypeContentBlockDelta,
				Index: &s.currentBlockIndex,
				Delta: &anthropic.AnthropicStreamDelta{
					Type: anthropic.AnthropicStreamDeltaTypeText,
					Text: &delta.Content,
				},
			}); err != nil {
				return err
			}
		}

		for _, tc := range delta.ToolCalls {
			if s.toolCallStates == nil {
				s.toolCallStates = make(map[int]*toolCallState)
			}
			st, ok := s.toolCallStates[tc.Index]
			if !ok {
				if err := s.closeCurrentBlock(w); err != nil {
					return err
				}
				// blockIdx 留待实际开 block 时再设置，避免指向刚关闭的旧 block。
				st = &toolCallState{blockIdx: -1}
				s.toolCallStates[tc.Index] = st
				s.currentToolCall = st
			}
			if tc.ID != "" {
				st.id = tc.ID
			}
			if tc.Function.Name != "" {
				st.name = strings.ToLower(tc.Function.Name)
			}
			if tc.Function.Arguments != "" {
				if !st.started {
					st.arguments += tc.Function.Arguments
				} else {
					if err := writeAnthropicSSE(w, &anthropic.AnthropicStreamEvent{
						Type:  anthropic.AnthropicStreamEventTypeContentBlockDelta,
						Index: &st.blockIdx,
						Delta: &anthropic.AnthropicStreamDelta{
							Type:        anthropic.AnthropicStreamDeltaTypeInputJSON,
							PartialJSON: &tc.Function.Arguments,
						},
					}); err != nil {
						return err
					}
					st.arguments += tc.Function.Arguments
				}
			}
			if !st.started && st.id != "" && st.name != "" {
				st.started = true
				s.currentBlockType = blockToolUse
				s.currentBlockIndex++
				st.blockIdx = s.currentBlockIndex
				s.currentToolCall = st
				// Input 留空：delta 自身就是合法 JSON 的片段，累加结果即为完整 JSON。
				// 若 start 时把 input 写成 "{}"，再叠加 {"foo":"bar"} 这种 delta，
				// 客户端拼接后会得到 "{}{foo":"bar}"，不是合法 JSON。
				if err := writeAnthropicSSE(w, &anthropic.AnthropicStreamEvent{
					Type:  anthropic.AnthropicStreamEventTypeContentBlockStart,
					Index: &st.blockIdx,
					ContentBlock: &anthropic.AnthropicContentBlock{
						Type:  anthropic.AnthropicContentBlockTypeToolUse,
						ID:    &st.id,
						Name:  &st.name,
						Input: json.RawMessage(""),
					},
				}); err != nil {
					return err
				}
				if st.arguments != "" {
					if err := writeAnthropicSSE(w, &anthropic.AnthropicStreamEvent{
						Type:  anthropic.AnthropicStreamEventTypeContentBlockDelta,
						Index: &st.blockIdx,
						Delta: &anthropic.AnthropicStreamDelta{
							Type:        anthropic.AnthropicStreamDeltaTypeInputJSON,
							PartialJSON: &st.arguments,
						},
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
	case blockText:
		return writeAnthropicSSE(w, &anthropic.AnthropicStreamEvent{
			Type:  anthropic.AnthropicStreamEventTypeContentBlockStart,
			Index: &s.currentBlockIndex,
			ContentBlock: &anthropic.AnthropicContentBlock{
				Type: anthropic.AnthropicContentBlockTypeText,
				Text: schemas.Ptr(""),
			},
		})
	case blockThinking:
		return writeAnthropicSSE(w, &anthropic.AnthropicStreamEvent{
			Type:  anthropic.AnthropicStreamEventTypeContentBlockStart,
			Index: &s.currentBlockIndex,
			ContentBlock: &anthropic.AnthropicContentBlock{
				Type:     anthropic.AnthropicContentBlockTypeThinking,
				Thinking: schemas.Ptr(""),
			},
		})
	}
	return nil
}

func (s *streamState) closeCurrentBlock(w io.Writer) error {
	switch s.currentBlockType {
	case blockText, blockThinking, blockToolUse:
		if s.currentBlockType == blockToolUse {
			if tc := s.currentToolCall; tc != nil {
				// 标记为已关闭，避免 finish() 重复发送 content_block_stop。
				// 即使 arguments 为空也需要标记，否则无参工具会触发双 stop。
				tc.done = true
			}
		}
		if err := writeAnthropicSSE(w, &anthropic.AnthropicStreamEvent{
			Type:  anthropic.AnthropicStreamEventTypeContentBlockStop,
			Index: &s.currentBlockIndex,
		}); err != nil {
			return err
		}
	}
	s.currentBlockType = blockNone
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
		if !tc.started || tc.done {
			continue
		}
		if err := writeAnthropicSSE(w, &anthropic.AnthropicStreamEvent{
			Type:  anthropic.AnthropicStreamEventTypeContentBlockStop,
			Index: &tc.blockIdx,
		}); err != nil {
			return err
		}
	}

	stopReason := anthropic.AnthropicStopReasonEndTurn
	if s.finishReason != nil {
		stopReason = anthropic.ConvertBifrostFinishReasonToAnthropic(*s.finishReason)
	}
	usageOut := 0
	if s.usage != nil {
		usageOut = s.usage.CompletionTokens
	}
	if err := writeAnthropicSSE(w, &anthropic.AnthropicStreamEvent{
		Type: anthropic.AnthropicStreamEventTypeMessageDelta,
		Delta: &anthropic.AnthropicStreamDelta{
			StopReason: &stopReason,
		},
		Usage: &anthropic.AnthropicUsage{OutputTokens: usageOut},
	}); err != nil {
		return err
	}
	return writeAnthropicSSE(w, &anthropic.AnthropicStreamEvent{
		Type: anthropic.AnthropicStreamEventTypeMessageStop,
	})
}

// ============== 输出 ==============

func writeAnthropicSSE(w io.Writer, event *anthropic.AnthropicStreamEvent) error {
	b, err := providerUtils.MarshalSorted(event)
	if err != nil {
		return fmt.Errorf("marshal sse event: %w", err)
	}
	if _, err := fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event.Type, string(b)); err != nil {
		return fmt.Errorf("write sse event: %w", err)
	}
	if f, ok := w.(http.Flusher); ok {
		f.Flush()
	}
	return nil
}

// ============== OpenAI SSE 解析类型 ==============

type openAIStreamChunk struct {
	ID      string                   `json:"id"`
	Object  string                   `json:"object"`
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
