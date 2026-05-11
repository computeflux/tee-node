package gov

import (
	"errors"
	"math/big"

	"github.com/centrifuge/go-substrate-rpc-client/v4/types"
	"github.com/wetee-dao/tee-dsecret/pkg/model"
)

var (
	// ErrMustCallByGov 必须由治理合约调用
	ErrMustCallByGov = errors.New("must call by gov")
	// ErrPublicJoinNotAllowed 不允许公开加入
	ErrPublicJoinNotAllowed = errors.New("public join not allowed")
	// ErrMemberExisted 成员已存在
	ErrMemberExisted = errors.New("member existed")
	// ErrMemberNotExisted 成员不存在
	ErrMemberNotExisted = errors.New("member not existed")
	// ErrMemberBalanceNotZero 成员余额不为零
	ErrMemberBalanceNotZero = errors.New("member balance not zero")
	// ErrLowBalance 余额不足
	ErrLowBalance = errors.New("low balance")
	// ErrTransferDisabled 转账已禁用
	ErrTransferDisabled = errors.New("transfer disabled")
	// ErrInsufficientAllowance 授权额度不足
	ErrInsufficientAllowance = errors.New("insufficient allowance")
	// ErrNoTrack 轨道不存在
	ErrNoTrack = errors.New("no track")
	// ErrInvalidProposal 无效的提案
	ErrInvalidProposal = errors.New("invalid proposal")
	// ErrInvalidProposalStatus 无效的提案状态
	ErrInvalidProposalStatus = errors.New("invalid proposal status")
	// ErrInvalidProposalCaller 无效的提案调用者
	ErrInvalidProposalCaller = errors.New("invalid proposal caller")
	// ErrInvalidDepositTime 无效的存款时间
	ErrInvalidDepositTime = errors.New("invalid deposit time")
	// ErrInvalidDeposit 无效的存款金额
	ErrInvalidDeposit = errors.New("invalid deposit")
	// ErrPropNotOngoing 提案未在进行中
	ErrPropNotOngoing = errors.New("proposal not ongoing")
	// ErrInvalidVoteTime 无效的投票时间
	ErrInvalidVoteTime = errors.New("invalid vote time")
	// ErrInvalidVote 无效的投票
	ErrInvalidVote = errors.New("invalid vote")
	// ErrInvalidVoteUser 无效的投票用户
	ErrInvalidVoteUser = errors.New("invalid vote user")
	// ErrInvalidVoteStatus 无效的投票状态
	ErrInvalidVoteStatus = errors.New("invalid vote status")
	// ErrProposalNotConfirmed 提案未确认
	ErrProposalNotConfirmed = errors.New("proposal not confirmed")
	// ErrProposalInDecision 提案决策中
	ErrProposalInDecision = errors.New("proposal in decision")
	// ErrSpendNotFound 支出未找到
	ErrSpendNotFound = errors.New("spend not found")
	// ErrSpendAlreadyExecuted 支出已执行
	ErrSpendAlreadyExecuted = errors.New("spend already executed")
)

// Gov 治理合约结构体，包含所有存储状态
type Gov struct {
	// api 合约API接口
	api model.ContractApi
	// members 成员账户映射 (keyPfx: member_)
	members *model.StoreMapping[model.UniAddr, model.Amount]
	// publicJoin 是否允许公开加入 (key: public_join)
	publicJoin *model.StoreValue[bool]
	// transferEnabled 转账是否启用 (key: transfer)
	transferEnabled *model.StoreValue[bool]
	// totalIssuance 总发行量 (key: issuance)
	totalIssuance *model.StoreValue[model.Amount]
	// defaultTrack 默认轨道ID (key: default_track)
	defaultTrack *model.StoreValue[uint32]
	// nextSpendIDStore 下一个支出ID (key: next_spend)
	nextSpendIDStore *model.StoreValue[uint64]
	// nextTrackIDStore 下一个轨道ID (key: next_track)
	nextTrackIDStore *model.StoreValue[uint32]
	// allowances 授权额度映射 (keyPfx: allowance_)
	allowances *model.StoreMapping[model.UniAddr, model.Amount]
	// tracks 轨道数据映射 (keyPfx: track_)
	tracks *model.StoreMapping[uint32, TrackData] `store:"keyPfx:tracks_v2"`
	// proposals 提案列表 (自增ID管理)
	proposals *model.StoreList[uint32, Proposal] `store:"keyPfx:proposals_v6"`
	// votesOfUser 用户投票列表映射 (keyPfx: votes_of_user_v1)
	// K1 = 用户地址，Ix = 投票索引，可按用户查询其所有投票
	votesOfUser *model.StoreList2D[model.UniAddr, uint32, VoteRef] `store:"keyPfx:votes_of_user_v1"`
	// votes 投票数据二维列表 (keyPfx: votes_v2)
	// K1 = proposalID, Ix = 内层索引，可按提案获取所有投票
	votes *model.StoreList2D[uint32, uint32, Vote] `store:"keyPfx:votes_v4"`
	// spends 支出数据映射 (keyPfx: spend_)
	spends *model.StoreMapping[uint64, Spend]
	// proposalResults 提案执行结果映射 (keyPfx: proposal_result_)
	proposalResults *model.StoreMapping[uint32, ProposalResult]
}

// GovQuery 治理合约查询接口
type GovQuery struct {
	Gov
}

// GovMutation 治理合约变更接口
type GovMutation struct {
	Gov
}

// Init 初始化治理合约，设置默认值和初始轨道
func (d GovMutation) Init() error {
	total := types.NewU256(*big.NewInt(0))
	publicJoin := true
	defaultTrack := TrackData{
		Name:               "default",
		PreparePeriod:      100,
		MaxDeciding:        100,
		ConfirmPeriod:      100,
		DecisionPeriod:     100,
		MinEnactmentPeriod: 0,
		DecisionDeposit:    model.Amount{Int: big.NewInt(100)},
		MaxBalance:         model.Amount{Int: big.NewInt(1_000_000)},
		MinApproval:        NewLinearDecreasingCurve(10000, 50, 100), // 从 100% 递减到 0.5%，长度 100 区块
		MinSupport:         NewLinearDecreasingCurve(10000, 50, 100), // 从 100% 递减到 0.5%，长度 100 区块
	}

	if err := d.publicJoin.Set(d.api.GetTxn(), publicJoin); err != nil {
		return err
	}

	if err := d.transferEnabled.Set(d.api.GetTxn(), false); err != nil {
		return err
	}

	if err := d.totalIssuance.Set(d.api.GetTxn(), total); err != nil {
		return err
	}

	if err := d.nextSpendIDStore.Set(d.api.GetTxn(), 0); err != nil {
		return err
	}

	if err := d.nextTrackIDStore.Set(d.api.GetTxn(), 0); err != nil {
		return err
	}

	if err := d.tracks.Set(d.api.GetTxn(), 0, defaultTrack); err != nil {
		return err
	}

	if err := d.defaultTrack.Set(d.api.GetTxn(), 0); err != nil {
		return err
	}

	if err := d.nextTrackIDStore.Set(d.api.GetTxn(), 1); err != nil {
		return err
	}

	return nil
}
