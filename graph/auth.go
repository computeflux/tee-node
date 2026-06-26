package graph

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/99designs/gqlgen/graphql"
	"github.com/computeflux/tee-provider/pkg/api/auth"
	"github.com/computeflux/tee-provider/pkg/api/gateway"
	"github.com/computeflux/tee-provider/pkg/types"
	teechain "github.com/computeflux/tee-provider/tee-chain"
	"github.com/vektah/gqlparser/v2/gqlerror"
)

var (
	gatewayTokenCtxKey = &contextKey{"gateway_token"}
)

type contextKey struct {
	name string
}

// GatewayTokenFromContext extracts the authenticated gateway token from context.
// Returns nil if no token was injected by AuthMiddleware.
func GatewayTokenFromContext(ctx context.Context) *types.GatewayTokenInfo {
	token, _ := ctx.Value(gatewayTokenCtxKey).(*types.GatewayTokenInfo)
	return token
}

// AuthCheck checks the API key is valid via context.
func AuthCheck(ctx context.Context, obj interface{}, next graphql.Resolver) (res interface{}, err error) {
	token := GatewayTokenFromContext(ctx)
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
func initToken(r *http.Request) *types.GatewayTokenInfo {
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

var authStoreAuthUserFn = auth.AuthStoreAuthUser

func persistAuthUserToChain(chain *teechain.TeeChain, user *types.AuthUser) {
	if chain == nil || user == nil {
		return
	}
	go func(u *types.AuthUser) {
		plain, err := json.Marshal(u)
		if err != nil {
			fmt.Printf("[AUTH] failed to marshal auth user %s: %v\n", u.Email, err)
			return
		}
		key := "auth_user:" + u.Email
		const maxRetries = 3
		var lastErr error
		for i := 0; i < maxRetries; i++ {
			if i > 0 {
				time.Sleep(time.Duration(i) * 500 * time.Millisecond)
			}
			lastErr = authStoreAuthUserFn(chain, key, plain)
			if lastErr == nil {
				return
			}
		}
		fmt.Printf("[AUTH] failed to persist auth user %s to chain after %d retries: %v\n", u.Email, maxRetries, lastErr)
	}(user)
}
