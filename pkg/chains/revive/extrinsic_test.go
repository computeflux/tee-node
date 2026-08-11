package revive

import (
	"math/big"
	"testing"

	"github.com/centrifuge/go-substrate-rpc-client/v4/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/computeflux/tee-provider/pkg/types/chaintypes"
	crypto "github.com/computeflux/tee-provider/pkg/types/crypto"
	"github.com/computeflux/tee-provider/tee-chain/runtime/module/batch"
	"github.com/computeflux/tee-provider/tee-chain/runtime/module/gateway"
	"github.com/computeflux/tee-provider/tee-chain/runtime/store"
)

// ── BatchCall ──────────────────────────────────────────────

func TestContract_BatchCall_InvalidMethod(t *testing.T) {
	c := &Contract{}
	_, err := c.BatchCall("invalid", nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not in batchMethods")
}

func TestContract_BatchCall_ValidMethods(t *testing.T) {
	validMethods := []string{"batch", "batch_all", "force_batch"}
	for _, m := range validMethods {
		t.Run(m, func(t *testing.T) {
			c := &Contract{}
			assert.NotPanics(t, func() {
				defer func() {
					_ = recover()
				}()
				_, _ = c.BatchCall(m, []chaintypes.ChainCall{})
			})
		})
	}
}

// ── newWithdrawCallParams（OCR 修复：Contract==nil 走原生提取，否则 ERC20）──

func testWithdrawApply(contract []byte) *gateway.WithdrawApply {
	return &gateway.WithdrawApply{
		ChainID:  1,
		Contract: contract,
		User:     store.UniAddr{T: 1, V: make([]byte, 20)},
		Credit:   store.U256{Int: big.NewInt(100)},
		Nonce:    42,
	}
}

func TestNewWithdrawCallParams_NilContract_UsesNativeWithdraw(t *testing.T) {
	// 回归：Contract == nil 必须走原生提现（CallOfWithdraw），
	// 而不是旧的错误分支（对 nil 做 toH160 会报 invalid H160 length）。
	p, err := newWithdrawCallParams(testWithdrawApply(nil))
	require.NoError(t, err)
	require.NotNil(t, p)
	assert.False(t, p.erc20, "nil contract 应走原生提现分支")
	assert.Equal(t, uint64(42), p.nonce)
	assert.Equal(t, types.NewU256(*big.NewInt(100)), p.points)
}

func TestNewWithdrawCallParams_WithContract_UsesErc20(t *testing.T) {
	contract := make([]byte, 20)
	for i := range contract {
		contract[i] = byte(i)
	}
	p, err := newWithdrawCallParams(testWithdrawApply(contract))
	require.NoError(t, err)
	require.NotNil(t, p)
	assert.True(t, p.erc20, "有 contract 应走 ERC20 分支")
	assert.Equal(t, contract, p.token[:], "ERC20 分支必须携带合约地址")
	assert.Equal(t, uint64(42), p.nonce)
}

func TestNewWithdrawCallParams_InvalidContract_Error(t *testing.T) {
	_, err := newWithdrawCallParams(testWithdrawApply(make([]byte, 19)))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "withdraw apply contract address")
}

func TestNewWithdrawCallParams_InvalidUser_Error(t *testing.T) {
	wa := testWithdrawApply(nil)
	wa.User.V = make([]byte, 19)
	_, err := newWithdrawCallParams(wa)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "withdraw apply user address")
}

// ── ChainId（OCR 修复：nil 检查）──────────────────────────

func TestContract_ChainId(t *testing.T) {
	c := &Contract{config: &chaintypes.ChainConfigType{ChainID: 7}}
	assert.Equal(t, uint32(7), c.ChainId())
}

func TestContract_ChainId_NilConfig(t *testing.T) {
	// 回归：config 为 nil 时不应 panic，返回 0。
	assert.NotPanics(t, func() {
		assert.Equal(t, uint32(0), (&Contract{}).ChainId())
	})
	assert.Equal(t, uint32(0), (*Contract)(nil).ChainId())
}

// ── TEECallToCall ──────────────────────────────────────────

func TestToH160(t *testing.T) {
	// 20 字节合法地址
	addr := make([]byte, 20)
	for i := range addr {
		addr[i] = byte(i)
	}
	h160, err := toH160(addr)
	require.NoError(t, err)
	assert.Equal(t, addr, h160[:])

	t.Run("too_short", func(t *testing.T) {
		_, err := toH160(make([]byte, 19))
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid H160 length")
	})

	t.Run("too_long", func(t *testing.T) {
		_, err := toH160(make([]byte, 21))
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid H160 length")
	})

	t.Run("empty", func(t *testing.T) {
		_, err := toH160(nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid H160 length")
	})
}

func TestContract_TEECallToCall(t *testing.T) {
	c := &Contract{}
	// 传合法的 32 字节 dkgKey 与未知 CallType，应走到 "invalid tee call" 分支
	call, err := c.TEECallToCall(&batch.HubCallIndex{CallType: 1}, make([]byte, 32))
	assert.ErrorContains(t, err, "invalid tee call")
	assert.Nil(t, call)
}

// ── CallOfMapAccount ───────────────────────────────────────

func TestContract_CallOfMapAccount(t *testing.T) {
	c := &Contract{}
	call, err := c.CallOfMapAccount(crypto.PubKey{})
	assert.ErrorContains(t, err, "not implemented")
	assert.Nil(t, call)
}

// ── GetChainUrls ───────────────────────────────────────────

func TestContract_GetChainUrls(t *testing.T) {
	urls := []string{"ws://localhost:9944", "ws://localhost:9945"}
	c := &Contract{config: &chaintypes.ChainConfigType{Urls: urls}}
	assert.Equal(t, urls, c.GetChainUrls())
}

func TestContract_GetChainUrls_NilConfig(t *testing.T) {
	c := &Contract{}
	assert.Nil(t, c.GetChainUrls())
}

// ── CheckMapAccount ────────────────────────────────────────

func TestContract_CheckMapAccount_NoClient(t *testing.T) {
	c := &Contract{}
	assert.NotPanics(t, func() {
		defer func() { _ = recover() }()
		_, _ = c.CheckMapAccount(crypto.PubKey{})
	})
}
