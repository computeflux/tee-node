package teechain

import (
	"bytes"
	"context"
	"crypto/rand"
	"testing"
	"time"

	"github.com/cockroachdb/pebble"
	abci "github.com/cometbft/cometbft/abci/types"
	"github.com/computeflux/tee-provider/pkg/chaintypes"
	teecrypto "github.com/computeflux/tee-provider/pkg/crypto"
	"github.com/computeflux/tee-provider/pkg/dkg"
	"github.com/computeflux/tee-provider/pkg/model"
	"github.com/computeflux/tee-provider/pkg/model/protoio"
	"github.com/computeflux/tee-provider/pkg/store"
	"github.com/stretchr/testify/require"
)

// mockPeer implements the minimal Peer interface for DKG testing.
type mockPeer struct{}

func (m *mockPeer) Send(message *model.PeerMsg) error              { return nil }
func (m *mockPeer) Sub(topic string, handler func(any) error) error { return nil }
func (m *mockPeer) AvailableNodes() []*teecrypto.PubKey            { return nil }
func (m *mockPeer) AllNodes() []*teecrypto.PubKey                  { return nil }

// setupTestDB creates a temporary PebbleDB and sets the global DBINS.
func setupTestDB(t testing.TB) {
	dir := t.TempDir()
	db, err := pebble.Open(dir, &pebble.Options{})
	require.NoError(t, err)
	store.DBINS = &store.DB{DB: db}
	t.Cleanup(func() {
		store.DBINS.Close()
		store.DBINS = nil
	})
}

// setupTestApp creates a bare-bones TeeChain for unit testing.
// It skips NewTeeChain to avoid CheckRuntimeV0 and loadAppState side-effects.
func setupTestApp(t testing.TB) *TeeChain {
	setupTestDB(t)
	priv, _, err := teecrypto.GenerateEd25519KeyPair(rand.Reader)
	require.NoError(t, err)

	app := &TeeChain{
		state:          AppState{},
		syncInProgress: make(map[string]bool),
		NodePriv:       priv,
	}
	return app
}

// setupTestAppWithDKG creates a TeeChain with a minimal DKG suitable for
// PrepareTx and partial-sign paths.
func setupTestAppWithDKG(t testing.TB) *TeeChain {
	app := setupTestApp(t)
	priv, pub, err := teecrypto.GenerateEd25519KeyPair(rand.Reader)
	require.NoError(t, err)

	app.dkg = &dkg.DKG{
		Signer:    priv,
		Threshold: 0, // threshold+1 = 1, matches AvailableNodeLen() == 1
		Nodes: []*chaintypes.Validator{{
			ValidatorId: *pub,
			P2pId:       *pub,
		}},
		DkgPubKeys: map[teecrypto.SuiteType]*teecrypto.PubKey{
			teecrypto.SuiteEd25519: pub,
		},
		Peer: &mockPeer{},
	}
	return app
}

// buildSignedTx serialises a SysCall into a signed Tx and returns the raw bytes.
func buildSignedTx(t testing.TB, call *model.SysCall, priv *teecrypto.PrivKey) []byte {
	return buildSignedTxWithType(t, call, priv, 0)
}

// buildSignedTxWithType serialises a SysCall with a specific callerType.
func buildSignedTxWithType(t testing.TB, call *model.SysCall, priv *teecrypto.PrivKey, callerType uint32) []byte {
	signer := priv.ToSigner(teecrypto.SuiteEd25519)
	sigBytes, err := call.BytesForSig()
	require.NoError(t, err)
	signature, err := signer.Sign(sigBytes)
	require.NoError(t, err)
	buf := new(bytes.Buffer)
	err = protoio.WriteMessage(call, buf)
	require.NoError(t, err)
	tx := &model.Tx{
		Caller:     signer.Public(),
		CallerType: callerType,
		Call:       buf.Bytes(),
		Signature:  signature,
	}
	return GetTxBytes(tx)
}

// buildHubCallTx creates a signed tx wrapping a HubCall with the given TeeCalls.
func buildHubCallTx(t testing.TB, priv *teecrypto.PrivKey, calls ...*model.TeeCall) []byte {
	call := &model.SysCall{
		Payload: &model.SysCall_HubCall{
			HubCall: &model.HubCall{Call: calls},
		},
	}
	return buildSignedTx(t, call, priv)
}

// buildSyncTxStart creates a signed tx wrapping SyncTxStart.
func buildSyncTxStart(t testing.TB, priv *teecrypto.PrivKey, txIndex int64, chainId uint32) []byte {
	call := &model.SysCall{
		Payload: &model.SysCall_SyncTxStart{
			SyncTxStart: &model.SyncTxStart{TxIndex: txIndex, ChainId: chainId},
		},
	}
	return buildSignedTx(t, call, priv)
}

// buildSyncTxEnd creates a signed tx wrapping SyncTxEnd.
func buildSyncTxEnd(t testing.TB, priv *teecrypto.PrivKey, txIndex int64, chainId uint32) []byte {
	call := &model.SysCall{
		Payload: &model.SysCall_SyncTxEnd{
			SyncTxEnd: &model.SyncTxEnd{TxIndex: txIndex, ChainId: chainId},
		},
	}
	return buildSignedTx(t, call, priv)
}

// ── Tests ──────────────────────────────────────────────────────────────

func TestFinalizeTx_SyncTxStateMachine(t *testing.T) {
	app := setupTestApp(t)
	priv := app.NodePriv
	chainId := uint32(42)

	// 1. SyncTxStart sets up the per-chain batch state.
	var txIndex int64 = 7
	tx1 := buildSyncTxStart(t, priv, txIndex, chainId)

	txn := store.DBINS.NewTransaction()
	_, err := app.FinalizeTx([][]byte{tx1}, txn, 1, time.Now(), nil)
	require.NoError(t, err)
	require.NoError(t, txn.Commit())

	// Verify HubSync state was written.
	state, err := store.GetJson[AsyncBatchState](GLOABL_STATE, hubSyncIndexKey(chainId))
	require.NoError(t, err)
	require.NotNil(t, state)
	require.Equal(t, txIndex, state.Going)

	// 2. SyncTxEnd marks the batch done.
	tx2 := buildSyncTxEnd(t, priv, txIndex, chainId)
	txn = store.DBINS.NewTransaction()
	_, err = app.FinalizeTx([][]byte{tx2}, txn, 2, time.Now(), nil)
	require.NoError(t, err)
	require.NoError(t, txn.Commit())

	// Verify state cleaned up (Done == Going).
	state, err = store.GetJson[AsyncBatchState](GLOABL_STATE, hubSyncIndexKey(chainId))
	require.NoError(t, err)
	require.NotNil(t, state)
	require.Equal(t, txIndex, state.Done)
}

func TestFinalizeTx_MultiChainTxIndexTracking(t *testing.T) {
	app := setupTestApp(t)
	priv := app.NodePriv

	chain1 := uint32(1)
	chain2 := uint32(2)
	txIndex := int64(10)

	// Simulate two SyncTxStart for the same txIndex but different chains.
	tx1 := buildSyncTxStart(t, priv, txIndex, chain1)
	tx2 := buildSyncTxStart(t, priv, txIndex, chain2)

	// Add dummy HubCalls so that txIndex -> chainIds mapping is persisted.
	hub1 := buildHubCallTx(t, priv, &model.TeeCall{ChainId: chain1, Tx: &model.TeeCall_PodStart{PodStart: &model.PodStart{}}})
	hub2 := buildHubCallTx(t, priv, &model.TeeCall{ChainId: chain2, Tx: &model.TeeCall_PodStart{PodStart: &model.PodStart{}}})

	txn := store.DBINS.NewTransaction()
	_, err := app.FinalizeTx([][]byte{tx1, hub1, tx2, hub2}, txn, 1, time.Now(), nil)
	require.NoError(t, err)
	require.NoError(t, txn.Commit())

	// Verify txIndexChains stored both chainIds.
	cids, err := getTxIndexChains(txIndex, store.DBINS.NewTransaction())
	require.NoError(t, err)
	require.Len(t, cids, 2)
	require.Contains(t, cids, chain1)
	require.Contains(t, cids, chain2)

	// 3. End chain1 only – tx_index should NOT be deleted yet.
	txEnd1 := buildSyncTxEnd(t, priv, txIndex, chain1)
	txn = store.DBINS.NewTransaction()
	_, err = app.FinalizeTx([][]byte{txEnd1}, txn, 2, time.Now(), nil)
	require.NoError(t, err)
	require.NoError(t, txn.Commit())

	// tx_index chains key should still exist.
	cids, err = getTxIndexChains(txIndex, store.DBINS.NewTransaction())
	require.NoError(t, err)
	require.Len(t, cids, 2)

	// 4. End chain2 – now everything should be cleaned up.
	txEnd2 := buildSyncTxEnd(t, priv, txIndex, chain2)
	txn = store.DBINS.NewTransaction()
	_, err = app.FinalizeTx([][]byte{txEnd2}, txn, 3, time.Now(), nil)
	require.NoError(t, err)
	require.NoError(t, txn.Commit())

	// tx_index store should be deleted.
	cids, err = getTxIndexChains(txIndex, store.DBINS.NewTransaction())
	require.NoError(t, err)
	require.Empty(t, cids)
}

func TestVerifyVoteExtension_EmptyAccept(t *testing.T) {
	app := setupTestApp(t)
	resp, err := app.VerifyVoteExtension(context.Background(), &abci.RequestVerifyVoteExtension{
		VoteExtension: []byte{},
	})
	require.NoError(t, err)
	require.Equal(t, abci.ResponseVerifyVoteExtension_ACCEPT, resp.Status)
}

func TestFinalizeBlock_Commit_FullFlow(t *testing.T) {
	app := setupTestApp(t)
	priv := app.NodePriv
	chainId := uint32(99)

	txIndex := int64(5)
	tx := buildSyncTxStart(t, priv, txIndex, chainId)

	// FinalizeBlock
	resp, err := app.FinalizeBlock(context.Background(), &abci.RequestFinalizeBlock{
		Txs:             [][]byte{tx},
		Height:          1,
		Time:            time.Now(),
		ProposerAddress: app.myProposerAddress(),
	})
	require.NoError(t, err)
	require.Len(t, resp.TxResults, 1)
	require.Equal(t, uint32(abci.CodeTypeOK), resp.TxResults[0].Code)

	// Commit
	_, err = app.Commit(context.Background(), &abci.RequestCommit{})
	require.NoError(t, err)

	// Verify state persisted.
	state, err := store.GetJson[AsyncBatchState](GLOABL_STATE, hubSyncIndexKey(chainId))
	require.NoError(t, err)
	require.NotNil(t, state)
	require.Equal(t, txIndex, state.Going)
}

func TestPrepareTx_GroupByChainId(t *testing.T) {
	app := setupTestAppWithDKG(t)
	priv := app.NodePriv

	chain1 := uint32(1)
	chain2 := uint32(2)

	// Construct mempool txs: two HubCalls on different chains.
	// Use different signers so they are not deduplicated by caller in PrepareTx.
	priv2, _, err := teecrypto.GenerateEd25519KeyPair(rand.Reader)
	require.NoError(t, err)

	hubTx1 := buildHubCallTx(t, priv,
		&model.TeeCall{ChainId: chain1, Time: 1, Tx: &model.TeeCall_PodStart{PodStart: &model.PodStart{}}},
	)
	hubTx2 := buildHubCallTx(t, priv2,
		&model.TeeCall{ChainId: chain2, Time: 2, Tx: &model.TeeCall_PodStart{PodStart: &model.PodStart{}}},
	)

	var finaltx [][]byte
	app.PrepareTx([][]byte{hubTx1, hubTx2}, &finaltx, 1, time.Now(), true)

	// Should produce: SyncTxStart(chain1), hubTx1, SyncTxStart(chain2), hubTx2
	require.GreaterOrEqual(t, len(finaltx), 4, "expected at least 4 txs (2 SyncTxStart + 2 HubCall)")

	// Count SyncTxStart messages and verify chainIds.
	starts := make(map[uint32]bool)
	for _, raw := range finaltx {
		tx := new(model.Tx)
		require.NoError(t, protoio.ReadMessage(bytes.NewBuffer(raw), tx))

		call := new(model.SysCall)
		require.NoError(t, protoio.ReadMessage(bytes.NewBuffer(tx.GetCall()), call))

		if s, ok := call.Payload.(*model.SysCall_SyncTxStart); ok {
			starts[s.SyncTxStart.ChainId] = true
		}
	}
	require.True(t, starts[chain1], "missing SyncTxStart for chain1")
	require.True(t, starts[chain2], "missing SyncTxStart for chain2")
}

func TestHandlePartialSign_Deduplication(t *testing.T) {
	app := setupTestApp(t)
	priv, pub, err := teecrypto.GenerateEd25519KeyPair(rand.Reader)
	require.NoError(t, err)

	// Use high threshold so SyncToHub goroutine is never triggered.
	app.dkg = &dkg.DKG{
		Signer:    priv,
		Threshold: 10,
		Nodes: []*chaintypes.Validator{{
			ValidatorId: *pub,
			P2pId:       *pub,
		}},
		DkgPubKeys: map[teecrypto.SuiteType]*teecrypto.PubKey{
			teecrypto.SuiteEd25519: pub,
		},
		Peer: &mockPeer{},
	}

	msg := &model.BlockPartialSign{
		From:    "node1",
		TxIndex: 1,
		ChainId: 1,
		HubSig:  []byte("sig1"),
	}

	// First handle should succeed.
	err = app.handlePartialSign(msg)
	require.NoError(t, err)

	// Verify signature saved.
	sigs, err := app.SigListOfTx(1, 1)
	require.NoError(t, err)
	require.Len(t, sigs, 1)

	// Second handle with same (txIndex, chainId) should still succeed
	// (signature dedup happens at storage layer; sync dedup happens via syncInProgress).
	err = app.handlePartialSign(msg)
	require.NoError(t, err)
}

// TestSyncToHub_SubmitRetryOnFailure is a lightweight regression test that
// verifies SyncToHub returns an error (because there is no real main-chain)
// and that the returned error contains the expected SyncTxRetry submission
// side-effect.  Since SubmitCallFromNode needs a live mempool, we only check
// the error path here.
func TestSyncToHub_ErrorPath(t *testing.T) {
	app := setupTestAppWithDKG(t)

	// Without a real chain connection, SyncToHub should fail when trying to
	// load the stored call.
	err := app.SyncToHub(1, 999, [][]byte{[]byte("fake-sig")})
	require.Error(t, err)
	require.Contains(t, err.Error(), "call not found")
}

// TestNextGlobalTxIndex_Monotonic ensures the global counter increments.
func TestNextGlobalTxIndex_Monotonic(t *testing.T) {
	setupTestDB(t)

	idx1, err := nextGlobalTxIndex()
	require.NoError(t, err)
	require.Greater(t, idx1, int64(0))

	idx2, err := nextGlobalTxIndex()
	require.NoError(t, err)
	require.Greater(t, idx2, idx1)
}

// BenchmarkFinalizeTx measures the cost of FinalizeTx under load.
func BenchmarkFinalizeTx(b *testing.B) {
	app := setupTestApp(b)
	priv := app.NodePriv
	chainId := uint32(1)

	// Pre-build one HubCall tx.
	hubTx := buildHubCallTx(b, priv,
		&model.TeeCall{ChainId: chainId, Time: 1, Tx: &model.TeeCall_PodStart{PodStart: &model.PodStart{}}},
	)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		txn := store.DBINS.NewTransaction()
		_, err := app.FinalizeTx([][]byte{hubTx}, txn, int64(i), time.Now(), nil)
		if err != nil {
			b.Fatal(err)
		}
		txn.Commit()
	}
}
