package protocol

import (
	"fmt"
	"io"

	"github.com/maximhq/bifrost/core/schemas"
)

// ConvertAnthropicRequestToGemini 把 Anthropic Messages API 请求转换为 Gemini 原生请求。
// 复用 Anthropic→OpenAI 与 OpenAI→Gemini 两条现有转换路径，避免重复实现。
func ConvertAnthropicRequestToGemini(body []byte, targetProvider uint16) ([]byte, error) {
	openAIReq, err := ConvertAnthropicRequestToOpenAI(body, targetProvider)
	if err != nil {
		return nil, fmt.Errorf("anthropic to openai: %w", err)
	}
	gemReq, err := ConvertOpenAIRequestToGemini(openAIReq, targetProvider)
	if err != nil {
		return nil, fmt.Errorf("openai to gemini: %w", err)
	}
	return gemReq, nil
}

// ConvertGeminiResponseToAnthropic 把 Gemini 原生响应转换为 Anthropic Messages API 响应。
func ConvertGeminiResponseToAnthropic(body []byte) ([]byte, *schemas.BifrostLLMUsage, error) {
	openAIResp, usage, err := ConvertGeminiResponseToOpenAI(body)
	if err != nil {
		return nil, nil, fmt.Errorf("gemini to openai: %w", err)
	}
	anthropicResp, _, err := ConvertOpenAIResponseToAnthropic(openAIResp)
	if err != nil {
		return nil, nil, fmt.Errorf("openai to anthropic: %w", err)
	}
	return anthropicResp, usage, nil
}

// StreamGeminiResponseToAnthropic 把 Gemini SSE 流实时转换为 Anthropic Messages API SSE 流。
// 通过管道把 Gemini→OpenAI 与 OpenAI→Anthropic 两个流式转换器串接起来。
func StreamGeminiResponseToAnthropic(r io.Reader, w io.Writer) (*schemas.BifrostLLMUsage, error) {
	pr, pw := io.Pipe()

	// StreamGeminiResponseToOpenAI 需要 http.Flusher；管道本身没有 Flush，包一层空实现。
	pipeW := &flushPipeWriter{pw}

	var (
		intermediateUsage *schemas.BifrostLLMUsage
		intermediateErr   error
	)
	go func() {
		intermediateUsage, intermediateErr = StreamGeminiResponseToOpenAI(r, pipeW)
		_ = pipeW.Close()
	}()

	outUsage, err := StreamOpenAIResponseToAnthropic(pr, w)
	if err != nil {
		return nil, err
	}
	if intermediateErr != nil {
		return nil, intermediateErr
	}
	// Gemini→OpenAI 阶段已经解析出 usage，优先使用；否则使用 OpenAI→Anthropic 阶段统计。
	if intermediateUsage != nil {
		return intermediateUsage, nil
	}
	return outUsage, nil
}

// flushPipeWriter 给 io.PipeWriter 加上空的 Flush，使其满足 http.Flusher。
type flushPipeWriter struct {
	*io.PipeWriter
}

func (w *flushPipeWriter) Flush() {}
