package teechain

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/cometbft/cometbft/config"
	"github.com/cometbft/cometbft/crypto/ed25519"
	"github.com/cometbft/cometbft/libs/log"
	nm "github.com/cometbft/cometbft/node"
	"github.com/cometbft/cometbft/p2p"
	"github.com/cometbft/cometbft/privval"
	"github.com/cometbft/cometbft/proxy"
	"github.com/cometbft/cometbft/rpc/client/local"
	"github.com/cometbft/cometbft/types"
	"github.com/computeflux/tee-provider/pkg/model"
	"github.com/computeflux/tee-provider/pkg/store"
	"github.com/stretchr/testify/require"
)

// setupCometBFTNode creates and starts a single in-process CometBFT node
// connected to the given TeeChain ABCI app.
//
// It returns the node and a cleanup function that must be called on test exit.
func setupCometBFTNode(t *testing.T, app *TeeChain) (*nm.Node, func()) {
	t.Helper()

	rootDir := t.TempDir()

	// Generate validator key (ed25519).
	privValKey := ed25519.GenPrivKey()
	privVal := types.NewMockPVWithParams(privValKey, false, false)
	pubKey, err := privVal.GetPubKey()
	require.NoError(t, err)

	// Generate P2P node key.
	nodeKeyPath := filepath.Join(rootDir, "node_key.json")
	nodeKey, err := p2p.LoadOrGenNodeKey(nodeKeyPath)
	require.NoError(t, err)

	// Load the runtime WASM so InitChain does not fail.
	wasmBytes, err := os.ReadFile("./contracts/wasm/runtime/runtime.wasm")
	if err != nil {
		// Fallback: try from repo root if running from tee-chain dir.
		wasmBytes, err = os.ReadFile("./tee-chain/contracts/wasm/runtime/runtime.wasm")
		require.NoError(t, err)
	}
	appState := map[string]interface{}{
		"runtime": map[string]string{
			"wasm": base64.StdEncoding.EncodeToString(wasmBytes),
		},
	}
	appStateBytes, err := json.Marshal(appState)
	require.NoError(t, err)

	// Build genesis doc with a single validator.
	genDoc := &types.GenesisDoc{
		ChainID:         fmt.Sprintf("test-chain-%d", time.Now().UnixNano()),
		GenesisTime:     time.Now(),
		InitialHeight:   1,
		ConsensusParams: types.DefaultConsensusParams(),
		Validators: []types.GenesisValidator{{
			Address: pubKey.Address(),
			PubKey:  pubKey,
			Power:   100,
			Name:    "validator0",
		}},
		AppHash:  []byte{},
		AppState: appStateBytes,
	}
	genDocPath := filepath.Join(rootDir, "genesis.json")
	require.NoError(t, genDoc.SaveAs(genDocPath))

	// Write validator key to the location CometBFT expects.
	privValFile := filepath.Join(rootDir, "config", "priv_validator_key.json")
	stateFile := filepath.Join(rootDir, "data", "priv_validator_state.json")
	require.NoError(t, os.MkdirAll(filepath.Dir(privValFile), 0o755))
	require.NoError(t, os.MkdirAll(filepath.Dir(stateFile), 0o755))
	pv := privval.NewFilePV(privValKey, privValFile, stateFile)
	pv.Save()

	// Configure CometBFT with short timeouts so tests run fast.
	cfg := config.DefaultConfig()
	cfg.SetRoot(rootDir)
	cfg.Consensus.TimeoutPropose = 200 * time.Millisecond
	cfg.Consensus.TimeoutProposeDelta = 0
	cfg.Consensus.TimeoutPrevote = 200 * time.Millisecond
	cfg.Consensus.TimeoutPrevoteDelta = 0
	cfg.Consensus.TimeoutPrecommit = 200 * time.Millisecond
	cfg.Consensus.TimeoutPrecommitDelta = 0
	cfg.Consensus.TimeoutCommit = 200 * time.Millisecond
	cfg.Consensus.SkipTimeoutCommit = true
	cfg.Consensus.CreateEmptyBlocks = true
	cfg.Consensus.CreateEmptyBlocksInterval = 0
	cfg.RPC.ListenAddress = "tcp://127.0.0.1:0" // random port
	cfg.P2P.ListenAddress = "tcp://127.0.0.1:0" // random port
	cfg.P2P.AllowDuplicateIP = true
	cfg.Moniker = "test-node"

	genDocProvider := func() (*types.GenesisDoc, error) { return genDoc, nil }

	logger := log.NewNopLogger()

	node, err := nm.NewNode(
		cfg,
		privVal,
		nodeKey,
		proxy.NewLocalClientCreator(app),
		genDocProvider,
		config.DefaultDBProvider,
		nm.DefaultMetricsProvider(cfg.Instrumentation),
		logger,
	)
	require.NoError(t, err)

	require.NoError(t, node.Start())

	// Wait for the node to produce at least one block.
	require.Eventually(t, func() bool {
		return node.BlockStore().Height() >= 1
	}, 5*time.Second, 100*time.Millisecond, "node did not produce first block")

	cleanup := func() {
		if node != nil {
			_ = node.Stop()
			node.Wait()
		}
	}

	return node, cleanup
}

// TestE2E_CometBFTNodeWithTeeChain verifies that a real CometBFT node can
// start, connect to TeeChain as its ABCI app, and produce blocks.
func TestE2E_CometBFTNodeWithTeeChain(t *testing.T) {
	app := setupTestApp(t)

	node, cleanup := setupCometBFTNode(t, app)
	defer cleanup()

	initialHeight := node.BlockStore().Height()
	require.GreaterOrEqual(t, initialHeight, int64(1))

	// Wait a bit more to confirm continuous block production.
	time.Sleep(500 * time.Millisecond)
	require.Greater(t, node.BlockStore().Height(), initialHeight,
		"node should continue producing blocks")
}

// TestE2E_SubmitTxViaRPC submits a signed transaction through the CometBFT
// RPC client and verifies it is eventually included in a block.
func TestE2E_SubmitTxViaRPC(t *testing.T) {
	app := setupTestApp(t)
	priv := app.NodePriv

	node, cleanup := setupCometBFTNode(t, app)
	defer cleanup()

	client := local.New(node)
	ctx := context.Background()

	// Build a SyncTxStart transaction.
	call := &model.SysCall{
		Payload: &model.SysCall_SyncTxStart{
			SyncTxStart: &model.SyncTxStart{TxIndex: 1, ChainId: 1},
		},
	}
	txBytes := buildSignedTx(t, call, priv)

	// Submit via RPC (async — returns immediately).
	res, err := client.BroadcastTxAsync(ctx, txBytes)
	require.NoError(t, err)
	require.Zero(t, res.Code, "tx rejected by CheckTx: %s", res.Log)

	// Poll for the transaction to be included in a block.
	var found bool
	require.Eventually(t, func() bool {
		txRes, err := client.Tx(ctx, res.Hash, false)
		if err == nil && txRes.Height > 0 {
			found = true
			return true
		}
		return false
	}, 5*time.Second, 200*time.Millisecond, "transaction was not included in a block")
	require.True(t, found)

	// Verify the side-effect was committed: AsyncBatchState should exist.
	state, err := store.GetJson[AsyncBatchState](GLOABL_STATE, hubSyncIndexKey(1))
	require.NoError(t, err)
	require.NotNil(t, state)
	require.Equal(t, int64(1), state.Going)
}

// TestE2E_MultiTxBlock submits several transactions and verifies they are
// all included in the same or consecutive blocks.
func TestE2E_MultiTxBlock(t *testing.T) {
	app := setupTestApp(t)
	priv := app.NodePriv

	node, cleanup := setupCometBFTNode(t, app)
	defer cleanup()

	client := local.New(node)
	ctx := context.Background()

	// Submit 3 distinct transactions.
	chainIds := []uint32{1, 2, 3}
	hashes := make([][]byte, len(chainIds))
	for i, cid := range chainIds {
		call := &model.SysCall{
			Payload: &model.SysCall_SyncTxStart{
				SyncTxStart: &model.SyncTxStart{TxIndex: int64(i + 1), ChainId: cid},
			},
		}
		txBytes := buildSignedTx(t, call, priv)
		res, err := client.BroadcastTxAsync(ctx, txBytes)
		require.NoError(t, err)
		require.Zero(t, res.Code)
		hashes[i] = res.Hash
	}

	// Wait for all to be committed.
	for i, hash := range hashes {
		require.Eventually(t, func() bool {
			txRes, err := client.Tx(ctx, hash, false)
			return err == nil && txRes.Height > 0
		}, 5*time.Second, 200*time.Millisecond, "tx %d not committed", i)
	}

	// Verify all chain states were written.
	for _, cid := range chainIds {
		state, err := store.GetJson[AsyncBatchState](GLOABL_STATE, hubSyncIndexKey(cid))
		require.NoError(t, err)
		require.NotNil(t, state)
		require.Greater(t, state.Going, int64(0))
	}
}

// TestE2E_BlockHeightProgression verifies block height increases monotonically
// over a short observation window.
func TestE2E_BlockHeightProgression(t *testing.T) {
	app := setupTestApp(t)

	node, cleanup := setupCometBFTNode(t, app)
	defer cleanup()

	client := local.New(node)
	ctx := context.Background()

	startHeight, err := client.Block(ctx, nil)
	require.NoError(t, err)
	require.NotNil(t, startHeight)
	base := startHeight.Block.Height

	// Wait for at least 3 more blocks.
	time.Sleep(1 * time.Second)

	endHeight, err := client.Block(ctx, nil)
	require.NoError(t, err)
	require.Greater(t, endHeight.Block.Height, base+2,
		"expected at least 3 more blocks, got %d -> %d", base, endHeight.Block.Height)
}

// TestE2E_InvalidTxRejectionAtCheckTx verifies that malformed transactions
// are rejected at CheckTx and never enter a block.
func TestE2E_InvalidTxRejectionAtCheckTx(t *testing.T) {
	app := setupTestApp(t)

	node, cleanup := setupCometBFTNode(t, app)
	defer cleanup()

	client := local.New(node)
	ctx := context.Background()

	res, err := client.BroadcastTxSync(ctx, []byte("totally-invalid"))
	require.NoError(t, err)
	require.NotZero(t, res.Code, "expected CheckTx to reject invalid tx")
}
