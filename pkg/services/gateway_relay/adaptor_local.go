package relay

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	localtunnel "github.com/computeflux/tee-provider/pkg/services/local_tunnel"
)

// ── LocalAdaptor ─────────────────────────────────────────

type LocalAdaptor struct{}

func (a *LocalAdaptor) Init(meta *Meta) {}

func (a *LocalAdaptor) GetRequestURL(meta *Meta) (string, error) {
	return "", nil
}

func (a *LocalAdaptor) SetupRequestHeader(ctx *RelayContext, req *http.Request, meta *Meta) error {
	return nil
}

func (a *LocalAdaptor) DoRequest(ctx *RelayContext, meta *Meta, body io.Reader) (*http.Response, error) {
	modelID := meta.OriginModelName
	if modelID == "" {
		return nil, fmt.Errorf("local adaptor: origin model name is empty")
	}

	path := meta.RequestURLPath
	if path == "" {
		path = "/v1/chat/completions"
	}

	headers := make(map[string]string)
	if ctx.R != nil {
		for k, vs := range ctx.R.Header {
			if len(vs) == 0 {
				continue
			}
			if strings.ToLower(k) == "origin" {
				continue
			}
			headers[k] = vs[0]
		}
	}

	resp, err := localtunnel.ForwardRequest(ctx.Context(), meta.ProviderId, modelID, body, path, meta.IsStream, headers)
	if err != nil {
		return nil, fmt.Errorf("local tunnel forward: %w", err)
	}
	return resp, nil
}

func (a *LocalAdaptor) DoResponse(ctx *RelayContext, resp *http.Response, meta *Meta) (*Usage, *ErrorWithStatusCode) {
	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return nil, &ErrorWithStatusCode{
			StatusCode: resp.StatusCode,
			Error: ErrorDetail{
				Message: string(body),
				Type:    "upstream_error",
			},
		}
	}

	ctx.W.Header().Set("X-Provider-Type", "local")

	contentType := resp.Header.Get("Content-Type")
	if strings.Contains(contentType, "text/event-stream") || meta.IsStream {
		return a.handleLocalStream(ctx, resp)
	}
	return a.handleLocalNormal(ctx, resp)
}

func (a *LocalAdaptor) GetProviderName() string {
	return "local"
}

// ── SSE helpers ──────────────────────────────────────────

func setEventStreamHeaders(ctx *RelayContext) {
	ctx.W.Header().Set("Content-Type", "text/event-stream")
	ctx.W.Header().Set("Cache-Control", "no-cache")
	ctx.W.Header().Set("X-Accel-Buffering", "no")
}

func writeSSEDone(ctx *RelayContext) {
	fmt.Fprintf(ctx.W, "data: [DONE]\n\n")
	if flusher, ok := ctx.W.(interface{ Flush() }); ok {
		flusher.Flush()
	}
}

// ── handlers ─────────────────────────────────────────────

func (a *LocalAdaptor) handleLocalStream(ctx *RelayContext, resp *http.Response) (*Usage, *ErrorWithStatusCode) {
	flusher, ok := ctx.W.(http.Flusher)
	if !ok {
		return nil, &ErrorWithStatusCode{StatusCode: 500, Error: ErrorDetail{Message: "streaming not supported"}}
	}

	setEventStreamHeaders(ctx)

	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 0, 64*1024), 64*1024)

	usage := &Usage{}
	doneSeen := false
	for scanner.Scan() {
		line := scanner.Text()

		trimmed := strings.TrimRight(strings.TrimSpace(line), "\r")
		if trimmed == "" {
			fmt.Fprintln(ctx.W)
			flusher.Flush()
			continue
		}
		if !strings.HasPrefix(trimmed, "data: ") && (strings.HasPrefix(trimmed, "{") || strings.HasPrefix(trimmed, "[")) {
			trimmed = "data: " + trimmed
		}

		fmt.Fprintf(ctx.W, "%s\n", trimmed)
		flusher.Flush()

		if strings.HasPrefix(trimmed, "data: ") && !strings.Contains(trimmed, "[DONE]") {
			data := strings.TrimPrefix(trimmed, "data: ")
			var chunk struct {
				Usage *Usage `json:"usage,omitempty"`
			}
			if json.Unmarshal([]byte(data), &chunk) == nil && chunk.Usage != nil {
				usage = chunk.Usage
			}
		}
		if strings.Contains(trimmed, "[DONE]") {
			doneSeen = true
		}
	}

	if !doneSeen {
		writeSSEDone(ctx)
	}
	return usage, nil
}

func (a *LocalAdaptor) handleLocalNormal(ctx *RelayContext, resp *http.Response) (*Usage, *ErrorWithStatusCode) {
	for k, vs := range resp.Header {
		if hopByHopHeaders[k] {
			continue
		}
		for _, v := range vs {
			ctx.W.Header().Add(k, v)
		}
	}
	ctx.W.WriteHeader(resp.StatusCode)

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, &ErrorWithStatusCode{StatusCode: 500, Error: ErrorDetail{Message: "failed to read response"}}
	}

	var result struct {
		Usage *Usage `json:"usage,omitempty"`
	}
	var usage *Usage
	if json.Unmarshal(body, &result) == nil {
		usage = result.Usage
	}

	ctx.W.Write(body)
	return usage, nil
}
