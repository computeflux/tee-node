package c2c

import (
	"crypto/ed25519"
	"crypto/rand"
	"fmt"
	"math/big"
	"testing"

	"github.com/computeflux/tee-provider/tee-chain/runtime/module/gateway"
	"github.com/computeflux/tee-provider/tee-chain/runtime/store"
	"github.com/computeflux/tee-provider/tee-chain/runtime/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// testChainID 测试用主链 ID（与 relay 一致：chainID + nonce 唯一确定一笔跨链充值）
const testChainID uint32 = 420420417

func newTestC2C() (*C2C, *testutil.MockRuntime) {
	rt := testutil.NewMockRuntime()
	return NewC2C(rt), rt
}

func addr(b byte) store.UniAddr { return store.UniAddr{T: 1, V: []byte{b}} }

// 预置：以系统账户（gov 治理）授权代码 + 注册验证节点(0x10)。
// 治理统一走系统账户（CallbySystem，由 gov.CallAsSudo 发起），不再单独治理地址。
// 同时生成“TEE 代码密钥”（ed25519）并配置 MockRuntime.VerifyTEEFunc：
// 对任意 TEE 报告返回 TEE 证明的代码三要素（CodeSigner=代码公钥、CodeSignature=代码指纹 code），
// 模拟验证节点链下 quote 验签后的返回。
func setupGovernance(t *testing.T) (*C2C, *testutil.MockRuntime, []byte, ed25519.PrivateKey) {
	t.Helper()
	c, rt := newTestC2C()
	m := C2CMutation{*c}
	code := []byte("c2c-code-v1")
	_, codePriv, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	codePub := codePriv.Public().(ed25519.PublicKey)
	rt.VerifyTEEFunc = func(_, _ []byte, _ int64, _ []byte, _ uint32) store.Result[store.TeeVerifyInfo, string] {
		return store.NewOk[store.TeeVerifyInfo, string](store.TeeVerifyInfo{
			TeeType:       1,
			CodeSigner:    codePub,
			CodeSignature: code,
		})
	}
	rt.System = true
	require.NoError(t, m.RegisterCode(code, "1.0.0"))
	require.NoError(t, m.RegisterNode(addr(0x10)))
	rt.System = false
	return c, rt, code, codePriv
}

// signRelease 用代码私钥对放款数据的 SCALE 编码签名，生成用户的 CodeSignature
// （与合约侧 verifyCodeSig 的验证算法一致：ed25519 RFC 8032；测试数据远小于 256 字节，不触发 blake2b 预哈希）。
func signRelease(t *testing.T, priv ed25519.PrivateKey, r Release) []byte {
	t.Helper()
	payload, err := store.ScaleEncode(r)
	require.NoError(t, err)
	return ed25519.Sign(priv, payload)
}

// releaseWithSig 把放款数据包装为携带 TEE 证明的放款请求。
// rt.Time 作为 TeeTime（与 MockRuntime 的区块时间一致，通过时间窗校验）。
func releaseWithSig(t *testing.T, rt *testutil.MockRuntime, priv ed25519.PrivateKey, r Release) ReleaseWithSig {
	t.Helper()
	return ReleaseWithSig{
		Data:          r,
		TeeReport:     []byte("tee-report"),
		CodeSignature: signRelease(t, priv, r),
		TeeTime:       rt.Time,
		TeeType:       0, // SGX（mock 下不影响验证，仅走完整编码路径）
	}
}

func cmpInt(a store.Amount, i int64) int { return a.Cmp(store.NewU256(big.NewInt(i))) }

func amt(i int64) store.Amount { return store.NewU256(big.NewInt(i)) }

// seedProviderCredits 通过 gateway 公开接口为 merchant 铺 provider 盈利（积分）：
// merchant 名下注册 provider + model（inPrice=1e6、outPrice=0），再以节点身份结算
// credits 个 input token，恰好使 provider credits 增加 credits（cost = tokens×1e6/1e6）。
func seedProviderCredits(t *testing.T, rt *testutil.MockRuntime, gw *gateway.Gateway, merchant store.UniAddr, credits uint64) {
	t.Helper()
	gm := gateway.GatewayMutation{Gateway: *gw}
	rt.Caller = merchant
	// 注册 provider 需真实质押：先充值质押金
	rt.System = true
	require.NoError(t, gm.Recharge(merchant, 200_000_000, testChainID, 1))
	rt.System = false
	require.NoError(t, gm.CreateProvider(fmt.Sprintf("shop-%02x", merchant.V[0]), "", "", gateway.RegionAsiaEast, 200_000_000))
	require.NoError(t, gm.RegisterModel("m", gateway.ModalityText, 128000, 1_000_000, 0, 0, gateway.OpenAI, "", nil, 0, 0, false, ""))
	// 结算必须节点（T == 0）发起。
	rt.Caller = store.UniAddr{T: 0}
	require.NoError(t, gm.SettleUsageWithRoute([]gateway.SettlementWithRoute{{
		User:       merchant,
		RouteID:    store.NewNone[uint64](),
		ProviderID: 0,
		ModelID:    0,
		TokenIn:    credits,
		Nonce:      []byte{0xAB},
	}}))
}

// ── Init / 治理 ──────────────────────────────────────────────

func TestInit(t *testing.T) {
	c, _ := newTestC2C()
	require.NoError(t, C2CMutation{*c}.Init())
}

func TestGovernanceRequiresSystem(t *testing.T) {
	c, rt, _, _ := setupGovernance(t)
	rt.System = false
	assert.Error(t, C2CMutation{*c}.RegisterCode([]byte("x"), "1.0"))
	assert.Error(t, C2CMutation{*c}.RegisterNode(addr(0x01)))
	assert.Error(t, C2CMutation{*c}.SetFxConfig(FxConfig{CnyPerPoint: 72000}))
}

// ── ① 代码白名单 ─────────────────────────────────────────────

func TestRegisterCode_AuthorizeAndRevoke(t *testing.T) {
	c, rt, code, _ := setupGovernance(t)
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
	c, rt, code, _ := setupGovernance(t)
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

func TestListAuthorizedCodes(t *testing.T) {
	c, rt, code, _ := setupGovernance(t)

	// setupGovernance 已授权 1 个代码版本（version=1.0.0）
	list, err := C2CQuery{*c}.ListAuthorizedCodes()
	require.NoError(t, err)
	require.Len(t, list, 1)
	assert.Equal(t, code, list[0].CodeHash)
	assert.Equal(t, "1.0.0", list[0].Version)

	// 再授权一个 → 列表 2 条，记录版本号与注册时间
	rt.Time = 1700000000
	rt.System = true
	require.NoError(t, C2CMutation{*c}.RegisterCode([]byte("c2c-code-v2"), "1.0.1"))
	rt.System = false

	list, err = C2CQuery{*c}.ListAuthorizedCodes()
	require.NoError(t, err)
	require.Len(t, list, 2)

	var found *AuthorizedCode
	for i := range list {
		if string(list[i].CodeHash) == "c2c-code-v2" {
			found = &list[i]
		}
	}
	require.NotNil(t, found, "新授权的代码版本应出现在列表中")
	assert.Equal(t, "1.0.1", found.Version)
	assert.Equal(t, uint64(1700000000), found.RegisteredAt)

	// 撤销后从列表消失
	rt.System = true
	require.NoError(t, C2CMutation{*c}.RevokeCode(code))
	rt.System = false

	list, err = C2CQuery{*c}.ListAuthorizedCodes()
	require.NoError(t, err)
	require.Len(t, list, 1)
	assert.Equal(t, []byte("c2c-code-v2"), list[0].CodeHash)
}

// ── ② 节点注册表 ─────────────────────────────────────────────

// RegisterNode 不再绑定代码版本：白名单为空时节点注册同样成功。
// 代码版本在 Release 时由“TEE 证明 + 白名单”校验，而非注册时绑定。
func TestRegisterNode_NoCodeBinding(t *testing.T) {
	c, rt := newTestC2C()
	rt.System = true
	assert.NoError(t, C2CMutation{*c}.RegisterNode(addr(0x10)))
	n, err := C2CQuery{*c}.GetNode(addr(0x10))
	require.NoError(t, err)
	assert.True(t, n.Active)
	rt.System = false
}

func TestRegisterNode_Duplicate(t *testing.T) {
	c, rt, _, _ := setupGovernance(t)
	rt.System = true
	assert.ErrorIs(t, C2CMutation{*c}.RegisterNode(addr(0x10)), ErrNodeExists)
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
	require.NoError(t, C2CMutation{*c}.RegisterNode(addr(0x10)))
	require.NoError(t, C2CMutation{*c}.RemoveNode(addr(0x10)))
	rt.System = false
}

func TestRemoveNode_OnlyGovernanceOrSelf(t *testing.T) {
	c, rt, _, _ := setupGovernance(t)
	rt.Caller = addr(0x99) // 第三方
	assert.Error(t, C2CMutation{*c}.RemoveNode(addr(0x10)))
}

func TestListNodes(t *testing.T) {
	c, rt, _, _ := setupGovernance(t)
	rt.System = true
	require.NoError(t, C2CMutation{*c}.RegisterNode(addr(0x20)))
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

// ── ③.5 商户盈利入 escrow（验证节点代商家操作）────────────────

func TestDepositFor_Success(t *testing.T) {
	c, rt, _, _ := setupGovernance(t)
	merchant := addr(0x30)
	gw := gateway.NewGateway(rt)
	seedProviderCredits(t, rt, gw, merchant, 200)

	rt.Caller = addr(0x10) // 注册验证节点
	require.NoError(t, C2CMutation{*c}.DepositFor(merchant, amt(120)))

	escrow, err := C2CQuery{*c}.GetMerchantEscrow(merchant)
	require.NoError(t, err)
	assert.Equal(t, 0, cmpInt(escrow, 120))
	// provider 盈利被扣减 120（200 → 80）。
	gq := gateway.GatewayQuery{Gateway: *gw}
	pc, err := gq.GetProviderCredit(0)
	require.NoError(t, err)
	assert.Equal(t, uint64(80), pc)
}

func TestDepositFor_NotNode(t *testing.T) {
	c, rt, _, _ := setupGovernance(t)
	rt.Caller = addr(0x50) // 未注册节点
	assert.ErrorIs(t, C2CMutation{*c}.DepositFor(addr(0x30), amt(10)), ErrNotNode)
}

func TestDepositFor_NoProvider(t *testing.T) {
	c, rt, _, _ := setupGovernance(t)
	rt.Caller = addr(0x10) // 节点
	// merchant 名下无 provider → gateway ErrProviderNotFound。
	assert.ErrorIs(t, C2CMutation{*c}.DepositFor(addr(0x60), amt(10)), gateway.ErrProviderNotFound)
}

func TestDepositFor_InsufficientProviderCredit(t *testing.T) {
	c, rt, _, _ := setupGovernance(t)
	gw := gateway.NewGateway(rt)
	seedProviderCredits(t, rt, gw, addr(0x30), 50)
	rt.Caller = addr(0x10)
	// provider 盈利 50 < 100 → gateway ErrLowCredit。
	assert.ErrorIs(t, C2CMutation{*c}.DepositFor(addr(0x30), amt(100)), gateway.ErrLowCredit)
}

func TestDepositFor_ZeroAmount(t *testing.T) {
	c, rt, _, _ := setupGovernance(t)
	rt.Caller = addr(0x10)
	assert.ErrorIs(t, C2CMutation{*c}.DepositFor(addr(0x30), amt(0)), ErrZeroAmount)
}

// ── ④ 受控放款（TEE 证明 + 链上 CodeSignature 验证）────────────

func TestRelease_FullCycle(t *testing.T) {
	c, rt, _, priv := setupGovernance(t)
	merchant := addr(0x30)
	buyer := addr(0x40)

	rt.Caller = merchant
	require.NoError(t, C2CMutation{*c}.Deposit(amt(1000)))

	rt.Caller = addr(0x10) // 节点
	require.NoError(t, C2CMutation{*c}.Release(releaseWithSig(t, rt, priv, Release{
		Merchant: merchant, Buyer: buyer, Amount: amt(400),
		OrderNo:   []byte("order-1"),
		FiatCents: 40000, Currency: "USD",
	})))

	escrow, _ := C2CQuery{*c}.GetMerchantEscrow(merchant)
	assert.Equal(t, 0, cmpInt(escrow, 600))
	// 放款直接进 gateway credits（c2c 合约不再维护 buyerBalance）。
	gw := gateway.NewGateway(rt)
	cred, err := gateway.GatewayQuery{Gateway: *gw}.GetCredit(buyer)
	require.NoError(t, err)
	assert.Equal(t, int64(400), cred)
}

func TestRelease_NotNode(t *testing.T) {
	c, rt, _, _ := setupGovernance(t)
	rt.Caller = addr(0x50) // 未注册节点
	require.NoError(t, C2CMutation{*c}.Deposit(amt(1000)))
	assert.ErrorIs(t, C2CMutation{*c}.Release(ReleaseWithSig{
		Data: Release{
			Merchant: addr(0x50), Buyer: addr(0x40), Amount: amt(10),
			OrderNo: []byte("o1"),
		},
	}), ErrNotNode)
}

func TestRelease_MissingTeeReport(t *testing.T) {
	c, rt, _, priv := setupGovernance(t)
	rt.Caller = addr(0x30)
	require.NoError(t, C2CMutation{*c}.Deposit(amt(1000)))
	rt.Caller = addr(0x10) // 节点
	// 未携带 TEE 报告 → 拒绝（代码版本必须由 TEE 硬件证明，不接受用户声称）。
	assert.ErrorIs(t, C2CMutation{*c}.Release(ReleaseWithSig{
		Data:          Release{Merchant: addr(0x30), Buyer: addr(0x40), Amount: amt(10), OrderNo: []byte("o1")},
		CodeSignature: signRelease(t, priv, Release{Merchant: addr(0x30), Buyer: addr(0x40), Amount: amt(10), OrderNo: []byte("o1")}),
	}), ErrTeeReportMissing)
}

func TestRelease_TeeVerifyFailed(t *testing.T) {
	c, rt, _, priv := setupGovernance(t)
	rt.Caller = addr(0x30)
	require.NoError(t, C2CMutation{*c}.Deposit(amt(1000)))
	rt.Caller = addr(0x10) // 节点
	// 验证节点 quote 验签失败（如报告伪造/过期）→ 拒绝。
	rt.VerifyTEEFunc = func(_, _ []byte, _ int64, _ []byte, _ uint32) store.Result[store.TeeVerifyInfo, string] {
		return store.NewErr[store.TeeVerifyInfo, string]("invalid report sign")
	}
	rel := Release{Merchant: addr(0x30), Buyer: addr(0x40), Amount: amt(10), OrderNo: []byte("o1")}
	assert.ErrorIs(t, C2CMutation{*c}.Release(ReleaseWithSig{
		Data:          rel,
		TeeReport:     []byte("forged-report"),
		CodeSignature: signRelease(t, priv, rel),
	}), ErrTeeVerifyFailed)
}

func TestRelease_CodeRevoked(t *testing.T) {
	c, rt, code, priv := setupGovernance(t)
	merchant := addr(0x30)
	rt.Caller = merchant
	require.NoError(t, C2CMutation{*c}.Deposit(amt(100)))
	// 治理撤销代码版本授权后，节点不得再以其放款。
	rt.System = true
	require.NoError(t, C2CMutation{*c}.RevokeCode(code))
	rt.System = false
	rt.Caller = addr(0x10)
	assert.ErrorIs(t, C2CMutation{*c}.Release(releaseWithSig(t, rt, priv, Release{
		Merchant: merchant, Buyer: addr(0x40), Amount: amt(10),
		OrderNo: []byte("r1"), FiatCents: 1000, Currency: "USD",
	})), ErrCodeNotAuthorized)
}

func TestRelease_InsufficientEscrow(t *testing.T) {
	c, rt, _, priv := setupGovernance(t)
	rt.Caller = addr(0x30)
	require.NoError(t, C2CMutation{*c}.Deposit(amt(100)))
	rt.Caller = addr(0x10)
	assert.ErrorIs(t, C2CMutation{*c}.Release(releaseWithSig(t, rt, priv, Release{
		Merchant: addr(0x30), Buyer: addr(0x40), Amount: amt(1000),
		OrderNo:   []byte("o1"),
		FiatCents: 100000, Currency: "USD",
	})), ErrInsufficientEscrow)
}

func TestRelease_DuplicateOrder(t *testing.T) {
	c, rt, _, priv := setupGovernance(t)
	merchant := addr(0x30)
	rt.Caller = merchant
	require.NoError(t, C2CMutation{*c}.Deposit(amt(1000)))
	rt.Caller = addr(0x10)
	r := Release{Merchant: merchant, Buyer: addr(0x40), Amount: amt(100), OrderNo: []byte("dup"), FiatCents: 10000, Currency: "USD"}
	require.NoError(t, C2CMutation{*c}.Release(releaseWithSig(t, rt, priv, r)))
	assert.ErrorIs(t, C2CMutation{*c}.Release(releaseWithSig(t, rt, priv, r)), ErrDuplicateRelease)
}

// ── ④.5 折算校验（按下单锁定汇率 LockedRate）──────────────────

func TestRelease_ConversionByLockedRate(t *testing.T) {
	c, rt, _, priv := setupGovernance(t)
	merchant := addr(0x30)
	buyer := addr(0x40)
	// config 汇率 8.0 CNY/积分，但订单锁定 7.2：折算须以 LockedRate 为准，不受 config 变动影响。
	rt.System = true
	require.NoError(t, C2CMutation{*c}.SetFxConfig(FxConfig{CnyPerPoint: 80000}))
	rt.System = false
	rt.Caller = merchant
	require.NoError(t, C2CMutation{*c}.Deposit(amt(1000)))
	rt.Caller = addr(0x10)

	// CNY 锁价 7.2：100 积分 × 72000 / 100 = 72000 分 ✓（若按 config 8.0 算为 80000，会误拒）。
	require.NoError(t, C2CMutation{*c}.Release(releaseWithSig(t, rt, priv, Release{
		Merchant: merchant, Buyer: buyer, Amount: amt(100),
		OrderNo: []byte("l1"), FiatCents: 72000, Currency: "CNY", LockedRate: 72000,
	})))
}

func TestRelease_ConversionMismatch(t *testing.T) {
	c, rt, _, priv := setupGovernance(t)
	merchant := addr(0x30)
	buyer := addr(0x40)
	rt.Caller = merchant
	require.NoError(t, C2CMutation{*c}.Deposit(amt(1000)))
	rt.Caller = addr(0x10)
	rel := func(order string, r Release) error {
		r.Merchant, r.Buyer, r.Amount, r.OrderNo = merchant, buyer, amt(100), []byte(order)
		return C2CMutation{*c}.Release(releaseWithSig(t, rt, priv, r))
	}

	// USD 报假 fiat（应为 100000，报 99999）。
	assert.ErrorIs(t, rel("m1", Release{FiatCents: 99999, Currency: "USD"}), ErrConversionMismatch)
	// CNY 报假 fiat（锁价 7.2 下应为 72000，报 71999）。
	assert.ErrorIs(t, rel("m2", Release{FiatCents: 71999, Currency: "CNY", LockedRate: 72000}), ErrConversionMismatch)
	// CNY 未锁价（LockedRate=0）→ 折算不通过。
	assert.ErrorIs(t, rel("m3", Release{FiatCents: 72000, Currency: "CNY"}), ErrConversionMismatch)
	// 不支持的币种。
	assert.ErrorIs(t, rel("m4", Release{FiatCents: 72000, Currency: "EUR"}), ErrConversionMismatch)
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
	// 非系统账户拒绝（节点也不行）。
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
	c, rt, _, priv := setupGovernance(t)
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
	require.NoError(t, C2CMutation{*c}.Release(releaseWithSig(t, rt, priv, Release{
		Merchant: merchant, Buyer: buyer, Amount: amt(2000),
		OrderNo: []byte("d1"), FiatCents: 200000, Currency: "USD",
	})))
	spent, _ := C2CQuery{*c}.BuyerSpentToday(buyer)
	assert.Equal(t, int64(200000), spent)
	// 第二单 2000 USD：累计将达 400000 > 300000 ✗。
	err := C2CMutation{*c}.Release(releaseWithSig(t, rt, priv, Release{
		Merchant: merchant, Buyer: buyer, Amount: amt(2000),
		OrderNo: []byte("d2"), FiatCents: 200000, Currency: "USD",
	}))
	assert.ErrorIs(t, err, ErrDailyLimitExceeded)
	// 跨日后重置：当日累计归零，可再下。
	rt.Time = 86400 * 8
	require.NoError(t, C2CMutation{*c}.Release(releaseWithSig(t, rt, priv, Release{
		Merchant: merchant, Buyer: buyer, Amount: amt(2000),
		OrderNo: []byte("d3"), FiatCents: 200000, Currency: "USD",
	})))
}

func TestRelease_PerOrderLimit(t *testing.T) {
	c, rt, _, priv := setupGovernance(t)
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
	err := C2CMutation{*c}.Release(releaseWithSig(t, rt, priv, Release{
		Merchant: merchant, Buyer: buyer, Amount: amt(2000),
		OrderNo: []byte("p1"), FiatCents: 200000, Currency: "USD",
	}))
	assert.ErrorIs(t, err, ErrOrderLimitExceeded)
}

func TestRelease_Commission(t *testing.T) {
	c, rt, _, priv := setupGovernance(t)
	merchant := addr(0x30)
	buyer := addr(0x40)
	rt.System = true
	// 当前 config 费率 4%（400）——但订单快照费率是 5%（500），合约须用快照值（不受 config 变更影响）。
	require.NoError(t, C2CMutation{*c}.SetFxConfig(FxConfig{
		CnyPerPoint: 72000, PlatformFeePct: 400,
	}))
	rt.System = false
	rt.Caller = merchant
	require.NoError(t, C2CMutation{*c}.Deposit(amt(1000)))
	rt.Caller = addr(0x10)
	// 放款 1000 积分：佣金 = floor(1000 × 500 / 10000) = 50（用订单快照费率 5%，非 config 4%）；买家得 950。
	require.NoError(t, C2CMutation{*c}.Release(releaseWithSig(t, rt, priv, Release{
		Merchant: merchant, Buyer: buyer, Amount: amt(1000),
		OrderNo: []byte("c1"), FiatCents: 100000, Currency: "USD", FeePct: 500,
	})))
	esc, _ := C2CQuery{*c}.GetMerchantEscrow(merchant)
	pc, _ := C2CQuery{*c}.GetPlatformCommission()
	assert.Equal(t, 0, cmpInt(esc, 0)) // escrow 全额扣 1000
	assert.Equal(t, 0, cmpInt(pc, 50)) // 佣金 50（快照费率 5%，非 config 4%）
	gw := gateway.NewGateway(rt)
	cred, err := gateway.GatewayQuery{Gateway: *gw}.GetCredit(buyer)
	require.NoError(t, err)
	assert.Equal(t, int64(950), cred) // 买家得 950（放款直接进 gateway credits）
}
