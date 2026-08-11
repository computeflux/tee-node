package teechain

import (
	"bytes"
	"encoding/binary"
	"testing"
	"time"

	gsrpc "github.com/centrifuge/go-substrate-rpc-client/v4/types"
	"github.com/computeflux/tee-provider/pkg/model"
	"github.com/computeflux/tee-provider/pkg/model/protoio"
	dbstore "github.com/computeflux/tee-provider/pkg/types"
	"github.com/computeflux/tee-provider/pkg/types/crypto"
	"github.com/stretchr/testify/require"
)

func setupTestDB(t *testing.T) *dbstore.DB {
	t.Helper()
	db, err := dbstore.NewDBWithPath(t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() { db.Close() })
	return db
}

// ---------- tx_nonce.go ----------

func TestGetCallerNonceKey(t *testing.T) {
	require.Equal(t, "call_nonce:0:0102", GetCallerNonceKey([]byte{0x01, 0x02}, 0))
	require.Equal(t, "call_nonce:2:ff", GetCallerNonceKey([]byte{0xff}, 2))
	require.Equal(t, "call_nonce:0:", GetCallerNonceKey(nil, 0))
}

func TestUint64BigEndianRoundTrip(t *testing.T) {
	for _, v := range []uint64{0, 1, 42, 1 << 40, ^uint64(0)} {
		bt := uint64BigEndian(v)
		require.Len(t, bt, 8)
		require.Equal(t, v, binary.BigEndian.Uint64(bt))
	}
}

func TestGetNextCallerNonce(t *testing.T) {
	setupTestDB(t)

	// 无记录 → 1
	require.Equal(t, uint64(1), GetNextCallerNonce([]byte("caller"), 0))

	// 写入 nonce=5 → 下一个为 6
	txn := dbstore.SECRET_DB.NewTransaction()
	require.NoError(t, TxnSetCallerNonce(txn, []byte("caller"), 0, 5))
	require.NoError(t, txn.Commit())
	require.Equal(t, uint64(6), GetNextCallerNonce([]byte("caller"), 0))
	// 不同 caller 互不影响
	require.Equal(t, uint64(1), GetNextCallerNonce([]byte("other"), 0))
	// 不同 callerType 互不影响
	require.Equal(t, uint64(1), GetNextCallerNonce([]byte("caller"), 1))
}

// 损坏的 nonce 数据（长度 < 8）不应 panic（回归测试：原实现 binary.BigEndian.Uint64 会越界 panic）
func TestGetNextCallerNonce_CorruptDataNoPanic(t *testing.T) {
	setupTestDB(t)

	require.NoError(t, dbstore.SECRET_DB.SetKey("", "call_nonce:0:deadbeef", []byte{0x01, 0x02}))
	require.Equal(t, uint64(1), GetNextCallerNonce([]byte{0xde, 0xad, 0xbe, 0xef}, 0))
}

func TestTxnNonceRoundTrip(t *testing.T) {
	setupTestDB(t)

	txn := dbstore.SECRET_DB.NewTransaction()
	current, err := TxnGetCurrentCallerNonce(txn, []byte("c"), 0)
	require.NoError(t, err)
	require.Equal(t, uint64(0), current)

	require.NoError(t, TxnSetCallerNonce(txn, []byte("c"), 0, 42))
	current, err = TxnGetCurrentCallerNonce(txn, []byte("c"), 0)
	require.NoError(t, err)
	require.Equal(t, uint64(42), current)

	// 未提交时全局读不到
	require.Equal(t, uint64(1), GetNextCallerNonce([]byte("c"), 0))

	require.NoError(t, txn.Commit())
	require.Equal(t, uint64(43), GetNextCallerNonce([]byte("c"), 0))
}

// ---------- tx_to_realy.go ----------

// HubSyncStep2 必须标记同步进行中（回归测试：原实现不写 LastSync，
// isHubSyncRuningForChain 永远返回 false，导致对同一 batch 重复发起同步）
func TestHubSyncStep2_MarksSyncRunning(t *testing.T) {
	setupTestDB(t)

	require.False(t, isHubSyncRuningForChain(1))

	txn := dbstore.SECRET_DB.NewTransaction()
	require.NoError(t, HubSyncStep2(1, []uint64{100}, txn))
	require.NoError(t, txn.Commit())

	require.True(t, isHubSyncRuningForChain(1))
}

func TestHubSyncEnd_ReleasesRunning(t *testing.T) {
	setupTestDB(t)

	txn := dbstore.SECRET_DB.NewTransaction()
	require.NoError(t, HubSyncStep2(1, []uint64{100}, txn))
	require.NoError(t, txn.Commit())
	require.True(t, isHubSyncRuningForChain(1))

	// 成功结束 → 释放
	txn = dbstore.SECRET_DB.NewTransaction()
	require.NoError(t, HubSyncEnd(1, []uint64{100}, true, txn))
	require.NoError(t, txn.Commit())
	require.False(t, isHubSyncRuningForChain(1))
}

func TestHubSyncStep2_EmptyIds(t *testing.T) {
	setupTestDB(t)
	txn := dbstore.SECRET_DB.NewTransaction()
	defer txn.Rollback()
	require.Error(t, HubSyncStep2(1, nil, txn))
}

func TestHubSyncEnd_WithoutStep2(t *testing.T) {
	setupTestDB(t)
	txn := dbstore.SECRET_DB.NewTransaction()
	defer txn.Rollback()
	require.Error(t, HubSyncEnd(1, []uint64{1}, true, txn))
}

// ---------- secret_store.go ----------

func writeSecretAt(t *testing.T, namespace, key string, val *model.SecretStore) {
	t.Helper()
	require.NoError(t, dbstore.DBSetProtoMessage(dbstore.SECRET_DB, namespace, key, val))
}

func testH160(prefix byte) gsrpc.H160 {
	b := bytes.Repeat([]byte{prefix}, 20)
	return gsrpc.H160(b)
}

func TestGetSecrets_Valid(t *testing.T) {
	setupTestDB(t)

	user := testH160(0xaa)
	writeSecretAt(t, SecretSpace, user.Hex()+"_1", &model.SecretStore{RawEncScrt: [][]byte{[]byte("s1")}})
	writeSecretAt(t, SecretSpace, user.Hex()+"_2", &model.SecretStore{RawEncScrt: [][]byte{[]byte("s2")}})

	secrets, err := (&TeeChain{}).GetSecrets(user, []uint64{1, 2, 3})
	require.NoError(t, err)
	require.Equal(t, [][]byte{[]byte("s1")}, secrets[1].RawEncScrt)
	require.Equal(t, [][]byte{[]byte("s2")}, secrets[2].RawEncScrt)
	require.Nil(t, secrets[3].RawEncScrt) // 请求了但不存在 → 空对象
}

// 格式异常的 key 不应 panic（回归测试：GetDiskKeys 原实现直接取 Split[2] 越界）
func TestGetDiskKeys_MalformedKeyNoPanic(t *testing.T) {
	setupTestDB(t)

	user := testH160(0xbb)
	writeSecretAt(t, DiskSpace, user.Hex()+"_1", &model.SecretStore{RawEncScrt: [][]byte{[]byte("k1")}})
	// 写入格式异常的 key（缺少 index 段）
	writeSecretAt(t, DiskSpace, user.Hex()+"_bad", &model.SecretStore{})

	_, err := (&TeeChain{}).GetDiskKeys(user, []uint64{1})
	require.Error(t, err)
}

func TestGetSecrets_MalformedKeyNoPanic(t *testing.T) {
	setupTestDB(t)

	user := testH160(0xcc)
	writeSecretAt(t, SecretSpace, user.Hex()+"_1", &model.SecretStore{})
	writeSecretAt(t, SecretSpace, user.Hex()+"_bad", &model.SecretStore{})

	_, err := (&TeeChain{}).GetSecrets(user, []uint64{1})
	require.Error(t, err)
}

// ---------- tx_sign.go ----------

func TestTxHash_Deterministic(t *testing.T) {
	priv, _, err := crypto.GenerateEd25519KeyPair(nil)
	require.NoError(t, err)
	signer := priv.ToSigner(crypto.SuiteEd25519)

	call := &model.SysCall{Payload: &model.SysCall_Empty{Empty: 1}}
	tx, err := signTx(call, signer)
	require.NoError(t, err)

	h1 := TxHash(tx)
	h2 := TxHash(tx)
	require.Equal(t, h1, h2)
	require.Len(t, h1, 64)

	// 不同交易 → 不同 hash
	call2 := &model.SysCall{Payload: &model.SysCall_Empty{Empty: 2}}
	tx2, err := signTx(call2, signer)
	require.NoError(t, err)
	require.NotEqual(t, h1, TxHash(tx2))
}

func TestGetTxBytes_RoundTrip(t *testing.T) {
	priv, _, err := crypto.GenerateEd25519KeyPair(nil)
	require.NoError(t, err)
	signer := priv.ToSigner(crypto.SuiteEd25519)

	call := &model.SysCall{Payload: &model.SysCall_Empty{Empty: time.Now().Unix()}}
	tx, err := signTx(call, signer)
	require.NoError(t, err)

	bt := GetTxBytes(tx)
	restored := &model.Tx{}
	require.NoError(t, protoio.Unmarshal(bt, restored))
	require.Equal(t, tx.Caller, restored.Caller)
	require.Equal(t, tx.Signature, restored.Signature)
	require.Equal(t, tx.Call, restored.Call)
}
