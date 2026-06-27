package gateway

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/computeflux/tee-provider/pkg/api/gateway/protocol"
	relay "github.com/computeflux/tee-provider/pkg/services/gateway_relay"
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
	endpoint := GetEndpointFromContext(r)
	modelData := GetModelFromContext(r)
	if endpoint == nil || modelData == nil {
		writeProtocolError(w, r, http.StatusServiceUnavailable, "no route", "server_error")
		return
	}
	providerTypeStr := providerTypeToLimiterKey(modelData.Data.ProviderType)
	rpmLimit, maxConc := GetProviderLimits(providerTypeStr)
	lim := getEndpointLimiter(endpoint.ID, rpmLimit)
	lim.SetLimits(rpmLimit, maxConc)
	if !lim.Allow() {
		rpm, inf := lim.Stats()
		writeProtocolError(w, r, http.StatusTooManyRequests,
			fmt.Sprintf("rate limited: %d/%d RPM, %d/%d inflight", rpm, rpmLimit, inf, maxConc), "rate_limit")
		return
	}
	defer lim.Done()

	bodyBytes := GetBodyFromContext(r)
	var reqBody struct {
		Model  string `json:"model"`
		Stream bool   `json:"stream"`
	}
	json.Unmarshal(bodyBytes, &reqBody)

	inboundProto := protocol.FromContext(r)
	providerProto := protocol.ProviderNativeProtocol(modelData.Data.ProviderType)

	requestPath := r.URL.Path
	if inboundProto == protocol.ProtocolAnthropic {
		requestPath = protocol.CanonicalAnthropicPath(requestPath)
	}
	upstreamPath, pathErr := protocol.ResolveUpstreamPath(inboundProto, providerProto, requestPath)
	if pathErr != nil {
		writeProtocolError(w, r, http.StatusNotImplemented, pathErr.Error(), "not_implemented")
		return
	}
	relayR := r.Clone(r.Context())
	relayR.URL.Path = upstreamPath

	// 同协议 = 直接转发
	if inboundProto == providerProto {
		usage, bizErr, err := relayToUpstreamFn(w, relayR, endpoint, modelData.Data.Name, bodyBytes, reqBody.Stream, modelData.Data.ProviderType)
		if bizErr != nil {
			EmitProviderMetric(modelData.Data.ProviderID, false)
			writeProtocolError(w, r, bizErr.StatusCode, bizErr.Error.Message, "upstream_error")
			return
		}
		if err != nil {
			EmitProviderMetric(modelData.Data.ProviderID, false)
			writeProtocolError(w, r, http.StatusBadGateway, err.Error(), "server_error")
			return
		}
		EmitProviderMetric(modelData.Data.ProviderID, true)
		token := GetTokenFromContext(r)
		if usage != nil {
			recordUsageAsyncFn(token.Addr, token.ID, modelData,
				uint64(usage.PromptTokens), uint64(usage.CompletionTokens), uint64(usage.PromptTokensDetails.CachedTokens))
		}
		return
	}

	// 跨协议转换
	usage, success := streamRelayFn(w, relayR, endpoint, modelData.Data.Name, bodyBytes, modelData.Data.ProviderType, reqBody.Stream, inboundProto, providerProto, bodyBytes)
	EmitProviderMetric(modelData.Data.ProviderID, success)
	token := GetTokenFromContext(r)
	if usage != nil && token != nil {
		recordUsageAsyncFn(token.Addr, token.ID, modelData,
			uint64(usage.PromptTokens), uint64(usage.CompletionTokens), uint64(usage.PromptTokensDetails.CachedTokens))
	}
}

var relayToUpstreamFn = relayToUpstream
var streamRelayFn = streamRelay

func relayToUpstream(w http.ResponseWriter, r *http.Request, endpoint *types.GatewayEndpoint, modelName string, bodyBytes []byte, isStream bool, providerType uint16) (*relay.Usage, *relay.ErrorWithStatusCode, error) {
	ctx := relay.NewRelayContext(w, r)
	meta := relay.GetMetaByContext(ctx)
	meta.BaseURL = endpoint.URL
	meta.APIKey = endpoint.APIKey
	meta.IsStream = isStream
	meta.OriginModelName = modelName
	meta.ProviderId = endpoint.ProviderID
	meta.ProviderType = providerType

	adaptor := relay.GetAdaptorForProviderTypeCode(providerType)
	adaptor.Init(meta)
	resp, err := adaptor.DoRequest(ctx, meta, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, nil, fmt.Errorf("upstream: %w", err)
	}

	usage, bizErr := adaptor.DoResponse(ctx, resp, meta)
	if resp != nil && resp.Body != nil {
		resp.Body.Close()
	}
	if bizErr != nil {
		return nil, bizErr, nil
	}
	return usage, nil, nil
}

func streamRelay(w http.ResponseWriter, r *http.Request, endpoint *types.GatewayEndpoint, modelName string, bodyBytes []byte, providerType uint16, isStream bool, inbound, provider protocol.Protocol, origBody []byte) (*relay.Usage, bool) {
	if !isStream {
		converted, conv := convertRequest(bodyBytes, inbound, provider, modelName, providerType)
		if conv != nil {
			writeProtocolError(w, r, http.StatusBadRequest, conv.Error(), "invalid_request")
			return nil, false
		}
		bodyBytes = converted
	}

	ctx := relay.NewRelayContext(w, r)
	meta := relay.GetMetaByContext(ctx)
	meta.BaseURL = endpoint.URL
	meta.APIKey = endpoint.APIKey
	meta.IsStream = isStream
	meta.OriginModelName = modelName
	meta.ProviderId = endpoint.ProviderID
	meta.ProviderType = providerType

	adaptor := relay.GetAdaptorForProviderTypeCode(providerType)
	adaptor.Init(meta)
	resp, err := adaptor.DoRequest(ctx, meta, bytes.NewReader(bodyBytes))
	if err != nil {
		writeProtocolError(w, r, http.StatusBadGateway, err.Error(), "server_error")
		return nil, false
	}
	defer resp.Body.Close()

	if isStream {
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		usage, err := streamConvert(resp.Body, w, inbound, provider)
		if err != nil {
			writeProtocolError(w, r, http.StatusInternalServerError, err.Error(), "server_error")
			return nil, false
		}
		return usage, true
	}

	body, _ := io.ReadAll(resp.Body)
	converted, usage, conv := convertResponse(body, inbound, provider, origBody, providerType)
	if conv != nil {
		writeProtocolError(w, r, http.StatusInternalServerError, conv.Error(), "server_error")
		return nil, false
	}
	w.Header().Set("Content-Type", "application/json")
	if _, err := w.Write(converted); err != nil {
		fmt.Printf("[relay] write response body: %v", err)
	}
	return usage, true
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

func providerTypeToLimiterKey(providerType uint16) string {
	switch providerType {
	case cgateway.ProviderClaude:
		return "anthropic"
	case cgateway.ProviderGemini:
		return "gemini"
	case cgateway.ProviderPersonal:
		return "local"
	default:
		return "openai"
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
