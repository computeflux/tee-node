package types

import (
	"errors"
	"fmt"
	"testing"

	"github.com/cockroachdb/pebble"
	"github.com/cometbft/cometbft/abci/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupTxnTest(t *testing.T) *Txn {
	db, err := NewDBWithPath(t.TempDir())
	require.NoError(t, err)
	SECRET_DB = db
	t.Cleanup(func() { SECRET_DB.Close() })
	return SECRET_DB.NewTransaction()
}

// ── Set / Get / Delete ─────────────────────────────────────

func TestTxn_SetGet(t *testing.T) {
	tx := setupTxnTest(t)

	err := tx.Set([]byte("key1"), []byte("value1"))
	require.NoError(t, err)

	v, err := tx.Get([]byte("key1"))
	require.NoError(t, err)
	assert.Equal(t, "value1", string(v))
}

func TestTxn_Get_NotFound(t *testing.T) {
	tx := setupTxnTest(t)

	_, err := tx.Get([]byte("nonexistent"))
	assert.True(t, errors.Is(err, pebble.ErrNotFound))
}

func TestTxn_Delete(t *testing.T) {
	tx := setupTxnTest(t)

	err := tx.Set([]byte("delkey"), []byte("val"))
	require.NoError(t, err)

	err = tx.Delete([]byte("delkey"))
	require.NoError(t, err)

	_, err = tx.Get([]byte("delkey"))
	assert.True(t, errors.Is(err, pebble.ErrNotFound))
}

// ── SetKey / GetKey ────────────────────────────────────────

func TestTxn_SetKey(t *testing.T) {
	tx := setupTxnTest(t)

	err := tx.SetKey("ns", "k1", []byte("v1"))
	require.NoError(t, err)

	// Verify via raw Get using comboKey
	v, err := tx.Get([]byte(comboKey("ns", "k1")))
	require.NoError(t, err)
	assert.Equal(t, "v1", string(v))
}

// ── ListByPrefix ───────────────────────────────────────────

func TestTxn_ListByPrefix_Basic(t *testing.T) {
	tx := setupTxnTest(t)

	tx.Set([]byte("a_1"), []byte("v1"))
	tx.Set([]byte("a_2"), []byte("v2"))
	tx.Set([]byte("a_3"), []byte("v3"))
	tx.Set([]byte("b_1"), []byte("v4"))

	keys, vals, err := tx.ListByPrefix([]byte("a_"))
	require.NoError(t, err)
	assert.Len(t, keys, 3)
	assert.Len(t, vals, 3)
	assert.Equal(t, "v1", string(vals[0]))
	assert.Equal(t, "v2", string(vals[1]))
	assert.Equal(t, "v3", string(vals[2]))
}

func TestTxn_ListByPrefix_NoMatch(t *testing.T) {
	tx := setupTxnTest(t)

	tx.Set([]byte("a_1"), []byte("v1"))

	keys, vals, err := tx.ListByPrefix([]byte("z_"))
	require.NoError(t, err)
	assert.Empty(t, keys)
	assert.Empty(t, vals)
}

func TestTxn_ListByPrefix_MultiPrefixIsolation(t *testing.T) {
	tx := setupTxnTest(t)

	tx.Set([]byte("p_a"), []byte("v1"))
	tx.Set([]byte("p_b"), []byte("v2"))
	tx.Set([]byte("q_a"), []byte("v3"))

	keys, vals, err := tx.ListByPrefix([]byte("p_"))
	require.NoError(t, err)
	assert.Len(t, keys, 2)
	assert.Len(t, vals, 2)

	keys, vals, err = tx.ListByPrefix([]byte("q_"))
	require.NoError(t, err)
	assert.Len(t, keys, 1)
	assert.Equal(t, "v3", string(vals[0]))
}

// ── ListByPrefixLimit ──────────────────────────────────────

func TestTxn_ListByPrefixLimit_NoOffsetNoLimit(t *testing.T) {
	tx := setupTxnTest(t)

	for i := 0; i < 5; i++ {
		tx.Set([]byte(fmt.Sprintf("pref_%d", i)), []byte(fmt.Sprintf("val_%d", i)))
	}

	_, vals, err := tx.ListByPrefixLimit([]byte("pref_"), 0, 0)
	require.NoError(t, err)
	assert.Len(t, vals, 5)
}

func TestTxn_ListByPrefixLimit_LimitOnly(t *testing.T) {
	tx := setupTxnTest(t)

	for i := 0; i < 5; i++ {
		tx.Set([]byte(fmt.Sprintf("pref_%d", i)), []byte(fmt.Sprintf("val_%d", i)))
	}

	_, vals, err := tx.ListByPrefixLimit([]byte("pref_"), 0, 2)
	require.NoError(t, err)
	assert.Len(t, vals, 2)
	assert.Equal(t, "val_0", string(vals[0]))
	assert.Equal(t, "val_1", string(vals[1]))
}

func TestTxn_ListByPrefixLimit_OffsetOnly(t *testing.T) {
	tx := setupTxnTest(t)

	for i := 0; i < 5; i++ {
		tx.Set([]byte(fmt.Sprintf("pref_%d", i)), []byte(fmt.Sprintf("val_%d", i)))
	}

	_, vals, err := tx.ListByPrefixLimit([]byte("pref_"), 2, 0)
	require.NoError(t, err)
	assert.Len(t, vals, 3)
	assert.Equal(t, "val_2", string(vals[0]))
	assert.Equal(t, "val_4", string(vals[2]))
}

func TestTxn_ListByPrefixLimit_OffsetAndLimit(t *testing.T) {
	tx := setupTxnTest(t)

	for i := 0; i < 5; i++ {
		tx.Set([]byte(fmt.Sprintf("pref_%d", i)), []byte(fmt.Sprintf("val_%d", i)))
	}

	_, vals, err := tx.ListByPrefixLimit([]byte("pref_"), 1, 2)
	require.NoError(t, err)
	assert.Len(t, vals, 2)
	assert.Equal(t, "val_1", string(vals[0]))
	assert.Equal(t, "val_2", string(vals[1]))
}

func TestTxn_ListByPrefixLimit_OffsetBeyondCount(t *testing.T) {
	tx := setupTxnTest(t)

	tx.Set([]byte("pref_a"), []byte("v1"))

	_, vals, err := tx.ListByPrefixLimit([]byte("pref_"), 10, 5)
	require.NoError(t, err)
	assert.Empty(t, vals)
}

func TestTxn_ListByPrefixLimit_LimitExceedsCount(t *testing.T) {
	tx := setupTxnTest(t)

	tx.Set([]byte("pref_a"), []byte("v1"))
	tx.Set([]byte("pref_b"), []byte("v2"))

	_, vals, err := tx.ListByPrefixLimit([]byte("pref_"), 0, 100)
	require.NoError(t, err)
	assert.Len(t, vals, 2)
}

// ── DeletekeysByPrefix ─────────────────────────────────────

func TestTxn_DeletekeysByPrefix(t *testing.T) {
	tx := setupTxnTest(t)

	tx.Set([]byte("test1"), []byte("v1"))
	tx.Set([]byte("test2"), []byte("v2"))
	tx.Set([]byte("other"), []byte("v3"))

	err := tx.DeletekeysByPrefix([]byte("test"))
	require.NoError(t, err)

	_, err = tx.Get([]byte("test1"))
	assert.True(t, errors.Is(err, pebble.ErrNotFound))
	_, err = tx.Get([]byte("test2"))
	assert.True(t, errors.Is(err, pebble.ErrNotFound))

	v, err := tx.Get([]byte("other"))
	require.NoError(t, err)
	assert.Equal(t, "v3", string(v))
}

// ── Rollback ───────────────────────────────────────────────

func TestTxn_Rollback(t *testing.T) {
	db, err := NewDBWithPath(t.TempDir())
	require.NoError(t, err)
	SECRET_DB = db
	defer SECRET_DB.Close()

	tx := SECRET_DB.NewTransaction()
	tx.Set([]byte("rollback_key"), []byte("rollback_val"))

	err = tx.Rollback()
	require.NoError(t, err)

	// New transaction should not see the rolled-back data
	tx2 := SECRET_DB.NewTransaction()
	_, err = tx2.Get([]byte("rollback_key"))
	assert.True(t, errors.Is(err, pebble.ErrNotFound))
}

// ── Commit ─────────────────────────────────────────────────

func TestTxn_Commit(t *testing.T) {
	db, err := NewDBWithPath(t.TempDir())
	require.NoError(t, err)
	SECRET_DB = db
	defer SECRET_DB.Close()

	tx := SECRET_DB.NewTransaction()
	tx.Set([]byte("commit_key"), []byte("commit_val"))

	err = tx.Commit()
	require.NoError(t, err)

	// New transaction should see committed data
	tx2 := SECRET_DB.NewTransaction()
	v, err := tx2.Get([]byte("commit_key"))
	require.NoError(t, err)
	assert.Equal(t, "commit_val", string(v))
}

// ── Count ──────────────────────────────────────────────────

func TestTxn_Count(t *testing.T) {
	tx := setupTxnTest(t)

	assert.Equal(t, uint32(0), tx.Count())

	tx.Set([]byte("k1"), []byte("v1"))
	assert.Equal(t, uint32(1), tx.Count())

	tx.Set([]byte("k2"), []byte("v2"))
	assert.Equal(t, uint32(2), tx.Count())

	tx.Delete([]byte("k1"))
	assert.Equal(t, uint32(3), tx.Count())
}

// ── TxnGetJson / TxnSetJson ────────────────────────────────

type testJsonStruct struct {
	Name  string `json:"name"`
	Value int    `json:"value"`
}

func TestTxn_TxnSetJson_TxnGetJson(t *testing.T) {
	tx := setupTxnTest(t)

	original := &testJsonStruct{Name: "test", Value: 42}
	err := TxnSetJson(tx, []byte("json_key"), original)
	require.NoError(t, err)

	result, err := TxnGetJson[testJsonStruct](tx, []byte("json_key"))
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, "test", result.Name)
	assert.Equal(t, 42, result.Value)
}

func TestTxn_TxnGetJson_NotFound(t *testing.T) {
	tx := setupTxnTest(t)

	result, err := TxnGetJson[testJsonStruct](tx, []byte("missing"))
	require.NoError(t, err)
	assert.Nil(t, result)
}

func TestTxn_TxnGetJson_EmptyValue(t *testing.T) {
	tx := setupTxnTest(t)

	tx.Set([]byte("empty_json"), []byte{})

	result, err := TxnGetJson[testJsonStruct](tx, []byte("empty_json"))
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, "", result.Name)
	assert.Equal(t, 0, result.Value)
}

// ── TxnGetProtoMessage / TxnSetProtoMessage ────────────────

func TestTxn_TxnSetProtoMessage_TxnGetProtoMessage(t *testing.T) {
	tx := setupTxnTest(t)

	original := &types.ValidatorUpdate{Power: 9999}
	err := TxnSetProtoMessage(tx, []byte("proto_key"), original)
	require.NoError(t, err)

	result, err := TxnGetProtoMessage[types.ValidatorUpdate](tx, []byte("proto_key"))
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, int64(9999), result.Power)
}

func TestTxn_TxnGetProtoMessage_NotFound(t *testing.T) {
	tx := setupTxnTest(t)

	result, err := TxnGetProtoMessage[types.ValidatorUpdate](tx, []byte("missing"))
	require.NoError(t, err)
	assert.Nil(t, result)
}

func TestTxn_TxnGetProtoMessageList(t *testing.T) {
	tx := setupTxnTest(t)

	err := TxnSetProtoMessage(tx, []byte("validator_a"), &types.ValidatorUpdate{Power: 100})
	require.NoError(t, err)
	err = TxnSetProtoMessage(tx, []byte("validator_b"), &types.ValidatorUpdate{Power: 200})
	require.NoError(t, err)

	list, err := TxnGetProtoMessageList[types.ValidatorUpdate](tx, []byte("validator_"))
	require.NoError(t, err)
	assert.Len(t, list, 2)
}

// ── TxnGetCodec / TxnSetCodec ──────────────────────────────

func TestTxn_TxnSetCodec_TxnGetCodec(t *testing.T) {
	tx := setupTxnTest(t)

	original := uint64(42)
	err := TxnSetCodec(tx, "ns", "codec_key", original)
	require.NoError(t, err)

	result, err := TxnGetCodec[uint64](tx, "ns", "codec_key")
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, uint64(42), *result)
}

func TestTxn_TxnGetCodec_NotFound(t *testing.T) {
	tx := setupTxnTest(t)

	result, err := TxnGetCodec[uint64](tx, "ns", "missing")
	require.NoError(t, err)
	assert.Nil(t, result)
}
