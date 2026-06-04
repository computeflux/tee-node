package evm

import (
	"errors"
	"fmt"
	"testing"

	"github.com/centrifuge/go-substrate-rpc-client/v4/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/computeflux/tee-provider/pkg/chains/evm/bindings"
	"github.com/computeflux/tee-provider/pkg/model"
)

// ── TEECallToCall ──────────────────────────────────────────

func TestEvmChain_TEECallToCall(t *testing.T) {
	e := &EvmChain{}
	call, err := e.TEECallToCall(&model.TeeCall{}, nil)
	require.NoError(t, err)
	assert.Equal(t, "tee_attest", call.EVM.Method)
}

// ── TxCallOfSetNextEpoch ───────────────────────────────────

func TestEvmChain_TxCallOfSetNextEpoch(t *testing.T) {
	e := &EvmChain{}
	call, err := e.TxCallOfSetNextEpoch(42, model.PubKey{})
	require.NoError(t, err)
	assert.Equal(t, "setNextEpoch", call.EVM.Method)
	assert.NotEmpty(t, call.EVM.Data)
}

// ── BatchCall ──────────────────────────────────────────────

func TestEvmChain_BatchCall(t *testing.T) {
	e := &EvmChain{}
	calls := []model.ChainCall{
		{EVM: model.EvmCall{Data: []byte{0x01}}},
		{EVM: model.EvmCall{Data: []byte{0x02}}},
	}
	call, err := e.BatchCall("batch", calls)
	require.NoError(t, err)
	assert.Equal(t, "batch", call.EVM.Method)
	assert.NotEmpty(t, call.EVM.Data)
}

func TestEvmChain_BatchCall_Empty(t *testing.T) {
	e := &EvmChain{}
	call, err := e.BatchCall("batch_all", []model.ChainCall{})
	require.NoError(t, err)
	assert.NotEmpty(t, call.EVM.Data)
}

// ── CallOfMapAccount ───────────────────────────────────────

func TestEvmChain_CallOfMapAccount(t *testing.T) {
	e := &EvmChain{}
	call, err := e.CallOfMapAccount(model.PubKey{})
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
	assert.NotEqual(t, model.PubKey{}, vals[0].ValidatorIdBLS)

	// second node has invalid BLS length
	assert.Equal(t, model.PubKey{}, vals[1].ValidatorIdBLS)
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

func (m *mockNonBLSSigner) Public() []byte                       { return nil }
func (m *mockNonBLSSigner) AccountID() types.AccountID             { return types.AccountID{} }
func (m *mockNonBLSSigner) Sign([]byte) ([]byte, error)            { return nil, nil }
func (m *mockNonBLSSigner) Verify([]byte, []byte) bool             { return false }
func (m *mockNonBLSSigner) SignType() uint8                        { return 0 }

// mockWrongSuiteSigner implements blsEIP2537Signer but with wrong suite
type mockWrongSuiteSigner struct {
	mockNonBLSSigner
}

func (m *mockWrongSuiteSigner) Suite() model.SuiteType { return model.SuiteEd25519 }
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
		cipher:     model.SuiteEd25519,
		urls:       []string{"http://localhost"},
		signerAddr: "0x1234",
	}
	assert.Equal(t, "evm", e.ChainType())
	assert.Equal(t, model.SuiteEd25519, e.GetCipher())
	assert.Equal(t, []string{"http://localhost"}, e.GetChainUrls())
	assert.Equal(t, "0x1234", e.GetSignerAddress())
	assert.Equal(t, uint32(1), e.ChainId())
}

// ── CheckMapAccount ────────────────────────────────────────

func TestEvmChain_CheckMapAccount(t *testing.T) {
	e := &EvmChain{}
	ok, err := e.CheckMapAccount(model.PubKey{})
	require.NoError(t, err)
	assert.True(t, ok)
}

// ── GetLatestNonce / GetRelayEvents ────────────────────────

func TestEvmChain_GetLatestNonce(t *testing.T) {
	e := &EvmChain{}
	nonce, err := e.GetLatestNonce()
	require.NoError(t, err)
	assert.Equal(t, uint64(0), nonce)
}

func TestEvmChain_GetRelayEvents(t *testing.T) {
	e := &EvmChain{}
	events, err := e.GetRelayEvents(0, 100)
	require.NoError(t, err)
	assert.Nil(t, events)
}
