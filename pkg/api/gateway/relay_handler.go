package gateway

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/computeflux/tee-provider/pkg/api/gateway/protocol"
	relay "github.com/computeflux/tee-provider/pkg/services/gateway_relay"
	"github.com/computeflux/tee-provider/pkg/services/scheduler"
	"github.com/computeflux/tee-provider/pkg/types"
	cgateway "github.com/computeflux/tee-provider/tee-chain/runtime/module/gateway"
	"github.com/maximhq/bifrost/core/schemas"
)

func RegisterGatewayRoutes(r chi.Router) {
	common := func(r chi.Router) {
		r.Use(RecoveryMiddleware)
		r.Use(RequestIDMiddleware)
		r.Use(LoggerMiddleware)
		r.Use(GzipDecodeMiddleware)
		r.Use(RateLimitMiddleware(60, time.Minute))
		r.Use(CacheMiddleware)
	}
	r.Route("/v1", func(r chi.Router) {
		common(r)
		r.Get("/models", HandleListModels)
		r.Group(func(r chi.Router) {
			r.Use(TokenAuthMiddleware)
			r.Group(func(r chi.Router) {
				r.Use(DistributeMiddleware)
				r.Post("/chat/completions", HandleRelay)
				r.Post("/completions", HandleRelay)
				r.Post("/embeddings", HandleRelay)
				r.Post("/images/generations", HandleRelay)
				r.Post("/audio/speech", HandleRelay)
				r.Post("/audio/transcriptions", HandleRelay)
			})
		})
	})
	r.Route("/anthropic", func(r chi.Router) {
		common(r)
		r.Group(func(r chi.Router) {
			r.Use(protocol.AnthropicMiddleware)
			r.Use(TokenAuthMiddleware)
			r.Group(func(r chi.Router) {
				r.Use(DistributeMiddleware)
				r.Post("/v1/messages", HandleRelay)
				r.Post("/v1/messages/count_tokens", HandleRelay)
				r.Post("/v1/complete", HandleRelay)
			})
		})
	})
}

func HandleRelay(w http.ResponseWriter, r *http.Request) {
	gw := scheduler.GetGlobal()
	if gw == nil {
		writeProtocolError(w, r, http.StatusServiceUnavailable, "scheduler not ready", "server_error")
		return
	}

	// 逻辑模型/路由名:去掉 "auto/" 或 "provider/" 前缀,只留 scheduler 的路由键。
	_, modelKey := parseProviderModel(GetModelNameFromContext(r))
	bodyBytes := GetBodyFromContext(r)
	token := GetTokenFromContext(r)

	var reqBody struct {
		Stream bool `json:"stream"`
	}
	json.Unmarshal(bodyBytes, &reqBody)

	inbound := protocol.FromContext(r)
	requestPath := r.URL.Path
	if inbound == protocol.ProtocolAnthropic {
		requestPath = protocol.CanonicalAnthropicPath(requestPath)
	}

	// session 亲和:同一用户+模型优先复用上次端点(命中上游 KV cache)。
	sessionID := ""
	if token != nil {
		sessionID = token.Owner + ":" + modelKey
	}

	// 把入口协议 + 请求路径塞进 ctx,scheduler 选完端点后由 provider client 据此转换请求体。
	ctx := scheduler.WithRequest(r.Context(), string(inbound), requestPath)

	if reqBody.Stream {
		res, err := gw.ScheduleStream(ctx, modelKey, bodyBytes, sessionID)
		if err != nil {
			st, msg, typ := mapScheduleError(err)
			writeProtocolError(w, r, st, msg, typ)
			return
		}
		ep := res.Meta.Endpoint
		prov := protocol.ProviderNativeProtocol(ep.ProviderType)

		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")

		usage, serr := writeRelayStream(ctx, w, res.Stream, inbound, prov)
		EmitProviderMetric(ep.ContractProviderID, serr == nil)
		if serr != nil {
			// header 多半已发出,无法再写 error body,仅记录。
			fmt.Printf("[relay] stream error model=%s provider=%d: %v\n", modelKey, ep.ContractProviderID, serr)
			return
		}
		recordRelayUsage(token, ep, usage)
		return
	}

	res, err := gw.Schedule(ctx, modelKey, bodyBytes, sessionID)
	if err != nil {
		st, msg, typ := mapScheduleError(err)
		writeProtocolError(w, r, st, msg, typ)
		return
	}
	ep := res.Meta.Endpoint
	prov := protocol.ProviderNativeProtocol(ep.ProviderType)

	var (
		outBody []byte
		usage   *relay.Usage
	)
	if inbound == prov {
		outBody = res.Body
		usage = parseNativeUsage(res.Body, prov)
	} else {
		var conv error
		outBody, usage, conv = convertResponse(res.Body, inbound, prov, bodyBytes, ep.ProviderType)
		if conv != nil {
			EmitProviderMetric(ep.ContractProviderID, false)
			writeProtocolError(w, r, http.StatusInternalServerError, conv.Error(), "server_error")
			return
		}
	}

	w.Header().Set("Content-Type", "application/json")
	if _, err := w.Write(outBody); err != nil {
		fmt.Printf("[relay] write response body: %v\n", err)
	}
	EmitProviderMetric(ep.ContractProviderID, true)
	recordRelayUsage(token, ep, usage)
}

// writeRelayStream 把 scheduler 返回的 provider-native 流写成 inbound 协议的 SSE。
//   - 同协议:逐行透传 + 尽力 sniff usage
//   - 跨协议:把 chunk 重组成 reader 喂给 streamConvert 转换
func writeRelayStream(ctx context.Context, w http.ResponseWriter, s scheduler.Stream, inbound, prov protocol.Protocol) (*relay.Usage, error) {
	flusher, _ := w.(http.Flusher)

	if inbound != prov {
		return streamConvert(newChunkReader(ctx, s), flushWriter{w: w, fl: flusher}, inbound, prov)
	}

	var usage *relay.Usage
	for {
		chunk, err := s.Next(ctx)
		if err != nil {
			return usage, err
		}
		if chunk != nil && len(chunk.Data) > 0 {
			if _, err := w.Write(chunk.Data); err != nil {
				return usage, err
			}
			if flusher != nil {
				flusher.Flush()
			}
			if u := sniffStreamLine(chunk.Data, prov); u != nil {
				usage = u
			}
		}
		if chunk != nil && chunk.IsFinal {
			return usage, nil
		}
	}
}

// sniffStreamLine 从一条 raw SSE 行(可能形如 "data: {...}\n")里提取 usage。
func sniffStreamLine(line []byte, prov protocol.Protocol) *relay.Usage {
	s := trimSSELine(line)
	if s == nil {
		return nil
	}
	return sniffStreamUsage(s, prov)
}

// trimSSELine 去掉行尾空白和可选的 "data:" 前缀,返回 payload(非 data 行返回 nil)。
func trimSSELine(line []byte) []byte {
	// 去尾部 \r\n 和空白
	end := len(line)
	for end > 0 && (line[end-1] == '\n' || line[end-1] == '\r' || line[end-1] == ' ') {
		end--
	}
	s := line[:end]
	if len(s) >= 5 && string(s[:5]) == "data:" {
		s = s[5:]
		for len(s) > 0 && s[0] == ' ' {
			s = s[1:]
		}
		return s
	}
	return nil
}

// recordRelayUsage 异步记录用量计费。Endpoint 已携带计费身份(ModelID/ContractProviderID)。
func recordRelayUsage(token *types.GatewayTokenInfo, ep *scheduler.Endpoint, usage *relay.Usage) {
	if usage == nil || token == nil || ep == nil {
		return
	}
	model := &cgateway.ModelInfoWithID{
		ID:   ep.ModelID,
		Data: cgateway.ModelInfo{ProviderID: ep.ContractProviderID, Name: ep.ModelName},
	}
	recordUsageAsyncFn(token.Addr, token.ID, model,
		uint64(usage.PromptTokens), uint64(usage.CompletionTokens), uint64(usage.PromptTokensDetails.CachedTokens))
}

// mapScheduleError 把 scheduler 错误映射为 HTTP 状态码 + 协议错误类型。
func mapScheduleError(err error) (status int, msg, errType string) {
	// 先处理带具体 HTTP status 的上游错误。
	// 注意:SchedError.Is 仅按 Class 匹配,ErrModelNotFound 也是 ClassClient,
	// 若先走 sentinel 分支会把所有 4xx 上游错误误判成 404,因此 Status!=0 优先。
	var se *scheduler.SchedError
	if errors.As(err, &se) && se.Status != 0 {
		typ := "upstream_error"
		switch {
		case se.Status == http.StatusTooManyRequests:
			typ = "rate_limit"
		case se.Status >= 400 && se.Status < 500:
			typ = "invalid_request"
		}
		return se.Status, se.Message, typ
	}
	switch {
	case errors.Is(err, scheduler.ErrModelNotFound):
		return http.StatusNotFound, "model not found", "not_found"
	case errors.Is(err, scheduler.ErrNoHealthyEndpoint), errors.Is(err, scheduler.ErrAllEndpointsDead):
		return http.StatusServiceUnavailable, "no available endpoint", "server_error"
	case errors.Is(err, scheduler.ErrCanceled):
		return http.StatusBadRequest, "request cancelled", "invalid_request"
	}
	return http.StatusBadGateway, err.Error(), "server_error"
}

func convertRequest(bodyBytes []byte, inbound, provider protocol.Protocol, modelName string, providerType uint16) ([]byte, error) {
	switch {
	case inbound == protocol.ProtocolAnthropic && provider == protocol.ProtocolOpenAI:
		return protocol.ConvertAnthropicRequestToOpenAI(bodyBytes, providerType)
	case inbound == protocol.ProtocolOpenAI && provider == protocol.ProtocolAnthropic:
		return protocol.ConvertOpenAIRequestToAnthropic(bodyBytes, providerType)
	case inbound == protocol.ProtocolOpenAI && provider == protocol.ProtocolGemini:
		return protocol.ConvertOpenAIRequestToGemini(bodyBytes, providerType)
	}
	return bodyBytes, nil
}

func convertResponse(body []byte, inbound, provider protocol.Protocol, origBody []byte, providerType uint16) ([]byte, *relay.Usage, error) {
	switch {
	case inbound == protocol.ProtocolAnthropic && provider == protocol.ProtocolOpenAI:
		out, usage, err := protocol.ConvertOpenAIResponseToAnthropic(body)
		if err != nil {
			return nil, nil, err
		}
		return out, bifrostUsageToRelay(usage), nil
	case inbound == protocol.ProtocolOpenAI && provider == protocol.ProtocolAnthropic:
		out, usage, err := protocol.ConvertAnthropicResponseToOpenAI(body)
		if err != nil {
			return nil, nil, err
		}
		return out, bifrostUsageToRelay(usage), nil
	case inbound == protocol.ProtocolOpenAI && provider == protocol.ProtocolGemini:
		out, usage, err := protocol.ConvertGeminiResponseToOpenAI(body)
		if err != nil {
			return nil, nil, err
		}
		return out, bifrostUsageToRelay(usage), nil
	default:
		return body, nil, fmt.Errorf("unsupported protocol pair: inbound=%v provider=%v", inbound, provider)
	}
}

// bifrostUsageToRelay 将 Bifrost 的 usage 转换为 relay 层的 usage。
// 注意：Bifrost 端的 CompletionTokensDetails.ReasoningTokens 等字段被故意忽略，
// 因为 relay.Usage 当前不携带这些信息。若后续计费/限流需要，应在 relay.Usage 中新增字段并在此补充映射。
func bifrostUsageToRelay(u *schemas.BifrostLLMUsage) *relay.Usage {
	if u == nil {
		return nil
	}
	cachedTokens := 0
	if u.PromptTokensDetails != nil {
		cachedTokens = u.PromptTokensDetails.CachedReadTokens
	}
	return &relay.Usage{
		PromptTokens:     u.PromptTokens,
		CompletionTokens: u.CompletionTokens,
		TotalTokens:      u.TotalTokens,
		PromptTokensDetails: relay.PromptTokensDetails{
			CachedTokens: cachedTokens,
		},
	}
}

func streamConvert(r io.Reader, w io.Writer, inbound, provider protocol.Protocol) (*relay.Usage, error) {
	switch {
	case inbound == protocol.ProtocolAnthropic && provider == protocol.ProtocolOpenAI:
		usage, err := protocol.StreamOpenAIResponseToAnthropic(r, w)
		if err != nil {
			return nil, err
		}
		return bifrostUsageToRelay(usage), nil
	case inbound == protocol.ProtocolOpenAI && provider == protocol.ProtocolAnthropic:
		usage, err := protocol.StreamAnthropicResponseToOpenAI(r, w)
		if err != nil {
			return nil, err
		}
		return bifrostUsageToRelay(usage), nil
	case inbound == protocol.ProtocolOpenAI && provider == protocol.ProtocolGemini:
		usage, err := protocol.StreamGeminiResponseToOpenAI(r, w)
		if err != nil {
			return nil, err
		}
		return bifrostUsageToRelay(usage), nil
	default:
		return nil, fmt.Errorf("unsupported stream protocol pair: inbound=%v provider=%v", inbound, provider)
	}
}

func HandleListModels(w http.ResponseWriter, r *http.Request) {
	models := make([]map[string]any, 0)
	for _, m := range getCachedGatewayModelsFn() {
		models = append(models, map[string]any{
			"id": m.Name, "object": "model", "owned_by": fmt.Sprintf("%x", m.Owner.V),
		})
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{"object": "list", "data": models})
}
