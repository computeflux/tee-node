package gov

import (
	"bytes"

	"github.com/wetee-dao/ink.go/util"
	"github.com/wetee-dao/tee-dsecret/pkg/model"
)

func (d GovQuery) Proposal(id uint32) (util.Option[ProposalWithID], error) {
	prop, err := d.proposals.Get(d.api.GetTxn(), id)
	if err != nil {
		return util.NewNone[ProposalWithID](), err
	}
	if prop == nil {
		return util.NewNone[ProposalWithID](), ErrInvalidProposal
	}

	// 填充 ID（StoreList 不自动存储 key）
	p := ProposalWithID{
		ID:       id,
		Proposal: *prop,
	}

	return util.NewSome(p), nil
}

func (d GovQuery) Proposals(startKey util.Option[uint32], size uint32) ([]ProposalWithID, error) {
	var startKeyPtr *uint32
	if startKey.IsSome() {
		startKeyPtr = &startKey.V
	}

	ids, props, err := d.proposals.DescList(d.api.GetTxn(), startKeyPtr, size)
	if err != nil {
		return nil, err
	}

	// 填充每个 proposal 的 ID
	ps := make([]ProposalWithID, len(ids))
	for i := range ids {
		ps[i] = ProposalWithID{
			ID:       ids[i],
			Proposal: props[i],
		}
	}

	return ps, nil
}

func (d GovQuery) ProposalStatus(id uint32) (util.Option[ProposalStatusQuery], error) {
	prop, err := d.proposal(id)
	if err != nil {
		return util.NewNone[ProposalStatusQuery](), err
	}
	if prop.Status.State != ProposalStatusOngoing {
		return util.NewSome(ProposalStatusQuery{
			State:              prop.Status.State,
			BlockHeight:        prop.Status.Block,
			ConfirmedNumber:    0,
			LastConfirmedBlock: 0,
		}), nil
	}

	status, _ := d.calculateProposalStatus(id, prop)
	return util.NewSome(status), nil
}

func (d GovMutation) SubmitProposal(call CallContent, trackID uint32) error {
	height := d.api.GetHeight()
	caller := d.api.GetCaller()

	member, err := d.members.GetOrDefault(d.api.GetTxn(), caller, model.ZeroAmount)
	if err != nil {
		return err
	}
	if member.Int.Sign() == 0 {
		return ErrMemberNotExisted
	}

	track, err := d.tracks.Get(d.api.GetTxn(), trackID)
	if err != nil {
		return err
	}
	if track == nil {
		return ErrNoTrack
	}

	amountInt := call.Amount
	maxBalanceInt := track.MaxBalance
	if amountInt.Cmp(maxBalanceInt.Int) > 0 {
		return ErrLowBalance
	}

	// 使用 StoreList.Insert 自动分配 ID
	p := Proposal{
		Call:        util.NewSome(call),
		TrackID:     trackID,
		Caller:      caller,
		Status:      ProposalStatus{State: ProposalStatusPending},
		SubmitBlock: height,
		Deposit: ProposalDeposit{
			Depositor: caller,
			Amount:    model.ZeroAmount,
			Block:     height,
		},
	}
	id, err := d.proposals.Insert(d.api.GetTxn(), p)
	if err != nil {
		return err
	}
	_ = id // ID 已由 StoreList 自动分配，无需额外处理
	return nil
}

func (d GovMutation) CancelProposal(proposalID uint32) error {
	caller := d.api.GetCaller()
	prop, err := d.proposal(proposalID)
	if err != nil {
		return err
	}
	if prop.Status.State != ProposalStatusPending {
		return ErrInvalidProposalStatus
	}
	if !bytes.Equal(prop.Caller.V, caller.V) {
		return ErrInvalidProposalCaller
	}
	prop.Status = ProposalStatus{State: ProposalStatusCanceled}
	return d.proposals.Update(d.api.GetTxn(), proposalID, prop)
}

func (d GovMutation) DepositProposal(proposalID uint32, amount model.Amount) error {
	height := d.api.GetHeight()
	caller := d.api.GetCaller()

	prop, err := d.proposal(proposalID)
	if err != nil {
		return err
	}
	if prop.Status.State != ProposalStatusPending {
		return ErrInvalidProposalStatus
	}
	track, err := d.track(prop.TrackID)
	if err != nil {
		return err
	}
	if height > prop.SubmitBlock+int64(track.PreparePeriod) {
		return ErrInvalidDepositTime
	}

	depositInt := track.DecisionDeposit
	if amount.Int.Cmp(depositInt.Int) < 0 {
		return ErrInvalidDeposit
	}

	// Deduct deposit from caller balance
	callerBalance, err := d.members.GetOrDefault(d.api.GetTxn(), caller, model.ZeroAmount)
	if err != nil {
		return err
	}
	if callerBalance.Int.Cmp(amount.Int) < 0 {
		return ErrLowBalance
	}
	callerBalance = model.AmountSub(callerBalance, amount)
	if err := d.members.Set(d.api.GetTxn(), caller, callerBalance); err != nil {
		return err
	}

	prop.Deposit = ProposalDeposit{Depositor: caller, Amount: amount, Block: height}
	prop.Status = ProposalStatus{State: ProposalStatusOngoing}
	return d.proposals.Update(d.api.GetTxn(), proposalID, prop)
}

func (d GovMutation) ExecProposal(proposalID uint32) error {
	height := d.api.GetHeight()
	prop, err := d.proposal(proposalID)
	if err != nil {
		return err
	}

	if prop.Status.State != ProposalStatusOngoing {
		return ErrPropNotOngoing
	}

	status, _ := d.calculateProposalStatus(proposalID, prop)
	track, err := d.track(prop.TrackID)
	if err != nil {
		return err
	}
	end := prop.Deposit.Block + int64(track.MaxDeciding)

	switch status.State {
	case ProposalStatusRejected:
		prop.Status = ProposalStatus{State: ProposalStatusRejected, Block: end}
		return d.proposals.Update(d.api.GetTxn(), proposalID, prop)
	case ProposalStatusConfirming:
		return ErrProposalNotConfirmed
	case ProposalStatusOngoing:
		return ErrProposalNotConfirmed
	}

	// ProposalStatusConfirmed - 检查决策期
	if height < end+int64(track.MinEnactmentPeriod) {
		return ErrProposalInDecision
	}

	// 执行提案中的调用并记录结果
	result := ProposalResult{}
	if prop.Call.IsSome() {
		call, _ := prop.Call.UnWrap()
		res, err := d.api.Call(model.UniAddr{}, model.ContractCall{
			Name:   call.Contract,
			Method: call.Selector[:],
			Args:   call.Args,
		})
		if err != nil {
			// 记录错误但不返回，提案仍标记为已批准
			result.ExecError = util.NewSome([]byte(err.Error()))
		} else {
			result.Result = util.NewSome(res)
		}

		// 保存执行结果
		if err := d.proposalResults.Set(d.api.GetTxn(), proposalID, result); err != nil {
			return err
		}
	}

	prop.Status = ProposalStatus{State: ProposalStatusApproved, Block: height}
	return d.proposals.Update(d.api.GetTxn(), proposalID, prop)
}
