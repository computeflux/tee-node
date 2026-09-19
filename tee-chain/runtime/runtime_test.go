package runtime

import (
	"testing"
	"time"

	dbstore "github.com/computeflux/tee-provider/pkg/types"
	"github.com/computeflux/tee-provider/tee-chain/runtime/module"
	"github.com/computeflux/tee-provider/tee-chain/runtime/store"
	"github.com/computeflux/tee-provider/tee-chain/runtime/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newTestTxn 创建真实 DB 事务（NewRuntime 需要具体的 *dbstore.Txn 类型）。
func newTestTxn(t *testing.T) *dbstore.Txn {
	t.Helper()
	db, err := dbstore.NewDBWithPath(t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	return db.NewTransaction()
}

func TestRuntimeGetters(t *testing.T) {
	ts := time.Unix(1700000000, 0)
	caller := store.UniAddr{T: 1, V: []byte{0x01}}
	sudo := store.UniAddr{T: 1, V: []byte{0x02}}
	txn := newTestTxn(t)

	r := NewRuntime(42, ts, txn, caller, sudo, nil)

	assert.Equal(t, int64(42), r.GetHeight())
	assert.Equal(t, int64(1700000000), r.GetBlockTime())
	assert.Equal(t, uint32(0), r.GetVersion())
	assert.Equal(t, caller, r.GetCaller())
	assert.Equal(t, sudo, r.GetSystemAccount())
	assert.NotNil(t, r.GetTxn())
}

func TestNewRuntimeWithVersion(t *testing.T) {
	txn := newTestTxn(t)
	r := NewRuntimeWithVersion(10, time.Unix(100, 0), txn, store.UniAddr{}, store.UniAddr{}, 18, nil)
	assert.Equal(t, uint32(18), r.GetVersion())
}

func TestCallbySystem(t *testing.T) {
	sudo := store.UniAddr{T: 2, V: []byte{0xaa}}
	txn := newTestTxn(t)

	t.Run("matching", func(t *testing.T) {
		r := NewRuntimeWithVersion(0, time.Now(), txn, sudo, sudo, 0, nil)
		assert.True(t, r.CallbySystem())
	})

	t.Run("different_value", func(t *testing.T) {
		r := NewRuntimeWithVersion(0, time.Now(), txn, store.UniAddr{T: 2, V: []byte{0xbb}}, sudo, 0, nil)
		assert.False(t, r.CallbySystem())
	})

	t.Run("different_type", func(t *testing.T) {
		r := NewRuntimeWithVersion(0, time.Now(), txn, store.UniAddr{T: 3, V: []byte{0xaa}}, sudo, 0, nil)
		assert.False(t, r.CallbySystem())
	})
}

func TestContractInitedRoundTrip(t *testing.T) {
	txn := newTestTxn(t)

	require.False(t, IsContractIsInited(txn, "gateway"))
	require.NoError(t, SetContractInited(txn, "gateway"))
	require.True(t, IsContractIsInited(txn, "gateway"))

	// 其他合约不受影响
	require.False(t, IsContractIsInited(txn, "auth"))
}

func TestQuery_Native_UnknownContract(t *testing.T) {
	rt := testutil.NewMockRuntime()
	data, err := Query(module.NativeVersion, &store.InContractCall{Name: []byte("unknown"), Method: store.MethodToSelectorBytes("anything")}, rt)
	require.NoError(t, err)
	assert.Nil(t, data)
}

func TestMutation_Native_UnknownContract(t *testing.T) {
	rt := testutil.NewMockRuntime()
	err := Mutation(module.NativeVersion, &store.InContractCall{Name: []byte("unknown"), Method: store.MethodToSelectorBytes("anything")}, rt)
	require.NoError(t, err)
}

func TestMutation_Init_SetsInitedFlag(t *testing.T) {
	rt := testutil.NewMockRuntime()
	call := &store.InContractCall{
		Name:   []byte("gateway"),
		Method: store.MethodToSelectorBytes("Init"),
	}
	require.False(t, IsContractIsInited(rt.GetTxn(), "gateway"))
	require.NoError(t, Mutation(module.NativeVersion, call, rt))
	require.True(t, IsContractIsInited(rt.GetTxn(), "gateway"))
}

func TestMutation_Init_SkipsWhenAlreadyInited(t *testing.T) {
	rt := testutil.NewMockRuntime()
	call := &store.InContractCall{
		Name:   []byte("gateway"),
		Method: store.MethodToSelectorBytes("Init"),
	}
	require.NoError(t, SetContractInited(rt.GetTxn(), "gateway"))

	// 已初始化后再次 Init 应被跳过（返回 nil）
	require.NoError(t, Mutation(module.NativeVersion, call, rt))
}

func TestMutation_ShortMethodSelector_NoPanic(t *testing.T) {
	rt := testutil.NewMockRuntime()

	for _, method := range [][]byte{nil, {}, {0x01}, {0x01, 0x02}, {0x01, 0x02, 0x03}} {
		call := &store.InContractCall{Name: []byte("gateway"), Method: method}
		err := Mutation(module.NativeVersion, call, rt)
		require.Error(t, err, "method %v must be rejected, not panic", method)
		assert.Contains(t, err.Error(), "invalid method selector length")
	}
}

func TestQuery_ShortMethodSelector_NoPanic(t *testing.T) {
	rt := testutil.NewMockRuntime()
	call := &store.InContractCall{Name: []byte("gateway"), Method: []byte{0x01}}
	_, err := Query(module.NativeVersion, call, rt)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid method selector length")
}

func TestRuntime_QueryAndCall_Delegate(t *testing.T) {
	// Runtime.Query / Runtime.Call 委托到包级 Query/Mutation，
	// 未知合约应静默返回空结果而不是报错。
	txn := newTestTxn(t)
	caller := store.UniAddr{T: 1, V: []byte{0x01}}
	r := NewRuntimeWithVersion(1, time.Now(), txn, caller, caller, module.NativeVersion, nil)

	data, err := r.Query(caller, &store.InContractCall{Name: []byte("nope"), Method: store.MethodToSelectorBytes("x")})
	require.NoError(t, err)
	assert.Nil(t, data)

	_, err = r.Call(caller, &store.InContractCall{Name: []byte("nope"), Method: store.MethodToSelectorBytes("x")})
	require.NoError(t, err)
}
