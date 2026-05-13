package gateway

import (
	"context"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/computeflux/tee-provider/pkg/model"
)

type contextKey string

const (
	ctxToken    contextKey = "gateway_token"
	ctxProvider  contextKey = "gateway_provider"
	ctxModel    contextKey = "gateway_model"
	ctxEndpoint contextKey = "gateway_endpoint"
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
		if token == nil || !token.IsActive {
			writeError(w, http.StatusUnauthorized, "invalid api key", "auth_error")
			return
		}
		ctx := context.WithValue(r.Context(), ctxToken, token)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// DistributeMiddleware routes based on "channel/model" format
// e.g. "openai/gpt-4" → find "openai" channel → pick endpoint by user hash → route
func DistributeMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Parse model from request body or query
		modelName := r.URL.Query().Get("model")
		if modelName == "" {
			modelName = "openai/gpt-3.5-turbo" // default
		}

		// Parse "channel/model" format
		providerName, actualModel := parseProviderModel(modelName)
		token := GetTokenFromContext(r)

		channel := findProviderCached(providerName)
		if channel == nil {
			writeError(w, http.StatusServiceUnavailable,
				"channel not found: "+providerName, "server_error")
			return
		}

		// Pick endpoint with consistent hash (sticky by user + channel)
		userKey := token.Owner
		if userKey == "" {
			userKey = r.RemoteAddr
		}
		endpoint := pickEndpoint(channel.ID, userKey+":"+providerName)
		if endpoint == nil {
			writeError(w, http.StatusServiceUnavailable,
				"no available endpoint for "+providerName, "server_error")
			return
		}

		ctx := context.WithValue(r.Context(), ctxProvider, channel)
		ctx = context.WithValue(ctx, ctxModel, actualModel)
		ctx = context.WithValue(ctx, ctxEndpoint, endpoint)
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

// ── Cached channel lookup ────────────────────────────────

func findProviderCached(name string) *model.GatewayProviderInfo {
	for _, ch := range GetCachedChannels() {
		if strings.EqualFold(ch.Name, name) && ch.IsActive {
			return ch
		}
	}
	return nil
}

// ── Endpoint balancer cache ──────────────────────────────

var (
	balancerMu  sync.RWMutex
	balancers   = make(map[uint64]*model.EndpointBalancer)
	balancerTTL = 10 * time.Second
	lastRefresh = make(map[uint64]time.Time)
)

func pickEndpoint(providerID uint64, userKey string) *model.GatewayEndpoint {
	balancerMu.RLock()
	b, ok := balancers[providerID]
	last, _ := lastRefresh[providerID]
	balancerMu.RUnlock()

	if !ok || time.Since(last) > balancerTTL {
		b = model.NewEndpointBalancer(providerID)
		balancerMu.Lock()
		balancers[providerID] = b
		lastRefresh[providerID] = time.Now()
		balancerMu.Unlock()
	}

	if b == nil {
		return nil
	}
	return b.Pick(userKey)
}

func disableEndpoint(providerID uint64, endpointID uint64) {
	balancerMu.RLock()
	b := balancers[providerID]
	balancerMu.RUnlock()
	if b != nil {
		b.OnEndpointDisabled(endpointID)
	}
}

func releaseEndpoint(providerID uint64, endpointID uint64) {
	balancerMu.RLock()
	b := balancers[providerID]
	balancerMu.RUnlock()
	if b != nil {
		b.Release(endpointID)
	}
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
