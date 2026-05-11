package gov

import (
	"encoding/hex"
	"fmt"

	"github.com/wetee-dao/tee-dsecret/pkg/model"
)

func (d GovQuery) TotalSupply() (model.Amount, error) {
	return d.totalIssuance.GetOrDefault(d.api.GetTxn(), model.ZeroAmount)
}

func (d GovQuery) BalanceOf(owner model.UniAddr) (model.Amount, error) {
	return d.members.GetOrDefault(d.api.GetTxn(), owner, model.ZeroAmount)
}

func (d GovMutation) Transfer(to model.UniAddr, value model.Amount) error {
	enabled, err := d.transferEnabled.GetOrDefault(d.api.GetTxn(), false)
	if err != nil {
		return err
	}
	if !enabled {
		return ErrTransferDisabled
	}
	return d.transfer(d.api.GetCaller(), to, value)
}

func (d GovMutation) TransferFrom(from, to model.UniAddr, amount model.Amount) error {
	enabled, err := d.transferEnabled.GetOrDefault(d.api.GetTxn(), false)
	if err != nil {
		return err
	}
	if !enabled {
		return ErrTransferDisabled
	}

	caller := d.api.GetCaller()
	allowance, err := d.allowances.GetOrDefault(d.api.GetTxn(), allowanceKey(from, caller), model.ZeroAmount)
	if err != nil {
		return err
	}
	if allowance.Int.Cmp(amount.Int) < 0 {
		return ErrInsufficientAllowance
	}
	if err := d.transfer(from, to, amount); err != nil {
		return err
	}
	return d.allowances.Set(d.api.GetTxn(), allowanceKey(from, caller), model.AmountSub(allowance, amount))
}

func (d GovMutation) transfer(from, to model.UniAddr, value model.Amount) error {
	if value.Int.Sign() <= 0 {
		return ErrLowBalance
	}

	sender, err := d.members.GetOrDefault(d.api.GetTxn(), from, model.ZeroAmount)
	if err != nil {
		return err
	}
	receiver, err := d.members.GetOrDefault(d.api.GetTxn(), to, model.ZeroAmount)
	if err != nil {
		return err
	}
	if sender.Int.Sign() == 0 || receiver.Int.Sign() == 0 {
		return ErrMemberNotExisted
	}

	if sender.Int.Cmp(value.Int) < 0 {
		return ErrLowBalance
	}

	sender = model.AmountSub(sender, value)
	receiver = model.AmountAdd(receiver, value)

	if err := d.members.Set(d.api.GetTxn(), from, sender); err != nil {
		return err
	}
	return d.members.Set(d.api.GetTxn(), to, receiver)
}

func (d GovMutation) Mint(to model.UniAddr, value model.Amount) error {
	if err := d.ensureGov(); err != nil {
		return err
	}

	// 获取当前余额
	balance, err := d.members.GetOrDefault(d.api.GetTxn(), to, model.ZeroAmount)
	if err != nil {
		return err
	}

	fmt.Println("Mint => ", hex.EncodeToString(to.V), balance.String(), value.String())

	// 增加余额
	if err := d.members.Set(d.api.GetTxn(), to, model.AmountAdd(balance, value)); err != nil {
		return err
	}

	// 增加总供应量
	total, err := d.totalIssuance.GetOrDefault(d.api.GetTxn(), model.ZeroAmount)
	if err != nil {
		return err
	}

	return d.totalIssuance.Set(d.api.GetTxn(), model.AmountAdd(total, value))
}
