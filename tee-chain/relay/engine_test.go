package relay

import (
	"os"
	"testing"

	"github.com/computeflux/tee-provider/pkg/model"
	"github.com/stretchr/testify/require"
)

func TestMain(m *testing.M) {
	// init test DB
	db, err := model.NewDB()
	if err != nil {
		panic(err)
	}
	defer db.Close()
	os.Exit(m.Run())
}

// ── SyncEvent ↔ SysCall 转换 ──────────────────────────

func TestMakeRelaySysCall_RoundTrip(t *testing.T) {
	ev := SyncEvent{
		ChainID:        2,
		Nonce:          5,
		BlockNum:       100,
		TargetContract: []byte("gateway"),
		EventType:      []byte("Recharge"),
		EventData:      [][]byte{[]byte("user"), []byte("1000")},
	}

	sysCall := MakeRelaySysCall(ev)
	require.NotNil(t, sysCall)

	rs, ok := sysCall.Payload.(*model.SysCall_RelaySync)
	require.True(t, ok)
	require.NotNil(t, rs.RelaySync)

	back := FromRelaySysCall(rs.RelaySync)
	require.Equal(t, ev.ChainID, back.ChainID)
	require.Equal(t, ev.Nonce, back.Nonce)
	require.Equal(t, ev.BlockNum, back.BlockNum)
	require.Equal(t, ev.TargetContract, back.TargetContract)
	require.Equal(t, ev.EventType, back.EventType)
	require.Equal(t, ev.EventData, back.EventData)
}

func TestMakeRelaySysCall_EmptyEvent(t *testing.T) {
	ev := SyncEvent{}
	sysCall := MakeRelaySysCall(ev)
	require.NotNil(t, sysCall)
	rs := sysCall.Payload.(*model.SysCall_RelaySync)
	require.NotNil(t, rs.RelaySync)
	require.Equal(t, uint32(0), rs.RelaySync.ChainId)
	require.Equal(t, uint64(0), rs.RelaySync.Nonce)
}

// ── BuildSyncEvent ────────────────────────────────────

func TestBuildSyncEvent(t *testing.T) {
	ev := BuildSyncEvent(1, 42, 0, []byte("gateway"), []byte("Withdraw"), [][]byte{[]byte("data")})
	require.Equal(t, uint32(1), ev.ChainID)
	require.Equal(t, uint64(42), ev.Nonce)
	require.Equal(t, []byte("gateway"), ev.TargetContract)
	require.Equal(t, []byte("Withdraw"), ev.EventType)
	require.Equal(t, [][]byte{[]byte("data")}, ev.EventData)
}

// ── Nonce 读写 ────────────────────────────────────────

func TestNonceSetAndGet(t *testing.T) {
	txn := model.DBINS.NewTransaction()
	defer txn.Rollback()

	err := setLastNonce(txn, 1, 0)
	require.NoError(t, err)

	n, err := getLastNonce(txn, 1)
	require.NoError(t, err)
	require.Equal(t, uint64(0), n)

	err = setLastNonce(txn, 1, 5)
	require.NoError(t, err)

	n, err = getLastNonce(txn, 1)
	require.NoError(t, err)
	require.Equal(t, uint64(5), n)
}

func TestGetLastNonce_NotRegistered(t *testing.T) {
	txn := model.DBINS.NewTransaction()
	defer txn.Rollback()

	_, err := getLastNonce(txn, 999)
	require.Error(t, err)
	require.Contains(t, err.Error(), "not registered")
}

// ── Nonce 后台读 ──────────────────────────────────────

func TestGetLastNonceBg_NotRegistered(t *testing.T) {
	_, err := getLastNonceBg(999)
	require.Error(t, err)
	require.Contains(t, err.Error(), "not registered")
}

// ── Mock Engine ───────────────────────────────────────

type mockAPI struct {
	txn *model.Txn
}

func (m *mockAPI) GetTxn() *model.Txn              { return m.txn }
func (m *mockAPI) GetHeight() int64                { return 0 }
func (m *mockAPI) GetCaller() model.UniAddr        { return model.UniAddr{} }
func (m *mockAPI) GetSystemAccount() model.UniAddr { return model.UniAddr{} }
func (m *mockAPI) CallbySystem() bool              { return false }

func (m *mockAPI) Call(_ model.UniAddr, _ model.ContractCall) ([]byte, error) {
	return nil, nil
}
func (m *mockAPI) Query(_ model.UniAddr, _ model.ContractCall) ([]byte, error) {
	return nil, nil
}

func newMockEngine() (*Engine, *model.Txn) {
	txn := model.DBINS.NewTransaction()
	return &Engine{api: &mockAPI{txn: txn}}, txn
}

// commitAndRefresh 提交 txn 并创建新 txn 给 engine
func commitAndRefresh(eng *Engine, oldTxn *model.Txn) *model.Txn {
	oldTxn.Commit()
	newTxn := model.DBINS.NewTransaction()
	eng.api = &mockAPI{txn: newTxn}
	return newTxn
}

// ── CheckRelayEvent ───────────────────────────────────

func TestCheckRelayEvent_FirstEvent(t *testing.T) {
	eng, txn := newMockEngine()
	defer txn.Rollback()

	setLastNonce(txn, 1, 0)
	txn = commitAndRefresh(eng, txn)

	ev := SyncEvent{ChainID: 1, Nonce: 1}
	err := eng.CheckRelayEvent(ev)
	require.NoError(t, err)
}

func TestCheckRelayEvent_WrongNonce(t *testing.T) {
	eng, txn := newMockEngine()
	defer txn.Rollback()

	setLastNonce(txn, 1, 0)
	txn = commitAndRefresh(eng, txn)

	ev := SyncEvent{ChainID: 1, Nonce: 2}
	err := eng.CheckRelayEvent(ev)
	require.Error(t, err)
	require.Contains(t, err.Error(), "expected nonce 1")
}

func TestCheckRelayEvent_Sequential(t *testing.T) {
	eng, txn := newMockEngine()
	defer txn.Rollback()

	setLastNonce(txn, 1, 0)
	txn = commitAndRefresh(eng, txn)

	require.NoError(t, eng.CheckRelayEvent(SyncEvent{ChainID: 1, Nonce: 1}))

	setLastNonce(txn, 1, 1)
	txn = commitAndRefresh(eng, txn)

	require.NoError(t, eng.CheckRelayEvent(SyncEvent{ChainID: 1, Nonce: 2}))
}

func TestCheckRelayEvent_MultiChain(t *testing.T) {
	eng, txn := newMockEngine()
	defer txn.Rollback()

	setLastNonce(txn, 1, 3)
	setLastNonce(txn, 2, 0)
	txn = commitAndRefresh(eng, txn)

	require.NoError(t, eng.CheckRelayEvent(SyncEvent{ChainID: 1, Nonce: 4}))
	require.NoError(t, eng.CheckRelayEvent(SyncEvent{ChainID: 2, Nonce: 1}))
	require.Error(t, eng.CheckRelayEvent(SyncEvent{ChainID: 1, Nonce: 5}))
}

// ── ApplyRelayEvent ──────────────────────────────────

func TestApplyRelayEvent_FirstEvent(t *testing.T) {
	eng, txn := newMockEngine()
	defer txn.Rollback()

	setLastNonce(txn, 1, 0)

	ev := SyncEvent{ChainID: 1, Nonce: 1, TargetContract: []byte("test"), EventType: []byte("test")}
	err := eng.ApplyRelayEvent(ev)
	require.NoError(t, err)

	n, _ := getLastNonce(txn, 1)
	require.Equal(t, uint64(1), n)
}

func TestApplyRelayEvent_WrongNonce(t *testing.T) {
	eng, txn := newMockEngine()
	defer txn.Rollback()

	setLastNonce(txn, 1, 5)

	ev := SyncEvent{ChainID: 1, Nonce: 3} // 小于当前
	err := eng.ApplyRelayEvent(ev)
	require.Error(t, err)
}

// ── LastNonce query ──────────────────────────────────

func TestEngine_LastNonce(t *testing.T) {
	eng, txn := newMockEngine()
	defer txn.Rollback()

	setLastNonce(txn, 3, 7)
	n, err := eng.LastNonce(3)
	require.NoError(t, err)
	require.Equal(t, uint64(7), n)
}
