package gov

import (
	"math/big"

	"github.com/computeflux/tee-provider/tee-chain/runtime/store"
)

func (d GovQuery) Members() ([]Member, error) {
	keys, balances, err := d.members.List(d.api.GetTxn())
	if err != nil {
		return nil, err
	}

	out := make([]Member, len(keys))
	for i := range keys {
		out[i] = Member{Account: keys[i], Balance: balances[i]}
	}
	return out, err
}

func (d GovQuery) GetPublicJoin() (bool, error) {
	return d.publicJoin.GetOrDefault(d.api.GetTxn(), false)
}

func (d GovMutation) PublicJoin() error {
	enabled, err := d.publicJoin.GetOrDefault(d.api.GetTxn(), false)
	if err != nil {
		return err
	}

	if !enabled {
		return ErrPublicJoinNotAllowed
	}

	return d.addMember(d.api.GetCaller(), store.ZeroAmount)
}

func (d GovMutation) SetPublicJoin(publicJoin bool) error {
	if err := d.ensureGov(); err != nil {
		return err
	}
	return d.publicJoin.Set(d.api.GetTxn(), publicJoin)
}

func (d GovMutation) Join(newUser store.UniAddr, balance store.Amount) error {
	if err := d.ensureGov(); err != nil {
		return err
	}

	return d.addMember(newUser, balance)
}

func (d GovMutation) Leave() error {
	caller := d.api.GetCaller()
	member, err := d.members.Get(d.api.GetTxn(), caller)
	if err != nil {
		return err
	}
	if member == nil {
		return ErrMemberNotExisted
	}

	if member.Int.Cmp(big.NewInt(0)) > 0 {
		return ErrMemberBalanceNotZero
	}
	return d.removeMember(caller)
}

func (d GovMutation) LeaveWithBurn() error {
	caller := d.api.GetCaller()
	return d.deleteMemberAndBurn(caller, false)
}

func (d GovMutation) Delete(account store.UniAddr) error {
	return d.deleteMemberAndBurn(account, true)
}

func (d Gov) addMember(account store.UniAddr, balance store.Amount) error {
	if len(account.V) == 0 {
		return ErrMemberNotExisted
	}
	if balance.Int.Sign() < 0 {
		return ErrLowBalance
	}
	member, err := d.members.Get(d.api.GetTxn(), account)
	if err != nil {
		return err
	}
	if member != nil {
		return ErrMemberExisted
	}

	if err := d.members.Set(d.api.GetTxn(), account, balance); err != nil {
		return err
	}

	total, err := d.totalIssuance.GetOrDefault(d.api.GetTxn(), store.ZeroAmount)
	if err != nil {
		return err
	}
	return d.totalIssuance.Set(d.api.GetTxn(), store.AmountAdd(total, balance))
}

func (d Gov) deleteMemberAndBurn(account store.UniAddr, requireGov bool) error {
	if requireGov {
		if err := d.ensureGov(); err != nil {
			return err
		}
	}

	member, err := d.members.Get(d.api.GetTxn(), account)
	if err != nil {
		return err
	}
	if member == nil {
		return ErrMemberNotExisted
	}

	total, err := d.totalIssuance.GetOrDefault(d.api.GetTxn(), store.ZeroAmount)
	if err != nil {
		return err
	}

	if err := d.totalIssuance.Set(d.api.GetTxn(), store.AmountSub(total, *member)); err != nil {
		return err
	}
	return d.removeMember(account)
}

func (d Gov) removeMember(account store.UniAddr) error {
	return d.members.Delete(d.api.GetTxn(), account)
}
