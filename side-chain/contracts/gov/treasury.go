package gov

import (
	"github.com/centrifuge/go-substrate-rpc-client/v4/types/codec"
	"github.com/wetee-dao/tee-dsecret/pkg/model"
)

func (d GovMutation) Spend(to model.UniAddr, amount model.Amount, trackID uint32) error {
	caller := d.api.GetCaller()

	member, err := d.members.GetOrDefault(d.api.GetTxn(), caller, model.ZeroAmount)
	if err != nil {
		return err
	}
	if member.Int.Sign() == 0 {
		return ErrMemberNotExisted
	}
	if member.Int.Cmp(amount.Int) < 0 {
		return ErrLowBalance
	}

	// Deduct amount from caller and escrow it
	member = model.AmountSub(member, amount)
	if err := d.members.Set(d.api.GetTxn(), caller, member); err != nil {
		return err
	}

	id, err := d.nextSpendIDStore.GetOrDefault(d.api.GetTxn(), uint64(0))
	if err != nil {
		return err
	}

	record := Spend{
		ID:      id,
		Caller:  caller,
		To:      to,
		Amount:  amount,
		TrackID: trackID,
	}

	if err := d.spends.Set(d.api.GetTxn(), id, record); err != nil {
		return err
	}

	if err := d.nextSpendIDStore.Set(d.api.GetTxn(), id+1); err != nil {
		return err
	}

	idbt, err := codec.Encode(id)
	if err != nil {
		return err
	}
	return d.SubmitProposal(CallContent{
		Contract: []byte("gov"),
		Selector: model.MethodToSelector("Payout"),
		Args:     [][]byte{idbt},
		Amount:   amount,
	}, trackID)
}

func (d GovMutation) Payout(spendID uint64) error {
	if err := d.ensureGov(); err != nil {
		return err
	}

	spend, err := d.spends.Get(d.api.GetTxn(), spendID)
	if err != nil {
		return err
	}
	if spend == nil {
		return ErrSpendNotFound
	}
	if spend.Payout {
		return ErrSpendAlreadyExecuted
	}

	// Transfer escrowed amount to the recipient
	recipient, err := d.members.GetOrDefault(d.api.GetTxn(), spend.To, model.ZeroAmount)
	if err != nil {
		return err
	}
	recipient = model.AmountAdd(recipient, spend.Amount)
	if err := d.members.Set(d.api.GetTxn(), spend.To, recipient); err != nil {
		return err
	}

	spend.Payout = true
	return d.spends.Set(d.api.GetTxn(), spend.ID, *spend)
}
