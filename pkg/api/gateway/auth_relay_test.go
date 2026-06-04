package gateway

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/computeflux/tee-provider/pkg/chaintypes"
	relay "github.com/computeflux/tee-provider/pkg/services/gateway_relay"
	"github.com/computeflux/tee-provider/pkg/store"
	teechain "github.com/computeflux/tee-provider/tee-chain"
	cgateway "github.com/computeflux/tee-provider/tee-chain/contracts/module/gateway"
	cstore "github.com/computeflux/tee-provider/tee-chain/contracts/store"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ── helpers ────────────────────────────────────────────────

func setupMockFns(t *testing.T) func() {
	// Save all mockable function pointers and restore on cleanup
	old := map[string]interface{}{
		"findTokenCachedFn":        findTokenCachedFn,
		"findProviderNameFn":       findProviderNameFn,
		"findGatewayModelByNameFn": findGatewayModelByNameFn,
		"getBridgeFn":              getBridgeFn,
		"gatewayGetCreditFn":       gatewayGetCreditFn,
		"getCachedGatewayModelsFn": getCachedGatewayModelsFn,
		"relayToUpstreamFn":        relayToUpstreamFn,
		"recordUsageAsyncFn":       recordUsageAsyncFn,
	}

	// Reset global state
	endpointLimiters = make(map[uint64]*EndpointRateLimiter)
	providerMetrics = make(map[uint64]*providerMetric)

	return func() {
		findTokenCachedFn = old["findTokenCachedFn"].(func(string) *store.GatewayTokenInfo)
		findProviderNameFn = old["findProviderNameFn"].(func(string) *cgateway.ProviderInfoWithID)
		findGatewayModelByNameFn = old["findGatewayModelByNameFn"].(func(uint64, string) *cgateway.ModelInfoWithID)
		getBridgeFn = old["getBridgeFn"].(func() *teechain.TeeChain)
		gatewayGetCreditFn = old["gatewayGetCreditFn"].(func(*teechain.TeeChain, chaintypes.UniAddr) (int64, error))
		getCachedGatewayModelsFn = old["getCachedGatewayModelsFn"].(func() []cgateway.ModelInfo)
		relayToUpstreamFn = old["relayToUpstreamFn"].(func(http.ResponseWriter, *http.Request, *store.GatewayEndpoint, string, []byte, bool) (*relay.Usage, *relay.ErrorWithStatusCode, error))
		recordUsageAsyncFn = old["recordUsageAsyncFn"].(func(chaintypes.UniAddr, uint64, *cgateway.ModelInfoWithID, uint64, uint64, uint64))
	}
}

func makeTestToken(active bool) *store.GatewayTokenInfo {
	return &store.GatewayTokenInfo{
		ID:       1,
		Owner:    "test-owner",
		Addr:     cstore.UniAddr{T: 0, V: []byte{0xab, 0xcd}},
		Key:      "test-key",
		IsActive: active,
	}
}

func makeTestProvider(name string, active bool) *cgateway.ProviderInfoWithID {
	return &cgateway.ProviderInfoWithID{
		ID: 1,
		Data: cgateway.ProviderInfo{
			Owner:    cstore.UniAddr{T: 0, V: []byte{0x01}},
			Name:     name,
			IsActive: active,
		},
	}
}

func makeTestModel(name string, active bool, apiKeys []string) *cgateway.ModelInfoWithID {
	return &cgateway.ModelInfoWithID{
		ID: 1,
		Data: cgateway.ModelInfo{
			Name:         name,
			Owner:        cstore.UniAddr{T: 0, V: []byte{0x02}},
			ProviderID:   1,
			ProviderType: 1,
			URI:          "https://api.openai.com",
			APIKeys:      apiKeys,
			IsActive:     active,
			CreatedAt:    uint64(time.Now().Unix()),
		},
	}
}

func requestWithToken(token *store.GatewayTokenInfo, body string) *http.Request {
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(body))
	if token != nil {
		req.Header.Set("Authorization", "Bearer "+token.Key)
		req = req.WithContext(context.WithValue(req.Context(), ctxToken, token))
	}
	return req
}

func requestWithContext(token *store.GatewayTokenInfo, modelData *cgateway.ModelInfoWithID, endpoint *store.GatewayEndpoint, body string) *http.Request {
	req := requestWithToken(token, body)
	if modelData != nil {
		req = req.WithContext(context.WithValue(req.Context(), ctxModel, modelData))
	}
	if endpoint != nil {
		req = req.WithContext(context.WithValue(req.Context(), ctxEndpoint, endpoint))
	}
	return req
}

// ── TokenAuthMiddleware ────────────────────────────────────

func TestTokenAuthMiddleware_MissingKey(t *testing.T) {
	cleanup := setupMockFns(t)
	defer cleanup()

	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	mw := TokenAuthMiddleware(next)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	// No Authorization header
	mw.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
	assert.Contains(t, w.Body.String(), "missing api key")
}

func TestTokenAuthMiddleware_ValidToken(t *testing.T) {
	cleanup := setupMockFns(t)
	defer cleanup()

	token := makeTestToken(true)
	findTokenCachedFn = func(key string) *store.GatewayTokenInfo {
		if key == token.Key {
			return token
		}
		return nil
	}

	var ctxToken *store.GatewayTokenInfo
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctxToken = GetTokenFromContext(r)
		w.WriteHeader(http.StatusOK)
	})

	mw := TokenAuthMiddleware(next)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	req.Header.Set("Authorization", "Bearer "+token.Key)
	mw.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	require.NotNil(t, ctxToken)
	assert.Equal(t, token.ID, ctxToken.ID)
	assert.Equal(t, token.Owner, ctxToken.Owner)
}

func TestTokenAuthMiddleware_InvalidToken(t *testing.T) {
	cleanup := setupMockFns(t)
	defer cleanup()

	findTokenCachedFn = func(key string) *store.GatewayTokenInfo { return nil }

	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	mw := TokenAuthMiddleware(next)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	req.Header.Set("Authorization", "Bearer bad-key")
	mw.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
	assert.Contains(t, w.Body.String(), "invalid api key")
}

func TestTokenAuthMiddleware_InactiveToken(t *testing.T) {
	cleanup := setupMockFns(t)
	defer cleanup()

	token := makeTestToken(false) // inactive
	findTokenCachedFn = func(key string) *store.GatewayTokenInfo {
		return token
	}

	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	mw := TokenAuthMiddleware(next)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	req.Header.Set("Authorization", "Bearer "+token.Key)
	mw.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
	assert.Contains(t, w.Body.String(), "invalid api key")
}

// ── DistributeMiddleware ───────────────────────────────────

func TestDistributeMiddleware_MissingModel(t *testing.T) {
	cleanup := setupMockFns(t)
	defer cleanup()

	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	mw := DistributeMiddleware(next)
	w := httptest.NewRecorder()
	req := requestWithToken(makeTestToken(true), `{}`)
	mw.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
	assert.Contains(t, w.Body.String(), "model is null")
}

func TestDistributeMiddleware_ValidRoute(t *testing.T) {
	cleanup := setupMockFns(t)
	defer cleanup()

	provider := makeTestProvider("openai", true)
	modelData := makeTestModel("gpt-4", true, []string{"sk-test"})

	findProviderNameFn = func(name string) *cgateway.ProviderInfoWithID { return provider }
	findGatewayModelByNameFn = func(pid uint64, name string) *cgateway.ModelInfoWithID { return modelData }
	getBridgeFn = func() *teechain.TeeChain { return nil } // skip credit check
	gatewayGetCreditFn = func(*teechain.TeeChain, chaintypes.UniAddr) (int64, error) { return 100, nil }

	var ctxModel *cgateway.ModelInfoWithID
	var ctxEndpoint *store.GatewayEndpoint
	var ctxBody []byte
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctxModel = GetModelFromContext(r)
		ctxEndpoint = GetEndpointFromContext(r)
		ctxBody = GetBodyFromContext(r)
		w.WriteHeader(http.StatusOK)
	})

	mw := DistributeMiddleware(next)
	w := httptest.NewRecorder()
	req := requestWithToken(makeTestToken(true), `{"model":"openai/gpt-4"}`)
	mw.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	require.NotNil(t, ctxModel)
	assert.Equal(t, "gpt-4", ctxModel.Data.Name)
	require.NotNil(t, ctxEndpoint)
	assert.Equal(t, "https://api.openai.com", ctxEndpoint.URL)
	assert.Equal(t, "sk-test", ctxEndpoint.APIKey)
	require.NotNil(t, ctxBody)
	assert.Contains(t, string(ctxBody), `"model":"gpt-4"`)
}

func TestDistributeMiddleware_ProviderNotFound(t *testing.T) {
	cleanup := setupMockFns(t)
	defer cleanup()

	findProviderNameFn = func(name string) *cgateway.ProviderInfoWithID { return nil }

	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	mw := DistributeMiddleware(next)
	w := httptest.NewRecorder()
	req := requestWithToken(makeTestToken(true), `{"model":"openai/gpt-4"}`)
	mw.ServeHTTP(w, req)

	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
	assert.Contains(t, w.Body.String(), "provider not available")
}

func TestDistributeMiddleware_ProviderInactive(t *testing.T) {
	cleanup := setupMockFns(t)
	defer cleanup()

	provider := makeTestProvider("openai", false) // inactive
	findProviderNameFn = func(name string) *cgateway.ProviderInfoWithID { return provider }

	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	mw := DistributeMiddleware(next)
	w := httptest.NewRecorder()
	req := requestWithToken(makeTestToken(true), `{"model":"openai/gpt-4"}`)
	mw.ServeHTTP(w, req)

	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
	assert.Contains(t, w.Body.String(), "provider not available")
}

func TestDistributeMiddleware_ProviderNameMismatch(t *testing.T) {
	cleanup := setupMockFns(t)
	defer cleanup()

	provider := makeTestProvider("wrong-name", true)
	findProviderNameFn = func(name string) *cgateway.ProviderInfoWithID { return provider }

	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	mw := DistributeMiddleware(next)
	w := httptest.NewRecorder()
	req := requestWithToken(makeTestToken(true), `{"model":"openai/gpt-4"}`)
	mw.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "provider name")
}

func TestDistributeMiddleware_ModelNotFound(t *testing.T) {
	cleanup := setupMockFns(t)
	defer cleanup()

	provider := makeTestProvider("openai", true)
	findProviderNameFn = func(name string) *cgateway.ProviderInfoWithID { return provider }
	findGatewayModelByNameFn = func(pid uint64, name string) *cgateway.ModelInfoWithID { return nil }

	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	mw := DistributeMiddleware(next)
	w := httptest.NewRecorder()
	req := requestWithToken(makeTestToken(true), `{"model":"openai/gpt-4"}`)
	mw.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
	assert.Contains(t, w.Body.String(), "model not found")
}

func TestDistributeMiddleware_NoEndpoints(t *testing.T) {
	cleanup := setupMockFns(t)
	defer cleanup()

	provider := makeTestProvider("openai", true)
	modelData := makeTestModel("gpt-4", true, []string{}) // no API keys

	findProviderNameFn = func(name string) *cgateway.ProviderInfoWithID { return provider }
	findGatewayModelByNameFn = func(pid uint64, name string) *cgateway.ModelInfoWithID { return modelData }

	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	mw := DistributeMiddleware(next)
	w := httptest.NewRecorder()
	req := requestWithToken(makeTestToken(true), `{"model":"openai/gpt-4"}`)
	mw.ServeHTTP(w, req)

	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
	assert.Contains(t, w.Body.String(), "no available endpoints")
}

func TestDistributeMiddleware_InsufficientCredit(t *testing.T) {
	cleanup := setupMockFns(t)
	defer cleanup()

	provider := makeTestProvider("openai", true)
	modelData := makeTestModel("gpt-4", true, []string{"sk-test"})

	findProviderNameFn = func(name string) *cgateway.ProviderInfoWithID { return provider }
	findGatewayModelByNameFn = func(pid uint64, name string) *cgateway.ModelInfoWithID { return modelData }
	getBridgeFn = func() *teechain.TeeChain { return &teechain.TeeChain{} } // non-nil bridge triggers credit check
	gatewayGetCreditFn = func(*teechain.TeeChain, chaintypes.UniAddr) (int64, error) { return 0, nil }

	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	mw := DistributeMiddleware(next)
	w := httptest.NewRecorder()
	req := requestWithToken(makeTestToken(true), `{"model":"openai/gpt-4"}`)
	mw.ServeHTTP(w, req)

	assert.Equal(t, http.StatusPaymentRequired, w.Code)
	assert.Contains(t, w.Body.String(), "insufficient credit")
}

func TestDistributeMiddleware_QueryStringModel(t *testing.T) {
	cleanup := setupMockFns(t)
	defer cleanup()

	provider := makeTestProvider("openai", true)
	modelData := makeTestModel("gpt-4", true, []string{"sk-test"})

	findProviderNameFn = func(name string) *cgateway.ProviderInfoWithID { return provider }
	findGatewayModelByNameFn = func(pid uint64, name string) *cgateway.ModelInfoWithID { return modelData }
	getBridgeFn = func() *teechain.TeeChain { return nil }

	var ctxModel *cgateway.ModelInfoWithID
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctxModel = GetModelFromContext(r)
		w.WriteHeader(http.StatusOK)
	})

	mw := DistributeMiddleware(next)
	w := httptest.NewRecorder()
	req := requestWithToken(makeTestToken(true), `{}`)
	q := req.URL.Query()
	q.Set("model", "openai/gpt-4")
	req.URL.RawQuery = q.Encode()
	mw.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	require.NotNil(t, ctxModel)
	assert.Equal(t, "gpt-4", ctxModel.Data.Name)
}

// ── HandleListModels ───────────────────────────────────────

func TestHandleListModels_WithModels(t *testing.T) {
	cleanup := setupMockFns(t)
	defer cleanup()

	getCachedGatewayModelsFn = func() []cgateway.ModelInfo {
		return []cgateway.ModelInfo{
			{Name: "gpt-4", Owner: cstore.UniAddr{T: 0, V: []byte{0x01}}, IsActive: true},
			{Name: "claude-3", Owner: cstore.UniAddr{T: 0, V: []byte{0x02}}, IsActive: true},
		}
	}

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/models", nil)
	HandleListModels(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var body map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &body)
	data := body["data"].([]interface{})
	assert.Len(t, data, 2)
	assert.Equal(t, "gpt-4", data[0].(map[string]interface{})["id"])
	assert.Equal(t, "claude-3", data[1].(map[string]interface{})["id"])
}

func TestHandleListModels_Empty(t *testing.T) {
	cleanup := setupMockFns(t)
	defer cleanup()

	getCachedGatewayModelsFn = func() []cgateway.ModelInfo { return nil }

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/models", nil)
	HandleListModels(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var body map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &body)
	data := body["data"].([]interface{})
	assert.Len(t, data, 0)
}

// ── HandleRelay ────────────────────────────────────────────

func TestHandleRelay_MissingEndpoint(t *testing.T) {
	cleanup := setupMockFns(t)
	defer cleanup()

	w := httptest.NewRecorder()
	req := requestWithContext(makeTestToken(true), makeTestModel("gpt-4", true, []string{"sk-test"}), nil, `{}`)
	HandleRelay(w, req)

	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
	assert.Contains(t, w.Body.String(), "no route")
}

func TestHandleRelay_MissingModel(t *testing.T) {
	cleanup := setupMockFns(t)
	defer cleanup()

	endpoint := &store.GatewayEndpoint{ID: 1, URL: "https://api.openai.com", APIKey: "sk-test"}
	w := httptest.NewRecorder()
	req := requestWithContext(makeTestToken(true), nil, endpoint, `{}`)
	HandleRelay(w, req)

	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
	assert.Contains(t, w.Body.String(), "no route")
}

func TestHandleRelay_RateLimited(t *testing.T) {
	cleanup := setupMockFns(t)
	defer cleanup()

	endpoint := &store.GatewayEndpoint{ID: 99, URL: "https://api.openai.com", APIKey: "sk-test"}
	modelData := makeTestModel("gpt-4", true, []string{"sk-test"})

	// Pre-exhaust the limiter: Allow 50 times (openai default maxConc=50) without Done
	lim := getEndpointLimiter(endpoint.ID, 0)
	for i := 0; i < 50; i++ {
		lim.Allow()
	}

	// Mock relay so we don't hit real upstream if rate limit doesn't trigger
	relayToUpstreamFn = func(w http.ResponseWriter, r *http.Request, ep *store.GatewayEndpoint, modelName string, body []byte, isStream bool) (*relay.Usage, *relay.ErrorWithStatusCode, error) {
		t.Fatal("relayToUpstream should not be called when rate limited")
		return nil, nil, nil
	}

	w := httptest.NewRecorder()
	req := requestWithContext(makeTestToken(true), modelData, endpoint, `{}`)
	HandleRelay(w, req)

	assert.Equal(t, http.StatusTooManyRequests, w.Code)
	assert.Contains(t, w.Body.String(), "rate limited")

	// Cleanup inflight
	for i := 0; i < 50; i++ {
		lim.Done()
	}
}

func TestHandleRelay_UpstreamBizError(t *testing.T) {
	cleanup := setupMockFns(t)
	defer cleanup()

	endpoint := &store.GatewayEndpoint{ID: 1, URL: "https://api.openai.com", APIKey: "sk-test"}
	modelData := makeTestModel("gpt-4", true, []string{"sk-test"})

	relayToUpstreamFn = func(w http.ResponseWriter, r *http.Request, ep *store.GatewayEndpoint, modelName string, body []byte, isStream bool) (*relay.Usage, *relay.ErrorWithStatusCode, error) {
		return nil, &relay.ErrorWithStatusCode{StatusCode: http.StatusBadRequest, Error: relay.ErrorDetail{Message: "bad request", Type: "invalid_request"}}, nil
	}

	w := httptest.NewRecorder()
	req := requestWithContext(makeTestToken(true), modelData, endpoint, `{}`)
	HandleRelay(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "bad request")
}

func TestHandleRelay_UpstreamTransportError(t *testing.T) {
	cleanup := setupMockFns(t)
	defer cleanup()

	endpoint := &store.GatewayEndpoint{ID: 1, URL: "https://api.openai.com", APIKey: "sk-test"}
	modelData := makeTestModel("gpt-4", true, []string{"sk-test"})

	relayToUpstreamFn = func(w http.ResponseWriter, r *http.Request, ep *store.GatewayEndpoint, modelName string, body []byte, isStream bool) (*relay.Usage, *relay.ErrorWithStatusCode, error) {
		return nil, nil, fmt.Errorf("connection refused")
	}

	w := httptest.NewRecorder()
	req := requestWithContext(makeTestToken(true), modelData, endpoint, `{}`)
	HandleRelay(w, req)

	assert.Equal(t, http.StatusBadGateway, w.Code)
	assert.Contains(t, w.Body.String(), "connection refused")
}

func TestHandleRelay_Success(t *testing.T) {
	cleanup := setupMockFns(t)
	defer cleanup()

	endpoint := &store.GatewayEndpoint{ID: 1, URL: "https://api.openai.com", APIKey: "sk-test"}
	modelData := makeTestModel("gpt-4", true, []string{"sk-test"})

	var recorded bool
	recordUsageAsyncFn = func(user chaintypes.UniAddr, tokenID uint64, model *cgateway.ModelInfoWithID, in uint64, out uint64, cached uint64) {
		recorded = true
	}

	relayToUpstreamFn = func(w http.ResponseWriter, r *http.Request, ep *store.GatewayEndpoint, modelName string, body []byte, isStream bool) (*relay.Usage, *relay.ErrorWithStatusCode, error) {
		return &relay.Usage{PromptTokens: 10, CompletionTokens: 5, TotalTokens: 15}, nil, nil
	}

	w := httptest.NewRecorder()
	req := requestWithContext(makeTestToken(true), modelData, endpoint, `{}`)
	HandleRelay(w, req)

	// Success: no explicit status code written by HandleRelay on success path
	// (relayToUpstream writes response directly to w)
	assert.True(t, recorded, "usage should be recorded")
}
