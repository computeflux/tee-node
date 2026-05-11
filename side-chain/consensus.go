package sidechain

import (
	"context"
	"encoding/binary"
	"fmt"
	"time"

	abci "github.com/cometbft/cometbft/abci/types"
	"github.com/cometbft/cometbft/version"
	"github.com/cosmos/gogoproto/proto"
	"github.com/wetee-dao/tee-dsecret/pkg/model/protoio"

	"github.com/wetee-dao/tee-dsecret/pkg/chains"
	"github.com/wetee-dao/tee-dsecret/pkg/dkg"
	"github.com/wetee-dao/tee-dsecret/pkg/model"
	bftbrigde "github.com/wetee-dao/tee-dsecret/pkg/network/bft-brigde"
	"github.com/wetee-dao/tee-dsecret/pkg/util"
)

const ApplicationVersion = 1

type SideChain struct {
	abci.BaseApplication
	state AppState

	dkg      *dkg.DKG
	p2p      *bftbrigde.BTFReactor
	NodePriv *model.PrivKey

	txCh *model.PersistChan[*model.BlockPartialSign]

	onGoingBlock        *model.Txn
	onGoingValidators   []abci.ValidatorUpdate
	currProposerAddress []byte

	chains map[uint32]*chains.ChainApi
}

func NewSideChain(light bool) (*SideChain, error) {
	state, err := loadAppState()
	if err != nil {
		return nil, err
	}

	c := &SideChain{
		state: state,
	}

	if !light {
		txCh, err := model.NewPersistChan[*model.BlockPartialSign]("back_tx", 1000)
		if err != nil {
			return nil, err
		}
		c.txCh = txCh
	}

	return c, nil
}

func (app *SideChain) Info(_ context.Context, info *abci.RequestInfo) (*abci.ResponseInfo, error) {
	return &abci.ResponseInfo{
		Version:          version.ABCIVersion,
		AppVersion:       ApplicationVersion,
		LastBlockHeight:  app.state.Height,
		LastBlockAppHash: app.state.Hash(),
	}, nil
}

func (app *SideChain) Query(ctx context.Context, query *abci.RequestQuery) (*abci.ResponseQuery, error) {
	util.LogWithGreen("Query")
	resp := abci.ResponseQuery{Key: query.Data}

	return &resp, nil
}

func (app *SideChain) InitChain(_ context.Context, req *abci.RequestInitChain) (*abci.ResponseInitChain, error) {
	util.LogWithGreen("InitChain")
	app.initValidators(req.Validators)
	appHash := app.state.Hash()

	// This parameter can also be set in the genesis file
	if req.ConsensusParams != nil && req.ConsensusParams.Abci != nil {
		req.ConsensusParams.Abci.VoteExtensionsEnableHeight = 1
	}
	return &abci.ResponseInitChain{ConsensusParams: req.ConsensusParams, AppHash: appHash}, nil
}

func (app *SideChain) CheckTx(_ context.Context, req *abci.RequestCheckTx) (*abci.ResponseCheckTx, error) {
	fmt.Println()
	util.LogWithGreen("START BLOCK", "--------------------------------------------------------------")
	LogWithTime("🚀 CheckTx")

	return &abci.ResponseCheckTx{Code: app.checkTx(req.Tx)}, nil
}

func (app *SideChain) PrepareProposal(_ context.Context, req *abci.RequestPrepareProposal) (*abci.ResponsePrepareProposal, error) {
	LogWithTime("🎁 PrepareProposal")

	// Check if the current epoch is valid
	epochTx := app.CheckEpochFromValidator()
	finalProposal := make([][]byte, 0, len(req.Txs)+2)
	if len(epochTx) > 0 {
		finalProposal = append(finalProposal, epochTx)
	}

	// 如果有未提交到主链的交易（同步进行中），只打包 mempool 中的 SyncTxRetry 等非 HubCall 交易，不打包新 HubCall
	if IsHubSyncRuning() {
		util.LogWithYellow("PrepareProposal", "pending sync to main chain, only pack retry/non-hub txs")
		app.PrepareTx(req.Txs, &finalProposal, req.Height, false)
		return &abci.ResponsePrepareProposal{Txs: finalProposal}, nil
	}

	epochStatus := app.GetEpochStatus()
	// Check if it is in the epoch transition phase
	if len(epochTx) == 0 && time.Now().Unix()-int64(epochStatus) > 120 {
		app.PrepareTx(req.Txs, &finalProposal, req.Height, true)
	} else {
		app.PrepareTx(req.Txs, &finalProposal, req.Height, false)
	}

	return &abci.ResponsePrepareProposal{Txs: finalProposal}, nil
}

func (app *SideChain) ProcessProposal(_ context.Context, req *abci.RequestProcessProposal) (*abci.ResponseProcessProposal, error) {
	LogWithTime("🌈 ProcessProposal")

	status := app.ProcessTx(req.Txs)
	return &abci.ResponseProcessProposal{Status: status}, nil
}

func (app *SideChain) FinalizeBlock(_ context.Context, req *abci.RequestFinalizeBlock) (*abci.ResponseFinalizeBlock, error) {
	// Iterate over Tx in current block
	app.onGoingBlock = model.DBINS.NewTransaction()
	respTxs, err := app.FinalizeTx(req.Txs, app.onGoingBlock, req.Height, req.ProposerAddress)
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
			ss58 = append(ss58, model.PubKeyFromByte(v.PubKey.GetEd25519()).SS58())
		}
		util.LogWithPurple("Validator updates", ss58)
	}

	// save proposer of currut block
	app.currProposerAddress = req.ProposerAddress

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
func (app *SideChain) Commit(_ context.Context, _ *abci.RequestCommit) (*abci.ResponseCommit, error) {
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

	LogWithTime("💤 Commit")
	util.LogWithGreen("END BLOCK  ", "--------------------------------------------------------------")

	return &abci.ResponseCommit{}, nil
}

func (app *SideChain) ExtendVote(_ context.Context, req *abci.RequestExtendVote) (*abci.ResponseExtendVote, error) {
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
	signer := app.dkg.Signer.ToSigner()
	if err := model.IssueReport(signer, teeCall); err != nil {
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

func (app *SideChain) VerifyVoteExtension(_ context.Context, req *abci.RequestVerifyVoteExtension) (*abci.ResponseVerifyVoteExtension, error) {
	// 解析 TEE 证明
	teeCall := &model.TeeCall{}
	if err := proto.Unmarshal(req.VoteExtension, teeCall); err != nil {
		LogWithTime("💊 VerifyVoteExtension: Unmarshal error: " + err.Error())
		return &abci.ResponseVerifyVoteExtension{Status: abci.ResponseVerifyVoteExtension_REJECT}, nil
	}

	// 验证 TEE 证明
	result, err := model.VerifyReport(teeCall)
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
