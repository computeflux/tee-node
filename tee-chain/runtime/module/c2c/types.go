package c2c

import (
	"github.com/computeflux/tee-provider/tee-chain/runtime/store"
)

// NodeRecord 验证节点记录（以节点地址为 key 存储）。
// CodeVersion 为该节点被授权运行的 c2c 代码指纹（TEE 测量值 / 签名）；
// release 时节点只能为其自身注册的 CodeVersion 背书，确保“节点运行的代码 ∈ 白名单”。
// scale-codec-gen:generate
type NodeRecord struct {
	CodeVersion  []byte
	RegisteredAt uint64
	Active       bool
}

// FxConfig 经济策略配置（治理写入，链上权威源）。所有数值整数缩放（ABI 不支持 float）。
//
//	CnyPerPoint        : 1 积分 = 多少 CNY，×10000（4 位小数）。7.20 → 72000。
//	MaxCnyPerPoint     : CNY 汇率上限，×10000。50.00 → 500000。
//	PlatformFeePct     : 平台佣金百分比，×100（2 位小数）。4.00 → 400。
//	MinOrderUsdCents   : 单笔最小额度（USD 分）。5 USD → 500。
//	MaxOrderUsdCents   : 单笔最大额度（USD 分）。1000 USD → 100000。
//	BuyerDailyLimitUsd : 买家日累计额度（USD 分）。3000 USD → 300000。
//
// scale-codec-gen:generate
type FxConfig struct {
	CnyPerPoint        int64
	MaxCnyPerPoint     int64
	PlatformFeePct     int64
	MinOrderUsdCents   int64
	MaxOrderUsdCents   int64
	BuyerDailyLimitUsd int64
}

// BuyerDaily 买家某日（UTC 自然日）累计 USD（分），用于日累计限额。
// 以买方地址为 key 单条记录，跨日由结算逻辑重置。
// scale-codec-gen:generate
type BuyerDaily struct {
	Day      string
	UsdCents int64
}

// Release 一笔放款请求：由注册验证节点发起，经 TEE 验证后将 AI 从商户 escrow 转给买方。
//
//	Merchant    商户（其 escrow 被扣减）
//	Buyer       买方（接收 AI，扣佣金）
//	Amount      放款 AI（积分）数量（从商户 escrow 全额扣减）
//	CodeVersion 处理该交易的 c2c 代码指纹（须与节点注册值一致且已在白名单）
//	OrderNo      订单号（去重 + 审计）
//	FiatCents    买家实付法币（分）：合约据此校验折算
//	Currency     币种 USD/CNY
//	LockedRate   下单时锁定的汇率（×10000）：合约据此校验折算（保留锁价，不受汇率变动影响）
//	FeePct       下单时锁定的佣金百分比（×100）：合约据此计算佣金并归平台
//
// scale-codec-gen:generate
type Release struct {
	Merchant   store.UniAddr
	Buyer      store.UniAddr
	Amount     store.Amount
	OrderNo    []byte
	FiatCents  int64
	Currency   string
	LockedRate int64
	FeePct     int64
}

// ReleaseWithSig 携带 TEE 证明的放款请求：
//
//	Data          放款数据（由 TEE 内代码签名）
//	TeeReport     验证节点在 TEE 内签发的 quote 字节（即 model.TeeCall 的 Report 字段，非整个 TeeCall）：
//	              quote 绑定 [Time || Caller || marshal(TeeCall_Text{Text: SCALE(Data)})]（节点用交易签名同一密钥签发），
//	              完整 model.TeeCall 由合约运行时用执行上下文补齐（Caller/Time/TeeType 均为用户不可伪造），
//	              节点侧 VerifyTEE 完成真实 quote 验签后，返回 TEE 证明的代码三要素
//	              （TeeType/CodeSigner/CodeSignature）——代码版本不再由用户口头声称，而由 TEE 硬件证明
//	CodeSignature  TEE 证明代码对 Data 的 SCALE 编码的 ed25519 签名：
//	              合约用 VerifyTEE 返回的 CodeSigner 验证，证明放款数据出自被 TEE 证明的代码
//	              （私钥在 TEE 飞地内，用户无法伪造）
//	TeeTime       节点签发 quote 时的时间戳（秒），即节点对 [Time || Caller || Text] 签名中的 Time：
//	              节点侧与 quote 内嵌时间戳同值；合约校验其与区块时间的偏差在窗口内（防旧 quote 重放）
//
// scale-codec-gen:generate
type ReleaseWithSig struct {
	Data          Release
	TeeReport     []byte
	CodeSignature []byte
	TeeTime       int64
}
