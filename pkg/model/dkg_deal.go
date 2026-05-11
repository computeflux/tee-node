package model

import (
	"encoding/json"
	"fmt"

	"github.com/pkg/errors"
	"go.dedis.ch/kyber/v4"
	pedersen "go.dedis.ch/kyber/v4/share/dkg/pedersen"
	"go.dedis.ch/kyber/v4/suites"
)

type ConsensusMsg struct {
	Sponsor          *Validator
	DealBundle       *DealBundle
	Epoch            uint32
	EpochTime        int64
	ShareCommits     KyberPoints
	OldValidators    []*Validator
	Validators       []*Validator
	ConsensusNodeNum int
}

type DealBundle struct {
	*pedersen.DealBundle
}

func (d DealBundle) MarshalJSON() ([]byte, error) {
	public := d.Public
	points := make([][]byte, len(public))
	for i, p := range public {
		b, err := p.MarshalBinary()
		if err != nil {
			return nil, fmt.Errorf("MarshalBinary public error: %w", err)
		}
		points[i] = b
	}

	return json.Marshal(struct {
		DealerIndex uint32
		Deals       []pedersen.Deal
		Public      [][]byte
		SessionID   []byte
		Signature   []byte
	}{
		DealerIndex: d.DealerIndex,
		Deals:       d.Deals,
		Public:      points,
		SessionID:   d.SessionID,
		Signature:   d.Signature,
	})
}

func (d *DealBundle) UnmarshalJSON(bt []byte) error {
	deal := struct {
		DealerIndex uint32
		Deals       []pedersen.Deal
		Public      [][]byte
		SessionID   []byte
		Signature   []byte
	}{}
	err := json.Unmarshal(bt, &deal)
	if err != nil {
		return errors.Wrap(err, "Deal UnmarshalJSON")
	}

	public := deal.Public
	points := make([]kyber.Point, len(public))
	suite := suites.MustFind("Ed25519")
	for i, p := range public {
		point := suite.Point()
		err := point.UnmarshalBinary(p)
		if err != nil {
			return errors.Wrap(err, "Deal UnmarshalJSON")
		}

		points[i] = point
	}

	d.DealBundle = &pedersen.DealBundle{
		DealerIndex: deal.DealerIndex,
		Deals:       deal.Deals,
		Public:      points,
		SessionID:   deal.SessionID,
		Signature:   deal.Signature,
	}

	return nil

}

type KyberPoints struct {
	Public []kyber.Point
}

func (d KyberPoints) MarshalJSON() ([]byte, error) {
	public := d.Public
	points := make([][]byte, len(public))
	for i, p := range public {
		b, err := p.MarshalBinary()
		if err != nil {
			return nil, fmt.Errorf("MarshalBinary public error: %w", err)
		}
		points[i] = b
	}

	return json.Marshal(points)
}

func (d *KyberPoints) UnmarshalJSON(bt []byte) error {
	public := [][]byte{}
	err := json.Unmarshal(bt, &public)
	if err != nil {
		return errors.Wrap(err, "Deal UnmarshalJSON")
	}

	points := make([]kyber.Point, len(public))
	suite := suites.MustFind("Ed25519")
	for i, p := range public {
		point := suite.Point()
		err := point.UnmarshalBinary(p)
		if err != nil {
			return errors.Wrap(err, "Deal UnmarshalJSON")
		}

		points[i] = point
	}

	d.Public = points
	return nil

}

type JustificationBundle struct {
	DealerIndex    uint32
	Justifications []Justification
	// SessionID of the current run
	SessionID []byte
	// Signature over the hash of the whole bundle
	Signature []byte
}

type Justification struct {
	ShareIndex uint32
	Share      []byte
}

func JustificationToProtocol(deal *pedersen.JustificationBundle) (*JustificationBundle, error) {
	js := deal.Justifications
	newJs := make([]Justification, len(js))
	for i, p := range js {
		b, err := p.Share.MarshalBinary()
		if err != nil {
			return nil, fmt.Errorf("MarshalBinary public error: %w", err)
		}
		newJs[i] = Justification{
			ShareIndex: p.ShareIndex,
			Share:      b,
		}
	}

	return &JustificationBundle{
		DealerIndex:    deal.DealerIndex,
		Justifications: newJs,
		SessionID:      deal.SessionID,
		Signature:      deal.Signature,
	}, nil
}

func ProtocolToJustification(suite suites.Suite, deal *JustificationBundle) (*pedersen.JustificationBundle, error) {
	js := deal.Justifications
	newJs := make([]pedersen.Justification, len(js))
	for i, p := range js {
		point := suite.Scalar()
		err := point.UnmarshalBinary(p.Share)
		if err != nil {
			return nil, fmt.Errorf("unmarshal commitment: %w", err)
		}

		newJs[i] = pedersen.Justification{
			ShareIndex: p.ShareIndex,
			Share:      point,
		}
	}

	return &pedersen.JustificationBundle{
		DealerIndex:    deal.DealerIndex,
		Justifications: newJs,
		SessionID:      deal.SessionID,
		Signature:      deal.Signature,
	}, nil
}
