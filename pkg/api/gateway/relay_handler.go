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
	"github.com/computeflux/tee-provider/pkg/store"
	cgateway "github.com/computeflux/tee-provider/tee-chain/contracts/module/gateway"
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
	dropThinking := inboundProto == protocol.ProtocolAnthropic && providerProto == protocol.ProtocolOpenAI
	streamRelayFn(w, relayR, endpoint, modelData.Data.Name, bodyBytes, modelData.Data.ProviderType, reqBody.Stream, dropThinking, inboundProto, providerProto, bodyBytes)
}

var relayToUpstreamFn = relayToUpstream
var streamRelayFn = streamRelay

func relayToUpstream(w http.ResponseWriter, r *http.Request, endpoint *store.GatewayEndpoint, modelName string, bodyBytes []byte, isStream bool, providerType uint16) (*relay.Usage, *relay.ErrorWithStatusCode, error) {
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

func streamRelay(w http.ResponseWriter, r *http.Request, endpoint *store.GatewayEndpoint, modelName string, bodyBytes []byte, providerType uint16, isStream bool, dropThinking bool, inbound, provider protocol.Protocol, origBody []byte) {
	if !isStream {
		converted, conv := convertRequest(bodyBytes, inbound, provider, modelName, providerType)
		if conv != nil {
			writeProtocolError(w, r, http.StatusBadRequest, conv.Error(), "invalid_request")
			return
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
		return
	}
	defer resp.Body.Close()

	if isStream {
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		streamConvert(resp.Body, w, inbound, provider, dropThinking)
		return
	}

	body, _ := io.ReadAll(resp.Body)
	converted, conv := convertResponse(body, inbound, provider, dropThinking, origBody, providerType)
	if conv != nil {
		writeProtocolError(w, r, http.StatusInternalServerError, conv.Error(), "server_error")
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Write(converted)
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

func convertResponse(body []byte, inbound, provider protocol.Protocol, dropThinking bool, origBody []byte, providerType uint16) ([]byte, error) {
	switch {
	case inbound == protocol.ProtocolAnthropic && provider == protocol.ProtocolOpenAI:
		return protocol.ConvertOpenAIResponseToAnthropic(body, dropThinking)
	case inbound == protocol.ProtocolOpenAI && provider == protocol.ProtocolAnthropic:
		return protocol.ConvertAnthropicResponseToOpenAI(body)
	case inbound == protocol.ProtocolOpenAI && provider == protocol.ProtocolGemini:
		return protocol.ConvertGeminiResponseToOpenAI(body)
	}
	return body, nil
}

func streamConvert(r io.Reader, w io.Writer, inbound, provider protocol.Protocol, dropThinking bool) {
	switch {
	case inbound == protocol.ProtocolAnthropic && provider == protocol.ProtocolOpenAI:
		protocol.StreamOpenAIResponseToAnthropic(r, w, dropThinking)
	case inbound == protocol.ProtocolOpenAI && provider == protocol.ProtocolAnthropic:
		protocol.StreamAnthropicResponseToOpenAI(r, w)
	case inbound == protocol.ProtocolOpenAI && provider == protocol.ProtocolGemini:
		protocol.StreamGeminiResponseToOpenAI(r, w)
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
