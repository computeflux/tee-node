package gateway

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/computeflux/tee-provider/pkg/types"
	teechain "github.com/computeflux/tee-provider/tee-chain"
	contracts "github.com/computeflux/tee-provider/tee-chain/runtime"
	cgateway "github.com/computeflux/tee-provider/tee-chain/runtime/module/gateway"
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
			writeProtocolError(w, r, http.StatusUnauthorized, "missing api key", "auth_error")
			return
		}

		token := findTokenCachedFn(key)
		if token == nil {
			GATEWAYLOG().Printf("[AUTH] token not found for key prefix: %s...", key[:min(16, len(key))])
			writeProtocolError(w, r, http.StatusUnauthorized, "invalid api key", "auth_error")
			return
		}

		if !token.IsActive {
			GATEWAYLOG().Printf("[AUTH] token inactive for key prefix: %s...", key[:min(16, len(key))])
			writeProtocolError(w, r, http.StatusUnauthorized, "invalid api key", "auth_error")
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
			writeProtocolError(w, r, http.StatusBadRequest, "failed to read request body", "invalid_request")
			return
		}

		// Parse model from JSON body first, fall back to query string.
		var bodyMap map[string]any
		modelNameArg := r.URL.Query().Get("model")
		if err := json.Unmarshal(bodyBytes, &bodyMap); err == nil {
			if v := bodyMap["model"]; v != nil {
				if s, ok := v.(string); ok {
					modelNameArg = s
				} else {
					writeProtocolError(w, r, http.StatusBadRequest, "model must be a string", "invalid_request")
					return
				}
			}
		}
		if modelNameArg == "" {
			writeProtocolError(w, r, http.StatusNotFound, "model is null", "not_found")
			return
		}

		// Parse "provider/model" format
		providerName, modelName := parseProviderModel(modelNameArg)

		var model *cgateway.ModelInfoWithID
		var endpoints []*types.GatewayEndpoint

		if providerName == "auto" {
			// 路由模式：auto/路由名 → 从路由内自动选择模型
			endpoints, model = resolveAutoRoute(modelName)
			if model == nil || len(endpoints) == 0 {
				writeProtocolError(w, r, http.StatusServiceUnavailable,
					"route not available: "+modelName, "server_error")
				return
			}
		} else {
			if modelNameArg != modelName {
				bodyMap["model"] = modelName
				if newBody, err := json.Marshal(bodyMap); err == nil {
					bodyBytes = newBody
				}
			}

			// 查询 Provider
			provider := findProviderNameFn(providerName)
			if provider == nil || !provider.Data.IsActive {
				writeProtocolError(w, r, http.StatusServiceUnavailable,
					"provider not available for model "+modelName, "server_error")
				return
			}

			// 查询模型
			model = findGatewayModelByNameFn(provider.ID, modelName)
			if model == nil {
				writeProtocolError(w, r, http.StatusNotFound, "model not found: "+modelName, "not_found")
				return
			}

			// 构建 endpoints：若模型属于路由则聚合路由内所有模型
			if model.Data.RouteID > 0 {
				endpoints = buildEndpointsFromRoute(model.Data.RouteID)
			} else {
				endpoints = buildEndpointsFromModel(&model.Data)
			}
			if len(endpoints) == 0 {
				writeProtocolError(w, r, http.StatusServiceUnavailable,
					"no available endpoints for model "+modelName, "server_error")
				return
			}
		}

		// 检查用户积分
		token := GetTokenFromContext(r)
		if bridge := getBridgeFn(); bridge != nil {
			credit, err := gatewayGetCreditFn(bridge, token.Addr)
			if err != nil {
				GATEWAYLOG().Printf("[AUTH] get credit error for owner=%s: %v", token.Owner, err)
			} else if credit <= 0 {
				GATEWAYLOG().Printf("[AUTH] insufficient credit for owner=%s, credit=%d", token.Owner, credit)
				writeProtocolError(w, r, http.StatusPaymentRequired, "insufficient credit", "insufficient_credit")
				return
			}
		}

		// 按用户 hash 选 endpoint（sticky）
		userKey := token.Owner
		if userKey == "" {
			userKey = r.RemoteAddr
		}

		b := types.NewEndpointBalancerFromEndpoints(endpoints)
		if b == nil {
			writeProtocolError(w, r, http.StatusServiceUnavailable,
				"no available endpoint for model "+modelName, "server_error")
			return
		}
		endpoint := b.Pick(userKey + ":" + modelName)
		if endpoint == nil {
			writeProtocolError(w, r, http.StatusServiceUnavailable,
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

// resolveAutoRoute resolves "auto/routeName" → endpoints from all models in the route.
func resolveAutoRoute(routeName string) ([]*types.GatewayEndpoint, *cgateway.ModelInfoWithID) {
	bridge := getBridge()
	if bridge == nil {
		return nil, nil
	}

	rw, err := teechain.WithRuntime(bridge, func(runtime *contracts.Runtime) (*cgateway.ModelRouteWithID, error) {
		return cgateway.GatewayQuery{Gateway: *cgateway.NewGateway(runtime)}.GetRouteByName(routeName)
	})
	if err != nil || rw == nil || !rw.Data.IsActive {
		return nil, nil
	}

	endpoints := buildEndpointsFromRoute(rw.ID)
	if len(endpoints) == 0 {
		return nil, nil
	}

	// 用路由内第一个模型作为 ctxModel（供 relay handler 使用）
	var modelWithID *cgateway.ModelInfoWithID
	if len(rw.Data.ModelIds) > 0 {
		m, err := teechain.WithRuntime(bridge, func(runtime *contracts.Runtime) (*cgateway.ModelInfo, error) {
			return cgateway.GatewaySecret{Gateway: *cgateway.NewGateway(runtime)}.GetModel(rw.Data.ModelIds[0])
		})
		if err == nil && m != nil {
			modelWithID = &cgateway.ModelInfoWithID{ID: rw.Data.ModelIds[0], Data: *m}
		}
	}

	return endpoints, modelWithID
}

// GetBodyFromContext returns cached body bytes set by DistributeMiddleware.
func GetBodyFromContext(r *http.Request) []byte {
	b, _ := r.Context().Value(ctxBody).([]byte)
	return b
}

// ── Context accessors ────────────────────────────────────
func GetTokenFromContext(r *http.Request) *types.GatewayTokenInfo {
	t, _ := r.Context().Value(ctxToken).(*types.GatewayTokenInfo)
	return t
}

func GetEndpointFromContext(r *http.Request) *types.GatewayEndpoint {
	e, _ := r.Context().Value(ctxEndpoint).(*types.GatewayEndpoint)
	return e
}

func GetModelFromContext(r *http.Request) *cgateway.ModelInfoWithID {
	m, _ := r.Context().Value(ctxModel).(*cgateway.ModelInfoWithID)
	return m
}
