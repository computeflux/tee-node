package gateway

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/computeflux/tee-provider/pkg/model"
	"github.com/computeflux/tee-provider/pkg/services/gateway/relay"
)

func RegisterGatewayRoutes(r chi.Router) {
	r.Route("/v1", func(r chi.Router) {
		r.Use(RecoveryMiddleware)
		r.Use(RequestIDMiddleware)
		r.Use(LoggerMiddleware)
		r.Use(GzipDecodeMiddleware)
		r.Use(RateLimitMiddleware(60, time.Minute))
		r.Use(CacheMiddleware)

		// 公开端点（模型列表）
		r.Get("/models", HandleListModels)

		// 需要 Token 认证的端点
		r.Group(func(r chi.Router) {
			r.Use(TokenAuthMiddleware)

			// AI 调用（需要 DistributeMiddleware 做模型路由）
			r.Group(func(r chi.Router) {
				r.Use(DistributeMiddleware)
				r.Post("/chat/completions", HandleRelay)
				r.Post("/completions", HandleRelay)
				r.Post("/embeddings", HandleRelay)
				r.Post("/images/generations", HandleRelay)
				r.Post("/audio/speech", HandleRelay)
				r.Post("/audio/transcriptions", HandleRelay)
			})

			// 历史查询（不需要 DistributeMiddleware）
			r.Get("/logs", HandleLogs)
			r.Post("/data/export", HandleDataExport)
		})
	})
}

func HandleRelay(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	channel := GetChannelFromContext(r)
	endpoint := GetEndpointFromContext(r)
	modelName := GetModelFromContext(r)
	token := GetTokenFromContext(r)
	if channel == nil || endpoint == nil {
		writeError(w, http.StatusServiceUnavailable, "no route", "server_error")
		return
	}

	// Endpoint 级别限流 — 这才是真正的瓶颈
	provider := relay.DetectProvider(endpoint.URL)
	rpmLimit, maxConc := GetProviderLimits(provider)
	lim := getEndpointLimiter(endpoint.ID, rpmLimit)
	lim.SetLimits(rpmLimit, maxConc)
	if !lim.Allow() {
		rpm, inf := lim.Stats()
		writeError(w, http.StatusTooManyRequests,
			fmt.Sprintf("rate limited: %d/%d RPM, %d/%d inflight", rpm, rpmLimit, inf, maxConc),
			"rate_limit")
		return
	}
	defer lim.Done()

	// Use cached body from DistributeMiddleware if available, otherwise read.
	bodyBytes := GetBodyFromContext(r)
	var reqBody struct {
		Model  string `json:"model"`
		Stream bool   `json:"stream"`
	}
	json.Unmarshal(bodyBytes, &reqBody)

	usage, bizErr, err := relayToUpstream(w, r, endpoint, modelName, bodyBytes, reqBody.Stream)

	// Business error (e.g. upstream 4xx): already contains status code and message.
	if bizErr != nil {
		EmitChannelMetric(channel.ID, false)
		writeError(w, bizErr.StatusCode, bizErr.Error.Message, "upstream_error")
		return
	}

	// Transport error (e.g. connection refused).
	if err != nil {
		EmitChannelMetric(channel.ID, false)
		writeError(w, http.StatusBadGateway, err.Error(), "server_error")
		return
	}

	EmitChannelMetric(channel.ID, true)

	var tokenID uint64
	if token != nil {
		tokenID = token.ID
	}
	var totalTokens, cachedTokens int
	if usage != nil {
		totalTokens = usage.TotalTokens
		cachedTokens = usage.PromptTokensDetails.CachedTokens
	}
	recordUsageAsync(tokenID, time.Now().Format("2006-01-02"), channel.ID, uint64(totalTokens), uint64(cachedTokens))

	elapsed := time.Since(start)
	tkns := 0
	if usage != nil {
		tkns = usage.TotalTokens
	}
	GATEWAYLOG().Printf("%s %s -> %s (%d tokens, %v)", r.Method, modelName, endpoint.URL, tkns, elapsed)
}

func relayToUpstream(w http.ResponseWriter, r *http.Request, endpoint *model.GatewayEndpoint, modelName string, bodyBytes []byte, isStream bool) (*relay.Usage, *relay.ErrorWithStatusCode, error) {
	ctx := relay.NewRelayContext(w, r)
	ctx.Set("request_model", modelName)

	meta := relay.GetByContext(ctx)
	meta.BaseURL = endpoint.URL
	meta.APIKey = endpoint.APIKey
	meta.IsStream = isStream

	adaptor := relay.GetAdaptorForProvider(relay.DetectProvider(endpoint.URL))
	adaptor.Init(meta)

	resp, err := adaptor.DoRequest(ctx, meta, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, nil, fmt.Errorf("upstream: %w", err)
	}
	defer resp.Body.Close()

	usage, bizErr := adaptor.DoResponse(ctx, resp, meta)
	if bizErr != nil {
		GATEWAYLOG().Printf("[MONITOR] upstream error: %s (status=%d)", bizErr.Error.Message, bizErr.StatusCode)
		// Return the bizErr so the caller writes the error response once.
		return nil, bizErr, nil
	}
	return usage, nil, nil
}

func HandleListModels(w http.ResponseWriter, r *http.Request) {
	models := make([]map[string]interface{}, 0)

	// 从合约读取模型列表
	for _, m := range getCachedGatewayModels() {
		models = append(models, map[string]interface{}{
			"id": m.Name, "object": "model", "owned_by": fmt.Sprintf("%x", m.Owner.V),
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"object": "list", "data": models})
}
