package gateway

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/computeflux/tee-provider/pkg/types"
	"github.com/computeflux/tee-provider/pkg/types/chaintypes"
	teechain "github.com/computeflux/tee-provider/tee-chain"
	cgateway "github.com/computeflux/tee-provider/tee-chain/runtime/module/gateway"
	"github.com/stretchr/testify/assert"
)

// ── helpers ──────────────────────────────────────────────────

// seedModelCache 直接写入 gatewayModelCache（绕过合约拉取），TTL 设为当前时间避免刷新。
// 必须同时设置 models 为非 nil，否则 getCachedGatewayModels 的 TTL 检查（models != nil）
// 不通过会触发重新拉取并覆盖 nameMap。
func seedModelCache(models map[string]cgateway.ModelInfoV2) {
	gatewayModelCache.mu.Lock()
	gatewayModelCache.nameMap = models
	gatewayModelCache.models = make([]cgateway.ModelInfoV2, 0, len(models))
	for _, m := range models {
		gatewayModelCache.models = append(gatewayModelCache.models, m)
	}
	gatewayModelCache.lastRefresh = time.Now()
	gatewayModelCache.mu.Unlock()
}

// makeDistributeReq 构造带 token context 的 DistributeMiddleware 请求。
func makeDistributeReq(t *testing.T, model string, token *types.GatewayTokenInfo) *http.Request {
	t.Helper()
	body, _ := json.Marshal(map[string]any{"model": model, "messages": []any{map[string]any{"role": "user", "content": "hi"}}})
	r := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	ctx := context.WithValue(r.Context(), ctxToken, token)
	return r.WithContext(ctx)
}

// stubCredit 临时覆盖 getBridgeFn / gatewayGetCreditFn。
// bridgeNil=true 时 getBridgeFn 返回 nil（跳过余额检查）。
func stubCredit(t *testing.T, bridgeNil bool, credit int64) {
	t.Helper()
	origBridge := getBridgeFn
	origCredit := gatewayGetCreditFn
	if bridgeNil {
		getBridgeFn = func() *teechain.TeeChain { return nil }
	} else {
		getBridgeFn = func() *teechain.TeeChain { return &teechain.TeeChain{} }
		gatewayGetCreditFn = func(_ *teechain.TeeChain, _ chaintypes.UniAddr) (int64, error) {
			return credit, nil
		}
	}
	t.Cleanup(func() {
		getBridgeFn = origBridge
		gatewayGetCreditFn = origCredit
	})
}

// ── Caller whitelist ─────────────────────────────────────────

func TestDistributeMiddleware_CallerWhitelist_Allowed(t *testing.T) {
	owner := "0xAABBCCDD00112233445566778899AABBCCDDEEFF"
	seedModelCache(map[string]cgateway.ModelInfoV2{
		"whitelist-model": {
			Name:    "whitelist-model",
			Meta:    `{"callers":["` + owner + `"]}`,
			RouteID: 1,
		},
	})
	stubCredit(t, true, 0) // bridge=nil → 跳过余额检查

	token := &types.GatewayTokenInfo{Owner: owner, Key: "test-key"}
	r := makeDistributeReq(t, "whitelist-model", token)

	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
	w := httptest.NewRecorder()
	DistributeMiddleware(next).ServeHTTP(w, r)

	assert.Equal(t, http.StatusOK, w.Code, "白名单内的 owner 应放行")
}

func TestDistributeMiddleware_CallerWhitelist_Denied(t *testing.T) {
	owner := "0xAABBCCDD00112233445566778899AABBCCDDEEFF"
	other := "0x1111111111111111111111111111111111111111"
	seedModelCache(map[string]cgateway.ModelInfoV2{
		"whitelist-model": {
			Name:    "whitelist-model",
			Meta:    `{"callers":["` + other + `"]}`,
			RouteID: 1,
		},
	})
	stubCredit(t, true, 0)

	token := &types.GatewayTokenInfo{Owner: owner, Key: "test-key"}
	r := makeDistributeReq(t, "whitelist-model", token)

	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
	w := httptest.NewRecorder()
	DistributeMiddleware(next).ServeHTTP(w, r)

	assert.Equal(t, http.StatusForbidden, w.Code, "白名单外的 owner 应 403")
}

func TestDistributeMiddleware_CallerWhitelist_CaseInsensitive(t *testing.T) {
	owner := "0xaabbccdd00112233445566778899aabbccddeeFF"
	seedModelCache(map[string]cgateway.ModelInfoV2{
		"case-model": {
			Name:    "case-model",
			Meta:    `{"callers":["0xAABBCCDD00112233445566778899AABBCCDDEEFF"]}`,
			RouteID: 1,
		},
	})
	stubCredit(t, true, 0)

	token := &types.GatewayTokenInfo{Owner: owner, Key: "test-key"}
	r := makeDistributeReq(t, "case-model", token)

	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
	w := httptest.NewRecorder()
	DistributeMiddleware(next).ServeHTTP(w, r)

	assert.Equal(t, http.StatusOK, w.Code, "白名单比对应忽略大小写")
}

func TestDistributeMiddleware_NoWhitelist_PassThrough(t *testing.T) {
	owner := "0xAABBCCDD00112233445566778899AABBCCDDEEFF"
	seedModelCache(map[string]cgateway.ModelInfoV2{
		"open-model": {
			Name:    "open-model",
			Meta:    `{"foo":"bar"}`, // 无 callers 字段
			RouteID: 1,
		},
	})
	stubCredit(t, true, 0)

	token := &types.GatewayTokenInfo{Owner: owner, Key: "test-key"}
	r := makeDistributeReq(t, "open-model", token)

	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
	w := httptest.NewRecorder()
	DistributeMiddleware(next).ServeHTTP(w, r)

	assert.Equal(t, http.StatusOK, w.Code, "无 callers 字段时不应限制")
}

// ── Free model credit bypass ─────────────────────────────────

func TestDistributeMiddleware_FreeModel_SkipsCredit(t *testing.T) {
	owner := "0xAABBCCDD00112233445566778899AABBCCDDEEFF"
	seedModelCache(map[string]cgateway.ModelInfoV2{
		"free-model": {
			Name:        "free-model",
			RouteID:     0,
			InPrice:     0,
			OutPrice:    0,
			CachedPrice: 0,
		},
	})
	// bridge 非 nil + credit=0：若未跳过余额检查会 402
	stubCredit(t, false, 0)

	token := &types.GatewayTokenInfo{Owner: owner, Key: "test-key"}
	r := makeDistributeReq(t, "free-model", token)

	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
	w := httptest.NewRecorder()
	DistributeMiddleware(next).ServeHTTP(w, r)

	assert.Equal(t, http.StatusOK, w.Code, "免费模型（RouteID=0, 价格全 0）应跳过余额检查")
}

func TestDistributeMiddleware_RoutedModel_ZeroPrice_RequiresCredit(t *testing.T) {
	owner := "0xAABBCCDD00112233445566778899AABBCCDDEEFF"
	seedModelCache(map[string]cgateway.ModelInfoV2{
		"routed-free": {
			Name:        "routed-free",
			RouteID:     1, // 加入路由 → 结算按路由价格
			InPrice:     0,
			OutPrice:    0,
			CachedPrice: 0,
		},
	})
	stubCredit(t, false, 0)

	token := &types.GatewayTokenInfo{Owner: owner, Key: "test-key"}
	r := makeDistributeReq(t, "routed-free", token)

	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
	w := httptest.NewRecorder()
	DistributeMiddleware(next).ServeHTTP(w, r)

	assert.Equal(t, http.StatusPaymentRequired, w.Code,
		"加入路由的模型（RouteID>0）即使模型级价格为 0 也不应判免费")
}

func TestDistributeMiddleware_PaidModel_RequiresCredit(t *testing.T) {
	owner := "0xAABBCCDD00112233445566778899AABBCCDDEEFF"
	seedModelCache(map[string]cgateway.ModelInfoV2{
		"paid-model": {
			Name:     "paid-model",
			RouteID:  0,
			InPrice:  1000,
			OutPrice: 2000,
		},
	})
	stubCredit(t, false, 0)

	token := &types.GatewayTokenInfo{Owner: owner, Key: "test-key"}
	r := makeDistributeReq(t, "paid-model", token)

	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
	w := httptest.NewRecorder()
	DistributeMiddleware(next).ServeHTTP(w, r)

	assert.Equal(t, http.StatusPaymentRequired, w.Code, "付费模型 + 零余额应 402")
}

func TestDistributeMiddleware_PaidModel_WithCredit_Passes(t *testing.T) {
	owner := "0xAABBCCDD00112233445566778899AABBCCDDEEFF"
	seedModelCache(map[string]cgateway.ModelInfoV2{
		"paid-model": {
			Name:     "paid-model",
			RouteID:  0,
			InPrice:  1000,
			OutPrice: 2000,
		},
	})
	stubCredit(t, false, 100)

	token := &types.GatewayTokenInfo{Owner: owner, Key: "test-key"}
	r := makeDistributeReq(t, "paid-model", token)

	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
	w := httptest.NewRecorder()
	DistributeMiddleware(next).ServeHTTP(w, r)

	assert.Equal(t, http.StatusOK, w.Code, "付费模型 + 有余额应放行")
}
