package gov

import (
	"encoding/hex"
	"math/big"
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

// enableTransfer 测试辅助：直接打开 gov 转账开关（Init 默认关闭）。
func enableTransfer(t *testing.T, rt *testutil.MockRuntime) {
	t.Helper()
	b, err := store.ScaleEncode(true)
	require.NoError(t, err)
	require.NoError(t, rt.Txn.Set(store.ComboNamespaceKey("gov", "transfer_enabled"), b))
}

// ── balance_change 事件 ──────────────────────────────────

// TestGov_BalanceChange_Mint Mint 发射一条 balance_change（kind=gov, delta=+value）。
func TestGov_BalanceChange_Mint(t *testing.T) {
	gov, rt := newTestGov()
	asGov(rt)
	require.NoError(t, GovMutation{*gov}.Init())

	user := store.UniAddr{T: 1, V: []byte{0xAB}}
	require.NoError(t, GovMutation{*gov}.Join(user, store.NewU256(big.NewInt(0))))

	require.NoError(t, GovMutation{*gov}.Mint(user, store.NewU256(big.NewInt(5000))))

	events := balanceChangeEvents(rt)
	require.Len(t, events, 1)
	assert.Equal(t, "gov", events[0]["kind"])
	assert.Equal(t, user.String(), events[0]["address"])
	assert.Equal(t, "5000", events[0]["delta"])
}

// TestGov_BalanceChange_Transfer 转账发射两条 balance_change（from -value / to +value）。
func TestGov_BalanceChange_Transfer(t *testing.T) {
	gov, rt := newTestGov()
	asGov(rt)
	require.NoError(t, GovMutation{*gov}.Init())
	enableTransfer(t, rt)

	alice := store.UniAddr{T: 1, V: []byte{0xAA}}
	bob := store.UniAddr{T: 1, V: []byte{0xBB}}
	require.NoError(t, GovMutation{*gov}.Join(alice, store.NewU256(big.NewInt(1000))))
	require.NoError(t, GovMutation{*gov}.Join(bob, store.NewU256(big.NewInt(0))))
	require.NoError(t, GovMutation{*gov}.Mint(alice, store.NewU256(big.NewInt(1000))))
	require.NoError(t, GovMutation{*gov}.Mint(bob, store.NewU256(big.NewInt(1))))
	rt.TakeEvents() // 清空 setup 阶段事件

	rt.Caller = alice
	rt.System = false
	require.NoError(t, GovMutation{*gov}.Transfer(bob, store.NewU256(big.NewInt(300))))

	events := balanceChangeEvents(rt)
	require.Len(t, events, 2)
	assert.Equal(t, "gov", events[0]["kind"])
	assert.Equal(t, alice.String(), events[0]["address"])
	assert.Equal(t, "-300", events[0]["delta"])
	assert.Equal(t, "gov", events[1]["kind"])
	assert.Equal(t, bob.String(), events[1]["address"])
	assert.Equal(t, "300", events[1]["delta"])
}

// TestGov_BalanceChange_TransferFrom 授权转账同样发射两条 balance_change。
func TestGov_BalanceChange_TransferFrom(t *testing.T) {
	gov, rt := newTestGov()
	asGov(rt)
	require.NoError(t, GovMutation{*gov}.Init())
	enableTransfer(t, rt)

	alice := store.UniAddr{T: 1, V: []byte{0xAA}}
	bob := store.UniAddr{T: 1, V: []byte{0xBB}}
	charlie := store.UniAddr{T: 1, V: []byte{0xCC}}
	require.NoError(t, GovMutation{*gov}.Join(alice, store.NewU256(big.NewInt(1000))))
	require.NoError(t, GovMutation{*gov}.Join(bob, store.NewU256(big.NewInt(0))))
	require.NoError(t, GovMutation{*gov}.Mint(alice, store.NewU256(big.NewInt(1000))))
	require.NoError(t, GovMutation{*gov}.Mint(bob, store.NewU256(big.NewInt(1))))
	rt.TakeEvents() // 清空 setup 阶段事件

	// alice 授权 charlie 代转（直接写 allowance，与链上存储格式一致）
	allowance, err := store.ScaleEncode(store.NewU256(big.NewInt(500)))
	require.NoError(t, err)
	keyBytes, err := store.ScaleEncode(allowanceKey(alice, charlie))
	require.NoError(t, err)
	require.NoError(t, rt.Txn.Set(store.ComboNamespaceKey("gov", "allowance_"+hex.EncodeToString(keyBytes)), allowance))

	rt.Caller = charlie
	require.NoError(t, GovMutation{*gov}.TransferFrom(alice, bob, store.NewU256(big.NewInt(200))))

	events := balanceChangeEvents(rt)
	require.Len(t, events, 2)
	assert.Equal(t, "gov", events[0]["kind"])
	assert.Equal(t, alice.String(), events[0]["address"])
	assert.Equal(t, "-200", events[0]["delta"])
	assert.Equal(t, "gov", events[1]["kind"])
	assert.Equal(t, bob.String(), events[1]["address"])
	assert.Equal(t, "200", events[1]["delta"])
}

// TestGov_BalanceChange_LargeValue 金额超过 int64 上限（> 2^63-1）时，
// delta 仍为精确的有符号十进制（不截断、符号正确）。
func TestGov_BalanceChange_LargeValue(t *testing.T) {
	gov, rt := newTestGov()
	asGov(rt)
	require.NoError(t, GovMutation{*gov}.Init())

	// 18 位精度代币：10 个代币 = 1e19 > 2^63-1 (≈9.22e18)
	bigValue := new(big.Int).Mul(big.NewInt(10), new(big.Int).Exp(big.NewInt(10), big.NewInt(18), nil))
	require.True(t, bigValue.Cmp(new(big.Int).Lsh(big.NewInt(1), 63)) > 0)

	user := store.UniAddr{T: 1, V: []byte{0xAB}}
	require.NoError(t, GovMutation{*gov}.Join(user, store.NewU256(big.NewInt(0))))
	require.NoError(t, GovMutation{*gov}.Mint(user, store.NewU256(bigValue)))

	events := balanceChangeEvents(rt)
	require.Len(t, events, 1)
	assert.Equal(t, "gov", events[0]["kind"])
	assert.Equal(t, user.String(), events[0]["address"])
	assert.Equal(t, bigValue.String(), events[0]["delta"])

	// 转账：from 的 delta 为精确负值（-1e19），to 为精确正值
	alice := store.UniAddr{T: 1, V: []byte{0xAA}}
	bob := store.UniAddr{T: 1, V: []byte{0xBB}}
	require.NoError(t, GovMutation{*gov}.Join(alice, store.NewU256(bigValue)))
	require.NoError(t, GovMutation{*gov}.Join(bob, store.NewU256(big.NewInt(1))))
	rt.TakeEvents()

	enableTransfer(t, rt)
	rt.Caller = alice
	rt.System = false
	require.NoError(t, GovMutation{*gov}.Transfer(bob, store.NewU256(bigValue)))

	events = balanceChangeEvents(rt)
	require.Len(t, events, 2)
	neg := new(big.Int).Neg(bigValue)
	assert.Equal(t, "gov", events[0]["kind"])
	assert.Equal(t, alice.String(), events[0]["address"])
	assert.Equal(t, neg.String(), events[0]["delta"])
	assert.Equal(t, "gov", events[1]["kind"])
	assert.Equal(t, bob.String(), events[1]["address"])
	assert.Equal(t, bigValue.String(), events[1]["delta"])
}
