package gateway

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/computeflux/tee-provider/pkg/types"
	cgateway "github.com/computeflux/tee-provider/tee-chain/runtime/module/gateway"
)

type contextKey string

const (
	ctxToken     contextKey = "gateway_token"
	ctxBody      contextKey = "gateway_body"
	ctxModelName contextKey = "gateway_model_name"
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

		// 去掉 "provider/" 前缀后重写 body 里的 model 字段,
		// 使发往上游的 model 名是裸模型名(如 "MiniMax-M3"),而非 "openai/MiniMax-M3"。
		if _, bareModel := parseProviderModel(modelNameArg); bareModel != modelNameArg && bodyMap != nil {
			bodyMap["model"] = bareModel
			if newBody, err := json.Marshal(bodyMap); err == nil {
				bodyBytes = newBody
			}
		}

		// 检查用户积分
		token := GetTokenFromContext(r)
		if token == nil {
			// 防御:DistributeMiddleware 必须嵌套在 TokenAuthMiddleware 之后使用。
			// 若被单独注册(漏掉认证层)这里直接 401,而不是在下面 token.Addr 处 panic。
			writeProtocolError(w, r, http.StatusUnauthorized, "authentication required", "auth_error")
			return
		}

		// 查找模型信息（用于免费模型跳过余额检查 + 调用者白名单检查）
		var modelInfo *cgateway.ModelInfoV2
		_ = getCachedGatewayModels() // 触发缓存刷新（TTL 内为 no-op），返回值在此不需要
		gatewayModelCache.mu.RLock()
		if m, ok := gatewayModelCache.nameMap[modelNameArg]; ok {
			modelInfo = &m
		} else if _, bare := parseProviderModel(modelNameArg); bare != modelNameArg {
			if m, ok := gatewayModelCache.nameMap[bare]; ok {
				modelInfo = &m
			}
		}
		gatewayModelCache.mu.RUnlock()

		// 调用者白名单检查（meta.callers 非空时，仅允许列表中的地址）
		if modelInfo != nil && modelInfo.Meta != "" {
			var meta struct {
				Callers []string `json:"callers"`
			}
			if json.Unmarshal([]byte(modelInfo.Meta), &meta) == nil && len(meta.Callers) > 0 {
				caller := strings.TrimPrefix(strings.ToLower(token.Owner), "0x")
				allowed := false
				for _, c := range meta.Callers {
					if strings.TrimPrefix(strings.ToLower(c), "0x") == caller {
						allowed = true
						break
					}
				}
				if !allowed {
					GATEWAYLOG().Printf("[AUTH] caller %s not in allowed list for model %s", token.Owner, modelNameArg)
					writeProtocolError(w, r, http.StatusForbidden, "caller not authorized for this model", "forbidden")
					return
				}
			}
		}

		// 免费模型（独立定价且所有价格均为 0）跳过余额检查。
		// 加入路由的模型（RouteID>0）结算时按路由价格计费（见 usage.go），
		// 不能仅凭模型级价格判定免费，故此处要求 RouteID==0。
		isFree := modelInfo != nil && modelInfo.RouteID == 0 &&
			modelInfo.InPrice == 0 && modelInfo.OutPrice == 0 && modelInfo.CachedPrice == 0
		if !isFree {
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
		}

		ctx := context.WithValue(r.Context(), ctxBody, bodyBytes)
		ctx = context.WithValue(ctx, ctxModelName, modelNameArg)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// ── "provider/model" parser ───────────────────────────────
// "openai/gpt-4" → ("openai", "gpt-4")
// "gpt-4"        → ("", "gpt-4")  // 裸模型名，需由调用方拒绝
func parseProviderModel(input string) (provider, model string) {
	parts := strings.SplitN(input, "/", 2)
	if len(parts) == 2 {
		return parts[0], parts[1]
	}
	return "", input
}

// isRouteName 判断请求名是否是 route 前缀（Edge/MiningPool/TEE，大小写不敏感）。
func isRouteName(input string) bool {
	lower := strings.ToLower(input)
	return strings.HasPrefix(lower, "edge/") ||
		strings.HasPrefix(lower, "miningpool/") ||
		strings.HasPrefix(lower, "tee/")
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

// ── Context accessors ────────────────────────────────────
func GetModelNameFromContext(r *http.Request) string {
	t, _ := r.Context().Value(ctxModelName).(string)
	return t
}
