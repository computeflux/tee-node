package relay

import (
	"io"
	"net/http"
	"strings"

	cgateway "github.com/computeflux/tee-provider/tee-chain/runtime/module/gateway"
)

// ── Types ─────────────────────────────────────────────────

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

// ── Adaptor factory ──────────────────────────────────────

func GetAdaptorForProviderTypeCode(providerType uint16) Adaptor {
	switch providerType {
	case cgateway.ProviderClaude:
		return &AnthropicAdaptor{}
	case cgateway.ProviderGemini:
		return &GeminiAdaptor{}
	case cgateway.ProviderPersonal:
		return &LocalAdaptor{}
	default:
		return &OpenAIAdaptor{}
	}
}

// ── Shared helpers ───────────────────────────────────────

// hopByHopHeaders are HTTP headers that should not be forwarded upstream.
var hopByHopHeaders = map[string]bool{
	"Connection":          true,
	"Keep-Alive":          true,
	"Proxy-Authenticate":  true,
	"Proxy-Authorization": true,
	"Te":                  true,
	"Trailers":            true,
	"Transfer-Encoding":   true,
	"Upgrade":             true,
	"Content-Encoding":    true,
	"Content-Length":      true,
}

func forwardClientHeaders(ctx *RelayContext, req *http.Request) {
	if ctx == nil || ctx.R == nil {
		return
	}
	skip := map[string]bool{
		"authorization":   true,
		"x-api-key":       true,
		"x-goog-api-key":  true,
		"content-type":    true,
		"content-length":  true,
		"host":            true,
		"accept-encoding": true,
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
