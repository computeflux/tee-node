package wasm

import (
	"context"
	"encoding/binary"
	"os"
	"strings"
	"testing"

	"github.com/computeflux/tee-provider/tee-chain/contracts/store"
	"github.com/stretchr/testify/require"
)

// ── mock 存储 ─────────────────────────────────────────────

type mockStoreTx struct {
	data map[string][]byte
}

func newMockStoreTx() *mockStoreTx {
	return &mockStoreTx{data: make(map[string][]byte)}
}

func (m *mockStoreTx) Get(key []byte) ([]byte, error) {
	return m.data[string(key)], nil
}

func (m *mockStoreTx) Set(key, value []byte) error {
	m.data[string(key)] = value
	return nil
}

func (m *mockStoreTx) Delete(key []byte) error {
	delete(m.data, string(key))
	return nil
}

func (m *mockStoreTx) ListByPrefix(prefix []byte) ([][]byte, [][]byte, error) {
	var keys, vals [][]byte
	for k, v := range m.data {
		if strings.HasPrefix(k, string(prefix)) {
			keys = append(keys, []byte(k))
			vals = append(vals, v)
		}
	}
	return keys, vals, nil
}

func (m *mockStoreTx) DeletekeysByPrefix(prefix []byte) error {
	for k := range m.data {
		if strings.HasPrefix(k, string(prefix)) {
			delete(m.data, k)
		}
	}
	return nil
}

func (m *mockStoreTx) Commit() error   { return nil }
func (m *mockStoreTx) Rollback() error { return nil }

// ── 测试运行时 ────────────────────────────────────────────

type testRuntime struct {
	txn    store.StoreTx
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

// ── 辅助函数 ──────────────────────────────────────────────

func loadCachedWasm(t *testing.T, e *Engine) {
	t.Helper()
	wasmBytes, err := os.ReadFile("runtime/runtime.wasm")
	require.NoError(t, err)
	ctx := context.Background()
	mod, err := e.rt.CompileModule(ctx, wasmBytes)
	require.NoError(t, err)
	e.cachedVer = 0
	e.cachedMod = mod
}

func decodeBool(t *testing.T, b []byte) bool {
	t.Helper()
	require.NotEmpty(t, b, "decodeBool: empty input")
	return b[0] != 0
}

func decodeUint32(t *testing.T, b []byte) uint32 {
	t.Helper()
	require.True(t, len(b) >= 4, "decodeUint32: need 4 bytes, got %d", len(b))
	return binary.LittleEndian.Uint32(b)
}

// queryMethod 返回查询方法的 selector（mutates=false，与生成代码中 ExecQuery 一致）。
func queryMethod(method string) []byte {
	s := store.MethodToSelectorWithMutates(method, false)
	return s[:]
}

// ── 测试合约查询（command 模式）───────────────────────────

func TestGovQuery_GetPublicJoin(t *testing.T) {
	e := NewEngine()
	loadCachedWasm(t, e)

	api := &testRuntime{
		txn:    newMockStoreTx(),
		height: 100,
		caller: store.UniAddr{T: 1, V: []byte("caller")},
		sudo:   store.UniAddr{T: 1, V: []byte("sudo")},
	}

	// 空 store，GetPublicJoin 返回默认值 false
	result, err := e.ExecuteQuery(0, &store.InContractCall{
		Name:   []byte("gov"),
		Method: queryMethod("GetPublicJoin"),
	}, api)
	require.NoError(t, err)
	require.Equal(t, false, decodeBool(t, result))
}

func TestGovQuery_DefaultTrack(t *testing.T) {
	e := NewEngine()
	loadCachedWasm(t, e)

	api := &testRuntime{
		txn:    newMockStoreTx(),
		height: 100,
		caller: store.UniAddr{T: 1, V: []byte("caller")},
		sudo:   store.UniAddr{T: 1, V: []byte("sudo")},
	}

	result, err := e.ExecuteQuery(0, &store.InContractCall{
		Name:   []byte("gov"),
		Method: queryMethod("DefaultTrack"),
	}, api)
	require.NoError(t, err)
	require.Equal(t, uint32(0), decodeUint32(t, result))
}

func TestGovQuery_TotalSupply(t *testing.T) {
	e := NewEngine()
	loadCachedWasm(t, e)

	api := &testRuntime{
		txn:    newMockStoreTx(),
		height: 100,
		caller: store.UniAddr{T: 1, V: []byte("caller")},
		sudo:   store.UniAddr{T: 1, V: []byte("sudo")},
	}

	result, err := e.ExecuteQuery(0, &store.InContractCall{
		Name:   []byte("gov"),
		Method: queryMethod("TotalSupply"),
	}, api)
	require.NoError(t, err)
	require.NotEmpty(t, result)
}

func TestGovQuery_Members(t *testing.T) {
	e := NewEngine()
	loadCachedWasm(t, e)

	api := &testRuntime{
		txn:    newMockStoreTx(),
		height: 100,
		caller: store.UniAddr{T: 1, V: []byte("caller")},
		sudo:   store.UniAddr{T: 1, V: []byte("sudo")},
	}

	result, err := e.ExecuteQuery(0, &store.InContractCall{
		Name:   []byte("gov"),
		Method: queryMethod("Members"),
	}, api)
	require.NoError(t, err)
	require.NotEmpty(t, result)
}

// ── 测试合约变更（command 模式） ────────────────────────────

func TestGovMutation_Init(t *testing.T) {
	e := NewEngine()
	loadCachedWasm(t, e)

	txn := newMockStoreTx()
	api := &testRuntime{
		txn:    txn,
		height: 100,
		caller: store.UniAddr{T: 1, V: []byte("caller")},
		sudo:   store.UniAddr{T: 1, V: []byte("sudo")},
	}

	// 调用 Init 初始化治理合约（通过 WASM Mutation）
	err := e.ExecuteMutation(0, &store.InContractCall{
		Name:   []byte("gov"),
		Method: store.MethodToSelectorBytes("Init"),
	}, api)
	require.NoError(t, err)

	// 验证 Init 确实写入了数据
	result, err := e.ExecuteQuery(0, &store.InContractCall{
		Name:   []byte("gov"),
		Method: queryMethod("GetPublicJoin"),
	}, api)
	require.NoError(t, err)
	require.Equal(t, true, decodeBool(t, result))
}

// ── 测试序列化 ─────────────────────────────────────────────

func TestSerializeCallData(t *testing.T) {
	call := &store.InContractCall{
		Name:   []byte("gateway"),
		Method: store.MethodToSelectorBytes("SettleUsage"),
		Args:   [][]byte{[]byte("hello"), []byte("world")},
	}
	data := serializeCallData(call, true)

	off := 1 // skip mode byte
	nameLen := int(data[off]) | int(data[off+1])<<8 | int(data[off+2])<<16 | int(data[off+3])<<24
	off += 4
	require.Equal(t, "gateway", string(data[off:off+nameLen]))
	off += nameLen
	require.Equal(t, store.MethodToSelectorBytes("SettleUsage"), data[off:off+4])
}

// ── 测试 WASM 模块编译+实例化 ──────────────────────────────

func TestWasmModuleInstantiation(t *testing.T) {
	e := NewEngine()
	loadCachedWasm(t, e)

	api := &testRuntime{
		txn:    newMockStoreTx(),
		height: 1,
		caller: store.UniAddr{T: 1, V: []byte("caller")},
		sudo:   store.UniAddr{T: 1, V: []byte("sudo")},
	}

	result, err := e.ExecuteQuery(0, &store.InContractCall{
		Name:   []byte("unknown"),
		Method: store.MethodToSelectorBytes("Unknown"),
	}, api)
	require.NoError(t, err)
	require.Empty(t, result)
}
