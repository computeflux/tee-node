package gateway

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/computeflux/tee-provider/pkg/services/scheduler"
	"github.com/computeflux/tee-provider/pkg/types"
	"github.com/computeflux/tee-provider/pkg/types/chaintypes"
	teechain "github.com/computeflux/tee-provider/tee-chain"
	cgateway "github.com/computeflux/tee-provider/tee-chain/runtime/module/gateway"
	cstore "github.com/computeflux/tee-provider/tee-chain/runtime/store"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ── helpers ────────────────────────────────────────────────

func setupMockFns(t *testing.T) func() {
	// Save all mockable function pointers and restore on cleanup
	old := map[string]any{
		"findTokenCachedFn":        findTokenCachedFn,
		"getBridgeFn":              getBridgeFn,
		"gatewayGetCreditFn":       gatewayGetCreditFn,
		"getCachedGatewayModelsFn": getCachedGatewayModelsFn,
		"recordUsageAsyncFn":       recordUsageAsyncFn,
	}

	// Reset global state
	providerMetrics = make(map[uint64]*providerMetric)

	return func() {
		findTokenCachedFn = old["findTokenCachedFn"].(func(string) *types.GatewayTokenInfo)
		getBridgeFn = old["getBridgeFn"].(func() *teechain.TeeChain)
		gatewayGetCreditFn = old["gatewayGetCreditFn"].(func(*teechain.TeeChain, chaintypes.UniAddr) (int64, error))
		getCachedGatewayModelsFn = old["getCachedGatewayModelsFn"].(func() []cgateway.ModelInfo)
		recordUsageAsyncFn = old["recordUsageAsyncFn"].(func(chaintypes.UniAddr, uint64, *cgateway.ModelInfoWithID, uint64, uint64, uint64))
		scheduler.ResetGlobal()
	}
}

func makeTestToken(active bool) *types.GatewayTokenInfo {
	return &types.GatewayTokenInfo{
		ID:       1,
		Owner:    "test-owner",
		Addr:     cstore.UniAddr{T: 0, V: []byte{0xab, 0xcd}},
		Key:      "test-key",
		IsActive: active,
	}
}

func requestWithToken(token *types.GatewayTokenInfo, body string) *http.Request {
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(body))
	if token != nil {
		req.Header.Set("Authorization", "Bearer "+token.Key)
		req = req.WithContext(context.WithValue(req.Context(), ctxToken, token))
	}
	return req
}

// ── fake scheduler plumbing ────────────────────────────────

// fakeAdaptor 实现 scheduler.ProviderClient，回放预置的上游响应，
// 并记录收到的 body / 是否流式，供断言。
type fakeAdaptor struct {
	respBody   string
	respStatus int
	gotBody    []byte
	gotPath    string
	gotStream  bool
}

func (f *fakeAdaptor) Do(ctx context.Context, ep *scheduler.Endpoint, body []byte) ([]byte, error) {
	sendBody, upstreamPath, err := scheduler.ResolveRequest(ctx, ep, body)
	if err != nil {
		return nil, &scheduler.UpstreamError{Status: http.StatusBadRequest, Message: err.Error()}
	}
	f.gotBody = sendBody
	f.gotPath = upstreamPath
	f.gotStream = false
	if f.respStatus >= 400 {
		return nil, &scheduler.UpstreamError{Status: f.respStatus, Message: f.respBody}
	}
	return []byte(f.respBody), nil
}

func (f *fakeAdaptor) Stream(ctx context.Context, ep *scheduler.Endpoint, body []byte) (scheduler.Stream, error) {
	sendBody, upstreamPath, err := scheduler.ResolveRequest(ctx, ep, body)
	if err != nil {
		return nil, &scheduler.UpstreamError{Status: http.StatusBadRequest, Message: err.Error()}
	}
	f.gotBody = sendBody
	f.gotPath = upstreamPath
	f.gotStream = true
	if f.respStatus >= 400 {
		return nil, &scheduler.UpstreamError{Status: f.respStatus, Message: f.respBody}
	}
	return &fakeStream{data: []byte(f.respBody)}, nil
}

// fakeStream 把预置响应体作为单个 chunk 返回，然后结束。
type fakeStream struct {
	data []byte
	sent bool
}

func (s *fakeStream) Next(ctx context.Context) (*scheduler.StreamChunk, error) {
	if !s.sent {
		s.sent = true
		return &scheduler.StreamChunk{Data: s.data, Index: 0}, nil
	}
	return &scheduler.StreamChunk{Index: 1, IsFinal: true}, nil
}

func (s *fakeStream) Close() error { return nil }

// installScheduler 构造一个挂着 fake provider client 的全局 scheduler，并注册一条
// 指向该 provider 的路由。返回后会在测试结束时 ResetGlobal。
func installScheduler(t *testing.T, providerName, modelKey, upstreamModel string, providerType uint16, fake scheduler.ProviderClient) {
	t.Helper()

	reg := scheduler.NewRegistry()
	reg.Replace([]*scheduler.ModelRoute{
		{
			Model: scheduler.ModelKey(modelKey),
			Endpoints: []*scheduler.Endpoint{
				{
					Provider:           scheduler.ProviderID(providerName),
					Protocol:           scheduler.ProviderID(providerName),
					KeyID:              "k1",
					KeyValue:           "sk-test",
					BaseURL:            "https://upstream.test",
					Weight:             1,
					Enabled:            true,
					Models:             []scheduler.ModelKey{scheduler.ModelKey(modelKey), "*"},
					ModelID:            1,
					ContractProviderID: 1,
					ModelName:          upstreamModel,
					ProviderType:       providerType,
				},
			},
		},
	})

	gw, err := scheduler.NewGatewayScheduler(reg)
	require.NoError(t, err)

	gw.RegisterProviders(map[string]scheduler.ProviderClient{providerName: fake})
	scheduler.SetRequestTransform(schedulerRequestTransform)
	gw.Start(context.Background())
	scheduler.SetGlobal(gw)
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
	findTokenCachedFn = func(key string) *types.GatewayTokenInfo {
		if key == token.Key {
			return token
		}
		return nil
	}

	var ctxToken *types.GatewayTokenInfo
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

	findTokenCachedFn = func(key string) *types.GatewayTokenInfo { return nil }

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
	findTokenCachedFn = func(key string) *types.GatewayTokenInfo {
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
// 端点/模型解析已下沉到 scheduler；中间件只负责:读 body、解析 model 名、查积分。

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

func TestDistributeMiddleware_PassesThrough(t *testing.T) {
	cleanup := setupMockFns(t)
	defer cleanup()

	getBridgeFn = func() *teechain.TeeChain { return nil } // skip credit check

	var gotModel string
	var gotBody []byte
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotModel = GetModelNameFromContext(r)
		gotBody = GetBodyFromContext(r)
		w.WriteHeader(http.StatusOK)
	})

	mw := DistributeMiddleware(next)
	w := httptest.NewRecorder()
	req := requestWithToken(makeTestToken(true), `{"model":"openai/gpt-4"}`)
	mw.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "openai/gpt-4", gotModel)
	require.NotNil(t, gotBody)
	// body 里的 model 前缀被剥掉,发往上游的是裸模型名。
	assert.Contains(t, string(gotBody), `"model":"gpt-4"`)
	assert.NotContains(t, string(gotBody), `openai/gpt-4`)
}

func TestDistributeMiddleware_QueryStringModel(t *testing.T) {
	cleanup := setupMockFns(t)
	defer cleanup()

	getBridgeFn = func() *teechain.TeeChain { return nil }

	var gotModel string
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotModel = GetModelNameFromContext(r)
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
	assert.Equal(t, "openai/gpt-4", gotModel)
}

func TestDistributeMiddleware_InsufficientCredit(t *testing.T) {
	cleanup := setupMockFns(t)
	defer cleanup()

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

	var body map[string]any
	json.Unmarshal(w.Body.Bytes(), &body)
	data := body["data"].([]any)
	assert.Len(t, data, 2)
	assert.Equal(t, "gpt-4", data[0].(map[string]any)["id"])
	assert.Equal(t, "claude-3", data[1].(map[string]any)["id"])
}

func TestHandleListModels_Empty(t *testing.T) {
	cleanup := setupMockFns(t)
	defer cleanup()

	getCachedGatewayModelsFn = func() []cgateway.ModelInfo { return nil }

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/models", nil)
	HandleListModels(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var body map[string]any
	json.Unmarshal(w.Body.Bytes(), &body)
	data := body["data"].([]any)
	assert.Len(t, data, 0)
}

// ── HandleRelay ────────────────────────────────────────────

func TestHandleRelay_SchedulerNotReady(t *testing.T) {
	cleanup := setupMockFns(t)
	defer cleanup()

	scheduler.ResetGlobal() // ensure no global

	w := httptest.NewRecorder()
	req := requestWithToken(makeTestToken(true), `{"model":"gpt-4"}`)
	req = req.WithContext(context.WithValue(req.Context(), ctxBody, []byte(`{"model":"gpt-4"}`)))
	req = req.WithContext(context.WithValue(req.Context(), ctxModelName, "openai/gpt-4"))
	HandleRelay(w, req)

	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
	assert.Contains(t, w.Body.String(), "scheduler not ready")
}

func TestHandleRelay_ModelNotConfigured(t *testing.T) {
	cleanup := setupMockFns(t)
	defer cleanup()

	installScheduler(t, "openai", "gpt-4", "gpt-4", cgateway.ProviderOpenAI, &fakeAdaptor{respBody: `{}`})

	w := httptest.NewRecorder()
	body := `{"model":"openai/unknown-model"}`
	req := requestWithToken(makeTestToken(true), body)
	req = req.WithContext(context.WithValue(req.Context(), ctxBody, []byte(body)))
	req = req.WithContext(context.WithValue(req.Context(), ctxModelName, "openai/unknown-model"))
	HandleRelay(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestHandleRelay_Success(t *testing.T) {
	cleanup := setupMockFns(t)
	defer cleanup()

	var recorded bool
	recordUsageAsyncFn = func(user chaintypes.UniAddr, tokenID uint64, model *cgateway.ModelInfoWithID, in uint64, out uint64, cached uint64) {
		recorded = true
		assert.Equal(t, uint64(10), in)
		assert.Equal(t, uint64(5), out)
	}

	fake := &fakeAdaptor{
		respBody: `{"id":"chatcmpl-1","object":"chat.completion","model":"gpt-4","choices":[{"index":0,"message":{"role":"assistant","content":"Hi"},"finish_reason":"stop"}],"usage":{"prompt_tokens":10,"completion_tokens":5,"total_tokens":15}}`,
	}
	installScheduler(t, "openai", "gpt-4", "gpt-4", cgateway.ProviderOpenAI, fake)

	w := httptest.NewRecorder()
	body := `{"model":"openai/gpt-4","messages":[{"role":"user","content":"hello"}]}`
	req := requestWithToken(makeTestToken(true), body)
	req.URL.Path = "/v1/chat/completions"
	req = req.WithContext(context.WithValue(req.Context(), ctxBody, []byte(body)))
	req = req.WithContext(context.WithValue(req.Context(), ctxModelName, "openai/gpt-4"))
	HandleRelay(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), `"chatcmpl-1"`)
	assert.True(t, recorded, "usage should be recorded")
	// 同协议透传:发往上游的 model 名应保持原样(由 reqBody 决定)
	assert.Equal(t, "/v1/chat/completions", fake.gotPath)
}

func TestHandleRelay_UpstreamError(t *testing.T) {
	cleanup := setupMockFns(t)
	defer cleanup()

	fake := &fakeAdaptor{respStatus: http.StatusBadRequest, respBody: `{"error":{"message":"bad request"}}`}
	installScheduler(t, "openai", "gpt-4", "gpt-4", cgateway.ProviderOpenAI, fake)

	w := httptest.NewRecorder()
	body := `{"model":"openai/gpt-4"}`
	req := requestWithToken(makeTestToken(true), body)
	req = req.WithContext(context.WithValue(req.Context(), ctxBody, []byte(body)))
	req = req.WithContext(context.WithValue(req.Context(), ctxModelName, "openai/gpt-4"))
	HandleRelay(w, req)

	// 上游 4xx 被 scheduler 归类为 client error,映射回对应状态码。
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

var _ = time.Now
