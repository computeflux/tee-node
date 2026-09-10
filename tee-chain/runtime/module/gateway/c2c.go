package gateway

import (
	"fmt"

	"github.com/computeflux/tee-provider/tee-chain/runtime/store"
)

// ── c2c 资金原语（GatewaySecret：不进 ABI / dispatch，仅内部模块构造调用）────
//
// c2c 业务（escrow / 放款 / 佣金 / 节点 / 汇率）全部在 module/c2c 合约；
// c2c 合约包内直接构造 GatewaySecret 调用这两个原语（同 txn、失败回滚）：
//   - WithdrawToC2C：扣 providerCredits（商家盈利 → c2c escrow 的扣减端）
//   - AddCredit：给买家加 gateway credits（c2c Release 放款的目标端）
//
// Secret 不对外，调用方只能是内部可信模块，因此不再维护服务账户白名单
//（原 SetC2CService / c2cService 存储已删）。

// WithdrawToC2C 扣减商家 provider 盈利（1e-6 分单位），供 c2c 合约转入 escrow。
// 校验 merchant 名下存在 provider 且盈利充足；不记录 escrow（由 c2c 调用方记账）。
func (g GatewaySecret) WithdrawToC2C(merchant store.UniAddr, amount uint64) error {
	if amount == 0 {
		return ErrZeroAmount
	}
	pidPtr, err := g.providerByOwner.Get(g.api.GetTxn(), merchant)
	if err != nil || pidPtr == nil {
		return ErrProviderNotFound
	}
	pb, err := g.providerCredits.Get(g.api.GetTxn(), *pidPtr)
	if err != nil || pb == nil {
		return ErrProviderNotFound
	}
	if *pb < amount {
		return ErrLowCredit
	}
	*pb -= amount
	if err := g.providerCredits.Set(g.api.GetTxn(), *pidPtr, *pb); err != nil {
		return fmt.Errorf("c2c: debit provider credits: %w", err)
	}
	return nil
}

// AddCredit 给买家加 gateway credits（1e-6 分单位）。
// 由 c2c 合约 Release 放款后调用（Secret 内部信任，无调用者校验）。
func (g GatewaySecret) AddCredit(user store.UniAddr, amount uint64) error {
	if amount == 0 {
		return ErrZeroAmount
	}

	bal, err := g.credits.Get(g.api.GetTxn(), user)
	if err != nil || bal == nil {
		bal = new(uint64)
	}

	*bal += amount
	if err := g.credits.Set(g.api.GetTxn(), user, *bal); err != nil {
		return fmt.Errorf("c2c: credit buyer: %w", err)
	}
	return nil
}
