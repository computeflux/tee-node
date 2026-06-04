package batch

import (
	"errors"
	"testing"

	"github.com/computeflux/tee-provider/tee-chain/contracts/store"
	"github.com/computeflux/tee-provider/tee-chain/contracts/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestBatch() (*Batch, *testutil.MockRuntime) {
	rt := testutil.NewMockRuntime()
	return NewBatch(rt), rt
}

func TestBatch_ExecuteBatch_Success(t *testing.T) {
	batch, rt := newTestBatch()
	callCount := 0
	rt.CallFunc = func(target store.UniAddr, call store.ContractCall) ([]byte, error) {
		callCount++
		return nil, nil
	}

	calls := []BatchCall{
		{Name: []byte("auth"), Method: []byte{0x01, 0x02, 0x03, 0x04}},
		{Name: []byte("gateway"), Method: []byte{0x05, 0x06, 0x07, 0x08}},
	}

	err := BatchMutation{*batch}.ExecuteBatch(calls)
	require.NoError(t, err)
	assert.Equal(t, 2, callCount)
}

func TestBatch_ExecuteBatch_EmptyName(t *testing.T) {
	batch, rt := newTestBatch()
	rt.CallFunc = func(store.UniAddr, store.ContractCall) ([]byte, error) {
		return nil, nil
	}

	calls := []BatchCall{
		{Name: []byte{}, Method: []byte{0x01, 0x02, 0x03, 0x04}},
	}

	err := BatchMutation{*batch}.ExecuteBatch(calls)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "empty contract name")
}

func TestBatch_ExecuteBatch_EmptyMethod(t *testing.T) {
	batch, rt := newTestBatch()
	rt.CallFunc = func(store.UniAddr, store.ContractCall) ([]byte, error) {
		return nil, nil
	}

	calls := []BatchCall{
		{Name: []byte("auth"), Method: []byte{}},
	}

	err := BatchMutation{*batch}.ExecuteBatch(calls)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "empty method selector")
}

func TestBatch_ExecuteBatch_SubcallError(t *testing.T) {
	batch, rt := newTestBatch()
	callCount := 0
	rt.CallFunc = func(store.UniAddr, store.ContractCall) ([]byte, error) {
		callCount++
		if callCount == 2 {
			return nil, errors.New("subcall failed")
		}
		return nil, nil
	}

	calls := []BatchCall{
		{Name: []byte("auth"), Method: []byte{0x01}},
		{Name: []byte("gateway"), Method: []byte{0x02}},
		{Name: []byte("gov"), Method: []byte{0x03}},
	}

	err := BatchMutation{*batch}.ExecuteBatch(calls)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "subcall failed")
	assert.Equal(t, 2, callCount, "should fail fast on second call")
}

func TestBatch_ExecuteBatch_EmptyCalls(t *testing.T) {
	batch, _ := newTestBatch()

	err := BatchMutation{*batch}.ExecuteBatch([]BatchCall{})
	require.NoError(t, err)
}

func TestBatchQuery_Status(t *testing.T) {
	batch, _ := newTestBatch()
	status, err := BatchQuery{*batch}.Status(0)
	require.NoError(t, err)
	assert.Equal(t, "ok", status)
}

func TestBatchCall_ToContractCall(t *testing.T) {
	bc := BatchCall{
		Name:   []byte("auth"),
		Method: []byte{0x01, 0x02, 0x03, 0x04},
		Args:   [][]byte{{0xAA, 0xBB}},
	}
	cc := bc.ToContractCall()
	assert.Equal(t, bc.Name, cc.GetName())
	assert.Equal(t, bc.Method, cc.GetMethod())
	assert.Equal(t, bc.Args, cc.GetArgs())
}
