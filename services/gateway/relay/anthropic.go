package relay

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// ── Anthropic types ──────────────────────────────────────

type AnthropicRequest struct {
	Model     string             `json:"model"`
	Messages  []AnthropicMessage `json:"messages"`
	System    string             `json:"system,omitempty"`
	MaxTokens int                `json:"max_tokens"`
	Stream    bool               `json:"stream,omitempty"`
}

type AnthropicMessage struct {
	Role    string            `json:"role"`
	Content []AnthropicContent `json:"content"`
}

type AnthropicContent struct {
	Type string `json:"type"`
	Text string `json:"text,omitempty"`
}

type AnthropicResponse struct {
	Id      string            `json:"id"`
	Model   string            `json:"model"`
	Content []AnthropicContent `json:"content"`
	Usage   struct {
		InputTokens  int `json:"input_tokens"`
		OutputTokens int `json:"output_tokens"`
	} `json:"usage"`
	Error *struct {
		Type    string `json:"type"`
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

// ── AnthropicAdaptor ─────────────────────────────────────

type AnthropicAdaptor struct {
	OpenAIAdaptor
}

func (a *AnthropicAdaptor) Init(meta *Meta) {}

func (a *AnthropicAdaptor) GetRequestURL(meta *Meta) (string, error) {
	return meta.BaseURL + "/v1/messages", nil
}

func (a *AnthropicAdaptor) SetupRequestHeader(ctx *RelayContext, req *http.Request, meta *Meta) error {
	req.Header.Set("x-api-key", meta.APIKey)
	req.Header.Set("anthropic-version", "2023-06-01")
	req.Header.Set("Content-Type", "application/json")
	return nil
}

func (a *AnthropicAdaptor) DoRequest(ctx *RelayContext, meta *Meta, body io.Reader) (*http.Response, error) {
	// Read OpenAI-format body
	raw, _ := io.ReadAll(body)
	var oaiReq GeneralOpenAIRequest
	json.Unmarshal(raw, &oaiReq)

	// Convert to Anthropic format
	antReq := AnthropicRequest{
		Model:     oaiReq.Model,
		MaxTokens: oaiReq.MaxTokens,
		Stream:    oaiReq.Stream,
	}
	if antReq.MaxTokens == 0 {
		antReq.MaxTokens = 4096
	}
	for _, msg := range oaiReq.Messages {
		if msg.Role == "system" && antReq.System == "" {
			antReq.System = msg.Content
			continue
		}
		antReq.Messages = append(antReq.Messages, AnthropicMessage{
			Role: msg.Role,
			Content: []AnthropicContent{{Type: "text", Text: msg.Content}},
		})
	}

	antBody, _ := json.Marshal(antReq)
	url, _ := a.GetRequestURL(meta)
	req, err := http.NewRequestWithContext(ctx.Context(), "POST", url, strings.NewReader(string(antBody)))
	if err != nil {
		return nil, err
	}
	a.SetupRequestHeader(ctx, req, meta)
	return http.DefaultClient.Do(req)
}

func (a *AnthropicAdaptor) DoResponse(ctx *RelayContext, resp *http.Response, meta *Meta) (*Usage, *ErrorWithStatusCode) {
	if meta.IsStream {
		return a.handleAnthropicStream(ctx, resp)
	}
	return a.handleAnthropicNormal(ctx, resp)
}

func (a *AnthropicAdaptor) handleAnthropicNormal(ctx *RelayContext, resp *http.Response) (*Usage, *ErrorWithStatusCode) {
	body, _ := io.ReadAll(resp.Body)
	var antResp AnthropicResponse
	json.Unmarshal(body, &antResp)

	if antResp.Error != nil {
		return nil, &ErrorWithStatusCode{
			StatusCode: resp.StatusCode,
			Error:      ErrorDetail{Message: antResp.Error.Message, Type: antResp.Error.Type},
		}
	}

	// Convert to OpenAI format
	content := ""
	for _, c := range antResp.Content {
		if c.Type == "text" {
			content += c.Text
		}
	}

	oaiResp := map[string]interface{}{
		"id":      antResp.Id,
		"object":  "chat.completion",
		"model":   antResp.Model,
		"choices": []map[string]interface{}{{"index": 0, "message": map[string]string{"role": "assistant", "content": content}, "finish_reason": "stop"}},
		"usage": map[string]int{
			"prompt_tokens":     antResp.Usage.InputTokens,
			"completion_tokens": antResp.Usage.OutputTokens,
			"total_tokens":      antResp.Usage.InputTokens + antResp.Usage.OutputTokens,
		},
	}

	ctx.W.Header().Set("Content-Type", "application/json")
	json.NewEncoder(ctx.W).Encode(oaiResp)

	return &Usage{
		PromptTokens:     antResp.Usage.InputTokens,
		CompletionTokens: antResp.Usage.OutputTokens,
		TotalTokens:      antResp.Usage.InputTokens + antResp.Usage.OutputTokens,
	}, nil
}

func (a *AnthropicAdaptor) handleAnthropicStream(ctx *RelayContext, resp *http.Response) (*Usage, *ErrorWithStatusCode) {
	flusher, _ := ctx.W.(http.Flusher)
	ctx.W.Header().Set("Content-Type", "text/event-stream")
	ctx.W.Header().Set("Cache-Control", "no-cache")

	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 0, 64*1024), 64*1024)

	totalTokens := 0
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		data := strings.TrimPrefix(line, "data: ")

		var evt struct {
			Type string `json:"type"`
			Delta *struct {
				Type string `json:"type"`
				Text string `json:"text"`
			} `json:"delta"`
			Usage *struct {
				OutputTokens int `json:"output_tokens"`
			} `json:"usage"`
		}
		if json.Unmarshal([]byte(data), &evt) != nil {
			continue
		}

		// Convert to OpenAI SSE format
		chunk := map[string]interface{}{
			"object": "chat.completion.chunk",
			"choices": []map[string]interface{}{{"index": 0, "delta": map[string]string{}}},
		}
		if evt.Delta != nil && evt.Delta.Text != "" {
			chunk["choices"].([]map[string]interface{})[0]["delta"] = map[string]string{"content": evt.Delta.Text}
		}
		if evt.Usage != nil {
			totalTokens = evt.Usage.OutputTokens
		}

		b, _ := json.Marshal(chunk)
		fmt.Fprintf(ctx.W, "data: %s\n\n", b)
		if flusher != nil {
			flusher.Flush()
		}
	}

	fmt.Fprintf(ctx.W, "data: [DONE]\n\n")
	if flusher != nil {
		flusher.Flush()
	}

	return &Usage{TotalTokens: totalTokens}, nil
}

func (a *AnthropicAdaptor) GetModelList() []string {
	return []string{"claude-3-opus", "claude-3-sonnet", "claude-3-haiku", "claude-3-5-sonnet"}
}

func (a *AnthropicAdaptor) GetChannelName() string {
	return "anthropic"
}
