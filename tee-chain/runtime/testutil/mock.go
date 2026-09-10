// Package testutil provides shared test helpers for contract modules.
package testutil

import (
	"strings"

	"github.com/computeflux/tee-provider/tee-chain/runtime/store"
)

// MockStoreTx is an in-memory implementation of store.StoreTx.
type MockStoreTx struct {
	Data map[string][]byte
}

func NewMockStoreTx() *MockStoreTx {
	return &MockStoreTx{Data: make(map[string][]byte)}
}

func (m *MockStoreTx) Get(key []byte) ([]byte, error) {
	return m.Data[string(key)], nil
}

func (m *MockStoreTx) Set(key, value []byte) error {
	m.Data[string(key)] = value
	return nil
}

func (m *MockStoreTx) Delete(key []byte) error {
	delete(m.Data, string(key))
	return nil
}

func (m *MockStoreTx) ListByPrefix(prefix []byte) ([][]byte, [][]byte, error) {
	var keys, vals [][]byte
	for k, v := range m.Data {
		if strings.HasPrefix(k, string(prefix)) {
			keys = append(keys, []byte(k))
			vals = append(vals, v)
		}
	}
	return keys, vals, nil
}

func (m *MockStoreTx) DeletekeysByPrefix(prefix []byte) error {
	for k := range m.Data {
		if strings.HasPrefix(k, string(prefix)) {
			delete(m.Data, k)
		}
	}
	return nil
}

func (m *MockStoreTx) Commit() error   { return nil }
func (m *MockStoreTx) Rollback() error { return nil }

// MockRuntime is a configurable implementation of store.ContractApi.
type MockRuntime struct {
	Txn           store.StoreTx
	Height        int64
	Time          int64
	Caller        store.UniAddr
	Sudo          store.UniAddr
	System        bool
	CallFunc      func(store.UniAddr, store.ContractCall) ([]byte, error)
	VerifyTEEFunc func([]byte) error
}

var _ store.ContractApi = (*MockRuntime)(nil)

func (r *MockRuntime) GetHeight() int64                { return r.Height }
func (r *MockRuntime) GetBlockTime() int64             { return r.Time }
func (r *MockRuntime) GetVersion() uint32              { return 0 }
func (r *MockRuntime) GetTxn() store.StoreTx           { return r.Txn }
func (r *MockRuntime) GetCaller() store.UniAddr        { return r.Caller }
func (r *MockRuntime) GetSystemAccount() store.UniAddr { return r.Sudo }
func (r *MockRuntime) CallbySystem() bool              { return r.System }
func (r *MockRuntime) Query(store.UniAddr, store.ContractCall) ([]byte, error) {
	return nil, nil
}
func (r *MockRuntime) Call(target store.UniAddr, call store.ContractCall) ([]byte, error) {
	if r.CallFunc != nil {
		return r.CallFunc(target, call)
	}
	return nil, nil
}
func (r *MockRuntime) VerifyTEE(report []byte) error {
	if r.VerifyTEEFunc != nil {
		return r.VerifyTEEFunc(report)
	}
	return nil
}

// NewMockRuntime creates a MockRuntime with defaults.
func NewMockRuntime() *MockRuntime {
	return &MockRuntime{
		Txn:    NewMockStoreTx(),
		Height: 1,
		Time:   1000,
		Caller: store.UniAddr{T: 1, V: []byte{0x01}},
		Sudo:   store.UniAddr{T: 1, V: []byte{0x99}},
		System: false,
	}
}
