package gateway

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"

	"github.com/computeflux/tee-provider/pkg/model"
)

type contextKey string

const (
	ctxToken    contextKey = "gateway_token"
	ctxProvider contextKey = "gateway_provider"
	ctxModel    contextKey = "gateway_model"
	ctxEndpoint contextKey = "gateway_endpoint"
	ctxBody     contextKey = "gateway_body"
)

// TokenAuthMiddleware validates bearer API key
func TokenAuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
		if key == "" {
			writeError(w, http.StatusUnauthorized, "missing api key", "auth_error")
			return
		}
		token := findTokenCached(key)
		if token == nil {
			log.Printf("[AUTH] token not found for key prefix: %s...", key[:min(16, len(key))])
			writeError(w, http.StatusUnauthorized, "invalid api key", "auth_error")
			return
		}
		if !token.IsActive {
			log.Printf("[AUTH] token inactive for key prefix: %s...", key[:min(16, len(key))])
			writeError(w, http.StatusUnauthorized, "invalid api key", "auth_error")
			return
		}
		ctx := context.WithValue(r.Context(), ctxToken, token)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// DistributeMiddleware routes based on "channel/model" format
// e.g. "openai/gpt-4" → query contract → verify model exists → pick endpoint by user hash → route
func DistributeMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Read body early to extract model name for correct routing.
		// Body is reset so downstream handler can re-read it.
		bodyBytes, err := readRequestBody(r)
		if err != nil {
			writeError(w, http.StatusBadRequest, "failed to read request body", "invalid_request")
			return
		}

		// Parse model from JSON body first, fall back to query string.
		var reqBody struct {
			Model string `json:"model"`
		}
		json.Unmarshal(bodyBytes, &reqBody)

		modelName := reqBody.Model
		if modelName == "" {
			modelName = r.URL.Query().Get("model")
		}
		if modelName == "" {
			writeError(w, http.StatusNotFound, "model is null", "not_found")
			return
		}

		// Parse "channel/model" format
		providerName, actualModel := parseProviderModel(modelName)
		token := GetTokenFromContext(r)

		// 1. 从合约查询模型（必须存在且活跃）
		m := findGatewayModelByName(actualModel)
		if m == nil {
			writeError(w, http.StatusNotFound, "model not found: "+actualModel, "not_found")
			return
		}

		fmt.Println("ProviderName", providerName)
		fmt.Println("model")
		fmt.Println(m)
		fmt.Println("ProviderType", m.ProviderType)
		fmt.Println("Model", actualModel)

		// 2. 验证请求的 Provider 与模型所属 Provider 一致
		expectedProvider := providerTypeName(m.ProviderType)
		// if !strings.EqualFold(expectedProvider, providerName) {
		// 	writeError(w, http.StatusBadRequest,
		// 		fmt.Sprintf("model %s belongs to provider %s, not %s", actualModel, expectedProvider, providerName),
		// 		"invalid_request")
		// 	return
		// }

		// 3. 从合约构建 endpoints（URI + APIKeys）
		endpoints := buildEndpointsFromModel(m)
		if len(endpoints) == 0 {
			writeError(w, http.StatusServiceUnavailable,
				"no available endpoints for model "+actualModel, "server_error")
			return
		}

		// 4. 按用户 hash 选 endpoint（sticky）
		userKey := token.Owner
		if userKey == "" {
			userKey = r.RemoteAddr
		}
		b := model.NewEndpointBalancerFromEndpoints(endpoints)
		if b == nil {
			writeError(w, http.StatusServiceUnavailable,
				"no available endpoint for model "+actualModel, "server_error")
			return
		}
		endpoint := b.Pick(userKey + ":" + actualModel)
		if endpoint == nil {
			writeError(w, http.StatusServiceUnavailable,
				"no available endpoint for model "+actualModel, "server_error")
			return
		}

		// 5. 构建虚拟 channel（供下游 metrics / usage 使用）
		channel := &model.GatewayProviderInfo{
			ID:       m.ProviderID,
			Name:     expectedProvider,
			IsActive: true,
		}

		ctx := context.WithValue(r.Context(), ctxProvider, channel)
		ctx = context.WithValue(ctx, ctxModel, actualModel)
		ctx = context.WithValue(ctx, ctxEndpoint, endpoint)
		ctx = context.WithValue(ctx, ctxBody, bodyBytes)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// ── "channel/model" parser ───────────────────────────────
// "openai/gpt-4" → ("openai", "gpt-4")
// "gpt-4"        → ("openai", "gpt-4")  (default channel)
func parseProviderModel(input string) (channel, model string) {
	parts := strings.SplitN(input, "/", 2)
	if len(parts) == 2 {
		return parts[0], parts[1]
	}
	return "openai", input
}

// GetBodyFromContext returns cached body bytes set by DistributeMiddleware.
func GetBodyFromContext(r *http.Request) []byte {
	b, _ := r.Context().Value(ctxBody).([]byte)
	return b
}

// ── Context accessors ────────────────────────────────────

func GetTokenFromContext(r *http.Request) *model.GatewayTokenInfo {
	t, _ := r.Context().Value(ctxToken).(*model.GatewayTokenInfo)
	return t
}

func GetChannelFromContext(r *http.Request) *model.GatewayProviderInfo {
	c, _ := r.Context().Value(ctxProvider).(*model.GatewayProviderInfo)
	return c
}

func GetModelFromContext(r *http.Request) string {
	m, _ := r.Context().Value(ctxModel).(string)
	return m
}

func GetEndpointFromContext(r *http.Request) *model.GatewayEndpoint {
	e, _ := r.Context().Value(ctxEndpoint).(*model.GatewayEndpoint)
	return e
}
