package runtime

import (
	"math/big"
	"testing"

	"github.com/computeflux/tee-provider/tee-chain/runtime/module"
	"github.com/computeflux/tee-provider/tee-chain/runtime/module/c2c"
	"github.com/computeflux/tee-provider/tee-chain/runtime/store"
	"github.com/computeflux/tee-provider/tee-chain/runtime/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func c2cScale[T any](t *testing.T, v T) []byte {
	t.Helper()
	b, err := store.ScaleEncode(v)
	require.NoError(t, err)
	return b
}

// qSel 查询方法选择器（mutates=false）。
func qSel(method string) []byte {
	bt := store.MethodToSelectorWithMutates(method, false)
	return bt[:]
}

func c2cAddr(b byte) store.UniAddr { return store.UniAddr{T: 1, V: []byte{b}} }

// TestC2C_Dispatch_EndToEnd 通过包级 Mutation/Query 走完整 dispatch：
// 方法选择器解析 + SCALE 参数编解码 + 合约逻辑，验证 call.go 注册正确。
func TestC2C_Dispatch_EndToEnd(t *testing.T) {
	rt := testutil.NewMockRuntime()
	gov := c2cAddr(0xAA)
	code := []byte("c2c-code-v1")

	// Init
	call := &store.InContractCall{Name: []byte("c2c"), Method: store.MethodToSelectorBytes("init"), Args: [][]byte{c2cScale(t, gov)}}
	require.NoError(t, Mutation(module.NativeVersion, call, rt))

	// RegisterCode
	call = &store.InContractCall{Name: []byte("c2c"), Method: store.MethodToSelectorBytes("register_code"), Args: [][]byte{c2cScale(t, code), c2cScale(t, "1.0.0")}}
	rt.Caller = gov
	require.NoError(t, Mutation(module.NativeVersion, call, rt))

	// RegisterNode（治理注册节点 0x10）
	call = &store.InContractCall{Name: []byte("c2c"), Method: store.MethodToSelectorBytes("register_node"), Args: [][]byte{c2cScale(t, c2cAddr(0x10)), c2cScale(t, code)}}
	rt.Caller = gov
	require.NoError(t, Mutation(module.NativeVersion, call, rt))

	// Deposit
	call = &store.InContractCall{Name: []byte("c2c"), Method: store.MethodToSelectorBytes("deposit"), Args: [][]byte{c2cScale(t, store.NewU256(big.NewInt(1000)))}}
	rt.Caller = c2cAddr(0x30)
	require.NoError(t, Mutation(module.NativeVersion, call, rt))

	// Release
	rel := c2c.Release{
		Merchant:    c2cAddr(0x30),
		Buyer:       c2cAddr(0x40),
		Amount:      store.NewU256(big.NewInt(400)),
		CodeVersion: code,
		OrderNo:     []byte("order-1"),
		FiatCents:   40000,
		Currency:    "USD",
	}
	call = &store.InContractCall{Name: []byte("c2c"), Method: store.MethodToSelectorBytes("release"), Args: [][]byte{c2cScale(t, rel)}}
	rt.Caller = c2cAddr(0x10)
	require.NoError(t, Mutation(module.NativeVersion, call, rt))

	// 查询买方可用余额（ExecQuery → SCALE 编码 → 解码）。
	qCall := &store.InContractCall{Name: []byte("c2c"), Method: qSel("get_buyer_balance"), Args: [][]byte{c2cScale(t, c2cAddr(0x40))}}
	data, err := Query(module.NativeVersion, qCall, rt)
	require.NoError(t, err)
	bal, err := store.DecodeScaleArgBytes[store.Amount](data)
	require.NoError(t, err)
	assert.Equal(t, 0, bal.Cmp(store.NewU256(big.NewInt(400))))

	// 查询商户 escrow。
	qCall = &store.InContractCall{Name: []byte("c2c"), Method: qSel("get_merchant_escrow"), Args: [][]byte{c2cScale(t, c2cAddr(0x30))}}
	data, err = Query(module.NativeVersion, qCall, rt)
	require.NoError(t, err)
	escrow, err := store.DecodeScaleArgBytes[store.Amount](data)
	require.NoError(t, err)
	assert.Equal(t, 0, escrow.Cmp(store.NewU256(big.NewInt(600))))
}

// TestC2C_Dispatch_UnknownMethod_NoPanic 未知方法应报错而非 panic。
func TestC2C_Dispatch_UnknownMethod_NoPanic(t *testing.T) {
	rt := testutil.NewMockRuntime()
	call := &store.InContractCall{Name: []byte("c2c"), Method: store.MethodToSelectorBytes("nonexistent"), Args: nil}
	err := Mutation(module.NativeVersion, call, rt)
	assert.Error(t, err)
}

// TestC2C_Dispatch_UnknownContract_NoPanic 未知合约应静默返回空结果。
func TestC2C_Dispatch_UnknownContract_NoPanic(t *testing.T) {
	rt := testutil.NewMockRuntime()
	call := &store.InContractCall{Name: []byte("c2c-does-not-exist"), Method: store.MethodToSelectorBytes("init"), Args: nil}
	_, err := Query(module.NativeVersion, call, rt)
	require.NoError(t, err) // 未知合约返回空，不报错
}
