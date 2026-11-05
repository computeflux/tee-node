package gateway

import (
	"math"
	"strconv"
	"testing"

	"github.com/computeflux/tee-provider/tee-chain/runtime/store"
	"github.com/computeflux/tee-provider/tee-chain/runtime/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// balanceChangeEvents 取出 MockRuntime 中所有 balance_change 事件（按发射顺序）。
func balanceChangeEvents(rt *testutil.MockRuntime) []map[string]string {
	events := rt.TakeEvents()
	var out []map[string]string
	for _, e := range events {
		if e.Type != "balance_change" {
			continue
		}
		attrs := make(map[string]string, len(e.Attributes))
		for _, a := range e.Attributes {
			attrs[a.Key] = a.Value
		}
		out = append(out, attrs)
	}
	return out
}

// ── balance_change 事件 ──────────────────────────────────

// TestGateway_BalanceChange_Recharge 充值发射一条 balance_change（kind=ai, delta=+amount）。
func TestGateway_BalanceChange_Recharge(t *testing.T) {
	gw, rt := newTestGateway()
	m := GatewayMutation{*gw}

	user := store.UniAddr{T: 1, V: []byte{0xAB}}
	rt.System = true
	require.NoError(t, m.Recharge(user, 1000, testChainID, 1))

	events := balanceChangeEvents(rt)
	require.Len(t, events, 1)
	assert.Equal(t, "ai", events[0]["kind"])
	assert.Equal(t, user.String(), events[0]["address"])
	assert.Equal(t, "1000", events[0]["delta"])
}

// TestGateway_BalanceChange_TransferCredit 转账发射两条 balance_change（from - / to +）。
func TestGateway_BalanceChange_TransferCredit(t *testing.T) {
	gw, rt := newTestGateway()
	m := GatewayMutation{*gw}

	from := store.UniAddr{T: 1, V: []byte{0xAB}}
	to := store.UniAddr{T: 1, V: []byte{0xCD}}

	rt.System = true
	require.NoError(t, m.Recharge(from, 1000, testChainID, 1))
	rt.System = false
	rt.TakeEvents() // 清空充值事件

	rt.Caller = from
	require.NoError(t, m.TransferCredit(to, 600))

	events := balanceChangeEvents(rt)
	require.Len(t, events, 2)
	assert.Equal(t, "ai", events[0]["kind"])
	assert.Equal(t, from.String(), events[0]["address"])
	assert.Equal(t, "-600", events[0]["delta"])
	assert.Equal(t, "ai", events[1]["kind"])
	assert.Equal(t, to.String(), events[1]["address"])
	assert.Equal(t, "600", events[1]["delta"])
}

// TestGateway_BalanceChange_SettleUsage 结算发射两条 balance_change
//（用户 -cost / provider owner +cost，一交易可多条）。
func TestGateway_BalanceChange_SettleUsage(t *testing.T) {
	gw, rt := newTestGateway()
	m := GatewayMutation{*gw}

	owner := rt.Caller
	seedProvider(t, gw, rt, "AI Corp", 200_000_000)
	require.NoError(t, m.RegisterModel("gpt-4", ModalityText, 128000, 1_000_000, 500_000, 2_000_000, OpenAI, "", nil, 0, 0, false, ""))

	user := store.UniAddr{T: 1, V: []byte{0xAB}}
	rt.System = true
	require.NoError(t, m.Recharge(user, 1_000_000, testChainID, 1))
	rt.System = false
	rt.TakeEvents() // 清空 setup 阶段事件

	// Settle 必须由 node（T == 0）调用
	rt.Caller.T = 0
	// in=1000, out=500 → cost = (1000*1_000_000 + 500*2_000_000) / 1_000_000 = 2000
	require.NoError(t, m.SettleUsageWithRoute([]SettlementWithRoute{
		{
			User:       user,
			RouteID:    store.NewNone[uint64](),
			ModelID:    0,
			ProviderID: 0,
			TokenIn:    1000,
			TokenOut:   500,
			Count:      1,
			Nonce:      []byte{0x01, 0x02, 0x03, 0x04},
		},
	}))

	events := balanceChangeEvents(rt)
	require.Len(t, events, 2)
	assert.Equal(t, "ai", events[0]["kind"])
	assert.Equal(t, user.String(), events[0]["address"])
	assert.Equal(t, "-2000", events[0]["delta"])
	assert.Equal(t, "merchant", events[1]["kind"])
	assert.Equal(t, owner.String(), events[1]["address"])
	assert.Equal(t, "2000", events[1]["delta"])
}

// TestGateway_BalanceChange_Withdraw 提现扣减发射一条 balance_change（kind=merchant, delta=-amount）。
func TestGateway_BalanceChange_Withdraw(t *testing.T) {
	gw, rt := newTestGateway()
	m := GatewayMutation{*gw}
	q := GatewayQuery{*gw}

	// ApplyWithdraw 要求 caller.T == 2 且为 provider owner
	rt.Caller.T = 2
	owner := rt.Caller
	seedProvider(t, gw, rt, "AI Corp", 200_000_000)
	require.NoError(t, m.RegisterModel("gpt-4", ModalityText, 128000, 1_000_000, 500_000, 2_000_000, OpenAI, "", nil, 0, 0, false, ""))

	user := store.UniAddr{T: 1, V: []byte{0xAB}}
	rt.System = true
	require.NoError(t, m.Recharge(user, 1_000_000, testChainID, 1))
	rt.System = false

	// 结算产生 provider 盈利
	rt.Caller.T = 0
	require.NoError(t, m.SettleUsageWithRoute([]SettlementWithRoute{
		{User: user, RouteID: store.NewNone[uint64](), ModelID: 0, ProviderID: 0, TokenIn: 1000000, TokenOut: 500000, Count: 1, Nonce: []byte{0x01}},
	}))
	rt.TakeEvents() // 清空结算事件

	provCredit, err := q.GetProviderCredit(0)
	require.NoError(t, err)
	require.Greater(t, provCredit, uint64(0))

	// 提现（owner 全额提走）
	rt.Caller.T = 2
	require.NoError(t, m.ApplyWithdraw(420420417, []byte{}, 0, provCredit, []byte("nonce1")))

	events := balanceChangeEvents(rt)
	require.Len(t, events, 1)
	assert.Equal(t, "merchant", events[0]["kind"])
	assert.Equal(t, owner.String(), events[0]["address"])
	assert.Equal(t, "-"+itoa(provCredit), events[0]["delta"])
}

// TestGateway_BalanceChange_WithdrawalCancelled 提现取消回补发射一条 balance_change（delta=+amount）。
func TestGateway_BalanceChange_WithdrawalCancelled(t *testing.T) {
	gw, rt := newTestGateway()
	m := GatewayMutation{*gw}
	q := GatewayQuery{*gw}

	rt.Caller.T = 2
	owner := rt.Caller
	seedProvider(t, gw, rt, "AI Corp", 200_000_000)
	require.NoError(t, m.RegisterModel("gpt-4", ModalityText, 128000, 1_000_000, 500_000, 2_000_000, OpenAI, "", nil, 0, 0, false, ""))

	user := store.UniAddr{T: 1, V: []byte{0xAB}}
	rt.System = true
	require.NoError(t, m.Recharge(user, 1_000_000, testChainID, 1))
	rt.System = false

	rt.Caller.T = 0
	require.NoError(t, m.SettleUsageWithRoute([]SettlementWithRoute{
		{User: user, RouteID: store.NewNone[uint64](), ModelID: 0, ProviderID: 0, TokenIn: 1000000, TokenOut: 500000, Count: 1, Nonce: []byte{0x01}},
	}))
	rt.TakeEvents()

	provCredit, err := q.GetProviderCredit(0)
	require.NoError(t, err)
	require.Greater(t, provCredit, uint64(0))

	rt.Caller.T = 2
	require.NoError(t, m.ApplyWithdraw(420420417, []byte{}, 0, provCredit, []byte("nonce1")))
	rt.TakeEvents() // 清空提现事件

	// 取消提现（node 调用），余额回补
	rt.Caller.T = 0
	require.NoError(t, m.WithdrawalCancelled(0))

	events := balanceChangeEvents(rt)
	require.Len(t, events, 1)
	assert.Equal(t, "merchant", events[0]["kind"])
	assert.Equal(t, owner.String(), events[0]["address"])
	assert.Equal(t, itoa(provCredit), events[0]["delta"])
}

// TestGateway_BalanceChange_C2C 挂单扣减（WithdrawToC2C, merchant -）与放款（AddCredit, ai +amount*10000）。
func TestGateway_BalanceChange_C2C(t *testing.T) {
	gw, rt := newTestGateway()
	m := GatewayMutation{*gw}
	q := GatewayQuery{*gw}
	s := GatewaySecret{*gw}

	rt.Caller.T = 2
	owner := rt.Caller
	seedProvider(t, gw, rt, "AI Corp", 200_000_000)
	require.NoError(t, m.RegisterModel("gpt-4", ModalityText, 128000, 1_000_000, 500_000, 2_000_000, OpenAI, "", nil, 0, 0, false, ""))

	user := store.UniAddr{T: 1, V: []byte{0xAB}}
	rt.System = true
	require.NoError(t, m.Recharge(user, 1_000_000, testChainID, 1))
	rt.System = false

	rt.Caller.T = 0
	require.NoError(t, m.SettleUsageWithRoute([]SettlementWithRoute{
		{User: user, RouteID: store.NewNone[uint64](), ModelID: 0, ProviderID: 0, TokenIn: 1000000, TokenOut: 500000, Count: 1, Nonce: []byte{0x01}},
	}))
	rt.TakeEvents()

	provCredit, err := q.GetProviderCredit(0)
	require.NoError(t, err)
	require.Greater(t, provCredit, uint64(0))

	// c2c 挂单：扣商家盈利
	rt.Caller = owner
	require.NoError(t, s.WithdrawToC2C(owner, provCredit))

	// c2c 放款：给买家加 credits（amount*10000）
	buyer := store.UniAddr{T: 1, V: []byte{0xEF}}
	require.NoError(t, s.AddCredit(buyer, 100))

	events := balanceChangeEvents(rt)
	require.Len(t, events, 2)
	assert.Equal(t, "merchant", events[0]["kind"])
	assert.Equal(t, owner.String(), events[0]["address"])
	assert.Equal(t, "-"+itoa(provCredit), events[0]["delta"])
	assert.Equal(t, "ai", events[1]["kind"])
	assert.Equal(t, buyer.String(), events[1]["address"])
	assert.Equal(t, "1000000", events[1]["delta"])
}

func itoa(v uint64) string {
	return strconv.FormatUint(v, 10)
}

// TestGateway_BalanceChange_LargeValue 金额 ≥ 2^63 时，delta 仍为精确的无符号十进制
//（不截断、不符号翻转）。旧实现用 int64(amount) 会在 ≥ 2^63 时翻成负数。
func TestGateway_BalanceChange_LargeValue(t *testing.T) {
	gw, rt := newTestGateway()
	m := GatewayMutation{*gw}

	user := store.UniAddr{T: 1, V: []byte{0xAB}}
	bigAmount := uint64(math.MaxUint64) // 2^64-1 > 2^63-1

	rt.System = true
	require.NoError(t, m.Recharge(user, bigAmount, testChainID, 1))

	events := balanceChangeEvents(rt)
	require.Len(t, events, 1)
	assert.Equal(t, "ai", events[0]["kind"])
	assert.Equal(t, user.String(), events[0]["address"])
	assert.Equal(t, strconv.FormatUint(bigAmount, 10), events[0]["delta"])
}
