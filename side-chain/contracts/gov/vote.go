package gov

import (
	"bytes"
	"math/big"

	"github.com/wetee-dao/ink.go/util"
	"github.com/wetee-dao/tee-dsecret/pkg/model"
)

// Vote 获取单个投票
func (d GovQuery) Vote(proposalID uint32, index uint32) (util.Option[Vote], error) {
	v, err := d.votes.Get(d.api.GetTxn(), proposalID, index)
	if err != nil {
		return util.NewNone[Vote](), err
	}
	if v == nil {
		return util.NewNone[Vote](), ErrInvalidVote
	}
	return util.NewSome(*v), nil
}

// VotesOfUser 获取用户的所有投票引用（分页）
func (d GovQuery) VotesOfUser(user model.UniAddr, startKey util.Option[uint32], size uint32) ([]Vote, error) {
	var startKeyPtr *uint32
	if startKey.IsSome() {
		startKeyPtr = &startKey.V
	}

	_, refs, err := d.votesOfUser.DescList(d.api.GetTxn(), user, startKeyPtr, size)
	if err != nil {
		return nil, err
	}

	votes := make([]Vote, 0, len(refs))
	for _, ref := range refs {
		vote, err := d.vote(ref.ProposalID, ref.VoteIndex)
		if err != nil {
			return nil, err
		}
		votes = append(votes, vote)
	}

	return votes, nil
}

// Votes 获取提案的所有投票（分页）
func (d GovQuery) Votes(proposalID uint32, startKey util.Option[uint32], size uint32) ([]Vote, error) {
	var startKeyPtr *uint32
	if startKey.IsSome() {
		startKeyPtr = &startKey.V
	}
	_, votes, err := d.votes.DescList(d.api.GetTxn(), proposalID, startKeyPtr, size)
	if err != nil {
		return nil, err
	}
	return votes, nil
}

// burnVoteForProposal 投票：从账户销毁 VOTE，并同步减少 totalIssuance（与 Mint 对称）。
func (d GovMutation) burnVoteForProposal(account model.UniAddr, amount model.Amount) error {
	if amount.Int.Sign() <= 0 {
		return ErrLowBalance
	}
	member, err := d.members.GetOrDefault(d.api.GetTxn(), account, model.ZeroAmount)
	if err != nil {
		return err
	}
	if member.Int.Cmp(amount.Int) < 0 {
		return ErrLowBalance
	}
	member = model.AmountSub(member, amount)
	total, err := d.totalIssuance.GetOrDefault(d.api.GetTxn(), model.ZeroAmount)
	if err != nil {
		return err
	}
	if total.Int.Cmp(amount.Int) < 0 {
		return ErrLowBalance
	}
	total = model.AmountSub(total, amount)
	if err := d.members.Set(d.api.GetTxn(), account, member); err != nil {
		return err
	}
	return d.totalIssuance.Set(d.api.GetTxn(), total)
}

// refundVoteForCancel 取消投票：向账户退回本票对应的 VOTE，并恢复 totalIssuance（冲销投票时的销毁）。
func (d GovMutation) refundVoteForCancel(account model.UniAddr, amount model.Amount) error {
	if amount.Int.Sign() <= 0 {
		return nil
	}
	member, err := d.members.GetOrDefault(d.api.GetTxn(), account, model.ZeroAmount)
	if err != nil {
		return err
	}
	member = model.AmountAdd(member, amount)
	total, err := d.totalIssuance.GetOrDefault(d.api.GetTxn(), model.ZeroAmount)
	if err != nil {
		return err
	}
	total = model.AmountAdd(total, amount)
	if err := d.members.Set(d.api.GetTxn(), account, member); err != nil {
		return err
	}
	return d.totalIssuance.Set(d.api.GetTxn(), total)
}

// SubmitVote 提交投票：销毁 lockAmount 数量的治理代币作为本票权重（ABI 参数名保持 lockAmount）。
func (d GovMutation) SubmitVote(proposalID uint32, opinionYes bool, lockAmount model.Amount) error {
	height := d.api.GetHeight()
	caller := d.api.GetCaller()

	member, err := d.members.GetOrDefault(d.api.GetTxn(), caller, model.ZeroAmount)
	if err != nil {
		return err
	}
	if member.Int.Sign() == 0 {
		return ErrMemberNotExisted
	}

	prop, err := d.proposal(proposalID)
	if err != nil {
		return err
	}
	if prop.Status.State != ProposalStatusOngoing {
		return ErrPropNotOngoing
	}
	track, err := d.track(prop.TrackID)
	if err != nil {
		return err
	}
	if height > prop.Deposit.Block+int64(track.MaxDeciding) {
		return ErrInvalidVoteTime
	}

	if member.Int.Cmp(lockAmount.Int) < 0 {
		return ErrLowBalance
	}

	if err := d.burnVoteForProposal(caller, lockAmount); err != nil {
		return err
	}

	one := big.NewInt(1)
	vote := Vote{
		ProposalID: proposalID,
		Caller:     caller,
		Pledge:     lockAmount,
		OpinionYes: opinionYes,
		VoteWeight: model.Amount{Int: one},
		VoteBlock:  height,
	}

	index, err := d.votes.Insert(d.api.GetTxn(), proposalID, vote)
	if err != nil {
		return err
	}

	vote.Index = index
	if err := d.votes.Update(d.api.GetTxn(), proposalID, index, vote); err != nil {
		return err
	}

	if _, err := d.votesOfUser.Insert(d.api.GetTxn(), caller, VoteRef{
		ProposalID: proposalID,
		VoteIndex:  index,
	}); err != nil {
		return err
	}

	return nil
}

// CancelVote 取消投票（提案 Ongoing）：退还本票 Pledge 的 VOTE、恢复 totalIssuance，并标记 Deleted。
func (d GovMutation) CancelVote(proposalID uint32, index uint32) error {
	caller := d.api.GetCaller()
	vote, err := d.vote(proposalID, index)
	if err != nil {
		return err
	}
	if vote.Deleted {
		return ErrInvalidVoteStatus
	}
	if !bytes.Equal(vote.Caller.V, caller.V) {
		return ErrInvalidVoteUser
	}
	prop, err := d.proposal(proposalID)
	if err != nil {
		return err
	}
	if prop.Status.State != ProposalStatusOngoing {
		return ErrPropNotOngoing
	}

	if err := d.refundVoteForCancel(caller, vote.Pledge); err != nil {
		return err
	}

	vote.Deleted = true
	return d.votes.Update(d.api.GetTxn(), proposalID, index, vote)
}

// calculateProposalStatus 计算提案状态
// 返回提案状态码（uint8）
//
// 提案状态流转：
//
//	Pending(0) -> 待存款，提案已提交但尚未支付押金
//	Ongoing(1) -> 进行中，正在投票阶段
//	Confirming(2) -> 确认中，阈值已满足，正在等待确认期结束
//	Confirmed(3) -> 已确认，确认期结束，等待执行
//	Approved(4) -> 已批准，提案通过
//	Rejected(5) -> 已拒绝，提案被否决
//	Canceled(6) -> 已取消，提案被取消
//
// 投票计算公式：
//
//	Approval = yes / (yes + no) * 10000（万分比）
//	Support = support / totalSupply * 10000（万分比）
//
// 通过条件：
//  1. approvalRatio >= minApproval && supportRatio >= minSupport
//  2. 连续满足条件的时间 > confirmPeriod
func (d Gov) calculateProposalStatus(id uint32, prop Proposal) (ProposalStatusQuery, error) {
	track, err := d.track(prop.TrackID)
	if err != nil {
		return ProposalStatusQuery{}, err
	}
	end := prop.Deposit.Block + int64(track.MaxDeciding)
	height := d.api.GetHeight()

	// 使用 ListAll 获取该提案的所有投票
	_, votes, err := d.votes.ListAll(d.api.GetTxn(), id)
	if err != nil {
		return ProposalStatusQuery{}, err
	}

	totalSupply, err := d.totalIssuance.GetOrDefault(d.api.GetTxn(), model.ZeroAmount)
	if err != nil {
		return ProposalStatusQuery{}, err
	}

	// 统计结果
	yes := big.NewInt(0)
	no := big.NewInt(0)
	support := big.NewInt(0)
	all := totalSupply.Int

	// 确认期逻辑
	var lastAchieveBlock int64 = 0
	confirmPeriod := int64(track.ConfirmPeriod)
	decisionPeriod := int64(track.DecisionPeriod)
	confirmed := 0

	for _, vote := range votes {
		if vote.Deleted {
			continue
		}

		// 使用投票时的相对区块偏移量计算阈值
		// x = voteBlock - depositBlock，表示从投票期开始后的第几个区块
		voteBlock := vote.VoteBlock
		voteOffset := voteBlock - prop.Deposit.Block
		minApproval := uint64(track.MinApproval.Y(voteOffset))
		minSupport := uint64(track.MinSupport.Y(voteOffset))

		// 累计投票
		pledge := vote.Pledge.Int
		weightedPledge := new(big.Int).Mul(pledge, vote.VoteWeight.Int)
		support.Add(support, pledge)

		if vote.OpinionYes {
			yes.Add(yes, weightedPledge)
		} else {
			no.Add(no, weightedPledge)
		}

		// 计算比率（万分比）
		totalVotes := new(big.Int).Add(yes, no)
		if totalVotes.Sign() > 0 && all.Sign() > 0 {
			// Approval = yes / (yes + no) * 10000
			approvalRatio := new(big.Int).Mul(yes, big.NewInt(10000))
			approvalRatio.Div(approvalRatio, totalVotes)

			// Support = support / all * 10000
			supportRatio := new(big.Int).Mul(support, big.NewInt(10000))
			supportRatio.Div(supportRatio, all)

			// 检查是否满足阈值
			if approvalRatio.Uint64() >= minApproval && supportRatio.Uint64() >= minSupport {
				// 检查确认期
				if lastAchieveBlock > 0 && voteBlock-lastAchieveBlock > confirmPeriod {
					// 已确认，等待执行
					return ProposalStatusQuery{
						State:              ProposalStatusConfirmed,
						BlockHeight:        height,
						ConfirmedNumber:    uint32(confirmed),
						LastConfirmedBlock: lastAchieveBlock,
					}, nil
				}

				// 记录首次满足阈值的区块
				if lastAchieveBlock == 0 {
					lastAchieveBlock = voteBlock
					confirmed++
				}
			} else {
				// 不满足阈值，重置
				lastAchieveBlock = 0
			}
		}
	}

	// 最后一次投票到现在也需要验证一下是否符合阈值，还需要推算出来已经符合的期限
	// 因为阈值在不断下降，可能在最后一次投票之后就已经满足条件
	if len(votes) > 0 && lastAchieveBlock == 0 {
		// 使用最后一次投票时的累计票数计算当前比率
		totalVotes := new(big.Int).Add(yes, no)
		if totalVotes.Sign() > 0 && all.Sign() > 0 {
			approvalRatio := new(big.Int).Mul(yes, big.NewInt(10000))
			approvalRatio.Div(approvalRatio, totalVotes)

			supportRatio := new(big.Int).Mul(support, big.NewInt(10000))
			supportRatio.Div(supportRatio, all)

			approvalRatioU64 := approvalRatio.Uint64()
			supportRatioU64 := supportRatio.Uint64()

			// 如果当前比率大于0，计算阈值曲线何时下降到可以满足条件
			if approvalRatioU64 > 0 && supportRatioU64 > 0 {
				currentOffset := height - prop.Deposit.Block

				// 使用反推函数计算达到当前比率所需的区块偏移量
				// 需要同时满足 approval 和 support，取较大的偏移量
				approvalOffset := track.MinApproval.InverseY(uint32(approvalRatioU64), currentOffset)
				supportOffset := track.MinSupport.InverseY(uint32(supportRatioU64), currentOffset)

				// 取较大的偏移量（需要同时满足两个条件）
				requiredOffset := approvalOffset
				if supportOffset > requiredOffset {
					requiredOffset = supportOffset
				}

				// 如果所需偏移量在当前区块之前，说明已经满足条件
				if requiredOffset < currentOffset {
					lastAchieveBlock = prop.Deposit.Block + requiredOffset
					confirmed++
				}
			}
		}
	}

	// 检查当前区块与最后满足阈值区块之间是否已超过确认期
	if lastAchieveBlock > 0 && height-lastAchieveBlock > confirmPeriod {
		// 检查是否有足够时间完成确认期（必须在 decisionPeriod 结束前）
		if lastAchieveBlock+confirmPeriod <= end+decisionPeriod {
			// 已确认，等待执行
			return ProposalStatusQuery{
				State:              ProposalStatusConfirmed,
				BlockHeight:        height,
				ConfirmedNumber:    uint32(confirmed),
				LastConfirmedBlock: lastAchieveBlock,
			}, nil
		}
		// 确认期时间不够，拒绝
		return ProposalStatusQuery{
			State:              ProposalStatusRejected,
			BlockHeight:        height,
			ConfirmedNumber:    uint32(confirmed),
			LastConfirmedBlock: 0,
		}, nil
	}

	// 超过截止区块，拒绝
	if height > end {
		return ProposalStatusQuery{
			State:              ProposalStatusRejected,
			BlockHeight:        height,
			ConfirmedNumber:    uint32(confirmed),
			LastConfirmedBlock: 0,
		}, nil
	}

	// 正在确认中，检查是否有足够时间完成确认期
	if lastAchieveBlock > 0 {
		if lastAchieveBlock+confirmPeriod > end+decisionPeriod {
			// 确认期时间不够，拒绝
			return ProposalStatusQuery{
				State:              ProposalStatusRejected,
				BlockHeight:        height,
				ConfirmedNumber:    uint32(confirmed),
				LastConfirmedBlock: 0,
			}, nil
		}
		return ProposalStatusQuery{
			State:              ProposalStatusConfirming,
			BlockHeight:        height,
			ConfirmedNumber:    uint32(confirmed),
			LastConfirmedBlock: lastAchieveBlock,
		}, nil
	}

	// 进行中
	return ProposalStatusQuery{
		State:              ProposalStatusOngoing,
		BlockHeight:        height,
		ConfirmedNumber:    uint32(confirmed),
		LastConfirmedBlock: lastAchieveBlock,
	}, nil
}
