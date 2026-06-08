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
