// Package c2c 提供 C2C 积分兑换的链上结算合约。
//
// 定位：合约不承载业务匹配，只做“可信锚 + 资金托管 + 受控放款”三件事，
// 与 web2 侧（TEE 内的 c2c 服务 + 验证节点）配合构成纵深防御：
//
//  1. 代码签名版本白名单：治理多签维护“被授权的 c2c 代码指纹（TEE 测量值/签名）”。
//     这是链上的信任锚——只有白名单内的代码版本才被承认。
//  2. 验证节点注册表：所有发起/验证 web2 交易状态、并调用合约放款节点的地址，
//     由治理注册与删除。节点只能为其自身注册的代码版本背书。
//  3. 收款（挂单托管）：商户把挂单 AI 金额存入合约 escrow。
//  4. 受控放款：注册验证节点在“TEE 证明代码 ∈ 节点注册值 + 白名单 + 订单未重复”后，
//     把 AI 从商户 escrow 转给买方。
//
// TEE 证书的真实密码学验证（quote 验签）在链下由验证节点完成（VerifyTEE 返回 TEE 证明的
// 代码三要素 TeeType/CodeSigner/CodeSignature）；合约层基于 TEE 证明在链上验证用户
// CodeSignature（证明放款数据出自被 TEE 证明的代码），并强制白名单策略。
package c2c

import (
	"bytes"
	"crypto/ed25519"
	"encoding/hex"
	"errors"
	"fmt"
	"math/big"
	"strconv"

	"github.com/computeflux/tee-provider/tee-chain/runtime/module/gateway"
	"github.com/computeflux/tee-provider/tee-chain/runtime/store"
	"golang.org/x/crypto/blake2b"
)

var (
	ErrZeroAmount          = errors.New("c2c: amount must be > 0")
	ErrCodeEmpty           = errors.New("c2c: code version empty")
	ErrCodeNotAuthorized   = errors.New("c2c: code version not authorized")
	ErrCodeExists          = errors.New("c2c: code version already authorized")
	ErrNotNode             = errors.New("c2c: caller is not a registered node")
	ErrNodeExists          = errors.New("c2c: node already registered")
	ErrCodeMismatch        = errors.New("c2c: release code version does not match node")
	ErrTeeReportMissing    = errors.New("c2c: tee report missing")
	ErrTeeVerifyFailed     = errors.New("c2c: tee report verify failed")
	ErrCodeSigInvalid      = errors.New("c2c: code signature invalid")
	ErrInsufficientEscrow  = errors.New("c2c: insufficient merchant escrow")
	ErrDuplicateRelease    = errors.New("c2c: order already released")
	ErrInsufficientBalance = errors.New("c2c: insufficient buyer balance")
	ErrZeroRate            = errors.New("c2c: cny rate must be > 0")
	ErrRateExceedsCap      = errors.New("c2c: cny rate exceeds cap")
	ErrUnsupportedCurrency = errors.New("c2c: unsupported currency")
	ErrConversionMismatch  = errors.New("c2c: fiat does not match locked rate")
	ErrOrderLimitExceeded  = errors.New("c2c: order usd out of [min,max]")
	ErrDailyLimitExceeded  = errors.New("c2c: exceeds buyer daily limit")
)

// C2C C2C 积分兑换链上结算合约。
// 治理复用 gov 合约：白名单/节点注册/经济策略的写操作仅接受系统账户调用
// （由 gov.CallAsSudo 以系统身份批量执行），不再单独维护治理地址。
type C2C struct {
	api store.ContractApi

	// authorizedCodes 代码签名版本白名单：hex(codeHash) → true。
	authorizedCodes *store.StoreMapping[string, bool]
	// nodes 验证节点注册表：节点地址 → NodeRecord。
	nodes *store.StoreMapping[store.UniAddr, NodeRecord]
	// merchantEscrow 商户挂单托管余额：商户 → 已锁定 AI。
	merchantEscrow *store.StoreMapping[store.UniAddr, store.Amount]
	// releasedNonces 放款去重：orderNo → true。
	releasedNonces *store.StoreMapping[string, bool]
	// fxConfig 经济策略配置（治理写入，单值）。
	fxConfig *store.StoreValue[FxConfig]
	// buyerDaily 买家日累计额度：买方 → BuyerDaily{day, usd_cents}。
	buyerDaily *store.StoreMapping[store.UniAddr, BuyerDaily]
	// platformBalance 平台佣金累计总额（放点时归平台，单值）。
	platformBalance *store.StoreValue[store.Amount]
}

// C2CQuery 只读接口。
type C2CQuery struct{ C2C }

// C2CMutation 状态变更接口。
type C2CMutation struct{ C2C }

// Init 初始化占位（治理由 gov 合约提供，无独立初始化状态）。
func (m C2CMutation) Init() error { return nil }

// ── ① 代码签名版本白名单（系统账户 / gov 治理） ──────────────

// RegisterCode 授权一个新的 c2c 代码签名版本。
func (m C2CMutation) RegisterCode(codeHash []byte, version string) error {
	if !m.api.CallbySystem() {
		return errors.New("c2c: not authorized (system account required)")
	}
	if len(codeHash) == 0 {
		return ErrCodeEmpty
	}
	key := hexKey(codeHash)
	existed, _ := m.authorizedCodes.Contains(m.api.GetTxn(), key)
	if existed {
		return ErrCodeExists
	}
	return m.authorizedCodes.Set(m.api.GetTxn(), key, true)
}

// RevokeCode 取消授权一个代码版本。
func (m C2CMutation) RevokeCode(codeHash []byte) error {
	if !m.api.CallbySystem() {
		return errors.New("c2c: not authorized (system account required)")
	}
	if len(codeHash) == 0 {
		return ErrCodeEmpty
	}
	return m.authorizedCodes.Delete(m.api.GetTxn(), hexKey(codeHash))
}

// ── ② 验证节点注册表（系统账户 / gov 治理） ───────────────────

// RegisterNode 由治理注册一个验证节点地址，并将其绑定到指定的代码版本。
// 被注册的代码版本必须已在白名单内，保证“节点运行的代码 ∈ 白名单”。
func (m C2CMutation) RegisterNode(node store.UniAddr, codeHash []byte) error {
	if !m.api.CallbySystem() {
		return errors.New("c2c: not authorized (system account required)")
	}
	if existed, _ := m.nodes.Contains(m.api.GetTxn(), node); existed {
		return ErrNodeExists
	}
	if !m.isCodeAuthorized(codeHash) {
		return ErrCodeNotAuthorized
	}
	return m.nodes.Set(m.api.GetTxn(), node, NodeRecord{
		CodeVersion:  codeHash,
		RegisteredAt: uint64(m.api.GetBlockTime()),
		Active:       true,
	})
}

// RemoveNode 删除一个验证节点（系统账户管理；也允许节点自行注销）。
func (m C2CMutation) RemoveNode(node store.UniAddr) error {
	if !m.api.CallbySystem() && !addrEqual(node, m.api.GetCaller()) {
		return errors.New("c2c: not authorized (system account or node self)")
	}
	return m.nodes.Delete(m.api.GetTxn(), node)
}

// ── ③ 收款 / 挂单托管 ─────────────────────────────────────────

// Deposit 商户把挂单 AI 金额存入合约 escrow。
func (m C2CMutation) Deposit(amount store.Amount) error {
	if amount.Cmp(store.ZeroAmount) == 0 {
		return ErrZeroAmount
	}
	merchant := m.api.GetCaller()
	cur, err := m.merchantEscrow.Get(m.api.GetTxn(), merchant)
	if err != nil || cur == nil {
		zero := store.ZeroAmount
		cur = &zero
	}
	*cur = store.AmountAdd(*cur, amount)
	return m.merchantEscrow.Set(m.api.GetTxn(), merchant, *cur)
}

// ── ③.5 商户盈利入 escrow（验证节点代商家操作）────────────────

// DepositFor 由注册验证节点发起：从 gateway 扣减 merchant 的 provider 盈利
// （GatewaySecret.WithdrawToC2C），并记入 c2c 合约 merchant escrow（同一 txn）。
// 供 web2 c2c 服务（节点身份）在商家挂单/补库存时调用。
func (m C2CMutation) DepositFor(merchant store.UniAddr, amount store.Amount) error {
	if amount.Cmp(store.ZeroAmount) == 0 {
		return ErrZeroAmount
	}
	// 调用方必须是已注册的验证节点。
	if existed, _ := m.nodes.Contains(m.api.GetTxn(), m.api.GetCaller()); !existed {
		return ErrNotNode
	}
	units := amountInt64(amount)
	if units <= 0 {
		return ErrZeroAmount
	}
	// 扣减 gateway providerCredits（Secret 构造调用，同 txn 原子）。
	gwSecret := gateway.GatewaySecret{Gateway: *gateway.NewGateway(m.api)}
	if err := gwSecret.WithdrawToC2C(merchant, uint64(units)); err != nil {
		return err
	}
	// 记入 c2c escrow。
	cur, err := m.merchantEscrow.Get(m.api.GetTxn(), merchant)
	if err != nil || cur == nil {
		zero := store.ZeroAmount
		cur = &zero
	}
	*cur = store.AmountAdd(*cur, amount)
	return m.merchantEscrow.Set(m.api.GetTxn(), merchant, *cur)
}

// ── ④ 受控放款 ────────────────────────────────────────────────

// Release 由注册验证节点发起：在“TEE 证明代码 = 节点注册值 + 白名单 + 订单未重复”后，
// 把 AI 从商户 escrow 转给买方。
// 安全模型：用户须携带 TEE quote（TeeReport）、签发时间（TeeTime）与代码对放款数据的签名（CodeSignature）；
// quote 绑定 [签发时间 || 节点 || SCALE(放款数据)]，由验证节点在 TEE 内签发（TeeTime 与节点签名内容中的 Time 同值）；
// 合约用运行时上下文（Caller/TeeType 用户不可伪造）+ 放款数据 + TeeTime 完成 VerifyTEE 验签（含时间窗防重放），
// 返回 TEE 证明的代码三要素，再链上验证用户 CodeSignature——代码版本由 TEE 硬件证明，用户无法伪造。
func (m C2CMutation) Release(sig ReleaseWithSig) error {
	// ① 调用方必须是已注册的验证节点。
	node, err := m.nodes.Get(m.api.GetTxn(), m.api.GetCaller())
	if err != nil || node == nil {
		return ErrNotNode
	}

	r := sig.Data

	// ② TEE 证明：用户必须携带 TEE quote 字节（即 model.TeeCall 的 Report 字段，非整个 TeeCall）。
	// 被证明的数据 Text 为放款数据 r 的 SCALE 编码（节点签发 quote 时绑定同一份数据）；
	// TeeTime 为节点签发时间（节点签名内容中的 Time，伪造需持有 TEE 内私钥）；
	// 完整 TeeCall 由运行时用执行上下文补齐（Caller/TeeType 均为用户不可伪造），
	// 并校验 TeeTime 与区块时间在窗口内（防旧 quote 重放）；
	// 节点侧 VerifyTEE 完成真实 quote 验签，返回 TEE 证明的代码三要素（TeeType/CodeSigner/CodeSignature）——
	// 代码版本由 TEE 硬件证明确定，而非用户声称。
	if len(sig.TeeReport) == 0 {
		return ErrTeeReportMissing
	}
	payload, err := store.ScaleEncode(r)
	if err != nil {
		return err
	}
	tee, err := m.api.VerifyTEE(sig.TeeReport, payload, sig.TeeTime).UnWrap()
	if err != nil {
		return ErrTeeVerifyFailed
	}

	// ③ TEE 证明的代码版本必须与节点注册值一致（节点只能为其自身代码背书）。
	if len(tee.CodeSignature) == 0 || !bytesEqual(node.CodeVersion, tee.CodeSignature) {
		return ErrCodeMismatch
	}

	// ④ 该代码版本必须在白名单内（链上信任锚）。
	if !m.isCodeAuthorized(tee.CodeSignature) {
		return ErrCodeNotAuthorized
	}

	// ⑤ 链上验证用户的 CodeSignature：证明放款数据出自 TEE 证明的代码
	// （代码私钥在 TEE 飞地内，用户无法伪造；签名内容为 Data 的 SCALE 编码，与 TEE 侧签名算法一致）。
	if len(sig.CodeSignature) == 0 {
		return ErrCodeSigInvalid
	}
	if !verifyCodeSig(tee.CodeSigner, payload, sig.CodeSignature) {
		return ErrCodeSigInvalid
	}

	// ⑥ 订单去重。
	if existed, _ := m.releasedNonces.Contains(m.api.GetTxn(), hexOrder(r.OrderNo)); existed {
		return ErrDuplicateRelease
	}

	// ⑦ 积分须为正。
	if r.Amount.Cmp(store.ZeroAmount) == 0 {
		return ErrZeroAmount
	}
	pt := amountInt64(r.Amount)

	// ⑧ 折算校验：用下单时锁定的 LockedRate 复核 fiat，防止节点报假金额。
	if convertCents(pt, r.Currency, r.LockedRate) != r.FiatCents {
		return ErrConversionMismatch
	}

	// ⑨ 读经济策略配置（未由治理配置时为全 0，限额/佣金降级不强制）。
	cfg, err := m.getFxConfigTxn()
	if err != nil {
		return err
	}

	// ⑩ 单笔限额（USD 口径恒为 pt×100）：仅配置时强制。
	usdCents := pt * 100
	if cfg.MinOrderUsdCents > 0 && usdCents < cfg.MinOrderUsdCents {
		return ErrOrderLimitExceeded
	}
	if cfg.MaxOrderUsdCents > 0 && usdCents > cfg.MaxOrderUsdCents {
		return ErrOrderLimitExceeded
	}

	// ⑪ 日累计限额：仅配置时强制；通过则累加当日（跨日自动重置）。
	if cfg.BuyerDailyLimitUsd > 0 {
		day := utcDay(m.api)
		spent, _ := m.buyerDaily.Get(m.api.GetTxn(), r.Buyer)
		daily := int64(0)
		if spent != nil && spent.Day == strconv.FormatInt(day, 10) {
			daily = spent.UsdCents
		}
		if daily+usdCents > cfg.BuyerDailyLimitUsd {
			return ErrDailyLimitExceeded
		}
		if spent == nil {
			spent = &BuyerDaily{Day: strconv.FormatInt(day, 10)}
		}
		spent.Day = strconv.FormatInt(day, 10)
		spent.UsdCents = daily + usdCents
		if err := m.buyerDaily.Set(m.api.GetTxn(), r.Buyer, *spent); err != nil {
			return err
		}
	}

	// ⑫ 佣金：platform = floor(pt × FeePct / 10000)；buyer = pt − commission。
	// 用订单快照费率 r.FeePct（下单时锁定），与 web2 侧一致，不受后续 config 变更影响。
	commission := int64(0)
	if r.FeePct > 0 {
		commission = pt * r.FeePct / 10000
	}
	buyerGet := pt - commission

	// ⑬ 商户 escrow 余额充足。
	escrow, err := m.merchantEscrow.Get(m.api.GetTxn(), r.Merchant)
	if err != nil || escrow == nil || escrow.Cmp(r.Amount) < 0 {
		return ErrInsufficientEscrow
	}

	// ⑭ 扣减商户 escrow。
	*escrow = store.AmountSub(*escrow, r.Amount)
	if err := m.merchantEscrow.Set(m.api.GetTxn(), r.Merchant, *escrow); err != nil {
		return err
	}

	// ⑮ 买方获得（已扣佣金）→ 入 gateway credits（买家消费积分的真源）。
	// 包内构造 GatewaySecret 直接调用（同一 txn、失败回滚）；AddCredit 为 Secret
	// 方法，不进 ABI / dispatch，外部交易无法触达。
	gwSecret := gateway.GatewaySecret{Gateway: *gateway.NewGateway(m.api)}
	if err := gwSecret.AddCredit(r.Buyer, uint64(buyerGet)); err != nil {
		return fmt.Errorf("c2c: credit buyer via gateway: %w", err)
	}

	// ⑯ 平台累积佣金。
	if commission > 0 {
		pb, err := m.platformBalance.Get(m.api.GetTxn())
		if err != nil || pb == nil {
			zero := store.ZeroAmount
			pb = &zero
		}
		*pb = store.AmountAdd(*pb, store.NewU256(big.NewInt(commission)))
		if err := m.platformBalance.Set(m.api.GetTxn(), *pb); err != nil {
			return err
		}
	}

	// ⑰ 去重。
	return m.releasedNonces.Set(m.api.GetTxn(), hexOrder(r.OrderNo), true)
}

// ── ⑤ 经济策略配置（系统账户 / gov 治理）────────────────────

// SetFxConfig 治理更新经济策略（汇率/佣金/限额）。仅系统账户可写
// （由 gov.CallAsSudo 以系统身份发起）。
func (m C2CMutation) SetFxConfig(cfg FxConfig) error {
	if !m.api.CallbySystem() {
		return errors.New("c2c: not authorized (system account required)")
	}
	if cfg.CnyPerPoint <= 0 {
		return ErrZeroRate
	}
	if cfg.MaxCnyPerPoint != 0 && cfg.MaxCnyPerPoint < cfg.CnyPerPoint {
		return ErrRateExceedsCap
	}
	return m.fxConfig.Set(m.api.GetTxn(), cfg)
}

// ── 查询 ──────────────────────────────────────────────────────

// IsCodeAuthorized 判断某代码版本是否在白名单内。
func (q C2CQuery) IsCodeAuthorized(codeHash []byte) (bool, error) {
	if len(codeHash) == 0 {
		return false, nil
	}
	v, _ := q.authorizedCodes.Contains(q.api.GetTxn(), hexKey(codeHash))
	return v, nil
}

// GetNode 读取某节点的注册信息。
func (q C2CQuery) GetNode(node store.UniAddr) (NodeRecord, error) {
	v, err := q.nodes.Get(q.api.GetTxn(), node)
	if err != nil {
		return NodeRecord{}, err
	}
	if v == nil {
		return NodeRecord{}, nil
	}
	return *v, nil
}

// ListNodes 列出全部注册节点。
func (q C2CQuery) ListNodes() ([]NodeRecord, error) {
	_, values, err := q.nodes.List(q.api.GetTxn())
	if err != nil {
		return nil, err
	}
	return values, nil
}

// GetMerchantEscrow 读取商户当前 escrow 余额。
func (q C2CQuery) GetMerchantEscrow(merchant store.UniAddr) (store.Amount, error) {
	v, err := q.merchantEscrow.Get(q.api.GetTxn(), merchant)
	if err != nil {
		return store.ZeroAmount, err
	}
	if v == nil {
		return store.ZeroAmount, nil
	}
	return *v, nil
}

// ── 查询 ──────────────────────────────────────────────────────

// GetFxConfig 读取当前经济策略配置（未配置返回全 0）。
func (q C2CQuery) GetFxConfig() (FxConfig, error) {
	v, err := q.fxConfig.Get(q.api.GetTxn())
	if err != nil {
		return FxConfig{}, err
	}
	if v == nil {
		return FxConfig{}, nil
	}
	return *v, nil
}

// Convert 积分→法币金额（分）。USD：points×100；CNY：points×rate/100（rate 为 ×10000 的汇率）。
func (q C2CQuery) Convert(points int64, currency string) (int64, error) {
	if points <= 0 {
		return 0, ErrZeroAmount
	}
	switch currency {
	case "USD", "":
		return points * 100, nil
	case "CNY":
		cfg, err := q.GetFxConfig()
		if err != nil {
			return 0, err
		}
		if cfg.CnyPerPoint <= 0 {
			return 0, ErrUnsupportedCurrency
		}
		return points * cfg.CnyPerPoint / 100, nil
	default:
		return 0, ErrUnsupportedCurrency
	}
}

// FiatToPoints 法币金额（分）→ 积分（CNY 向上取整）。
func (q C2CQuery) FiatToPoints(fiatCents int64, currency string) (int64, error) {
	if fiatCents <= 0 {
		return 0, ErrZeroAmount
	}
	switch currency {
	case "USD", "":
		return fiatCents / 100, nil
	case "CNY":
		cfg, err := q.GetFxConfig()
		if err != nil {
			return 0, err
		}
		if cfg.CnyPerPoint <= 0 {
			return 0, ErrUnsupportedCurrency
		}
		// points = ceil(fiatCents × 100 / CnyPerPoint)
		return (fiatCents*100 + cfg.CnyPerPoint - 1) / cfg.CnyPerPoint, nil
	default:
		return 0, ErrUnsupportedCurrency
	}
}

// BuyerSpentToday 买家当日累计 USD（分）；无当日记录返回 0。
func (q C2CQuery) BuyerSpentToday(buyer store.UniAddr) (int64, error) {
	v, err := q.buyerDaily.Get(q.api.GetTxn(), buyer)
	if err != nil {
		return 0, err
	}
	if v == nil || v.Day != strconv.FormatInt(utcDay(q.api), 10) {
		return 0, nil
	}
	return v.UsdCents, nil
}

// GetPlatformCommission 读取平台累计佣金。
func (q C2CQuery) GetPlatformCommission() (store.Amount, error) {
	v, err := q.platformBalance.Get(q.api.GetTxn())
	if err != nil {
		return store.ZeroAmount, err
	}
	if v == nil {
		return store.ZeroAmount, nil
	}
	return *v, nil
}

// ── 辅助 ──────────────────────────────────────────────────────

// isCodeAuthorized 合约内部：代码版本是否在白名单。
func (m C2C) isCodeAuthorized(codeHash []byte) bool {
	if len(codeHash) == 0 {
		return false
	}

	v, _ := m.authorizedCodes.Contains(m.api.GetTxn(), hexKey(codeHash))
	return v
}

func hexKey(codeHash []byte) string { return hex.EncodeToString(codeHash) }

func hexOrder(orderNo []byte) string {
	if len(orderNo) == 0 {
		return ""
	}
	return hex.EncodeToString(orderNo)
}

func addrEqual(a, b store.UniAddr) bool {
	return a.T == b.T && bytes.Equal(a.V, b.V)
}

func bytesEqual(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// verifyCodeSig 链上验证代码签名（ed25519，RFC 8032）。
// 算法与 TEE 侧 model.Ed25519SignVerify 完全一致：32 字节公钥长度校验（不可信输入防 panic）
// + 消息 >256 字节时 blake2b-256 预哈希 + ed25519 验签。
// 合约层用标准库直接实现（不 import pkg/model），避免其依赖链（pkg/types/crypto → ego cgo）
// 进入 WASM/TinyGo 构建。
func verifyCodeSig(codeSigner, payload, codeSignature []byte) bool {
	if len(codeSigner) != ed25519.PublicKeySize {
		return false
	}
	msg := payload
	if len(msg) > 256 {
		h := blake2b.Sum256(msg)
		msg = h[:]
	}
	return ed25519.Verify(codeSigner, msg, codeSignature)
}

// getFxConfigTxn 读取 FxConfig（未配置返回全 0，不报错）。
func (m C2C) getFxConfigTxn() (FxConfig, error) {
	v, err := m.fxConfig.Get(m.api.GetTxn())
	if err != nil {
		return FxConfig{}, err
	}
	if v == nil {
		return FxConfig{}, nil
	}
	return *v, nil
}

// convertCents 按锁定汇率把积分折算成法币（分）。USD：pts×100；CNY：pts×rate/100（rate×10000）。
// 不支持的币种返回 -1，供调用方判定为折算不匹配。
func convertCents(points int64, currency string, rate10000 int64) int64 {
	switch currency {
	case "USD", "":
		return points * 100
	case "CNY":
		if rate10000 <= 0 {
			return -1
		}
		return points * rate10000 / 100
	default:
		return -1
	}
}

// utcDay 返回当前 UTC 自然日编号（unix 秒 / 86400），跨日即不同编号。
func utcDay(api store.ContractApi) int64 {
	return api.GetBlockTime() / 86400
}

// amountInt64 把链上金额转 int64（积分/额度量级小，安全）。
func amountInt64(a store.Amount) int64 {
	if a.Int == nil {
		return 0
	}
	return a.Int.Int64()
}
