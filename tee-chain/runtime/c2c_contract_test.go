package runtime

import (
	"crypto/ed25519"
	"crypto/rand"
	"math/big"
	"testing"

	"github.com/computeflux/tee-provider/tee-chain/runtime/module"
	"github.com/computeflux/tee-provider/tee-chain/runtime/module/c2c"
	"github.com/computeflux/tee-provider/tee-chain/runtime/module/gateway"
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

	// Init（无参占位；治理统一走系统账户 / gov 合约）
	call := &store.InContractCall{Name: []byte("c2c"), Method: store.MethodToSelectorBytes("init"), Args: nil}
	require.NoError(t, Mutation(module.NativeVersion, call, rt))

	// RegisterCode / RegisterNode（系统账户 = gov 治理）
	call = &store.InContractCall{Name: []byte("c2c"), Method: store.MethodToSelectorBytes("register_code"), Args: [][]byte{c2cScale(t, code), c2cScale(t, "1.0.0")}}
	rt.System = true
	require.NoError(t, Mutation(module.NativeVersion, call, rt))

	call = &store.InContractCall{Name: []byte("c2c"), Method: store.MethodToSelectorBytes("register_node"), Args: [][]byte{c2cScale(t, c2cAddr(0x10)), c2cScale(t, code)}}
	require.NoError(t, Mutation(module.NativeVersion, call, rt))
	rt.System = false

	_ = gov

	// TEE 证明：代码密钥对放款数据签名，验证节点（mock）返回 TEE 证明的代码三要素。
	_, codePriv, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	codePub := codePriv.Public().(ed25519.PublicKey)
	rt.VerifyTEEFunc = func([]byte) store.Result[store.TeeVerifyInfo, string] {
		return store.NewOk[store.TeeVerifyInfo, string](store.TeeVerifyInfo{
			TeeType:       1,
			CodeSigner:    codePub,
			CodeSignature: code,
		})
	}

	// Deposit
	call = &store.InContractCall{Name: []byte("c2c"), Method: store.MethodToSelectorBytes("deposit"), Args: [][]byte{c2cScale(t, store.NewU256(big.NewInt(1000)))}}
	rt.Caller = c2cAddr(0x30)
	require.NoError(t, Mutation(module.NativeVersion, call, rt))

	// Release（携带 TEE 证明：代码密钥对放款数据 SCALE 编码签名）
	rel := c2c.Release{
		Merchant:  c2cAddr(0x30),
		Buyer:     c2cAddr(0x40),
		Amount:    store.NewU256(big.NewInt(400)),
		OrderNo:   []byte("order-1"),
		FiatCents: 40000,
		Currency:  "USD",
	}
	codeSig := ed25519.Sign(codePriv, c2cScale(t, rel))
	call = &store.InContractCall{Name: []byte("c2c"), Method: store.MethodToSelectorBytes("release"), Args: [][]byte{c2cScale(t, c2c.ReleaseWithSig{
		Data:          rel,
		TeeReport:     []byte("tee-report"),
		CodeSignature: codeSig,
	})}}
	rt.Caller = c2cAddr(0x10)
	require.NoError(t, Mutation(module.NativeVersion, call, rt))

	// 查询买方 credits：Release 后放款直接进 gateway credits（包内跨合约调用），
	// c2c 不再维护 buyerBalance。
	gw := gateway.NewGateway(rt)
	cred, err := gateway.GatewayQuery{Gateway: *gw}.GetCredit(c2cAddr(0x40))
	require.NoError(t, err)
	assert.Equal(t, int64(400), cred)

	// 查询商户 escrow。
	qCall := &store.InContractCall{Name: []byte("c2c"), Method: qSel("get_merchant_escrow"), Args: [][]byte{c2cScale(t, c2cAddr(0x30))}}
	data, err := Query(module.NativeVersion, qCall, rt)
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
