package gateway

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"github.com/computeflux/tee-provider/pkg/api/gateway/protocol"
	relay "github.com/computeflux/tee-provider/pkg/services/gateway_relay"
	"github.com/computeflux/tee-provider/pkg/services/scheduler"
)

// parseNativeUsage 从「同协议(无需转换)的非流式响应体」中解析 usage。
// 跨协议时 usage 由 convertResponse 直接给出，不走这里。
func parseNativeUsage(body []byte, proto protocol.Protocol) *relay.Usage {
	switch proto {
	case protocol.ProtocolAnthropic:
		var r struct {
			Usage struct {
				InputTokens          int `json:"input_tokens"`
				OutputTokens         int `json:"output_tokens"`
				CacheReadInputTokens int `json:"cache_read_input_tokens"`
			} `json:"usage"`
		}
		if json.Unmarshal(body, &r) != nil {
			return nil
		}
		return &relay.Usage{
			PromptTokens:        r.Usage.InputTokens,
			CompletionTokens:    r.Usage.OutputTokens,
			TotalTokens:         r.Usage.InputTokens + r.Usage.OutputTokens,
			PromptTokensDetails: relay.PromptTokensDetails{CachedTokens: r.Usage.CacheReadInputTokens},
		}
	case protocol.ProtocolGemini:
		var r struct {
			UsageMetadata struct {
				PromptTokenCount        int `json:"promptTokenCount"`
				CandidatesTokenCount    int `json:"candidatesTokenCount"`
				TotalTokenCount         int `json:"totalTokenCount"`
				CachedContentTokenCount int `json:"cachedContentTokenCount"`
			} `json:"usageMetadata"`
		}
		if json.Unmarshal(body, &r) != nil {
			return nil
		}
		return &relay.Usage{
			PromptTokens:        r.UsageMetadata.PromptTokenCount,
			CompletionTokens:    r.UsageMetadata.CandidatesTokenCount,
			TotalTokens:         r.UsageMetadata.TotalTokenCount,
			PromptTokensDetails: relay.PromptTokensDetails{CachedTokens: r.UsageMetadata.CachedContentTokenCount},
		}
	default: // OpenAI 兼容
		var r struct {
			Usage *struct {
				PromptTokens        int `json:"prompt_tokens"`
				CompletionTokens    int `json:"completion_tokens"`
				TotalTokens         int `json:"total_tokens"`
				PromptTokensDetails struct {
					CachedTokens int `json:"cached_tokens"`
				} `json:"prompt_tokens_details"`
			} `json:"usage"`
		}
		if json.Unmarshal(body, &r) != nil || r.Usage == nil {
			return nil
		}
		return &relay.Usage{
			PromptTokens:        r.Usage.PromptTokens,
			CompletionTokens:    r.Usage.CompletionTokens,
			TotalTokens:         r.Usage.TotalTokens,
			PromptTokensDetails: relay.PromptTokensDetails{CachedTokens: r.Usage.PromptTokensDetails.CachedTokens},
		}
	}
}

// sniffStreamUsage 从「同协议流式」的单条 SSE data payload 中尽力解析 usage。
// payload 是去掉前后空白的 data: 行内容（不含 "data:" 前缀）。
func sniffStreamUsage(payload []byte, proto protocol.Protocol) *relay.Usage {
	if len(payload) == 0 || payload[0] != '{' {
		return nil
	}
	switch proto {
	case protocol.ProtocolAnthropic:
		// message_start: {"type":"message_start","message":{"usage":{"input_tokens":N}}}
		// message_delta: {"type":"message_delta","usage":{"output_tokens":N}}
		var ev struct {
			Type    string `json:"type"`
			Message struct {
				Usage *struct {
					InputTokens  int `json:"input_tokens"`
					OutputTokens int `json:"output_tokens"`
				} `json:"usage"`
			} `json:"message"`
			Usage *struct {
				InputTokens  int `json:"input_tokens"`
				OutputTokens int `json:"output_tokens"`
			} `json:"usage"`
		}
		if json.Unmarshal(payload, &ev) != nil {
			return nil
		}
		u := ev.Usage
		if u == nil {
			u = ev.Message.Usage
		}
		if u == nil {
			return nil
		}
		return &relay.Usage{
			PromptTokens:     u.InputTokens,
			CompletionTokens: u.OutputTokens,
			TotalTokens:      u.InputTokens + u.OutputTokens,
		}
	default: // OpenAI 兼容：usage 在含 stream_options.include_usage 的末尾 chunk
		return parseNativeUsage(payload, protocol.ProtocolOpenAI)
	}
}

// chunkReader 把 scheduler.Stream（逐行 raw SSE 字节）重新拼成 io.Reader，
// 供跨协议转换器（streamConvert）消费。chunk.Data 已含换行，直接顺序拼接即可还原原始 SSE 流。
type chunkReader struct {
	ctx  context.Context
	s    scheduler.Stream
	buf  []byte
	done bool
}

func newChunkReader(ctx context.Context, s scheduler.Stream) *chunkReader {
	return &chunkReader{ctx: ctx, s: s}
}

func (cr *chunkReader) Read(p []byte) (int, error) {
	for len(cr.buf) == 0 {
		if cr.done {
			return 0, io.EOF
		}
		chunk, err := cr.s.Next(cr.ctx)
		if err != nil {
			if errors.Is(err, io.EOF) || errors.Is(err, scheduler.ErrStreamClosed) {
				cr.done = true
				return 0, io.EOF
			}
			return 0, err
		}
		if chunk == nil {
			continue
		}
		cr.buf = chunk.Data
		if chunk.IsFinal {
			cr.done = true
		}
	}
	n := copy(p, cr.buf)
	cr.buf = cr.buf[n:]
	return n, nil
}

// flushWriter 在每次 Write 后调用 Flush，保证 SSE 实时下发。
type flushWriter struct {
	w  io.Writer
	fl http.Flusher
}

func (fw flushWriter) Write(p []byte) (int, error) {
	n, err := fw.w.Write(p)
	if fw.fl != nil {
		fw.fl.Flush()
	}
	return n, err
}
