package wasm

import (
	"testing"

	"github.com/computeflux/tee-provider/pkg/model"
	"github.com/computeflux/tee-provider/tee-chain/contracts/store"
	"github.com/stretchr/testify/require"
)

type testRuntime struct {
	txn    *model.Txn
	height int64
	caller store.UniAddr
	sudo   store.UniAddr
}

var _ store.ContractApi = (*testRuntime)(nil)

func (r *testRuntime) GetHeight() int64                                        { return r.height }
func (r *testRuntime) GetBlockTime() int64                                     { return 0 }
func (r *testRuntime) GetVersion() uint32                                      { return 1 }
func (r *testRuntime) GetTxn() store.StoreTx                                   { return r.txn }
func (r *testRuntime) GetCaller() store.UniAddr                                { return r.caller }
func (r *testRuntime) GetSystemAccount() store.UniAddr                         { return r.sudo }
func (r *testRuntime) CallbySystem() bool                                      { return false }
func (r *testRuntime) Query(store.UniAddr, store.ContractCall) ([]byte, error) { return nil, nil }
func (r *testRuntime) Call(store.UniAddr, store.ContractCall) ([]byte, error)  { return nil, nil }

func TestEngineExecute_Echo(t *testing.T) {
	// TODO: requires pebble DB setup — skip for now
	t.Skip("requires DB setup")
}

func TestEngineExecute_TinyGoRuntime(t *testing.T) {
	t.Skip("requires runtime.wasm build")
}

func TestSerializeCallData_Roundtrip(t *testing.T) {
	call := &store.InContractCall{
		Name:   []byte("gateway"),
		Method: store.MethodToSelectorBytes("SettleUsage"),
		Args:   [][]byte{[]byte("hello"), []byte("world")},
	}
	data := serializeCallData(call)

	off := 0
	nameLen := int(data[0]) | int(data[1])<<8 | int(data[2])<<16 | int(data[3])<<24
	off += 4
	require.Equal(t, "gateway", string(data[off:off+nameLen]))
	off += nameLen
	require.Equal(t, store.MethodToSelectorBytes("SettleUsage"), data[off:off+4])
}
