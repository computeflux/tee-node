package gov

import (
	"encoding/hex"
	"fmt"
	"math/big"

	"github.com/computeflux/tee-provider/tee-chain/runtime/store"
)

// emitBalanceChange 在余额写入点发射统一的 balance_change 事件。
// attrs：kind（gov/ai/merchant）、address（UniAddr 的 T:hex(V) 编码，resolver 侧再转展示地址）、
// delta（有符号十进制，扣减为负）。delta 为 *big.Int（Amount 是 U256/256 位），
// 用 String() 输出避免 int64 截断。
// 仅 append 内存切片（Runtime.EmitEvent 内部校验 + 上限），失败静默丢弃，不影响交易执行。
func emitBalanceChange(api store.ContractApi, kind string, addr store.UniAddr, delta *big.Int) {
	api.EmitEvent("balance_change", map[string]string{
		"kind":    kind,
		"address": addr.String(),
		"delta":   delta.String(),
	})
}

func (d GovQuery) TotalSupply() (store.Amount, error) {
	return d.totalIssuance.GetOrDefault(d.api.GetTxn(), store.ZeroAmount)
}

func (d GovQuery) BalanceOf(owner store.UniAddr) (store.Amount, error) {
	return d.members.GetOrDefault(d.api.GetTxn(), owner, store.ZeroAmount)
}

func (d GovMutation) Transfer(to store.UniAddr, value store.Amount) error {
	enabled, err := d.transferEnabled.GetOrDefault(d.api.GetTxn(), false)
	if err != nil {
		return err
	}
	if !enabled {
		return ErrTransferDisabled
	}
	return d.transfer(d.api.GetCaller(), to, value)
}

func (d GovMutation) TransferFrom(from, to store.UniAddr, amount store.Amount) error {
	enabled, err := d.transferEnabled.GetOrDefault(d.api.GetTxn(), false)
	if err != nil {
		return err
	}
	if !enabled {
		return ErrTransferDisabled
	}

	caller := d.api.GetCaller()
	allowance, err := d.allowances.GetOrDefault(d.api.GetTxn(), allowanceKey(from, caller), store.ZeroAmount)
	if err != nil {
		return err
	}
	if allowance.Int.Cmp(amount.Int) < 0 {
		return ErrInsufficientAllowance
	}
	if err := d.transfer(from, to, amount); err != nil {
		return err
	}
	return d.allowances.Set(d.api.GetTxn(), allowanceKey(from, caller), store.AmountSub(allowance, amount))
}

func (d GovMutation) transfer(from, to store.UniAddr, value store.Amount) error {
	if value.Int.Sign() <= 0 {
		return ErrLowBalance
	}

	sender, err := d.members.GetOrDefault(d.api.GetTxn(), from, store.ZeroAmount)
	if err != nil {
		return err
	}
	receiver, err := d.members.GetOrDefault(d.api.GetTxn(), to, store.ZeroAmount)
	if err != nil {
		return err
	}
	if sender.Int.Sign() == 0 || receiver.Int.Sign() == 0 {
		return ErrMemberNotExisted
	}

	if sender.Int.Cmp(value.Int) < 0 {
		return ErrLowBalance
	}

	sender = store.AmountSub(sender, value)
	receiver = store.AmountAdd(receiver, value)

	if err := d.members.Set(d.api.GetTxn(), from, sender); err != nil {
		return err
	}
	if err := d.members.Set(d.api.GetTxn(), to, receiver); err != nil {
		return err
	}
	emitBalanceChange(d.api, "gov", from, new(big.Int).Neg(value.Int))
	emitBalanceChange(d.api, "gov", to, value.Int)
	return nil
}

func (d GovMutation) Mint(to store.UniAddr, value store.Amount) error {
	if err := d.ensureGov(); err != nil {
		return err
	}

	// 获取当前余额
	balance, err := d.members.GetOrDefault(d.api.GetTxn(), to, store.ZeroAmount)
	if err != nil {
		return err
	}

	fmt.Println("Mint => ", hex.EncodeToString(to.V), balance.String(), value.String())

	// 增加余额
	if err := d.members.Set(d.api.GetTxn(), to, store.AmountAdd(balance, value)); err != nil {
		return err
	}

	// 增加总供应量
	total, err := d.totalIssuance.GetOrDefault(d.api.GetTxn(), store.ZeroAmount)
	if err != nil {
		return err
	}
	if err := d.totalIssuance.Set(d.api.GetTxn(), store.AmountAdd(total, value)); err != nil {
		return err
	}
	emitBalanceChange(d.api, "gov", to, value.Int)
	return nil
}
