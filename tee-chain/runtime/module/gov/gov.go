package gov

import (
	"bytes"
	"errors"
	"fmt"
	"math/big"

	"github.com/computeflux/tee-provider/tee-chain/runtime/store"
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
	// ErrSudoDisabled sudo 已被永久禁用
	ErrSudoDisabled = errors.New("sudo disabled")
	// ErrNotSudo 调用者不是 sudo 账户
	ErrNotSudo = errors.New("not sudo")
	// ErrSudoNotSet sudo 账户未设置
	ErrSudoNotSet = errors.New("sudo not set")
)

// Gov 治理合约结构体，包含所有存储状态
// scale-codec-gen:skip
type Gov struct {
	// api 合约API接口
	api store.ContractApi
	// members 成员账户映射 (keyPfx: member_)
	members *store.StoreMapping[store.UniAddr, store.Amount]
	// publicJoin 是否允许公开加入 (key: public_join)
	publicJoin *store.StoreValue[bool]
	// transferEnabled 转账是否启用 (key: transfer)
	transferEnabled *store.StoreValue[bool]
	// totalIssuance 总发行量 (key: issuance)
	totalIssuance *store.StoreValue[store.Amount]
	// defaultTrack 默认轨道ID (key: default_track)
	defaultTrack *store.StoreValue[uint32]
	// nextSpendIDStore 下一个支出ID (key: next_spend)
	nextSpendIDStore *store.StoreValue[uint64]
	// nextTrackIDStore 下一个轨道ID (key: next_track)
	nextTrackIDStore *store.StoreValue[uint32]
	// allowances 授权额度映射 (keyPfx: allowance_)
	allowances *store.StoreMapping[store.UniAddr, store.Amount]
	// tracks 轨道数据映射 (keyPfx: track_)
	tracks *store.StoreMapping[uint32, TrackData] `store:"keyPfx:tracks"`
	// proposals 提案列表 (自增ID管理)
	proposals *store.StoreList[uint32, Proposal] `store:"keyPfx:proposals"`
	// votesOfUser 用户投票列表映射 (keyPfx: votes_of_user_v1)
	// K1 = 用户地址，Ix = 投票索引，可按用户查询其所有投票
	votesOfUser *store.StoreList2D[store.UniAddr, uint32, VoteRef] `store:"keyPfx:votes_of_user"`
	// votes 投票数据二维列表 (keyPfx: votes_v2)
	// K1 = proposalID, Ix = 内层索引，可按提案获取所有投票
	votes *store.StoreList2D[uint32, uint32, Vote] `store:"keyPfx:votes"`
	// spends 支出数据映射 (keyPfx: spend_)
	spends *store.StoreMapping[uint64, Spend]
	// proposalResults 提案执行结果映射 (keyPfx: proposal_result_)
	proposalResults *store.StoreMapping[uint32, ProposalResult]
	// sudo 超级管理员账户地址，可用于系统调用（key: sudo）
	sudo *store.StoreValue[store.UniAddr]
	// sudoDisabled sudo 是否已被永久禁用（key: sudo_disabled）
	sudoDisabled *store.StoreValue[bool]
	// pendingRTUpgrade stores pending runtime upgrade info (JSON-encoded []byte)
	runtimeVersion  *store.StoreValue[uint32]                          // key: runtime_version
	runtimeMetas    *store.StoreMapping[uint32, PendingRuntimeUpgrade] // keyPfx: runtime_meta_
	runtimeAtHeight *store.StoreMapping[uint64, uint32]                // keyPfx: runtime_at_height_
	// dkgPub DKG 公钥账户地址，可用于系统调用（key: dkg_pub）
	dkgPub *store.StoreValue[store.UniAddr]
}

// GovQuery 治理合约查询接口
// scale-codec-gen:skip
type GovQuery struct {
	Gov
}

// GovMutation 治理合约变更接口
// scale-codec-gen:skip
type GovMutation struct {
	Gov
}

// Init 初始化治理合约，设置默认值和初始轨道
func (d GovMutation) Init() error {
	total := store.NewU256(big.NewInt(0))
	publicJoin := true
	defaultTrack := TrackData{
		Name:               "default",
		PreparePeriod:      100,
		MaxDeciding:        100,
		ConfirmPeriod:      100,
		DecisionPeriod:     100,
		MinEnactmentPeriod: 0,
		DecisionDeposit:    store.Amount{Int: big.NewInt(100)},
		MaxBalance:         store.Amount{Int: big.NewInt(1_000_000)},
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

// ── Sudo ─────────────────────────────────────────────────────

// ensureSudo 检查调用者是否为 sudo 账户且 sudo 未被禁用。
func (d Gov) ensureSudo() error {
	sudoAddr, err := d.sudo.Get(d.api.GetTxn())
	if err != nil || sudoAddr == nil {
		return ErrSudoNotSet
	}
	disabled, err := d.sudoDisabled.Get(d.api.GetTxn())
	if err != nil {
		return err
	}
	if disabled != nil && *disabled {
		return ErrSudoDisabled
	}
	caller := d.api.GetCaller()
	if !bytes.Equal(sudoAddr.V, caller.V) || sudoAddr.T != caller.T {
		return ErrNotSudo
	}
	return nil
}

// SetSudo 设置 sudo 账户（仅系统调用）。
func (d GovMutation) SetSudo(addr store.UniAddr) error {
	if !d.api.CallbySystem() {
		return errors.New("must call by system")
	}
	return d.sudo.Set(d.api.GetTxn(), addr)
}

// DisableSudo 永久禁用 sudo 权限（仅 sudo 账户可调用，不可逆）。
func (d GovMutation) DisableSudo() error {
	if err := d.ensureSudo(); err != nil {
		return err
	}
	return d.sudoDisabled.Set(d.api.GetTxn(), true)
}

// CallAsSudo 以 sudo 权限批量执行合约调用（仅 sudo 账户，未禁用时可调用）。
func (d GovMutation) CallAsSudo(calls []store.InContractCall) error {
	if err := d.ensureSudo(); err != nil {
		return err
	}
	for i, call := range calls {
		if len(call.Name) == 0 {
			return fmt.Errorf("sudo call %d: empty contract name", i)
		}
		if len(call.Method) == 0 {
			return fmt.Errorf("sudo call %d: empty method selector", i)
		}
		_, err := d.api.Call(d.api.GetCaller(), &call)
		if err != nil {
			return fmt.Errorf("sudo call %d (%s): %w", i, string(call.Name), err)
		}
	}
	return nil
}

// Sudo 查询当前 sudo 账户地址。
func (q GovQuery) Sudo() (*store.UniAddr, error) {
	return q.sudo.Get(q.api.GetTxn())
}

// IsSudoDisabled 查询 sudo 是否已被永久禁用。
func (q GovQuery) IsSudoDisabled() (bool, error) {
	v, err := q.sudoDisabled.Get(q.api.GetTxn())
	if err != nil || v == nil {
		return false, err
	}
	return *v, nil
}

// RuntimeUpgrade 运行时升级：授权、验证、存储版本元数据并更新当前版本。
// 系统调用（dkgpub）直接通过；非系统调用需 sudo 权限。
// 整个升级流程完全在合约内完成，无需链上层额外处理。
func (d GovMutation) RuntimeUpgrade(upgrade PendingRuntimeUpgrade) error {
	if !d.api.CallbySystem() {
		if err := d.ensureSudo(); err != nil {
			return err
		}
	}
	if len(upgrade.WasmBytes) == 0 {
		return errors.New("runtime upgrade: wasm_bytes is empty")
	}

	// 版本检查：必须为当前版本 + 1
	cver, err := d.runtimeVersion.Get(d.api.GetTxn())
	if err != nil {
		return err
	}
	currentVersion := uint32(0)
	if cver != nil {
		currentVersion = *cver
	}
	if upgrade.Version != currentVersion+1 {
		return errors.New("runtime upgrade: invalid version (must be current + 1)")
	}

	// 记录激活高度
	height := uint64(d.api.GetHeight())
	upgrade.ActivationHeight = height

	// 存储版本元数据
	if err := d.runtimeMetas.Set(d.api.GetTxn(), upgrade.Version, upgrade); err != nil {
		return err
	}

	// 更新当前版本
	if err := d.runtimeVersion.Set(d.api.GetTxn(), upgrade.Version); err != nil {
		return err
	}

	// 记录版本激活高度
	if err := d.runtimeAtHeight.Set(d.api.GetTxn(), height, upgrade.Version); err != nil {
		return err
	}

	return nil
}

// RuntimeVersion 查询当前运行时版本号。
func (q GovQuery) RuntimeVersion() (*uint32, error) {
	return q.runtimeVersion.Get(q.api.GetTxn())
}

// RuntimeMeta 查询指定版本的元数据。
func (q GovQuery) RuntimeMeta(version uint32) (*PendingRuntimeUpgrade, error) {
	return q.runtimeMetas.Get(q.api.GetTxn(), version)
}

// RuntimeAtHeight 查询指定高度激活的版本号。
func (q GovQuery) RuntimeAtHeight(height int64) (*uint32, error) {
	return q.runtimeAtHeight.Get(q.api.GetTxn(), uint64(height))
}
