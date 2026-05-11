package gov

import (
	"github.com/wetee-dao/ink.go/util"
	"github.com/wetee-dao/tee-dsecret/pkg/model"
)

var (
	ProposalStatusPending    uint8 = 0 // 待存款，提案已提交但尚未支付押金
	ProposalStatusOngoing    uint8 = 1 // 进行中，正在投票阶段
	ProposalStatusConfirming uint8 = 2 // 确认中，阈值已满足，正在等待确认期结束
	ProposalStatusConfirmed  uint8 = 3 // 已确认，确认期结束，等待执行
	ProposalStatusApproved   uint8 = 4 // 已批准，提案通过
	ProposalStatusRejected   uint8 = 5 // 已拒绝，提案被否决
	ProposalStatusCanceled   uint8 = 6 // 已取消，提案被取消
)

// Member 成员账户与余额。
type Member struct {
	Account model.UniAddr // 成员账户地址
	Balance model.Amount  // 成员余额
}

// TrackData 治理轨道参数。
type TrackData struct {
	Name               string       // 轨道名称
	PreparePeriod      uint32       // 准备期（区块数），提案提交后需要等待的准备时间
	MaxDeciding        uint32       // 最大决策期（区块数），投票持续的最大时间
	ConfirmPeriod      uint32       // 确认期（区块数），阈值满足后需要持续的时间才能通过
	DecisionPeriod     uint32       // 决定期（区块数）
	MinEnactmentPeriod uint32       // 最小执行期（区块数），提案通过后到执行的最短时间
	DecisionDeposit    model.Amount // 决定押金金额，提案人需要质押的金额
	MaxBalance         model.Amount // 最大能执行的金额，提案涉及金额的上限
	MinApproval        Curve        // 投票通过阈值曲线，yes/(yes+no) 需达到的比例
	MinSupport         Curve        // 投票参与率曲线，support/totalSupply 需达到的比例
}

// TrackWithID 带ID的轨道数据
type TrackWithID struct {
	ID    uint32    // 轨道ID
	Track TrackData // 轨道数据
}

// CallContent 提案内嵌调用内容。
type CallContent struct {
	Contract []byte       // 目标合约地址
	Selector [4]byte      // 方法选择器
	Args     [][]byte     // 调用参数
	Amount   model.Amount // 转账金额
}

// ProposalStatus 提案状态
type ProposalStatus struct {
	State uint8 // 状态码：0-待存款,1-进行中,2-确认中,3-已确认,4-已批准,5-已拒绝,6-已取消
	Block int64 // 状态变更时的区块号
}

type ProposalStatusQuery struct {
	// 当前的动态状态
	State uint8
	// 当前区块
	BlockHeight int64
	// 尝试确认的次数
	ConfirmedNumber uint32
	// 最后确认区块
	LastConfirmedBlock int64
}

// ProposalDeposit 提案押金信息
type ProposalDeposit struct {
	Depositor model.UniAddr // 押金支付者地址
	Amount    model.Amount  // 押金金额
	Block     int64         // 支付押金的区块号
}

// Proposal 提案数据
type Proposal struct {
	Call        util.Option[CallContent] // 提案调用内容
	TrackID     uint32                   // 所属轨道ID
	Caller      model.UniAddr            // 提案发起人
	Status      ProposalStatus           // 提案状态
	SubmitBlock int64                    // 提交区块
	Deposit     ProposalDeposit          // 押金信息
}

// ProposalWithID 带ID的提案数据
type ProposalWithID struct {
	ID       uint32   // 提案ID
	Proposal Proposal // 提案数据
}

// ProposalResult 提案执行结果
type ProposalResult struct {
	Result    util.Option[[]byte] // 执行返回值
	ExecError util.Option[[]byte] // 执行错误信息
}

// Vote 投票记录
type Vote struct {
	Index       uint32        // 投票索引
	ProposalID  uint32        // 提案ID
	Caller      model.UniAddr // 投票人地址
	Pledge      model.Amount  // 本笔投入的 VOTE 数量：投票时销毁，取消时按此额退回并恢复 totalIssuance（计票权重）
	OpinionYes  bool          // 是否赞成
	VoteWeight  model.Amount  // 投票权重
	VoteBlock   int64         // 投票时的区块号
	Deleted     bool          // 是否已删除；取消投票时先退还本笔 Pledge 对应 VOTE 并恢复 totalIssuance，再移出计票
}

// Spend 支出记录
type Spend struct {
	ID      uint64        // 支出ID
	Caller  model.UniAddr // 发起人地址
	To      model.UniAddr // 接收人地址
	Amount  model.Amount  // 支出金额
	TrackID uint32        // 所属轨道ID
	Payout  bool          // 是否已支付
}

// VoteRef 投票引用，用于记录用户的投票
// 存储在 votesOfUser 中，方便用户查询自己的投票
type VoteRef struct {
	ProposalID uint32 // 提案ID
	VoteIndex  uint32 // 投票索引
}
