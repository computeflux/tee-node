package graph

import (
	"context"
	"net/http"
	"strings"

	"github.com/99designs/gqlgen/graphql"
	"github.com/computeflux/tee-provider/pkg/api/gateway"
	"github.com/computeflux/tee-provider/pkg/model"
	"github.com/vektah/gqlparser/v2/gqlerror"
)

var (
	gatewayTokenCtxKey = &contextKey{"gateway_token"}
)

type contextKey struct {
	name string
}

// AuthCheck checks the API key is valid via context.
func AuthCheck(ctx context.Context, obj interface{}, next graphql.Resolver) (res interface{}, err error) {
	token := ctx.Value(gatewayTokenCtxKey).(*model.GatewayTokenInfo)
	if token == nil {
		err = gqlerror.Errorf("Please log in first.")
		return
	}
	if !token.IsActive {
		err = gqlerror.Errorf("API key is inactive.")
		return
	}
	return next(ctx)
}

// AuthMiddleware creates a middleware that decodes the Authorization header
// and validates the API Key (shared with REST Gateway).
func AuthMiddleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			token := initToken(r)

			// put it in context
			ctx := context.WithValue(r.Context(), gatewayTokenCtxKey, token)

			// and call the next with our new context
			r = r.WithContext(ctx)
			next.ServeHTTP(w, r)
		})
	}
}

// initToken decodes the Authorization header and validates the API Key.
// Supports: Bearer <apiKey>
func initToken(r *http.Request) *model.GatewayTokenInfo {
	header := r.Header
	token, ok := header["Authorization"]
	if !ok || len(token) == 0 {
		return nil
	}

	raw := token[0]

	// Try Bearer API Key
	if strings.HasPrefix(raw, "Bearer ") {
		key := strings.TrimPrefix(raw, "Bearer ")
		return gateway.FindTokenByKey(key)
	}

	// Fallback: bare API Key
	return gateway.FindTokenByKey(raw)
}
