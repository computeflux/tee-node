package evm

import (
	"encoding/hex"
	"errors"
	"fmt"
	"math/big"
	"testing"

	"github.com/centrifuge/go-substrate-rpc-client/v4/types"
	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/computeflux/tee-provider/pkg/chains/evm/bindings"
	"github.com/computeflux/tee-provider/pkg/types/chaintypes"
	crypto "github.com/computeflux/tee-provider/pkg/types/crypto"
	"github.com/computeflux/tee-provider/tee-chain/runtime/module/batch"
	"github.com/computeflux/tee-provider/tee-chain/runtime/module/gateway"
	"github.com/computeflux/tee-provider/tee-chain/runtime/store"
)

// ── TEECallToCall ──────────────────────────────────────────

func TestEvmChain_TEECallToCall_UnsupportedType(t *testing.T) {
	e := &EvmChain{}
	call, err := e.TEECallToCall(&batch.HubCallIndex{CallType: 1}, nil)
	assert.ErrorContains(t, err, "unsupported hub call type")
	assert.Nil(t, call)
}

func TestBuildWithdrawCall_Native(t *testing.T) {
	wa := &gateway.WithdrawApply{
		User:   store.UniAddr{T: 1, V: make([]byte, 20)},
		Credit: store.U256{Int: big.NewInt(100)},
		Nonce:  42,
	}

	call, err := buildWithdrawCall(wa)
	require.NoError(t, err)
	assert.Equal(t, "withdraw", call.EVM.Method)

	abiInst, err := bindings.TokenMetaData.GetAbi()
	require.NoError(t, err)
	args, err := abiInst.Methods["withdraw"].Inputs.Unpack(call.EVM.Data[4:])
	require.NoError(t, err)
	require.Len(t, args, 3)
	assert.Equal(t, common.BytesToAddress(wa.User.V), args[0])
	assert.Equal(t, int64(100), args[1].(*big.Int).Int64())
	assert.Equal(t, uint64(42), args[2])
}

func TestBuildWithdrawCall_ERC20(t *testing.T) {
	tokenAddr := make([]byte, 20)
	tokenAddr[0] = 0xAA
	wa := &gateway.WithdrawApply{
		Contract: tokenAddr,
		User:     store.UniAddr{T: 1, V: make([]byte, 20)},
		Credit:   store.U256{Int: big.NewInt(7)},
		Nonce:    9,
	}

	call, err := buildWithdrawCall(wa)
	require.NoError(t, err)
	assert.Equal(t, "withdrawErc20", call.EVM.Method)

	abiInst, err := bindings.TokenMetaData.GetAbi()
	require.NoError(t, err)
	args, err := abiInst.Methods["withdrawErc20"].Inputs.Unpack(call.EVM.Data[4:])
	require.NoError(t, err)
	require.Len(t, args, 4)
	assert.Equal(t, common.BytesToAddress(tokenAddr), args[0])
	assert.Equal(t, common.BytesToAddress(wa.User.V), args[1])
	assert.Equal(t, int64(7), args[2].(*big.Int).Int64())
	assert.Equal(t, uint64(9), args[3])
}

func TestBuildWithdrawCall_InvalidAddresses(t *testing.T) {
	_, err := buildWithdrawCall(&gateway.WithdrawApply{
		User:   store.UniAddr{V: make([]byte, 19)},
		Credit: store.U256{Int: big.NewInt(1)},
	})
	assert.ErrorContains(t, err, "user")

	_, err = buildWithdrawCall(&gateway.WithdrawApply{
		Contract: make([]byte, 19),
		User:     store.UniAddr{V: make([]byte, 20)},
		Credit:   store.U256{Int: big.NewInt(1)},
	})
	assert.ErrorContains(t, err, "token")
}

func TestBuildWithdrawCall_NativeMarker(t *testing.T) {
	// The gateway stores the token symbol for native withdrawals ("ETH" is
	// what the frontend sends); it must not be treated as an ERC20 address.
	for _, marker := range []string{"ETH", "eth", "native", "0x0"} {
		wa := &gateway.WithdrawApply{
			Contract: []byte(marker),
			User:     store.UniAddr{T: 2, V: make([]byte, 20)},
			Credit:   store.U256{Int: big.NewInt(5)},
			Nonce:    3,
		}
		call, err := buildWithdrawCall(wa)
		require.NoError(t, err, marker)
		assert.Equal(t, "withdraw", call.EVM.Method, marker)
		if call.EVM.Method != "withdraw" {
			continue
		}
		assert.NotContains(t, string(call.EVM.Data[:4]), "withdrawErc20")
	}
}

func TestBuildWithdrawCall_Nil(t *testing.T) {
	_, err := buildWithdrawCall(nil)
	assert.Error(t, err)

	_, err = buildWithdrawCall(&gateway.WithdrawApply{
		User: store.UniAddr{V: make([]byte, 20)},
	})
	assert.ErrorContains(t, err, "nil credit")
}

// ── TxCallOfSetNextEpoch ───────────────────────────────────

func TestEvmChain_TxCallOfSetNextEpoch(t *testing.T) {
	e := &EvmChain{}
	call, err := e.TxCallOfSetNextEpoch(42, crypto.PubKey{})
	require.NoError(t, err)
	assert.Equal(t, "setNextEpoch", call.EVM.Method)
	assert.NotEmpty(t, call.EVM.Data)
}

// ── BatchCall ──────────────────────────────────────────────

func TestEvmChain_BatchCall(t *testing.T) {
	e := &EvmChain{}
	calls := []chaintypes.ChainCall{
		{EVM: chaintypes.EvmCall{Data: []byte{0x01}}},
		{EVM: chaintypes.EvmCall{Data: []byte{0x02}}},
	}
	call, err := e.BatchCall("batch", calls)
	require.NoError(t, err)
	assert.Equal(t, "batch", call.EVM.Method)
	assert.NotEmpty(t, call.EVM.Data)
}

func TestEvmChain_BatchCall_Empty(t *testing.T) {
	e := &EvmChain{}
	call, err := e.BatchCall("batch_all", []chaintypes.ChainCall{})
	require.NoError(t, err)
	assert.NotEmpty(t, call.EVM.Data)
}

// ── CallOfMapAccount ───────────────────────────────────────

func TestEvmChain_CallOfMapAccount(t *testing.T) {
	e := &EvmChain{}
	call, err := e.CallOfMapAccount(crypto.PubKey{})
	// mapAccount method is not present in the Subnet ABI
	assert.Error(t, err)
	assert.Nil(t, call)
}

// ── extractRevertData / isHex ──────────────────────────────

func TestExtractRevertData(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want []byte
	}{
		{"nil", nil, nil},
		{"no hex", errors.New("some error"), nil},
		{"with 0x prefix", errors.New("execution reverted: 0x08c379a0"), []byte{0x08, 0xc3, 0x79, 0xa0}},
		{"wrapped error", fmt.Errorf("outer: %w", errors.New("inner 0x1234abcd")), []byte{0x12, 0x34, 0xab, 0xcd}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractRevertData(tt.err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestIsHex(t *testing.T) {
	assert.True(t, isHex('0'))
	assert.True(t, isHex('9'))
	assert.True(t, isHex('a'))
	assert.True(t, isHex('f'))
	assert.True(t, isHex('A'))
	assert.True(t, isHex('F'))
	assert.False(t, isHex('g'))
	assert.False(t, isHex(' '))
	assert.False(t, isHex('-'))
}

// mockDataError mirrors go-ethereum's rpc.jsonError: the message is a plain
// "execution reverted" while the revert payload lives in ErrorData().
type mockDataError struct {
	msg  string
	data interface{}
}

func (e *mockDataError) Error() string          { return e.msg }
func (e *mockDataError) ErrorData() interface{} { return e.data }

func TestExtractRevertData_DataError(t *testing.T) {
	selector := []byte{0x10, 0xe4, 0x16, 0xaa}
	tests := []struct {
		name string
		err  error
		want []byte
	}{
		{"string data", &mockDataError{msg: "execution reverted", data: "0x10e416aa"}, selector},
		{"bytes data", &mockDataError{msg: "execution reverted", data: selector}, selector},
		{"hexutil data", &mockDataError{msg: "execution reverted", data: hexutil.Bytes(selector)}, selector},
		{"nested data", &mockDataError{msg: "execution reverted", data: map[string]interface{}{"data": "0x10e416aa"}}, selector},
		{"wrapped", fmt.Errorf("estimate: %w", &mockDataError{msg: "execution reverted", data: "0x10e416aa"}), selector},
		{"empty data", &mockDataError{msg: "execution reverted", data: ""}, nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, extractRevertData(tt.err))
		})
	}
}

func TestRevertReason_GatewayCustomError(t *testing.T) {
	abiInst, err := bindings.BaseBLSMetaData.GetAbi()
	require.NoError(t, err)

	errDef := abiInst.Errors["BLSSignatureInvalid"]
	revertErr := &mockDataError{
		msg:  "execution reverted",
		data: "0x" + hex.EncodeToString(errDef.ID[:4]),
	}

	assert.Equal(t, "reverted: BLSSignatureInvalid", revertReason(revertErr, ethereum.CallMsg{}, nil, abiInst))
}

func TestRevertReason_ErrorString(t *testing.T) {
	strType, err := abi.NewType("string", "", nil)
	require.NoError(t, err)
	payload, err := abi.Arguments{{Type: strType}}.Pack("boom")
	require.NoError(t, err)
	raw := append([]byte{0x08, 0xc3, 0x79, 0xa0}, payload...)

	revertErr := &mockDataError{msg: "execution reverted", data: hexutil.Bytes(raw)}
	assert.Equal(t, "reverted: boom", revertReason(revertErr, ethereum.CallMsg{}, nil, nil))
}

func TestRevertReason_NoData(t *testing.T) {
	abiInst, err := bindings.BaseBLSMetaData.GetAbi()
	require.NoError(t, err)

	assert.Equal(t, "", revertReason(errors.New("execution reverted"), ethereum.CallMsg{}, nil, abiInst))
}

// ── buildValidators ────────────────────────────────────────

func TestBuildValidators(t *testing.T) {
	ids := []uint64{1, 2}
	nodes := []bindings.SecretNode{
		{
			Id:          1,
			ValidatorId: [32]byte{1},
			P2pId:       [32]byte{2},
			Bls:         make([]byte, 96),
		},
		{
			Id:          2,
			ValidatorId: [32]byte{3},
			P2pId:       [32]byte{4},
			Bls:         []byte{0x01}, // invalid length, should be skipped
		},
	}
	// fill bls with non-zero for first node
	for i := range nodes[0].Bls {
		nodes[0].Bls[i] = byte(i)
	}

	vals := buildValidators(ids, nodes)
	require.Len(t, vals, 2)
	assert.Equal(t, uint64(1), vals[0].NodeID)
	assert.Equal(t, []byte{1}, vals[0].ValidatorId.Byte()[:1])
	assert.Equal(t, []byte{2}, vals[0].P2pId.Byte()[:1])
	assert.NotEqual(t, crypto.PubKey{}, vals[0].ValidatorIdBLS)

	// second node has invalid BLS length
	assert.Equal(t, crypto.PubKey{}, vals[1].ValidatorIdBLS)
}

func TestBuildValidators_Empty(t *testing.T) {
	vals := buildValidators([]uint64{}, []bindings.SecretNode{})
	assert.Empty(t, vals)
}

func TestBuildValidators_MoreIDsThanNodes(t *testing.T) {
	ids := []uint64{1, 2, 3}
	nodes := []bindings.SecretNode{
		{Id: 1, ValidatorId: [32]byte{1}, P2pId: [32]byte{2}},
	}
	vals := buildValidators(ids, nodes)
	require.Len(t, vals, 3)
	assert.Equal(t, uint64(1), vals[0].NodeID)
	assert.Equal(t, uint64(2), vals[1].NodeID)
	assert.Equal(t, make([]byte, 32), vals[1].ValidatorId.Byte())
}

// ── getBLSSigner ───────────────────────────────────────────

// mockNonBLSSigner implements chain.SignerType but NOT blsEIP2537Signer
type mockNonBLSSigner struct{}

func (m *mockNonBLSSigner) Public() []byte              { return nil }
func (m *mockNonBLSSigner) AccountID() types.AccountID  { return types.AccountID{} }
func (m *mockNonBLSSigner) Sign([]byte) ([]byte, error) { return nil, nil }
func (m *mockNonBLSSigner) Verify([]byte, []byte) bool  { return false }
func (m *mockNonBLSSigner) SignType() uint8             { return 0 }

// mockWrongSuiteSigner implements blsEIP2537Signer but with wrong suite
type mockWrongSuiteSigner struct {
	mockNonBLSSigner
}

func (m *mockWrongSuiteSigner) Suite() crypto.SuiteType { return crypto.SuiteEd25519 }
func (m *mockWrongSuiteSigner) BLSSignEIP2537(msg []byte) (signature []byte, negSigEIP []byte, hmG1EIP []byte, err error) {
	return nil, nil, nil, nil
}

func TestEvmChain_GetBLSSigner_NonBLS(t *testing.T) {
	e := &EvmChain{}
	_, err := e.getBLSSigner(&mockNonBLSSigner{})
	assert.ErrorContains(t, err, "does not support BLS EIP-2537")
}

func TestEvmChain_GetBLSSigner_WrongSuite(t *testing.T) {
	e := &EvmChain{}
	_, err := e.getBLSSigner(&mockWrongSuiteSigner{})
	assert.ErrorContains(t, err, "signer suite is not BLS12381")
}

// ── Identity methods ───────────────────────────────────────

func TestEvmChain_Identity(t *testing.T) {
	e := &EvmChain{
		chainId:    1,
		cipher:     crypto.SuiteEd25519,
		urls:       []string{"http://localhost"},
		signerAddr: "0x1234",
	}
	assert.Equal(t, "evm", e.ChainType())
	assert.Equal(t, crypto.SuiteEd25519, e.GetCipher())
	assert.Equal(t, []string{"http://localhost"}, e.GetChainUrls())
	assert.Equal(t, "0x1234", e.GetSignerAddress())
	assert.Equal(t, uint32(1), e.ChainId())
}

// ── CheckMapAccount ────────────────────────────────────────────

func TestEvmChain_CheckMapAccount(t *testing.T) {
	e := &EvmChain{}
	ok, err := e.CheckMapAccount(crypto.PubKey{})
	require.NoError(t, err)
	assert.True(t, ok)
}
