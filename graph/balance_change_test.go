package graph

import (
	"encoding/hex"
	"testing"

	abci "github.com/cometbft/cometbft/abci/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ── balanceChangesFromEvents ─────────────────────────────
//
// tx_detail 的 balance_changes 字段：只读解析 events 中 type=balance_change
// 的事件，address 用 uniAddrDisplay 转展示形式（0x EVM / SS58），delta 带符号。
// 旧交易（无事件）返回空列表不报错。

// evAttr 构造一个 abci 事件属性。
func evAttr(k, v string) abci.EventAttribute {
	return abci.EventAttribute{Key: k, Value: v, Index: true}
}

// evBalanceChange 构造一条 balance_change 事件（address 为 T:hex(V) 编码）。
func evBalanceChange(kind, addrEncoded, delta string) abci.Event {
	return abci.Event{
		Type: "balance_change",
		Attributes: []abci.EventAttribute{
			evAttr("kind", kind),
			evAttr("address", addrEncoded),
			evAttr("delta", delta),
		},
	}
}

func TestBalanceChangesFromEvents_Empty(t *testing.T) {
	// 无事件（旧交易）→ 空列表（非 nil），不报错
	out := balanceChangesFromEvents(nil)
	require.NotNil(t, out)
	assert.Empty(t, out)

	out = balanceChangesFromEvents([]abci.Event{})
	require.NotNil(t, out)
	assert.Empty(t, out)
}

func TestBalanceChangesFromEvents_NoBalanceChange(t *testing.T) {
	// 有事件但无 balance_change 类型 → 空列表
	events := []abci.Event{
		{Type: "credit_transfer", Attributes: []abci.EventAttribute{evAttr("user", "0xaaa")}},
		{Type: "other", Attributes: []abci.EventAttribute{evAttr("x", "y")}},
	}
	out := balanceChangesFromEvents(events)
	require.NotNil(t, out)
	assert.Empty(t, out)
}

func TestBalanceChangesFromEvents_EVM(t *testing.T) {
	// EVM 地址（T=2, 20 字节）→ 0x 展示地址；delta 带符号
	addr := "2:" + "90f8bf6a479f320ead074411a4b0e7944ea8c9c1" // 20 字节 hex
	events := []abci.Event{
		evBalanceChange("ai", addr, "100"),
		evBalanceChange("ai", addr, "-100"),
	}
	out := balanceChangesFromEvents(events)
	require.Len(t, out, 2)

	assert.Equal(t, "ai", out[0].Kind)
	assert.Equal(t, "0x90f8bf6a479f320ead074411a4b0e7944ea8c9c1", out[0].Address)
	assert.Equal(t, "100", out[0].Delta)

	assert.Equal(t, "-100", out[1].Delta)
}

func TestBalanceChangesFromEvents_SS58(t *testing.T) {
	// SS58 地址（T=1, 32 字节公钥）→ SS58 展示地址
	pub32 := make([]byte, 32)
	for i := range pub32 {
		pub32[i] = byte(i)
	}
	addrEncoded := "1:" + hex.EncodeToString(pub32)
	events := []abci.Event{
		evBalanceChange("gov", addrEncoded, "50"),
	}
	out := balanceChangesFromEvents(events)
	require.Len(t, out, 1)

	assert.Equal(t, "gov", out[0].Kind)
	// SS58 展示地址非空且非 0x 前缀
	assert.NotEmpty(t, out[0].Address)
	assert.NotContains(t, out[0].Address, "0x")
	assert.Equal(t, "50", out[0].Delta)
}

func TestBalanceChangesFromEvents_MixedAndSkip(t *testing.T) {
	// 混合事件：balance_change + 其它类型 + 属性缺失的 balance_change（应跳过）
	addr := "2:" + "90f8bf6a479f320ead074411a4b0e7944ea8c9c1"
	events := []abci.Event{
		{Type: "noise", Attributes: []abci.EventAttribute{evAttr("a", "b")}},
		evBalanceChange("merchant", addr, "7"),
		// 属性缺失（无 delta）→ 跳过
		{
			Type: "balance_change",
			Attributes: []abci.EventAttribute{
				evAttr("kind", "gov"),
				evAttr("address", addr),
			},
		},
		// 非法 address 编码 → 跳过
		evBalanceChange("gov", "not-a-valid-encoded-addr", "1"),
	}
	out := balanceChangesFromEvents(events)
	require.Len(t, out, 1)
	assert.Equal(t, "merchant", out[0].Kind)
	assert.Equal(t, "7", out[0].Delta)
}

func TestParseUniAddrEncoded(t *testing.T) {
	t.Run("evm_20byte", func(t *testing.T) {
		a, ok := parseUniAddrEncoded("2:" + "90f8bf6a479f320ead074411a4b0e7944ea8c9c1")
		require.True(t, ok)
		assert.Equal(t, uint32(2), a.T)
		assert.Len(t, a.V, 20)
	})

	t.Run("ss58_32byte", func(t *testing.T) {
		pub := make([]byte, 32)
		for i := range pub {
			pub[i] = byte(i)
		}
		a, ok := parseUniAddrEncoded("1:" + hex.EncodeToString(pub))
		require.True(t, ok)
		assert.Equal(t, uint32(1), a.T)
		assert.Len(t, a.V, 32)
	})

	t.Run("no_colon", func(t *testing.T) {
		_, ok := parseUniAddrEncoded("deadbeef")
		assert.False(t, ok)
	})

	t.Run("bad_t", func(t *testing.T) {
		_, ok := parseUniAddrEncoded("x:deadbeef")
		assert.False(t, ok)
	})

	t.Run("bad_hex", func(t *testing.T) {
		_, ok := parseUniAddrEncoded("2:zzzz")
		assert.False(t, ok)
	})

	t.Run("empty_v", func(t *testing.T) {
		// T 存在但 V 为空 hex → 合法（空字节）
		a, ok := parseUniAddrEncoded("2:")
		assert.True(t, ok)
		assert.Equal(t, uint32(2), a.T)
		assert.Empty(t, a.V)
	})
}
