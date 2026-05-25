package gateway

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/computeflux/tee-provider/pkg/model"
	modelpkg "github.com/computeflux/tee-provider/pkg/model"
	cgateway "github.com/computeflux/tee-provider/tee-chain/contracts/gateway"
)

type contextKey string

const (
	ctxToken    contextKey = "gateway_token"
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
		fmt.Println(token.Addr)
		if token == nil {
			GATEWAYLOG().Printf("[AUTH] token not found for key prefix: %s...", key[:min(16, len(key))])
			writeError(w, http.StatusUnauthorized, "invalid api key", "auth_error")
			return
		}
		if !token.IsActive {
			GATEWAYLOG().Printf("[AUTH] token inactive for key prefix: %s...", key[:min(16, len(key))])
			writeError(w, http.StatusUnauthorized, "invalid api key", "auth_error")
			return
		}

		ctx := context.WithValue(r.Context(), ctxToken, token)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// DistributeMiddleware routes based on "provider/model" format
func DistributeMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Read body early to extract model name for correct routing.
		// Body is reset so downstream handler can re-read it.
		bodyBytes, err := readRequestBody(r)
		if err != nil || bodyBytes == nil {
			writeError(w, http.StatusBadRequest, "failed to read request body", "invalid_request")
			return
		}

		// Parse model from JSON body first, fall back to query string.
		var bodyMap map[string]any
		modelNameArg := r.URL.Query().Get("model")
		if err := json.Unmarshal(bodyBytes, &bodyMap); err == nil {
			if bodyMap["model"] != nil {
				modelNameArg = bodyMap["model"].(string)
			}
		}
		if modelNameArg == "" {
			writeError(w, http.StatusNotFound, "model is null", "not_found")
			return
		}

		// Parse "provider/model" format
		providerName, modelName := parseProviderModel(modelNameArg)
		if modelNameArg != modelName {
			bodyMap["model"] = modelName
			if newBody, err := json.Marshal(bodyMap); err == nil {
				bodyBytes = newBody
			}
		}

		// 用 ProviderID 查询 Provider，验证名称与活跃状态
		provider := findProviderName(providerName)
		if provider == nil || !provider.Data.IsActive {
			writeError(w, http.StatusServiceUnavailable,
				"provider not available for model "+modelName, "server_error")
			return
		}
		if provider.Data.Name != providerName {
			writeError(w, http.StatusBadRequest,
				"provider name \""+providerName+"\" mismatch for model "+modelName+"\"provider.name"+provider.Data.Name+"\"", "invalid_request")
			return
		}

		// 1. 从合约查询模型（必须存在且活跃）
		model := findGatewayModelByName(provider.ID, modelName)
		if model == nil {
			writeError(w, http.StatusNotFound, "model not found: "+modelName, "not_found")
			return
		}

		// 3. 从合约构建 endpoints（URI + APIKeys）
		endpoints := buildEndpointsFromModel(&model.Data)
		if len(endpoints) == 0 {
			writeError(w, http.StatusServiceUnavailable,
				"no available endpoints for model "+modelName, "server_error")
			return
		}

		// 4. 按用户 hash 选 endpoint（sticky）
		token := GetTokenFromContext(r)
		userKey := token.Owner
		if userKey == "" {
			userKey = r.RemoteAddr
		}
		b := modelpkg.NewEndpointBalancerFromEndpoints(endpoints)
		if b == nil {
			writeError(w, http.StatusServiceUnavailable,
				"no available endpoint for model "+modelName, "server_error")
			return
		}

		endpoint := b.Pick(userKey + ":" + modelName)
		if endpoint == nil {
			writeError(w, http.StatusServiceUnavailable,
				"no available endpoint for model "+modelName, "server_error")
			return
		}

		ctx := context.WithValue(r.Context(), ctxModel, model)
		ctx = context.WithValue(ctx, ctxEndpoint, endpoint)
		ctx = context.WithValue(ctx, ctxBody, bodyBytes)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// ── "provider/model" parser ───────────────────────────────
// "openai/gpt-4" → ("openai", "gpt-4")
// "gpt-4"        → ("openai", "gpt-4")  (default provider)
func parseProviderModel(input string) (provider, model string) {
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

func GetTokenFromContext(r *http.Request) *modelpkg.GatewayTokenInfo {
	t, _ := r.Context().Value(ctxToken).(*modelpkg.GatewayTokenInfo)
	return t
}

func GetEndpointFromContext(r *http.Request) *model.GatewayEndpoint {
	e, _ := r.Context().Value(ctxEndpoint).(*model.GatewayEndpoint)
	return e
}

func GetModelFromContext(r *http.Request) *cgateway.DataWithID[cgateway.ModelInfo] {
	m, _ := r.Context().Value(ctxModel).(*cgateway.DataWithID[cgateway.ModelInfo])
	return m
}
