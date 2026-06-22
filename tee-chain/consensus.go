package teechain

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	abci "github.com/cometbft/cometbft/abci/types"
	"github.com/cometbft/cometbft/crypto"
	"github.com/cometbft/cometbft/version"
	teecrypto "github.com/computeflux/tee-provider/pkg/crypto"
	"github.com/computeflux/tee-provider/pkg/model"
	"github.com/computeflux/tee-provider/pkg/model/protoio"
	"github.com/computeflux/tee-provider/pkg/store"
	"github.com/computeflux/tee-provider/pkg/teetypes"
	"github.com/cosmos/gogoproto/proto"
	"github.com/pkg/errors"

	"github.com/computeflux/tee-provider/pkg/dkg"
	bftbrigde "github.com/computeflux/tee-provider/pkg/network/bft-brigde"
	"github.com/computeflux/tee-provider/pkg/util"
	"github.com/computeflux/tee-provider/tee-chain/relay"
)

const ApplicationVersion = 1

type TeeChain struct {
	abci.BaseApplication
	state AppState

	dkg      *dkg.DKG
	p2p      *bftbrigde.BTFReactor
	NodePriv *teecrypto.PrivKey

	txCh      *store.PersistChan[*model.BlockPartialSign]
	txReadyCh chan struct{} // 通知 ApplyToChain 可以提交新交易到区块

	onGoingBlock        *store.Txn
	onGoingValidators   []abci.ValidatorUpdate
	currProposerAddress []byte
	lastCommitAt        time.Time // 上次 Commit 的时间

	relay      *relay.Engine
	relayCache *relayCache

	lastProposerWasMe atomic.Bool // 上一个区块是否由本节点出块
	relayWorkerOnce   sync.Once   // 保证 relay worker 只启动一次

	syncMu         sync.Mutex
	syncInProgress map[string]bool // key: "{txIndex}_{chainId}"，防止重复提交主链
}

// myProposerAddress 返回本节点对应的 CometBFT proposer address
func (app *TeeChain) myProposerAddress() []byte {
	if app.NodePriv == nil {
		return nil
	}
	pub := app.NodePriv.GetPublicForSuite(teecrypto.SuiteEd25519)
	return crypto.AddressHash(pub.PublicKey)
}

func NewTeeChain(light bool) (*TeeChain, error) {
	state, err := loadAppState()
	if err != nil {
		return nil, errors.Wrap(err, "LoadAppState")
	}

	c := &TeeChain{
		state:          state,
		syncInProgress: make(map[string]bool),
	}

	if err := CheckRuntimeV0(); err != nil {
		return nil, err
	}

	if !light {
		txCh, err := store.NewPersistChan[*model.BlockPartialSign]("back_tx", 1000)
		if err != nil {
			return nil, err
		}
		c.txCh = txCh

		// 初始化交易就绪通知 channel
		c.txReadyCh = make(chan struct{}, 1)

		// 启动后台 goroutine 异步执行 ApplyToChain
		go c.applyTxLoop()
	}

	return c, nil
}

func (app *TeeChain) Info(_ context.Context, info *abci.RequestInfo) (*abci.ResponseInfo, error) {
	return &abci.ResponseInfo{
		Version:          version.ABCIVersion,
		AppVersion:       ApplicationVersion,
		LastBlockHeight:  app.state.Height,
		LastBlockAppHash: app.state.Hash(),
	}, nil
}

func (app *TeeChain) Query(ctx context.Context, query *abci.RequestQuery) (*abci.ResponseQuery, error) {
	util.LogWithGreen("Query")
	resp := abci.ResponseQuery{Key: query.Data}

	return &resp, nil
}

func (app *TeeChain) InitChain(_ context.Context, req *abci.RequestInitChain) (*abci.ResponseInitChain, error) {
	util.LogWithGreen("InitChain")
	app.initValidators(req.Validators)

	// 从 genesis app_state 读取初始 WASM（若提供则直接使用，无需本地文件）
	initRuntimeFromGenesis := func() []byte {
		if len(req.AppStateBytes) == 0 {
			return nil
		}
		var state struct {
			Runtime struct {
				Wasm string `json:"wasm"` // base64
			} `json:"runtime"`
		}
		if err := json.Unmarshal(req.AppStateBytes, &state); err != nil {
			util.LogError("InitChain parse app_state", err)
			return nil
		}
		if state.Runtime.Wasm == "" {
			return nil
		}
		b, err := base64.StdEncoding.DecodeString(state.Runtime.Wasm)
		if err != nil {
			util.LogError("InitChain decode wasm", err)
			return nil
		}
		return b
	}()

	if err := InitRuntimeVersion(initRuntimeFromGenesis); err != nil {
		return nil, fmt.Errorf("init runtime version: %w", err)
	}

	appHash := app.state.Hash()

	// This parameter can also be set in the genesis file
	if req.ConsensusParams != nil && req.ConsensusParams.Abci != nil {
		req.ConsensusParams.Abci.VoteExtensionsEnableHeight = 1
	}
	return &abci.ResponseInitChain{ConsensusParams: req.ConsensusParams, AppHash: appHash}, nil
}

func (app *TeeChain) CheckTx(_ context.Context, req *abci.RequestCheckTx) (*abci.ResponseCheckTx, error) {
	fmt.Println()
	util.LogWithGreen("START BLOCK", "--------------------------------------------------------------")
	ok := app.checkTx(req.Tx)
	LogWithTime("🚀 CheckTx", ok)

	return &abci.ResponseCheckTx{Code: ok}, nil
}

func (app *TeeChain) PrepareProposal(_ context.Context, req *abci.RequestPrepareProposal) (*abci.ResponsePrepareProposal, error) {
	finalProposal := make([][]byte, 0, len(req.Txs)+32)

	// ── Relay: 出块节点拉取主链事件, 预置到区块头部 ──────
	if app.relay != nil && app.dkg != nil {
		relayTxs := app.prepareRelayTxs()
		finalProposal = append(finalProposal, relayTxs...)
		if len(relayTxs) > 0 {
			LogWithTime(fmt.Sprintf("🎁 Relay: %d events", len(relayTxs)))
		}
	}

	// Check if the current epoch is valid
	epochTx := app.CheckEpochFromValidator()
	// finalProposal 已在上面声明
	if len(epochTx) > 0 {
		finalProposal = append(finalProposal, epochTx)
	}

	// 如果有未提交到主链的交易（同步进行中），只打包 mempool 中的 SyncTxEnd 等系统交易
	if IsHubSyncRuning() {
		util.LogWithYellow("PrepareProposal", "pending sync to main chain, only pack system txs")
		app.PrepareTx(req.Txs, &finalProposal, req.Height, req.Time, false)
		return &abci.ResponsePrepareProposal{Txs: finalProposal}, nil
	}

	epochStatus := app.GetEpochStatus()

	// Check if it is in the epoch transition phase
	if len(epochTx) == 0 && time.Now().Unix()-int64(epochStatus) > 120 {
		app.PrepareTx(req.Txs, &finalProposal, req.Height, req.Time, true)
	} else {
		app.PrepareTx(req.Txs, &finalProposal, req.Height, req.Time, false)
	}

	LogWithTime("🎁 PrepareProposal " + fmt.Sprint(len(finalProposal)) + " txs")
	return &abci.ResponsePrepareProposal{Txs: finalProposal}, nil
}

func (app *TeeChain) ProcessProposal(_ context.Context, req *abci.RequestProcessProposal) (*abci.ResponseProcessProposal, error) {
	LogWithTime("🌈 ProcessProposal")

	// ── Relay: 验证出块节点预置的 relay tx ──────────────
	if app.relay != nil {
		// 跟踪区块内每个链的预期 nonce (一个区块可能有多个 relay tx)
		chainExpected := make(map[uint32]uint64)
		for _, txbt := range req.Txs {
			if !app.verifyRelayTx(txbt, chainExpected) {
				LogWithTime("🌈 ProcessProposal: invalid relay tx, REJECT")
				return &abci.ResponseProcessProposal{Status: abci.ResponseProcessProposal_REJECT}, nil
			}
		}
	}

	status := app.ProcessTx(req.Txs, req.Height, req.Time)
	return &abci.ResponseProcessProposal{Status: status}, nil
}

// verifyRelayTx 验证单笔 relay tx (ProcessProposal 调用, chainExpected 跟踪区块内 nonce)
func (app *TeeChain) verifyRelayTx(txbt []byte, chainExpected map[uint32]uint64) bool {
	txbox := new(model.Tx)
	if err := protoio.ReadMessage(bytes.NewBuffer(txbt), txbox); err != nil {
		return true
	}

	call := new(model.SysCall)
	if err := protoio.ReadMessage(bytes.NewBuffer(txbox.Call), call); err != nil {
		return true
	}

	rs, ok := call.Payload.(*model.SysCall_RelaySync)
	if !ok {
		return true // 非 relay tx, 放行
	}
	if rs.RelaySync == nil {
		LogWithTime("Relay reject: nil RelaySync payload")
		return false
	}

	ev := relay.FromRelaySysCall(rs.RelaySync)
	// 获取已提交 DB 的 last + 区块内预期值取最大
	dbLast, dbErr := app.relay.LastNonce(ev.ChainID)
	if dbErr != nil {
		LogWithTime("Relay reject:", dbErr)
		return false
	}
	expected := dbLast + 1
	if blkExpected, exists := chainExpected[ev.ChainID]; exists && blkExpected > expected {
		expected = blkExpected
	}
	if ev.Nonce != expected {
		LogWithTime("Relay reject:", fmt.Sprintf("chain=%d nonce=%d expected=%d (db=%d)", ev.ChainID, ev.Nonce, expected, dbLast))
		return false
	}
	// 更新区块内预期值 (下一个 relay tx 的 nonce 必须 +1)
	chainExpected[ev.ChainID] = expected + 1
	return true
}

func (app *TeeChain) FinalizeBlock(_ context.Context, req *abci.RequestFinalizeBlock) (*abci.ResponseFinalizeBlock, error) {
	// Iterate over Tx in current block
	app.onGoingBlock = store.SECRET_DB.NewTransaction()
	respTxs, err := app.FinalizeTx(req.Txs, app.onGoingBlock, req.Height, req.Time, req.ProposerAddress)
	if err != nil {
		app.onGoingBlock.Rollback()
		app.onGoingBlock = nil
		return nil, err
	}

	// Sync validator updates to consensus
	var validatorUpdates []abci.ValidatorUpdate
	if app.onGoingValidators != nil {
		validatorUpdates = app.onGoingValidators
		ss58 := []string{}
		for _, v := range app.onGoingValidators {
			ss58 = append(ss58, teecrypto.PubKeyFromByte(v.PubKey.GetEd25519()).SS58())
		}
		util.LogWithPurple("Validator updates", ss58)
	}

	// save proposer of currut block
	app.currProposerAddress = req.ProposerAddress
	app.lastProposerWasMe.Store(bytes.Equal(req.ProposerAddress, app.myProposerAddress()))

	app.state.Height = req.Height
	response := &abci.ResponseFinalizeBlock{
		TxResults:        respTxs,
		AppHash:          app.state.Hash(),
		ValidatorUpdates: validatorUpdates,
	}

	LogWithTime("📦 Finalize Block =>", util.Green+" "+fmt.Sprint(req.Height)+" "+util.Reset)
	return response, nil
}

// Commit the application state
func (app *TeeChain) Commit(_ context.Context, _ *abci.RequestCommit) (*abci.ResponseCommit, error) {
	defer func() {
		app.onGoingBlock = nil
	}()
	if app.onGoingBlock == nil {
		return nil, fmt.Errorf("no ongoing block to commit")
	}
	if err := app.onGoingBlock.Commit(); err != nil {
		return nil, err
	}

	app.onGoingValidators = nil
	err := saveAppState(&app.state)
	if err != nil {
		return nil, err
	}

	app.lastCommitAt = time.Now()
	LogWithTime("💤 Commit")
	util.LogWithGreen("END BLOCK  ", "--------------------------------------------------------------")

	// 异步通知 ApplyToChain 可以提交新交易到区块
	app.notifyApplyToChain()
	return &abci.ResponseCommit{}, nil
}

func (app *TeeChain) ExtendVote(_ context.Context, req *abci.RequestExtendVote) (*abci.ResponseExtendVote, error) {
	// 检查 DKG 是否初始化
	if app.dkg == nil || app.dkg.Signer == nil {
		return &abci.ResponseExtendVote{VoteExtension: []byte("")}, nil
	}

	// 构造投票数据用于 TEE 证明
	voteData := make([]byte, 16)
	binary.BigEndian.PutUint64(voteData[0:8], uint64(req.Height))
	copy(voteData[8:16], req.Hash[:8])

	// 创建 TEE 调用
	teeCall := &model.TeeCall{
		Tx: &model.TeeCall_Text{Text: voteData},
	}

	// 生成 TEE 证明
	signer := app.dkg.Signer.ToSigner(teecrypto.SuiteEd25519)
	if err := teetypes.IssueReport(signer, teeCall); err != nil {
		LogWithTime("💊 ExtendVote: IssueReport error: " + err.Error())
		return &abci.ResponseExtendVote{VoteExtension: []byte("")}, nil
	}

	// 序列化 TEE 证明
	voteExtension, err := protoio.MarshalDeterministic(teeCall)
	if err != nil {
		LogWithTime("💊 ExtendVote: Marshal error: " + err.Error())
		return &abci.ResponseExtendVote{VoteExtension: []byte("")}, nil
	}

	LogWithTime("💊 Issue TEE report")
	return &abci.ResponseExtendVote{VoteExtension: voteExtension}, nil
}

func (app *TeeChain) VerifyVoteExtension(_ context.Context, req *abci.RequestVerifyVoteExtension) (*abci.ResponseVerifyVoteExtension, error) {
	if len(req.VoteExtension) == 0 {
		return &abci.ResponseVerifyVoteExtension{Status: abci.ResponseVerifyVoteExtension_ACCEPT}, nil
	}

	// 解析 TEE 证明
	teeCall := &model.TeeCall{}
	if err := proto.Unmarshal(req.VoteExtension, teeCall); err != nil {
		LogWithTime("💊 VerifyVoteExtension: Unmarshal error: " + err.Error())
		return &abci.ResponseVerifyVoteExtension{Status: abci.ResponseVerifyVoteExtension_REJECT}, nil
	}

	// 验证 TEE 证明
	result, err := teetypes.VerifyReport(teeCall)
	if err != nil {
		LogWithTime("💊 VerifyVoteExtension: VerifyReport error: " + err.Error())
		return &abci.ResponseVerifyVoteExtension{Status: abci.ResponseVerifyVoteExtension_REJECT}, nil
	}

	// 只对当前高度附近的投票检查时间戳有效性（5分钟内）
	// 历史区块的投票扩展可能已经过期，但仍然需要验证其签名有效性
	// if time.Now().Unix()-teeCall.Time > 300 {
	// 	LogWithTime("💊 VerifyVoteExtension: timestamp too old")
	// 	return &abci.ResponseVerifyVoteExtension{Status: abci.ResponseVerifyVoteExtension_REJECT}, nil
	// }

	// 记录验证结果
	_ = result // TeeVerifyResult 包含 CodeSigner, CodeSignature 等信息
	LogWithTime("💊 Verify TEE report")
	return &abci.ResponseVerifyVoteExtension{Status: abci.ResponseVerifyVoteExtension_ACCEPT}, nil
}
