package gov

import (
	"github.com/wetee-dao/ink.go/util"
)

func (d GovQuery) Tracks() ([]TrackWithID, error) {
	ids, tracks, err := d.tracks.List(d.api.GetTxn())
	if err != nil {
		return nil, err
	}

	ts := make([]TrackWithID, len(ids))
	for i := range ids {
		ts[i] = TrackWithID{ID: ids[i], Track: tracks[i]}
	}

	return ts, nil
}

func (d GovQuery) Track(id uint32) (util.Option[TrackData], error) {
	track, err := d.tracks.Get(d.api.GetTxn(), id)
	if err != nil {
		return util.NewNone[TrackData](), err
	}
	if track == nil {
		return util.NewNone[TrackData](), ErrNoTrack
	}
	return util.NewSome(*track), nil
}

func (d GovQuery) DefaultTrack() (uint32, error) {
	return d.defaultTrack.GetOrDefault(d.api.GetTxn(), 0)
}

func (d GovMutation) AddTrack(track TrackData) error {
	if err := d.ensureGov(); err != nil {
		return err
	}
	next, err := d.nextTrackIDStore.GetOrDefault(d.api.GetTxn(), uint32(0))
	if err != nil {
		return err
	}
	if err := d.tracks.Set(d.api.GetTxn(), next, track); err != nil {
		return err
	}
	return d.nextTrackIDStore.Set(d.api.GetTxn(), next+1)
}

func (d GovMutation) SetDefaultTrack(trackID uint32) error {
	if err := d.ensureGov(); err != nil {
		return err
	}
	track, err := d.tracks.Get(d.api.GetTxn(), trackID)
	if err != nil {
		return err
	}
	if track == nil {
		return ErrNoTrack
	}
	return d.defaultTrack.Set(d.api.GetTxn(), trackID)
}
