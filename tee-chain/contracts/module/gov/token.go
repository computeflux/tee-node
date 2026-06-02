package gov

import (
	"encoding/hex"
	"fmt"

	"github.com/computeflux/tee-provider/tee-chain/contracts/store"
)

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
	return d.members.Set(d.api.GetTxn(), to, receiver)
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

	return d.totalIssuance.Set(d.api.GetTxn(), store.AmountAdd(total, value))
}
