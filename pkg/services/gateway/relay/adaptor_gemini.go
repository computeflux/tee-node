package relay

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// ── Gemini types ─────────────────────────────────────────

type GeminiRequest struct {
	Contents         []GeminiContent  `json:"contents"`
	GenerationConfig *GeminiGenConfig `json:"generationConfig,omitempty"`
	SafetySettings   []GeminiSafety   `json:"safetySettings,omitempty"`
}

type GeminiContent struct {
	Role  string       `json:"role,omitempty"`
	Parts []GeminiPart `json:"parts"`
}

type GeminiPart struct {
	Text string `json:"text,omitempty"`
}

type GeminiGenConfig struct {
	Temperature     float64 `json:"temperature,omitempty"`
	MaxOutputTokens int     `json:"maxOutputTokens,omitempty"`
}

type GeminiSafety struct {
	Category  string `json:"category"`
	Threshold string `json:"threshold"`
}

type GeminiResponse struct {
	Candidates []struct {
		Content struct {
			Parts []GeminiPart `json:"parts"`
		} `json:"content"`
		FinishReason string `json:"finishReason"`
	} `json:"candidates"`
	UsageMetadata *struct {
		PromptTokenCount     int `json:"promptTokenCount"`
		CandidatesTokenCount int `json:"candidatesTokenCount"`
		TotalTokenCount      int `json:"totalTokenCount"`
	} `json:"usageMetadata"`
	Error *struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

// ── GeminiAdaptor ────────────────────────────────────────

type GeminiAdaptor struct{}

func (a *GeminiAdaptor) Init(meta *Meta) {}

func (a *GeminiAdaptor) GetRequestURL(meta *Meta) (string, error) {
	action := "generateContent"
	if meta.IsStream {
		action = "streamGenerateContent?alt=sse"
	}
	return fmt.Sprintf("%s/v1beta/models/%s:%s", meta.BaseURL, meta.OriginModelName, action), nil
}

func (a *GeminiAdaptor) SetupRequestHeader(ctx *RelayContext, req *http.Request, meta *Meta) error {
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-goog-api-key", meta.APIKey)
	return nil
}

func (a *GeminiAdaptor) DoRequest(ctx *RelayContext, meta *Meta, body io.Reader) (*http.Response, error) {
	raw, _ := io.ReadAll(body)
	var oaiReq GeneralOpenAIRequest
	json.Unmarshal(raw, &oaiReq)

	// Convert to Gemini format
	gemReq := GeminiRequest{
		SafetySettings: []GeminiSafety{
			{Category: "HARM_CATEGORY_HARASSMENT", Threshold: "BLOCK_NONE"},
			{Category: "HARM_CATEGORY_HATE_SPEECH", Threshold: "BLOCK_NONE"},
			{Category: "HARM_CATEGORY_SEXUALLY_EXPLICIT", Threshold: "BLOCK_NONE"},
			{Category: "HARM_CATEGORY_DANGEROUS_CONTENT", Threshold: "BLOCK_NONE"},
		},
		GenerationConfig: &GeminiGenConfig{
			Temperature:     oaiReq.Temperature,
			MaxOutputTokens: oaiReq.MaxTokens,
		},
	}

	for _, msg := range oaiReq.Messages {
		role := "user"
		if msg.Role == "assistant" || msg.Role == "model" {
			role = "model"
		}
		gemReq.Contents = append(gemReq.Contents, GeminiContent{
			Role:  role,
			Parts: []GeminiPart{{Text: msg.Content}},
		})
	}

	gemBody, _ := json.Marshal(gemReq)
	url, _ := a.GetRequestURL(meta)
	req, err := http.NewRequestWithContext(ctx.Context(), "POST", url, strings.NewReader(string(gemBody)))
	if err != nil {
		return nil, err
	}
	a.SetupRequestHeader(ctx, req, meta)
	return DoHTTP(req)
}

func (a *GeminiAdaptor) DoResponse(ctx *RelayContext, resp *http.Response, meta *Meta) (*Usage, *ErrorWithStatusCode) {
	if meta.IsStream {
		return a.handleGeminiStream(ctx, resp)
	}
	return a.handleGeminiNormal(ctx, resp)
}

func (a *GeminiAdaptor) handleGeminiNormal(ctx *RelayContext, resp *http.Response) (*Usage, *ErrorWithStatusCode) {
	body, _ := io.ReadAll(resp.Body)
	var gemResp GeminiResponse
	json.Unmarshal(body, &gemResp)

	if gemResp.Error != nil {
		return nil, &ErrorWithStatusCode{
			StatusCode: resp.StatusCode,
			Error:      ErrorDetail{Message: gemResp.Error.Message, Type: "gemini_error"},
		}
	}

	// Convert to OpenAI format
	content := ""
	if len(gemResp.Candidates) > 0 && len(gemResp.Candidates[0].Content.Parts) > 0 {
		content = gemResp.Candidates[0].Content.Parts[0].Text
	}
	usage := &Usage{}
	if gemResp.UsageMetadata != nil {
		usage.PromptTokens = gemResp.UsageMetadata.PromptTokenCount
		usage.CompletionTokens = gemResp.UsageMetadata.CandidatesTokenCount
		usage.TotalTokens = gemResp.UsageMetadata.TotalTokenCount
	}

	oaiResp := map[string]interface{}{
		"object": "chat.completion",
		"choices": []map[string]interface{}{{
			"index":         0,
			"message":       map[string]string{"role": "assistant", "content": content},
			"finish_reason": "stop",
		}},
		"usage": map[string]int{
			"prompt_tokens": usage.PromptTokens, "completion_tokens": usage.CompletionTokens, "total_tokens": usage.TotalTokens,
		},
	}

	ctx.W.Header().Set("Content-Type", "application/json")
	json.NewEncoder(ctx.W).Encode(oaiResp)
	return usage, nil
}

func (a *GeminiAdaptor) handleGeminiStream(ctx *RelayContext, resp *http.Response) (*Usage, *ErrorWithStatusCode) {
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
			Candidates []struct {
				Content struct {
					Parts []GeminiPart `json:"parts"`
				} `json:"content"`
			} `json:"candidates"`
		}
		json.Unmarshal([]byte(data), &evt)

		text := ""
		if len(evt.Candidates) > 0 && len(evt.Candidates[0].Content.Parts) > 0 {
			text = evt.Candidates[0].Content.Parts[0].Text
		}

		chunk := map[string]interface{}{
			"object": "chat.completion.chunk",
			"choices": []map[string]interface{}{{
				"index": 0, "delta": map[string]string{"content": text},
			}},
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

func (a *GeminiAdaptor) GetModelList() []string {
	return []string{"gemini-1.5-pro", "gemini-1.5-flash", "gemini-2.0-flash"}
}

func (a *GeminiAdaptor) GetChannelName() string {
	return "gemini"
}
