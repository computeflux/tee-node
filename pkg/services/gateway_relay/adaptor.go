package relay

import (
	"io"
	"net/http"
	"strings"
)

// ── Request / Response types ─────────────────────────────

type GeneralOpenAIRequest struct {
	Model       string    `json:"model"`
	Messages    []Message `json:"messages"`
	Stream      bool      `json:"stream,omitempty"`
	Temperature float64   `json:"temperature,omitempty"`
	MaxTokens   int       `json:"max_tokens,omitempty"`
}

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type ImageRequest struct {
	Prompt string `json:"prompt"`
	Model  string `json:"model,omitempty"`
	N      int    `json:"n,omitempty"`
	Size   string `json:"size,omitempty"`
}

type Usage struct {
	PromptTokens        int                 `json:"prompt_tokens"`
	CompletionTokens    int                 `json:"completion_tokens"`
	TotalTokens         int                 `json:"total_tokens"`
	PromptTokensDetails PromptTokensDetails `json:"prompt_tokens_details,omitempty"`
}

type PromptTokensDetails struct {
	CachedTokens int `json:"cached_tokens"`
}

type ErrorWithStatusCode struct {
	StatusCode int
	Error      ErrorDetail
}

type ErrorDetail struct {
	Message string `json:"message"`
	Type    string `json:"type"`
	Code    string `json:"code,omitempty"`
}

// ── Adaptor interface ────────────────────────────────────

type Adaptor interface {
	Init(meta *Meta)
	GetRequestURL(meta *Meta) (string, error)
	SetupRequestHeader(ctx *RelayContext, req *http.Request, meta *Meta) error
	DoRequest(ctx *RelayContext, meta *Meta, requestBody io.Reader) (*http.Response, error)
	DoResponse(ctx *RelayContext, resp *http.Response, meta *Meta) (usage *Usage, err *ErrorWithStatusCode)
	GetProviderName() string
}

// forwardClientHeaders 把客户端请求头透传到上游请求，跳过认证、内容长度等需要被覆盖的头部。
func forwardClientHeaders(ctx *RelayContext, req *http.Request) {
	if ctx == nil || ctx.R == nil {
		return
	}
	skip := map[string]bool{
		"authorization":     true,
		"x-api-key":         true,
		"x-goog-api-key":    true,
		"content-type":      true,
		"content-length":    true,
		"host":              true,
		"accept-encoding":   true, // 让 Go HTTP client 自动解压；转发压缩头会导致返回原始压缩流
	}
	for k, vs := range ctx.R.Header {
		lower := strings.ToLower(k)
		if hopByHopHeaders[k] || skip[lower] {
			continue
		}
		for _, v := range vs {
			req.Header.Add(k, v)
		}
	}
}

// ── Adaptor factory ──────────────────────────────────────

func GetAdaptor(apiType int) Adaptor {
	switch apiType {
	case 1:
		return &AnthropicAdaptor{}
	case 2:
		return &GeminiAdaptor{}
	default:
		return &OpenAIAdaptor{}
	}
}

// ── Relay Mode Detection ─────────────────────────────────

func GetRelayMode(path string) int {
	if strings.HasPrefix(path, "/v1/chat/completions") {
		return 1 // ChatCompletions
	}
	if strings.HasPrefix(path, "/v1/completions") {
		return 2 // Completions
	}
	if strings.HasPrefix(path, "/v1/embeddings") || strings.HasSuffix(path, "embeddings") {
		return 3 // Embeddings
	}
	if strings.HasPrefix(path, "/v1/images/generations") {
		return 5 // ImagesGenerations
	}
	if strings.HasPrefix(path, "/v1/audio/speech") {
		return 8 // AudioSpeech
	}
	if strings.HasPrefix(path, "/v1/audio/transcriptions") {
		return 9 // AudioTranscription
	}
	return 0
}
