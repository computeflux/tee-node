package graph

import (
	"encoding/hex"
	"math/big"
	"testing"

	"github.com/computeflux/tee-provider/pkg/model"
	"github.com/computeflux/tee-provider/pkg/model/protoio"
	"github.com/computeflux/tee-provider/pkg/types/crypto"
	cstore "github.com/computeflux/tee-provider/tee-chain/runtime/store"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ── normalizeAddress：EVM / SS58 / hex 公钥 归一化（候选 T 集）──

func TestNormalizeAddress(t *testing.T) {
	t.Run("evm_0x", func(t *testing.T) {
		na, err := normalizeAddress("0xAb5801a7D398351b8bE11C439e05C5B3259aeC9B")
		require.NoError(t, err)
		require.Len(t, na.candidates, 1)
		assert.Equal(t, uint32(2), na.candidates[0].T)
		assert.Len(t, na.candidates[0].V, 20)
		// 展示形式为小写 0x
		assert.Equal(t, "0xab5801a7d398351b8be11c439e05c5b3259aec9b", na.display)
	})

	t.Run("evm_lowercase_stable", func(t *testing.T) {
		na, err := normalizeAddress("0xab5801a7d398351b8be11c439e05c5b3259aec9b")
		require.NoError(t, err)
		require.Len(t, na.candidates, 1)
		assert.Equal(t, uint32(2), na.candidates[0].T)
		assert.Len(t, na.candidates[0].V, 20)
	})

	t.Run("evm_wrong_length", func(t *testing.T) {
		_, err := normalizeAddress("0xdeadbeef")
		require.Error(t, err)
	})

	// 关键回归：20 字节 EVM 地址（TxDetail.From 渲染成 SS58 后回环）必须归一化为
	// T:2，而不是 T:0（链节点命名空间），否则 balance 恒为 0 / 交易匹配不到。
	t.Run("ss58_20byte_evm_maps_to_t2", func(t *testing.T) {
		evm20 := make20(0xab)
		ss58 := crypto.SS58Encode(evm20, 42)
		na, err := normalizeAddress(ss58)
		require.NoError(t, err)
		require.Len(t, na.candidates, 1)
		assert.Equal(t, uint32(2), na.candidates[0].T, "20 字节 SS58 必须映射到 EVM T:2")
		assert.Equal(t, evm20, na.candidates[0].V)
		assert.Equal(t, "0xab00000000000000000000000000000000000000", na.display)
	})

	t.Run("ss58_32byte_pubkey_candidates_t1_t3", func(t *testing.T) {
		// 32 字节公钥：sr25519(T:1) / ed25519(T:3) 无法从地址区分 → 两个候选
		pub := make([]byte, 32)
		for i := range pub {
			pub[i] = byte(i)
		}
		ss58 := crypto.SS58Encode(pub, 42)
		na, err := normalizeAddress(ss58)
		require.NoError(t, err)
		require.Len(t, na.candidates, 2)
		assert.Equal(t, uint32(1), na.candidates[0].T)
		assert.Equal(t, uint32(3), na.candidates[1].T)
		assert.Equal(t, pub, na.candidates[0].V)
		assert.Equal(t, ss58, na.display)
	})

	t.Run("hex_pubkey_32", func(t *testing.T) {
		pub := make([]byte, 32)
		for i := range pub {
			pub[i] = byte(i + 1)
		}
		na, err := normalizeAddress(hex.EncodeToString(pub))
		require.NoError(t, err)
		require.Len(t, na.candidates, 2)
		assert.Equal(t, uint32(1), na.candidates[0].T)
		assert.Equal(t, uint32(3), na.candidates[1].T)
		assert.Equal(t, pub, na.candidates[0].V)
		// hex 公钥展示为 SS58
		assert.Equal(t, crypto.PubKeyFromByte(pub).SS58(), na.display)
	})

	t.Run("hex_pubkey_20_evm", func(t *testing.T) {
		evm20 := make20(0xcd)
		na, err := normalizeAddress(hex.EncodeToString(evm20))
		require.NoError(t, err)
		require.Len(t, na.candidates, 1)
		assert.Equal(t, uint32(2), na.candidates[0].T)
		assert.Equal(t, evm20, na.candidates[0].V)
	})

	t.Run("empty", func(t *testing.T) {
		_, err := normalizeAddress("   ")
		require.Error(t, err)
	})

	t.Run("garbage", func(t *testing.T) {
		_, err := normalizeAddress("not-an-address-!!")
		require.Error(t, err)
	})
}

// ── uniAddrDisplay：链上 UniAddr → 展示地址 ────────────────

func TestUniAddrDisplay(t *testing.T) {
	t.Run("evm_t2", func(t *testing.T) {
		v := make([]byte, 20)
		v[0] = 0xab
		assert.Equal(t, "0xab00000000000000000000000000000000000000", uniAddrDisplay(cstore.UniAddr{T: 2, V: v}))
	})

	t.Run("ed25519_pubkey", func(t *testing.T) {
		pub := make([]byte, 32)
		for i := range pub {
			pub[i] = byte(i)
		}
		assert.Equal(t, crypto.PubKeyFromByte(pub).SS58(), uniAddrDisplay(cstore.UniAddr{T: 0, V: pub}))
	})

	t.Run("unrecognized", func(t *testing.T) {
		assert.Equal(t, "", uniAddrDisplay(cstore.UniAddr{T: 9, V: []byte{1, 2, 3}}))
	})
}

// ── uniAddrInSet：候选集匹配 ───────────────────────────────

func TestUniAddrInSet(t *testing.T) {
	a := cstore.UniAddr{T: 2, V: make20(0xaa)}
	set := []cstore.UniAddr{{T: 1, V: make20(0xbb)}, {T: 2, V: make20(0xaa)}}
	assert.True(t, uniAddrInSet(a, set))
	assert.False(t, uniAddrInSet(cstore.UniAddr{T: 1, V: make20(0xaa)}, set), "T 不同不应匹配")
	assert.False(t, uniAddrInSet(a, nil))
}

// ── balanceOf 查询 selector 匹配（reviewer 关键点）─────────
//
// ContractQuery 内部走 MethodToSelectorBytes（默认 mutates=true），而 balanceOf
// 是查询方法（mutates=false），selector 不同。这里断言我们传入的 hex selector
// 经 MethodToSelector 解析后，恰好等于 gov_gen.go 中 balance_of 的查询 selector，
// 从而命中 GovQuery.ExecQuery 的 balance_of 分支而非 default（unknown method）。

func TestBalanceOfSelectorMatchesGovQuery(t *testing.T) {
	// gov_gen.go 中 balance_of 的查询 selector（mutates=false）
	want := [4]byte{0x87, 0xe0, 0xff, 0x32}

	// 我们传给 ContractQuery 的 hex selector
	got := cstore.MethodToSelector(govBalanceOfSelectorHex)
	assert.Equal(t, want, got, "balanceOf selector 必须命中 gov 查询分支")

	// 反证：若误用 mutates=true 的方法名分支，selector 会不同（不会命中查询分支）
	assert.NotEqual(t, want, cstore.MethodToSelector("balance_of"),
		"mutates=true 的 selector 与查询 selector 不同，证明必须显式传查询 selector")
}

// ── matchAddressTx：方向 + 对手方语义（reviewer 关键点）────

// buildTransferTx 构造一笔 gov transfer 交易（from→to, amount）。
func buildTransferTx(t *testing.T, from, to cstore.UniAddr, amount cstore.U256) []byte {
	t.Helper()
	toArg, err := cstore.ScaleEncode(to)
	require.NoError(t, err)
	amtArg, err := cstore.ScaleEncode(amount)
	require.NoError(t, err)
	call := &model.SysCall{
		Payload: &model.SysCall_Contract{
			Contract: &model.ContractCall{
				Name:   []byte("gov"),
				Method: cstore.MethodToSelectorBytes("transfer"),
				Args:   [][]byte{toArg, amtArg},
			},
		},
	}
	callBytes, err := call.MarshalVT()
	require.NoError(t, err)
	tx := &model.Tx{
		Caller:     from.V,
		CallerType: from.T,
		Call:       callBytes,
	}
	txBytes, err := protoio.Marshal(tx)
	require.NoError(t, err)
	return txBytes
}

func TestMatchAddressTx_DirectionAndCounterparty(t *testing.T) {
	// 三个地址：alice（付款方）、bob（收款方）、carol（无关）
	alice := cstore.UniAddr{T: 2, V: make20(0xaa)}
	bob := cstore.UniAddr{T: 2, V: make20(0xbb)}
	carol := cstore.UniAddr{T: 2, V: make20(0xcc)}
	amount := cstore.NewU256(bigFromStr(t, "12345"))
	rawTx := buildTransferTx(t, alice, bob, amount)
	transferSel := cstore.MethodToSelectorBytes("transfer")

	t.Run("out_direction_target_is_sender", func(t *testing.T) {
		item, ok := matchAddressTx(rawTx, 100, 0, "2026-01-01T00:00:00Z", []cstore.UniAddr{alice}, transferSel)
		require.True(t, ok)
		assert.Equal(t, "out", item.Direction)
		// out：对手方 = 收款方 bob
		require.NotNil(t, item.Counterparty)
		assert.Equal(t, uniAddrDisplay(bob), *item.Counterparty)
		require.NotNil(t, item.Amount)
		assert.Equal(t, "12345", *item.Amount)
	})

	t.Run("in_direction_target_is_receiver", func(t *testing.T) {
		item, ok := matchAddressTx(rawTx, 100, 0, "2026-01-01T00:00:00Z", []cstore.UniAddr{bob}, transferSel)
		require.True(t, ok)
		assert.Equal(t, "in", item.Direction)
		// in：对手方 = 付款方 alice（不是收款方自己）
		require.NotNil(t, item.Counterparty)
		assert.Equal(t, uniAddrDisplay(alice), *item.Counterparty)
		require.NotNil(t, item.Amount)
		assert.Equal(t, "12345", *item.Amount)
	})

	t.Run("unrelated_address_no_match", func(t *testing.T) {
		_, ok := matchAddressTx(rawTx, 100, 0, "2026-01-01T00:00:00Z", []cstore.UniAddr{carol}, transferSel)
		assert.False(t, ok)
	})

	t.Run("self_transfer_counterparty_empty", func(t *testing.T) {
		// 自转：from == to == alice，对手方应留空（避免“自己转给自己”）
		selfTx := buildTransferTx(t, alice, alice, amount)
		item, ok := matchAddressTx(selfTx, 100, 0, "2026-01-01T00:00:00Z", []cstore.UniAddr{alice}, transferSel)
		require.True(t, ok)
		assert.Equal(t, "out", item.Direction)
		assert.Nil(t, item.Counterparty)
	})

	t.Run("candidate_set_matches_any_t", func(t *testing.T) {
		// 32 字节公钥的候选集（T:1 / T:3）：交易以 T:3 发起也应命中
		pub := make([]byte, 32)
		for i := range pub {
			pub[i] = byte(i + 7)
		}
		sender := cstore.UniAddr{T: 3, V: pub}
		receiver := cstore.UniAddr{T: 2, V: make20(0xee)}
		tx := buildTransferTx(t, sender, receiver, amount)
		candidates := []cstore.UniAddr{{T: 1, V: pub}, {T: 3, V: pub}}
		item, ok := matchAddressTx(tx, 100, 0, "2026-01-01T00:00:00Z", candidates, transferSel)
		require.True(t, ok, "候选集应命中 T:3 发起的交易")
		assert.Equal(t, "out", item.Direction)
	})
}

// make20 构造 20 字节 EVM 地址（首字节为 v）。
func make20(v byte) []byte {
	b := make([]byte, 20)
	b[0] = v
	return b
}

// bigFromStr 解析十进制字符串为 *big.Int。
func bigFromStr(t *testing.T, s string) *big.Int {
	t.Helper()
	bi, ok := new(big.Int).SetString(s, 10)
	require.True(t, ok)
	return bi
}
