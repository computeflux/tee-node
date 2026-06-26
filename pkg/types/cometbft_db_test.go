package types

import (
	"bytes"
	"testing"

	dbm "github.com/cometbft/cometbft-db"
	"github.com/cometbft/cometbft/state"
	cstore "github.com/cometbft/cometbft/store"
	"github.com/cometbft/cometbft/types"
	"github.com/stretchr/testify/require"
)

func TestEncryptedDB_Basic(t *testing.T) {
	base := newTestDB(t)

	db, err := NewEncryptedDB(base)
	require.NoError(t, err)
	defer db.Close()

	require.NoError(t, db.Set([]byte("foo"), []byte("bar")))

	v, err := db.Get([]byte("foo"))
	require.NoError(t, err)
	require.Equal(t, []byte("bar"), v)

	// Underlying value must not be plaintext.
	raw, err := base.Get([]byte("foo"))
	require.NoError(t, err)
	require.False(t, bytes.Contains(raw, []byte("bar")))
}

func TestEncryptedDB_EmptyValue(t *testing.T) {
	base := newTestDB(t)

	db, err := NewEncryptedDB(base)
	require.NoError(t, err)
	defer db.Close()

	require.NoError(t, db.Set([]byte("empty"), []byte{}))

	v, err := db.Get([]byte("empty"))
	require.NoError(t, err)
	require.Equal(t, []byte{}, v)
}

func TestEncryptedDB_HasDelete(t *testing.T) {
	base := newTestDB(t)

	db, err := NewEncryptedDB(base)
	require.NoError(t, err)
	defer db.Close()

	require.NoError(t, db.Set([]byte("k"), []byte("v")))

	has, err := db.Has([]byte("k"))
	require.NoError(t, err)
	require.True(t, has)

	require.NoError(t, db.Delete([]byte("k")))

	has, err = db.Has([]byte("k"))
	require.NoError(t, err)
	require.False(t, has)
}

func TestEncryptedDB_Iterator(t *testing.T) {
	base := newTestDB(t)

	db, err := NewEncryptedDB(base)
	require.NoError(t, err)
	defer db.Close()

	pairs := map[string]string{
		"a": "1",
		"b": "2",
		"c": "3",
	}
	for k, v := range pairs {
		require.NoError(t, db.Set([]byte(k), []byte(v)))
	}

	itr, err := db.Iterator(nil, nil)
	require.NoError(t, err)
	defer itr.Close()

	count := 0
	for ; itr.Valid(); itr.Next() {
		count++
		require.Contains(t, pairs, string(itr.Key()))
		require.Equal(t, pairs[string(itr.Key())], string(itr.Value()))
	}
	require.Equal(t, 3, count)
}

func TestEncryptedDB_Batch(t *testing.T) {
	base := newTestDB(t)

	db, err := NewEncryptedDB(base)
	require.NoError(t, err)
	defer db.Close()

	batch := db.NewBatch()
	require.NoError(t, batch.Set([]byte("x"), []byte("1")))
	require.NoError(t, batch.Set([]byte("y"), []byte("2")))
	require.NoError(t, batch.WriteSync())
	require.NoError(t, batch.Close())

	x, err := db.Get([]byte("x"))
	require.NoError(t, err)
	require.Equal(t, []byte("1"), x)

	y, err := db.Get([]byte("y"))
	require.NoError(t, err)
	require.Equal(t, []byte("2"), y)
}

func TestEncryptedDB_NoKeyRequired(t *testing.T) {
	base := newTestDB(t)
	db, err := NewEncryptedDB(base)
	require.NoError(t, err)
	require.NoError(t, db.Close())
}

func newTestDB(t *testing.T) dbm.DB {
	t.Helper()
	db, err := dbm.NewDB("test", dbm.GoLevelDBBackend, t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	return db
}

// TestEncryptedDB_CometBFTStore verifies that CometBFT's BlockStore and StateStore
// write sealed values when backed by an EncryptedDB wrapper.
func TestEncryptedDB_CometBFTStore(t *testing.T) {
	raw, err := dbm.NewDB("test", dbm.GoLevelDBBackend, t.TempDir())
	require.NoError(t, err)
	defer raw.Close()

	db, err := NewEncryptedDB(raw)
	require.NoError(t, err)
	defer db.Close()

	// --- BlockStore roundtrip ---
	blockStore := cstore.NewBlockStore(db)

	lastCommit0 := &types.Commit{Height: 0}
	block1 := types.MakeBlock(1, []types.Tx{types.Tx("tx1")}, lastCommit0, nil)
	block1.Header.ProposerAddress = make([]byte, 20)
	parts1, err := block1.MakePartSet(types.BlockPartSizeBytes)
	require.NoError(t, err)
	seenCommit1 := &types.Commit{Height: 1}
	blockStore.SaveBlock(block1, parts1, seenCommit1)

	loaded1 := blockStore.LoadBlock(1)
	require.NotNil(t, loaded1)
	require.True(t, bytes.Equal(block1.Hash(), loaded1.Hash()))

	// --- StateStore roundtrip ---
	privVal := types.NewMockPV()
	pubKey, err := privVal.GetPubKey()
	require.NoError(t, err)

	genDoc := &types.GenesisDoc{
		ChainID:       "test-chain",
		InitialHeight: 1,
		Validators: []types.GenesisValidator{
			{Address: pubKey.Address(), PubKey: pubKey, Power: 10},
		},
	}
	genState, err := state.MakeGenesisState(genDoc)
	require.NoError(t, err)

	stateStore := state.NewStore(db, state.StoreOptions{})
	require.NoError(t, stateStore.Bootstrap(genState))

	loadedState, err := stateStore.Load()
	require.NoError(t, err)
	require.Equal(t, genState.LastBlockHeight, loadedState.LastBlockHeight)

	// --- Ensure values are not stored as plaintext ---
	iter, err := raw.Iterator(nil, nil)
	require.NoError(t, err)
	defer iter.Close()

	plainCount := 0
	for ; iter.Valid(); iter.Next() {
		if bytes.Contains(iter.Value(), []byte("tx1")) {
			plainCount++
		}
	}
	require.Zero(t, plainCount, "expected no plaintext values in underlying DB")
}
