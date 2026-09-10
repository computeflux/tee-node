package c2c

import (
	"math/big"
	"testing"

	"github.com/computeflux/tee-provider/tee-chain/runtime/module/gateway"
	"github.com/computeflux/tee-provider/tee-chain/runtime/store"
	"github.com/computeflux/tee-provider/tee-chain/runtime/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestC2C() (*C2C, *testutil.MockRuntime) {
	rt := testutil.NewMockRuntime()
	return NewC2C(rt), rt
}

func addr(b byte) store.UniAddr { return store.UniAddr{T: 1, V: []byte{b}} }

// 预置：以系统账户（gov 治理）授权代码 + 注册验证节点(0x10)。
// 治理统一走系统账户（CallbySystem，由 gov.CallAsSudo 发起），不再单独治理地址。
func setupGovernance(t *testing.T) (*C2C, *testutil.MockRuntime, store.UniAddr, []byte) {
	t.Helper()
	c, rt := newTestC2C()
	m := C2CMutation{*c}
	gov := addr(0xAA) // 仅作占位返回，治理已统一走系统账户
	code := []byte("c2c-code-v1")
	rt.System = true
	require.NoError(t, m.RegisterCode(code, "1.0.0"))
	require.NoError(t, m.RegisterNode(addr(0x10), code))
	rt.System = false
	return c, rt, gov, code
}

func cmpInt(a store.Amount, i int64) int { return a.Cmp(store.NewU256(big.NewInt(i))) }

func amt(i int64) store.Amount { return store.NewU256(big.NewInt(i)) }

// ── Init / 治理 ──────────────────────────────────────────────

func TestInit(t *testing.T) {
	c, _ := newTestC2C()
	require.NoError(t, C2CMutation{*c}.Init())
}

func TestGovernanceRequiresSystem(t *testing.T) {
	c, rt, _, _ := setupGovernance(t)
	rt.System = false
	assert.Error(t, C2CMutation{*c}.RegisterCode([]byte("x"), "1.0"))
	assert.Error(t, C2CMutation{*c}.RegisterNode(addr(0x01), []byte("x")))
	assert.Error(t, C2CMutation{*c}.SetFxConfig(FxConfig{CnyPerPoint: 72000}))
}

// ── ① 代码白名单 ─────────────────────────────────────────────

func TestRegisterCode_AuthorizeAndRevoke(t *testing.T) {
	c, rt, _, code := setupGovernance(t)
	ok, err := C2CQuery{*c}.IsCodeAuthorized(code)
	require.NoError(t, err)
	require.True(t, ok)

	rt.System = true
	require.NoError(t, C2CMutation{*c}.RevokeCode(code))
	rt.System = false
	ok, err = C2CQuery{*c}.IsCodeAuthorized(code)
	require.NoError(t, err)
	assert.False(t, ok)
}

func TestRegisterCode_Duplicate(t *testing.T) {
	c, rt, _, code := setupGovernance(t)
	rt.System = true
	assert.ErrorIs(t, C2CMutation{*c}.RegisterCode(code, "1.0.1"), ErrCodeExists)
	rt.System = false
}

func TestRegisterCode_EmptyHash(t *testing.T) {
	c, rt, _, _ := setupGovernance(t)
	rt.System = true
	assert.ErrorIs(t, C2CMutation{*c}.RegisterCode(nil, "x"), ErrCodeEmpty)
	rt.System = false
}

func TestIsCodeAuthorized_Empty(t *testing.T) {
	c, _ := newTestC2C()
	ok, err := C2CQuery{*c}.IsCodeAuthorized(nil)
	require.NoError(t, err)
	assert.False(t, ok)
}

// ── ② 节点注册表 ─────────────────────────────────────────────

func TestRegisterNode_RequiresAuthorizedCode(t *testing.T) {
	c, rt := newTestC2C()
	rt.System = true
	// 系统账户调用，但未授权代码 → ErrCodeNotAuthorized。
	assert.ErrorIs(t, C2CMutation{*c}.RegisterNode(addr(0x10), []byte("unauthorized")), ErrCodeNotAuthorized)
	rt.System = false
}

func TestRegisterNode_Duplicate(t *testing.T) {
	c, rt, _, code := setupGovernance(t)
	rt.System = true
	assert.ErrorIs(t, C2CMutation{*c}.RegisterNode(addr(0x10), code), ErrNodeExists)
	rt.System = false
}

func TestRemoveNode_SelfAndGovernance(t *testing.T) {
	c, rt, _, _ := setupGovernance(t)

	// 节点自行注销。
	rt.Caller = addr(0x10)
	require.NoError(t, C2CMutation{*c}.RemoveNode(addr(0x10)))
	_, err := C2CQuery{*c}.GetNode(addr(0x10))
	require.NoError(t, err)

	// 系统账户重新注册后再移除。
	rt.System = true
	require.NoError(t, C2CMutation{*c}.RegisterNode(addr(0x10), []byte("c2c-code-v1")))
	require.NoError(t, C2CMutation{*c}.RemoveNode(addr(0x10)))
	rt.System = false
}

func TestRemoveNode_OnlyGovernanceOrSelf(t *testing.T) {
	c, rt, _, _ := setupGovernance(t)
	rt.Caller = addr(0x99) // 第三方
	assert.Error(t, C2CMutation{*c}.RemoveNode(addr(0x10)))
}

func TestListNodes(t *testing.T) {
	c, rt, _, code := setupGovernance(t)
	rt.System = true
	require.NoError(t, C2CMutation{*c}.RegisterNode(addr(0x20), code))
	rt.System = false

	nodes, err := C2CQuery{*c}.ListNodes()
	require.NoError(t, err)
	require.Len(t, nodes, 2)
}

func TestGetNode(t *testing.T) {
	c, _, _, _ := setupGovernance(t)
	n, err := C2CQuery{*c}.GetNode(addr(0x10))
	require.NoError(t, err)
	assert.True(t, n.Active)
	assert.Equal(t, []byte("c2c-code-v1"), n.CodeVersion)
}

// ── ③ 挂单托管 ───────────────────────────────────────────────

func TestDeposit_Accumulates(t *testing.T) {
	c, rt, _, _ := setupGovernance(t)
	rt.Caller = addr(0x30)
	require.NoError(t, C2CMutation{*c}.Deposit(amt(100)))
	require.NoError(t, C2CMutation{*c}.Deposit(amt(50)))
	bal, err := C2CQuery{*c}.GetMerchantEscrow(addr(0x30))
	require.NoError(t, err)
	assert.Equal(t, 0, cmpInt(bal, 150))
}

func TestDeposit_ZeroAmount(t *testing.T) {
	c, rt, _, _ := setupGovernance(t)
	rt.Caller = addr(0x30)
	assert.ErrorIs(t, C2CMutation{*c}.Deposit(amt(0)), ErrZeroAmount)
}

// ── ④ 受控放款 ───────────────────────────────────────────────

func TestRelease_FullCycle(t *testing.T) {
	c, rt, _, code := setupGovernance(t)
	merchant := addr(0x30)
	buyer := addr(0x40)

	rt.Caller = merchant
	require.NoError(t, C2CMutation{*c}.Deposit(amt(1000)))

	rt.Caller = addr(0x10) // 节点
	require.NoError(t, C2CMutation{*c}.Release(Release{
		Merchant: merchant, Buyer: buyer, Amount: amt(400),
		CodeVersion: code, OrderNo: []byte("order-1"),
		FiatCents: 40000, Currency: "USD",
	}))

	escrow, _ := C2CQuery{*c}.GetMerchantEscrow(merchant)
	bal, _ := C2CQuery{*c}.GetBuyerBalance(buyer)
	assert.Equal(t, 0, cmpInt(escrow, 600))
	// Release 不再记 c2c buyerBalance，放款直接进 gateway credits。
	assert.Equal(t, 0, cmpInt(bal, 0))

	// 放款已入 gateway credits（包内构造 GatewaySecret 调用，共享 txn）。
	gw := gateway.NewGateway(rt)
	cred, err := gateway.GatewayQuery{Gateway: *gw}.GetCredit(buyer)
	require.NoError(t, err)
	assert.Equal(t, int64(400), cred)
}

func TestRelease_NotNode(t *testing.T) {
	c, rt, _, code := setupGovernance(t)
	rt.Caller = addr(0x50) // 未注册节点
	require.NoError(t, C2CMutation{*c}.Deposit(amt(1000)))
	assert.ErrorIs(t, C2CMutation{*c}.Release(Release{
		Merchant: addr(0x50), Buyer: addr(0x40), Amount: amt(10),
		CodeVersion: code, OrderNo: []byte("o1"),
	}), ErrNotNode)
}

func TestRelease_CodeMismatch(t *testing.T) {
	c, rt, _, _ := setupGovernance(t)
	rt.Caller = addr(0x30)
	require.NoError(t, C2CMutation{*c}.Deposit(amt(1000)))
	rt.Caller = addr(0x10) // 节点
	assert.ErrorIs(t, C2CMutation{*c}.Release(Release{
		Merchant: addr(0x30), Buyer: addr(0x40), Amount: amt(10),
		CodeVersion: []byte("other"), OrderNo: []byte("o1"),
	}), ErrCodeMismatch)
}

func TestRelease_UnauthorizedCode(t *testing.T) {
	c, rt, _, _ := setupGovernance(t)
	// 系统账户用未授权代码注册节点 → 拒绝。
	rt.System = true
	assert.ErrorIs(t, C2CMutation{*c}.RegisterNode(addr(0x11), []byte("rogue")), ErrCodeNotAuthorized)
	rt.System = false
}

func TestRelease_InsufficientEscrow(t *testing.T) {
	c, rt, _, code := setupGovernance(t)
	rt.Caller = addr(0x30)
	require.NoError(t, C2CMutation{*c}.Deposit(amt(100)))
	rt.Caller = addr(0x10)
	assert.ErrorIs(t, C2CMutation{*c}.Release(Release{
		Merchant: addr(0x30), Buyer: addr(0x40), Amount: amt(1000),
		CodeVersion: code, OrderNo: []byte("o1"),
		FiatCents: 100000, Currency: "USD",
	}), ErrInsufficientEscrow)
}

func TestRelease_DuplicateOrder(t *testing.T) {
	c, rt, _, code := setupGovernance(t)
	merchant := addr(0x30)
	rt.Caller = merchant
	require.NoError(t, C2CMutation{*c}.Deposit(amt(1000)))
	rt.Caller = addr(0x10)
	r := Release{Merchant: merchant, Buyer: addr(0x40), Amount: amt(100), CodeVersion: code, OrderNo: []byte("dup"), FiatCents: 10000, Currency: "USD"}
	require.NoError(t, C2CMutation{*c}.Release(r))
	assert.ErrorIs(t, C2CMutation{*c}.Release(r), ErrDuplicateRelease)
}

func TestBuyerWithdraw_Insufficient(t *testing.T) {
	c, rt, _, _ := setupGovernance(t)
	rt.Caller = addr(0x40)
	assert.ErrorIs(t, C2CMutation{*c}.BuyerWithdraw(amt(10)), ErrInsufficientBalance)
	assert.ErrorIs(t, C2CMutation{*c}.BuyerWithdraw(amt(0)), ErrZeroAmount)
}

// ── ⑤ 经济策略 / 汇率 / 限额 / 佣金 ──────────────────────────

func mustConvert(t *testing.T, q C2CQuery, points int64, currency string) int64 {
	t.Helper()
	v, err := q.Convert(points, currency)
	require.NoError(t, err)
	return v
}

func mustFiatToPoints(t *testing.T, q C2CQuery, fiatCents int64, currency string) int64 {
	t.Helper()
	v, err := q.FiatToPoints(fiatCents, currency)
	require.NoError(t, err)
	return v
}

func TestSetFxConfig_GovernanceOnly(t *testing.T) {
	c, rt, _, _ := setupGovernance(t)
	// 非系统账户拒绝。
	rt.System = false
	rt.Caller = addr(0x10) // 节点
	assert.Error(t, C2CMutation{*c}.SetFxConfig(FxConfig{CnyPerPoint: 72000}))

	// 系统账户（gov 治理）写入。
	rt.System = true
	require.NoError(t, C2CMutation{*c}.SetFxConfig(FxConfig{
		CnyPerPoint: 72000, MaxCnyPerPoint: 500000, PlatformFeePct: 400,
		MinOrderUsdCents: 500, MaxOrderUsdCents: 100000, BuyerDailyLimitUsd: 300000,
	}))
	rt.System = false
	got, _ := C2CQuery{*c}.GetFxConfig()
	assert.Equal(t, int64(72000), got.CnyPerPoint)
	assert.Equal(t, int64(400), got.PlatformFeePct)
	assert.Equal(t, int64(300000), got.BuyerDailyLimitUsd)
}

func TestSetFxConfig_Validation(t *testing.T) {
	c, rt, _, _ := setupGovernance(t)
	rt.System = true
	assert.ErrorIs(t, C2CMutation{*c}.SetFxConfig(FxConfig{CnyPerPoint: 0}), ErrZeroRate)
	assert.ErrorIs(t, C2CMutation{*c}.SetFxConfig(FxConfig{CnyPerPoint: 72000, MaxCnyPerPoint: 100}), ErrRateExceedsCap)
	rt.System = false
}

func TestConvert_USD_CNY(t *testing.T) {
	c, rt, _, _ := setupGovernance(t)
	rt.System = true
	require.NoError(t, C2CMutation{*c}.SetFxConfig(FxConfig{CnyPerPoint: 72000}))
	rt.System = false
	q := C2CQuery{*c}
	// USD：100 积分 = 10000 分（$100）。
	assert.Equal(t, int64(10000), mustConvert(t, q, 100, "USD"))
	// CNY：100 积分 × 7.2 = 720 元 = 72000 分。
	assert.Equal(t, int64(72000), mustConvert(t, q, 100, "CNY"))

	// CNY 未配置汇率报错。
	c2, _, _, _ := setupGovernance(t)
	_, err := C2CQuery{*c2}.Convert(100, "CNY")
	assert.ErrorIs(t, err, ErrUnsupportedCurrency)
}

func TestFiatToPoints(t *testing.T) {
	c, rt, _, _ := setupGovernance(t)
	rt.System = true
	require.NoError(t, C2CMutation{*c}.SetFxConfig(FxConfig{CnyPerPoint: 72000}))
	rt.System = false
	q := C2CQuery{*c}
	assert.Equal(t, int64(100), mustFiatToPoints(t, q, 10000, "USD"))
	assert.Equal(t, int64(100), mustFiatToPoints(t, q, 72000, "CNY"))
}

func TestRelease_DailyLimitEnforced(t *testing.T) {
	c, rt, _, code := setupGovernance(t)
	merchant := addr(0x30)
	buyer := addr(0x40)
	rt.Time = 86400 * 7 // 固定某日
	rt.System = true
	require.NoError(t, C2CMutation{*c}.SetFxConfig(FxConfig{
		CnyPerPoint: 72000, BuyerDailyLimitUsd: 300000,
	}))
	rt.System = false
	rt.Caller = merchant
	require.NoError(t, C2CMutation{*c}.Deposit(amt(100000)))
	rt.Caller = addr(0x10) // 节点
	// 第一单 2000 USD：当日累计 200000 ≤ 300000 ✓。
	require.NoError(t, C2CMutation{*c}.Release(Release{
		Merchant: merchant, Buyer: buyer, Amount: amt(2000), CodeVersion: code,
		OrderNo: []byte("d1"), FiatCents: 200000, Currency: "USD",
	}))
	spent, _ := C2CQuery{*c}.BuyerSpentToday(buyer)
	assert.Equal(t, int64(200000), spent)
	// 第二单 2000 USD：累计将达 400000 > 300000 ✗。
	err := C2CMutation{*c}.Release(Release{
		Merchant: merchant, Buyer: buyer, Amount: amt(2000), CodeVersion: code,
		OrderNo: []byte("d2"), FiatCents: 200000, Currency: "USD",
	})
	assert.ErrorIs(t, err, ErrDailyLimitExceeded)
	// 跨日后重置：当日累计归零，可再下。
	rt.Time = 86400 * 8
	require.NoError(t, C2CMutation{*c}.Release(Release{
		Merchant: merchant, Buyer: buyer, Amount: amt(2000), CodeVersion: code,
		OrderNo: []byte("d3"), FiatCents: 200000, Currency: "USD",
	}))
}

func TestRelease_PerOrderLimit(t *testing.T) {
	c, rt, _, code := setupGovernance(t)
	merchant := addr(0x30)
	buyer := addr(0x40)
	rt.System = true
	require.NoError(t, C2CMutation{*c}.SetFxConfig(FxConfig{
		CnyPerPoint: 72000, MaxOrderUsdCents: 100000, MinOrderUsdCents: 500,
	}))
	rt.System = false
	rt.Caller = merchant
	require.NoError(t, C2CMutation{*c}.Deposit(amt(100000)))
	rt.Caller = addr(0x10)
	// 2000 积分 = 2000 USD > 1000 USD 上限 ✗。
	err := C2CMutation{*c}.Release(Release{
		Merchant: merchant, Buyer: buyer, Amount: amt(2000), CodeVersion: code,
		OrderNo: []byte("p1"), FiatCents: 200000, Currency: "USD",
	})
	assert.ErrorIs(t, err, ErrOrderLimitExceeded)
}

func TestRelease_Commission(t *testing.T) {
	c, rt, _, code := setupGovernance(t)
	merchant := addr(0x30)
	buyer := addr(0x40)
	rt.System = true
	require.NoError(t, C2CMutation{*c}.SetFxConfig(FxConfig{
		CnyPerPoint: 72000, PlatformFeePct: 400,
	}))
	rt.System = false
	rt.Caller = merchant
	require.NoError(t, C2CMutation{*c}.Deposit(amt(1000)))
	rt.Caller = addr(0x10)
	// 放款 1000 积分：佣金 = floor(1000 × 400 / 10000) = 40；买家得 960。
	require.NoError(t, C2CMutation{*c}.Release(Release{
		Merchant: merchant, Buyer: buyer, Amount: amt(1000), CodeVersion: code,
		OrderNo: []byte("c1"), FiatCents: 100000, Currency: "USD",
	}))
	bal, _ := C2CQuery{*c}.GetBuyerBalance(buyer)
	esc, _ := C2CQuery{*c}.GetMerchantEscrow(merchant)
	pc, _ := C2CQuery{*c}.GetPlatformCommission()
	assert.Equal(t, 0, cmpInt(bal, 0)) // 放款直接进 gateway credits
	assert.Equal(t, 0, cmpInt(esc, 0)) // escrow 全额扣 1000
	assert.Equal(t, 0, cmpInt(pc, 40))
	gw := gateway.NewGateway(rt)
	cred, err := gateway.GatewayQuery{Gateway: *gw}.GetCredit(buyer)
	require.NoError(t, err)
	assert.Equal(t, int64(960), cred) // 买家得 960
}

func TestRelease_ConversionMismatch(t *testing.T) {
	c, rt, _, code := setupGovernance(t)
	merchant := addr(0x30)
	buyer := addr(0x40)
	rt.Caller = merchant
	require.NoError(t, C2CMutation{*c}.Deposit(amt(1000)))
	rt.Caller = addr(0x10)
	// 报假 fiat（应为 100000，报 99999）。
	err := C2CMutation{*c}.Release(Release{
		Merchant: merchant, Buyer: buyer, Amount: amt(1000), CodeVersion: code,
		OrderNo: []byte("m1"), FiatCents: 99999, Currency: "USD",
	})
	assert.ErrorIs(t, err, ErrConversionMismatch)
}
