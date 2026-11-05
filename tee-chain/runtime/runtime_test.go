package runtime

import (
	"fmt"
	"strings"
	"testing"
	"time"

	abci "github.com/cometbft/cometbft/abci/types"
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

	r := NewRuntime(42, ts, txn, caller, sudo)

	assert.Equal(t, int64(42), r.GetHeight())
	assert.Equal(t, int64(1700000000), r.GetBlockTime())
	assert.Equal(t, uint32(0), r.GetVersion())
	assert.Equal(t, caller, r.GetCaller())
	assert.Equal(t, sudo, r.GetSystemAccount())
	assert.NotNil(t, r.GetTxn())
}

func TestNewRuntimeWithVersion(t *testing.T) {
	txn := newTestTxn(t)
	r := NewRuntimeWithVersion(10, time.Unix(100, 0), txn, store.UniAddr{}, store.UniAddr{}, 18)
	assert.Equal(t, uint32(18), r.GetVersion())
}

func TestCallbySystem(t *testing.T) {
	sudo := store.UniAddr{T: 2, V: []byte{0xaa}}
	txn := newTestTxn(t)

	t.Run("matching", func(t *testing.T) {
		r := NewRuntimeWithVersion(0, time.Now(), txn, sudo, sudo, 0)
		assert.True(t, r.CallbySystem())
	})

	t.Run("different_value", func(t *testing.T) {
		r := NewRuntimeWithVersion(0, time.Now(), txn, store.UniAddr{T: 2, V: []byte{0xbb}}, sudo, 0)
		assert.False(t, r.CallbySystem())
	})

	t.Run("different_type", func(t *testing.T) {
		r := NewRuntimeWithVersion(0, time.Now(), txn, store.UniAddr{T: 3, V: []byte{0xaa}}, sudo, 0)
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
	r := NewRuntimeWithVersion(1, time.Now(), txn, caller, caller, module.NativeVersion)

	data, err := r.Query(caller, &store.InContractCall{Name: []byte("nope"), Method: store.MethodToSelectorBytes("x")})
	require.NoError(t, err)
	assert.Nil(t, data)

	_, err = r.Call(caller, &store.InContractCall{Name: []byte("nope"), Method: store.MethodToSelectorBytes("x")})
	require.NoError(t, err)
}

// ── 事件管道测试 ──────────────────────────────────────────────

func TestRuntime_EmitEvent_TakeEvents(t *testing.T) {
	txn := newTestTxn(t)
	r := NewRuntime(1, time.Now(), txn, store.UniAddr{}, store.UniAddr{})

	// 初始无事件
	assert.Empty(t, r.TakeEvents())

	// 发射单个事件
	r.EmitEvent("credit_transfer", map[string]string{
		"from":    "0xaaa",
		"to":      "0xbbb",
		"amount":  "1000",
	})

	events := r.TakeEvents()
	require.Len(t, events, 1)
	assert.Equal(t, "credit_transfer", events[0].Type)
	require.Len(t, events[0].Attributes, 3)

	// 属性应标记 Index=true
	for _, attr := range events[0].Attributes {
		assert.True(t, attr.Index)
	}

	// TakeEvents 后应清空
	assert.Empty(t, r.TakeEvents())
}

func TestRuntime_EmitEvent_Multiple(t *testing.T) {
	txn := newTestTxn(t)
	r := NewRuntime(1, time.Now(), txn, store.UniAddr{}, store.UniAddr{})

	r.EmitEvent("balance_changed", map[string]string{"user": "0xaaa", "delta": "100"})
	r.EmitEvent("order_created", map[string]string{"order_id": "42"})
	r.EmitEvent("credit_earned", map[string]string{"chain_id": "1", "nonce": "7"})

	events := r.TakeEvents()
	require.Len(t, events, 3)
	assert.Equal(t, "balance_changed", events[0].Type)
	assert.Equal(t, "order_created", events[1].Type)
	assert.Equal(t, "credit_earned", events[2].Type)
}

func TestRuntime_EmitEvent_EmptyAttrs(t *testing.T) {
	txn := newTestTxn(t)
	r := NewRuntime(1, time.Now(), txn, store.UniAddr{}, store.UniAddr{})

	r.EmitEvent("noop", nil)

	events := r.TakeEvents()
	require.Len(t, events, 1)
	assert.Equal(t, "noop", events[0].Type)
	assert.Empty(t, events[0].Attributes)
}

// TestRuntime_EmitEvent_ReservedTypeRejected 验证：系统保留事件名（relay_synced / contract_call / tx）
// 与非法格式（空/大写/超长）被拒绝，防止合约伪造链上系统事件/保留 tag。
func TestRuntime_EmitEvent_ReservedTypeRejected(t *testing.T) {
	txn := newTestTxn(t)
	r := NewRuntime(1, time.Now(), txn, store.UniAddr{}, store.UniAddr{})

	// 系统保留名（含 TxIndexer 保留前缀 "tx"）
	r.EmitEvent("relay_synced", map[string]string{"chain_id": "1"})
	r.EmitEvent("contract_call", map[string]string{"contract": "gateway"})
	r.EmitEvent("tx", map[string]string{"height": "1"})
	// 非法格式
	r.EmitEvent("", map[string]string{"a": "b"})
	r.EmitEvent("Credit_Transfer", map[string]string{"a": "b"})
	r.EmitEvent("1abc", map[string]string{"a": "b"})
	r.EmitEvent("has-dash", map[string]string{"a": "b"})
	r.EmitEvent(string(make([]byte, 65)), map[string]string{"a": "b"})

	assert.Empty(t, r.TakeEvents(), "all invalid/reserved event types must be rejected")

	// 合法事件不受影响
	r.EmitEvent("valid_event", map[string]string{"a": "b"})
	events := r.TakeEvents()
	require.Len(t, events, 1)
	assert.Equal(t, "valid_event", events[0].Type)
}

// TestRuntime_EmitEvent_AttrValidation 验证：属性 key/value 校验（防保留 tag 与资源放大）。
// 含 '.' 的 key（歧义 tag）、空 key、超长 key/value、控制字符 key 均被拒绝。
func TestRuntime_EmitEvent_AttrValidation(t *testing.T) {
	txn := newTestTxn(t)
	r := NewRuntime(1, time.Now(), txn, store.UniAddr{}, store.UniAddr{})

	// 含 '.' 的 key：TxIndexer 用 type+"."+key 拼 tag，含 '.' 产生歧义
	r.EmitEvent("ev1", map[string]string{"bad.key": "v"})
	// 空 key
	r.EmitEvent("ev2", map[string]string{"": "v"})
	// 超长 key（>64）
	r.EmitEvent("ev3", map[string]string{string(make([]byte, 65)): "v"})
	// 超长 value（>256）
	r.EmitEvent("ev4", map[string]string{"k": string(make([]byte, 257))})
	// 控制字符 key
	r.EmitEvent("ev5", map[string]string{"bad\x00key": "v"})
	// 属性个数超限（>32）
	tooMany := make(map[string]string)
	for i := 0; i < 33; i++ {
		tooMany[fmt.Sprintf("k%d", i)] = "v"
	}
	r.EmitEvent("ev6", tooMany)

	assert.Empty(t, r.TakeEvents(), "all invalid attrs must be rejected")

	// 边界值合法：key=64、value=256、32 个属性（用可打印字符 'a' 填充）
	txn2 := newTestTxn(t)
	r2 := NewRuntime(1, time.Now(), txn2, store.UniAddr{}, store.UniAddr{})
	boundary := make(map[string]string)
	for i := 0; i < 32; i++ {
		boundary[fmt.Sprintf("k%d", i)] = strings.Repeat("a", 256)
	}
	r2.EmitEvent("ev_ok", boundary)
	r2.EmitEvent("ev_ok2", map[string]string{strings.Repeat("a", 64): "v"})
	events := r2.TakeEvents()
	require.Len(t, events, 2)
	require.Len(t, events[0].Attributes, 32)
}

// TestRuntime_EmitEvent_AttrsSorted 验证：事件属性按 key 排序（map 迭代顺序随机，
// 不排序会导致节点间 ExecTxResult 不一致 → 潜在共识分歧）。
func TestRuntime_EmitEvent_AttrsSorted(t *testing.T) {
	txn := newTestTxn(t)
	r := NewRuntime(1, time.Now(), txn, store.UniAddr{}, store.UniAddr{})

	r.EmitEvent("sorted_check", map[string]string{
		"zeta":   "1",
		"alpha":  "2",
		"mid":    "3",
		"beta_2": "4",
	})

	events := r.TakeEvents()
	require.Len(t, events, 1)
	require.Len(t, events[0].Attributes, 4)
	keys := make([]string, 0, 4)
	for _, a := range events[0].Attributes {
		keys = append(keys, a.Key)
	}
	assert.Equal(t, []string{"alpha", "beta_2", "mid", "zeta"}, keys)
}

// TestRuntime_EmitEvent_PerTxCap 验证：单笔交易事件条数上限（store.MaxEventsPerTx），
// 超限事件被丢弃，防止 WASM 合约循环 emit_event 撑大区块结果与索引库。
func TestRuntime_EmitEvent_PerTxCap(t *testing.T) {
	txn := newTestTxn(t)
	r := NewRuntime(1, time.Now(), txn, store.UniAddr{}, store.UniAddr{})

	for i := 0; i < store.MaxEventsPerTx+10; i++ {
		r.EmitEvent(fmt.Sprintf("ev_%d", i), map[string]string{"i": fmt.Sprint(i)})
	}

	events := r.TakeEvents()
	assert.Len(t, events, store.MaxEventsPerTx, "events beyond MaxEventsPerTx must be dropped")
	// 保留的是前 MaxEventsPerTx 个
	assert.Equal(t, "ev_0", events[0].Type)
	assert.Equal(t, fmt.Sprintf("ev_%d", store.MaxEventsPerTx-1), events[store.MaxEventsPerTx-1].Type)
}

func TestRuntime_EmitEvent_NilAttrs(t *testing.T) {
	txn := newTestTxn(t)
	r := NewRuntime(1, time.Now(), txn, store.UniAddr{}, store.UniAddr{})

	r.EmitEvent("test_event", map[string]string{})

	events := r.TakeEvents()
	require.Len(t, events, 1)
	assert.Equal(t, "test_event", events[0].Type)
	assert.Empty(t, events[0].Attributes)
}

func TestMockRuntime_EmitEvent_TakeEvents(t *testing.T) {
	mr := testutil.NewMockRuntime()

	mr.EmitEvent("test_event", map[string]string{"key": "value"})

	events := mr.TakeEvents()
	require.Len(t, events, 1)
	assert.Equal(t, "test_event", events[0].Type)
	require.Len(t, events[0].Attributes, 1)
	assert.Equal(t, "key", events[0].Attributes[0].Key)
	assert.Equal(t, "value", events[0].Attributes[0].Value)
	assert.True(t, events[0].Attributes[0].Index)

	// 清空后无事件
	assert.Empty(t, mr.TakeEvents())
}

// TestRuntime_Call_SharedEventSink 验证：跨合约调用（Runtime.Call 派生的子 Runtime）
// 发射的事件累积到同一事件管道，父 Runtime.TakeEvents 能取到（不丢失）。
func TestRuntime_Call_SharedEventSink(t *testing.T) {
	txn := newTestTxn(t)
	r := NewRuntime(1, time.Now(), txn, store.UniAddr{}, store.UniAddr{})

	// 模拟跨合约调用：子 Runtime 发射事件（与 Runtime.Call 内部行为一致：
	// 新建子 Runtime 并共享父的事件 sink）
	sub := &Runtime{unix: r.unix, height: r.height, version: r.version, txn: r.txn, caller: r.caller, sudo: r.sudo, events: r.events}
	sub.EmitEvent("sub_contract_event", map[string]string{"from": "sub"})

	r.EmitEvent("parent_event", map[string]string{"from": "parent"})

	events := r.TakeEvents()
	require.Len(t, events, 2, "cross-contract events must be captured in the shared sink")
	assert.Equal(t, "sub_contract_event", events[0].Type)
	assert.Equal(t, "parent_event", events[1].Type)

	// 取出后清空
	assert.Empty(t, r.TakeEvents())
}

// 验证 Runtime 满足 store.ContractApi 接口（含新增的 EmitEvent/TakeEvents）
var _ store.ContractApi = (*Runtime)(nil)

// 验证 abci.Event 类型正确性
func TestRuntime_EventType_IsAbciEvent(t *testing.T) {
	txn := newTestTxn(t)
	r := NewRuntime(1, time.Now(), txn, store.UniAddr{}, store.UniAddr{})
	r.EmitEvent("type_check", map[string]string{"a": "b"})

events := r.TakeEvents()
	var _ []abci.Event = events
}
