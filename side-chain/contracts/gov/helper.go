package gov

import (
	"bytes"

	"github.com/wetee-dao/tee-dsecret/pkg/model"
)

func (d Gov) ensureGov() error {
	sudo := d.api.GetSudoAccount()
	caller := d.api.GetCaller()

	if sudo.V == nil || !bytes.Equal(sudo.V, caller.V) {
		return ErrMustCallByGov
	}
	return nil
}

func (d Gov) proposal(id uint32) (Proposal, error) {
	prop, err := d.proposals.Get(d.api.GetTxn(), id)
	if err != nil {
		return Proposal{}, err
	}
	if prop == nil {
		return Proposal{}, ErrInvalidProposal
	}
	return *prop, nil
}

func (d Gov) vote(proposalID uint32, index uint32) (Vote, error) {
	v, err := d.votes.Get(d.api.GetTxn(), proposalID, index)
	if err != nil {
		return Vote{}, err
	}
	if v == nil {
		return Vote{}, ErrInvalidVote
	}
	return *v, nil
}

func (d Gov) track(id uint32) (TrackData, error) {
	t, err := d.tracks.Get(d.api.GetTxn(), id)
	if err != nil {
		return TrackData{}, err
	}
	if t == nil {
		return TrackData{}, ErrNoTrack
	}
	return *t, nil
}

func allowanceKey(owner, spender model.UniAddr) model.UniAddr {
	// 组合 owner 和 spender 作为 allowance 的 key
	key := make([]byte, len(owner.V)+len(spender.V))
	copy(key, owner.V)
	copy(key[len(owner.V):], spender.V)
	return model.UniAddr{T: 0, V: key}
}
