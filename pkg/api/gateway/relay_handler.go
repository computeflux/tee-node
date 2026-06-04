package gateway

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/computeflux/tee-provider/pkg/model"
	relay "github.com/computeflux/tee-provider/pkg/services/gateway_relay"
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
		})
	})
}

func HandleRelay(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	endpoint := GetEndpointFromContext(r)
	modelData := GetModelFromContext(r)
	if endpoint == nil || modelData == nil {
		writeError(w, http.StatusServiceUnavailable, "no route", "server_error")
		return
	}

	// Endpoint 级别限流 — 这才是真正的瓶颈
	providerType := relay.DetectProvider(endpoint.URL)
	rpmLimit, maxConc := GetProviderLimits(providerType)
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

	// Business error (e.g. upstream 4xx): already contains status code and message.
	usage, bizErr, err := relayToUpstreamFn(w, r, endpoint, modelData.Data.Name, bodyBytes, reqBody.Stream)
	if bizErr != nil {
		EmitProviderMetric(modelData.Data.ProviderID, false)
		writeError(w, bizErr.StatusCode, bizErr.Error.Message, "upstream_error")
		return
	}

	// Transport error (e.g. connection refused).
	if err != nil {
		EmitProviderMetric(modelData.Data.ProviderID, false)
		writeError(w, http.StatusBadGateway, err.Error(), "server_error")
		return
	}

	// 统计调用成功率和使用量
	EmitProviderMetric(modelData.Data.ProviderID, true)

	// 统计用户 Token 使用
	token := GetTokenFromContext(r)
	if usage != nil {
		recordUsageAsyncFn(token.Addr, token.ID, modelData, uint64(usage.PromptTokens), uint64(usage.CompletionTokens), uint64(usage.PromptTokensDetails.CachedTokens))
	}

	// 记录调用时间
	elapsed := time.Since(start)
	tkns := 0
	if usage != nil {
		tkns = usage.TotalTokens
	}
	GATEWAYLOG().Printf("%s %s -> %s (%d tokens, %v)", r.Method, modelData.Data.Name, endpoint.URL, tkns, elapsed)
}

// relayToUpstreamFn 可被测试替换为 mock。
var relayToUpstreamFn = relayToUpstream

func relayToUpstream(w http.ResponseWriter, r *http.Request, endpoint *model.GatewayEndpoint, modelName string, bodyBytes []byte, isStream bool) (*relay.Usage, *relay.ErrorWithStatusCode, error) {
	ctx := relay.NewRelayContext(w, r)

	meta := relay.GetMetaByContext(ctx)
	meta.BaseURL = endpoint.URL
	meta.APIKey = endpoint.APIKey
	meta.IsStream = isStream
	meta.OriginModelName = modelName

	adaptor := relay.GetAdaptorForProviderType(relay.DetectProvider(endpoint.URL))
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
	for _, m := range getCachedGatewayModelsFn() {
		models = append(models, map[string]interface{}{
			"id": m.Name, "object": "model", "owned_by": fmt.Sprintf("%x", m.Owner.V),
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"object": "list", "data": models})
}
