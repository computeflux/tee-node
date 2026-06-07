package teechain

import (
	"bytes"
	"context"
	"crypto/rand"
	"fmt"
	"sync"
	"testing"
	"time"

	abci "github.com/cometbft/cometbft/abci/types"
	"github.com/computeflux/tee-provider/pkg/chaintypes"
	teecrypto "github.com/computeflux/tee-provider/pkg/crypto"
	"github.com/computeflux/tee-provider/pkg/dkg"
	"github.com/computeflux/tee-provider/pkg/model"
	"github.com/computeflux/tee-provider/pkg/model/protoio"
	"github.com/computeflux/tee-provider/pkg/store"
	"github.com/stretchr/testify/require"
)

// ── Level 2: concurrent / multi-node / full-flow tests ───────────────

// setupTestAppWithHighThreshold creates a TeeChain whose DKG threshold is
// high enough that SyncToHub is never triggered in normal test flows.
func setupTestAppWithHighThreshold(t testing.TB) *TeeChain {
	app := setupTestApp(t)
	priv, pub, err := teecrypto.GenerateEd25519KeyPair(rand.Reader)
	require.NoError(t, err)

	app.dkg = &dkg.DKG{
		Signer:    priv,
		Threshold: 100, // never triggers with test-scale signatures
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

// TestHandlePartialSign_ConcurrentNoRace fires 100 goroutines that call
// handlePartialSign concurrently. Run with -race to verify that syncMu,
// syncInProgress and SavePartialSig are all race-free.
func TestHandlePartialSign_ConcurrentNoRace(t *testing.T) {
	app := setupTestAppWithHighThreshold(t)

	const n = 100
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			msg := &model.BlockPartialSign{
				From:    fmt.Sprintf("node-%d", i),
				TxIndex: 1,
				ChainId: 42,
				HubSig:  []byte(fmt.Sprintf("sig-%d", i)),
			}
			_ = app.handlePartialSign(msg)
		}(i)
	}
	wg.Wait()

	// All signatures should be persisted despite concurrent writes.
	sigs, err := app.SigListOfTx(1, 42)
	require.NoError(t, err)
	require.Len(t, sigs, n)
}

// TestHandlePartialSign_MultiChainIsolation verifies that signatures for
// different chainIds are stored independently and never leak across chains.
func TestHandlePartialSign_MultiChainIsolation(t *testing.T) {
	app := setupTestAppWithHighThreshold(t)

	for chainId := uint32(1); chainId <= 3; chainId++ {
		for i := 0; i < 5; i++ {
			err := app.handlePartialSign(&model.BlockPartialSign{
				From:    fmt.Sprintf("node-%d", i),
				TxIndex: 99,
				ChainId: chainId,
				HubSig:  []byte(fmt.Sprintf("sig-%d", i)),
			})
			require.NoError(t, err)
		}
	}

	for chainId := uint32(1); chainId <= 3; chainId++ {
		sigs, err := app.SigListOfTx(99, chainId)
		require.NoError(t, err)
		require.Len(t, sigs, 5, "chain %d should have exactly 5 signatures", chainId)
	}
}

// TestABCI_FullBlockLifecycle exercises the complete consensus path:
// PrepareProposal → ProcessProposal → FinalizeBlock → Commit.
func TestABCI_FullBlockLifecycle(t *testing.T) {
	app := setupTestAppWithDKG(t)
	priv := app.NodePriv
	ctx := context.Background()
	height := int64(1)
	now := time.Now()

	// Build two distinct txs (different callers so PrepareTx doesn't dedup).
	priv2, _, err := teecrypto.GenerateEd25519KeyPair(rand.Reader)
	require.NoError(t, err)

	tx1 := buildHubCallTx(t, priv,
		&model.TeeCall{ChainId: 1, Time: 1, Tx: &model.TeeCall_PodStart{PodStart: &model.PodStart{}}},
	)
	tx2 := buildHubCallTx(t, priv2,
		&model.TeeCall{ChainId: 2, Time: 2, Tx: &model.TeeCall_PodStart{PodStart: &model.PodStart{}}},
	)

	rawTxs := [][]byte{tx1, tx2}

	// 1. PrepareProposal ---------------------------------------------------
	prepResp, err := app.PrepareProposal(ctx, &abci.RequestPrepareProposal{
		Txs:    rawTxs,
		Height: height,
		Time:   now,
	})
	require.NoError(t, err)
	require.NotNil(t, prepResp)
	// At least 4 txs: 2×SyncTxStart + 2×HubCall
	require.GreaterOrEqual(t, len(prepResp.Txs), 4, "expected SyncTxStart per chain + original txs")

	// 2. ProcessProposal ---------------------------------------------------
	procResp, err := app.ProcessProposal(ctx, &abci.RequestProcessProposal{
		Txs:             prepResp.Txs,
		Height:          height,
		Time:            now,
		ProposedLastCommit: abci.CommitInfo{},
	})
	require.NoError(t, err)
	require.Equal(t, abci.ResponseProcessProposal_ACCEPT, procResp.Status)

	// 3. FinalizeBlock -----------------------------------------------------
	finResp, err := app.FinalizeBlock(ctx, &abci.RequestFinalizeBlock{
		Txs:             prepResp.Txs,
		Height:          height,
		Time:            now,
		ProposerAddress: app.myProposerAddress(),
	})
	require.NoError(t, err)
	require.Len(t, finResp.TxResults, len(prepResp.Txs))
	for _, tr := range finResp.TxResults {
		require.Zero(t, tr.Code, "tx failed: %s", tr.Log)
	}

	// 4. Commit ------------------------------------------------------------
	_, err = app.Commit(ctx, &abci.RequestCommit{})
	require.NoError(t, err)

	// 5. Verify side-effects persisted -------------------------------------
	// SyncTxStart for chain 1 should have written AsyncBatchState.
	state1, err := store.GetJson[AsyncBatchState](GLOABL_STATE, hubSyncIndexKey(1))
	require.NoError(t, err)
	require.NotNil(t, state1)
	require.Greater(t, state1.Going, int64(0))

	state2, err := store.GetJson[AsyncBatchState](GLOABL_STATE, hubSyncIndexKey(2))
	require.NoError(t, err)
	require.NotNil(t, state2)
	require.Greater(t, state2.Going, int64(0))
}

// TestABCI_ProcessProposal_InvalidTxRejection verifies that a malformed tx
// causes ProcessProposal to return REJECT.
func TestABCI_ProcessProposal_InvalidTxRejection(t *testing.T) {
	app := setupTestApp(t)
	ctx := context.Background()

	// A completely invalid tx bytes.
	badTx := []byte("not-a-valid-tx")

	procResp, err := app.ProcessProposal(ctx, &abci.RequestProcessProposal{
		Txs:             [][]byte{badTx},
		Height:          1,
		Time:            time.Now(),
		ProposedLastCommit: abci.CommitInfo{},
	})
	require.NoError(t, err)
	require.Equal(t, abci.ResponseProcessProposal_REJECT, procResp.Status)
}

// TestABCI_SequenceHeightMonotonic ensures that state.Height advances
// correctly across multiple FinalizeBlock+Commit rounds.
func TestABCI_SequenceHeightMonotonic(t *testing.T) {
	app := setupTestApp(t)
	ctx := context.Background()
	proposer := app.myProposerAddress()

	for h := int64(1); h <= 5; h++ {
		resp, err := app.FinalizeBlock(ctx, &abci.RequestFinalizeBlock{
			Txs:             [][]byte{},
			Height:          h,
			Time:            time.Now(),
			ProposerAddress: proposer,
		})
		require.NoError(t, err)
		require.Equal(t, h, app.state.Height)

		_, err = app.Commit(ctx, &abci.RequestCommit{})
		require.NoError(t, err)

		// AppHash should be non-nil after first block.
		require.NotNil(t, resp.AppHash)
	}
}

// TestRelayTx_DedupInPrepareTx verifies that PrepareTx drops duplicate
// transactions from the same caller.
func TestRelayTx_DedupInPrepareTx(t *testing.T) {
	app := setupTestAppWithDKG(t)
	priv := app.NodePriv

	// Two HubCalls from the SAME caller → second should be deduplicated.
	tx1 := buildHubCallTx(t, priv,
		&model.TeeCall{ChainId: 1, Time: 1, Tx: &model.TeeCall_PodStart{PodStart: &model.PodStart{}}},
	)
	tx2 := buildHubCallTx(t, priv,
		&model.TeeCall{ChainId: 1, Time: 2, Tx: &model.TeeCall_PodStart{PodStart: &model.PodStart{}}},
	)

	var finaltx [][]byte
	app.PrepareTx([][]byte{tx1, tx2}, &finaltx, 1, time.Now(), true)

	// Should only contain SyncTxStart + tx1 (tx2 deduped).
	require.Less(t, len(finaltx), 4, "expected tx2 to be deduplicated")

	// Count HubCall txs in finaltx.
	hubCount := 0
	for _, raw := range finaltx {
		tx := new(model.Tx)
		_ = protoio.ReadMessage(bytes.NewBuffer(raw), tx)
		call := new(model.SysCall)
		_ = protoio.ReadMessage(bytes.NewBuffer(tx.GetCall()), call)
		if _, ok := call.Payload.(*model.SysCall_HubCall); ok {
			hubCount++
		}
	}
	require.Equal(t, 1, hubCount, "only one HubCall should remain after dedup")
}
