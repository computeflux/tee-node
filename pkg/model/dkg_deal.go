package model

import (
	"encoding/json"
	"fmt"

	"github.com/pkg/errors"
	"go.dedis.ch/kyber/v4"
	pedersen "go.dedis.ch/kyber/v4/share/dkg/pedersen"
	"go.dedis.ch/kyber/v4/suites"
)

// DeepCopyIf returns a deep copy of any value by JSON round-tripping.
// This is used to avoid sharing mutable state across goroutines.
func DeepCopyIf[T any](src T) (*T, error) {
	bytes, err := json.Marshal(src)
	if err != nil {
		return nil, fmt.Errorf("DeepCopyIf Marshal: %w", err)
	}
	dst := new(T)
	err = json.Unmarshal(bytes, dst)
	if err != nil {
		return nil, fmt.Errorf("DeepCopyIf Unmarshal: %w", err)
	}
	return dst, nil
}

type ConsensusMsg struct {
	Sponsor          *Validator
	DealBundles      map[SuiteType]*DealBundle
	Epoch            uint32
	EpochTime        int64
	ShareCommits     KyberPoints
	OldValidators    []*Validator
	Validators       []*Validator
	ConsensusNodeNum int
}

// jsonConsensusMsg is the JSON-serializable form of ConsensusMsg (string keys).
type jsonConsensusMsg struct {
	Sponsor          *Validator             `json:"Sponsor,omitempty"`
	DealBundles      map[string]*DealBundle `json:"DealBundles,omitempty"`
	Epoch            uint32                 `json:"Epoch"`
	EpochTime        int64                  `json:"EpochTime"`
	ShareCommits     KyberPoints            `json:"ShareCommits"`
	OldValidators    []*Validator           `json:"OldValidators,omitempty"`
	Validators       []*Validator           `json:"Validators,omitempty"`
	ConsensusNodeNum int                    `json:"ConsensusNodeNum"`
}

func (c ConsensusMsg) MarshalJSON() ([]byte, error) {
	stringBundles := make(map[string]*DealBundle, len(c.DealBundles))
	for st, bundle := range c.DealBundles {
		stringBundles[st.String()] = bundle
	}
	return json.Marshal(jsonConsensusMsg{
		Sponsor:          c.Sponsor,
		DealBundles:      stringBundles,
		Epoch:            c.Epoch,
		EpochTime:        c.EpochTime,
		ShareCommits:     c.ShareCommits,
		OldValidators:    c.OldValidators,
		Validators:       c.Validators,
		ConsensusNodeNum: c.ConsensusNodeNum,
	})
}

func (c *ConsensusMsg) UnmarshalJSON(bt []byte) error {
	var raw jsonConsensusMsg
	if err := json.Unmarshal(bt, &raw); err != nil {
		return err
	}
	c.Sponsor = raw.Sponsor
	c.DealBundles = make(map[SuiteType]*DealBundle, len(raw.DealBundles))
	for stStr, bundle := range raw.DealBundles {
		st, err := SuiteTypeFromString(stStr)
		if err != nil {
			continue // skip unknown suite types
		}
		c.DealBundles[st] = bundle
	}
	c.Epoch = raw.Epoch
	c.EpochTime = raw.EpochTime
	c.ShareCommits = raw.ShareCommits
	c.OldValidators = raw.OldValidators
	c.Validators = raw.Validators
	c.ConsensusNodeNum = raw.ConsensusNodeNum
	return nil
}

// ConsensusRespMsg carries responses for multiple suites.
type ConsensusRespMsg struct {
	Responses map[SuiteType][]byte
}

// jsonConsensusRespMsg is the JSON-serializable form of ConsensusRespMsg.
type jsonConsensusRespMsg struct {
	Responses map[string][]byte `json:"responses"`
}

func (c ConsensusRespMsg) MarshalJSON() ([]byte, error) {
	stringResp := make(map[string][]byte, len(c.Responses))
	for st, data := range c.Responses {
		stringResp[st.String()] = data
	}
	return json.Marshal(jsonConsensusRespMsg{Responses: stringResp})
}

func (c *ConsensusRespMsg) UnmarshalJSON(bt []byte) error {
	var raw jsonConsensusRespMsg
	if err := json.Unmarshal(bt, &raw); err != nil {
		return err
	}
	c.Responses = make(map[SuiteType][]byte, len(raw.Responses))
	for stStr, data := range raw.Responses {
		st, err := SuiteTypeFromString(stStr)
		if err != nil {
			continue
		}
		c.Responses[st] = data
	}
	return nil
}

type DealBundle struct {
	*pedersen.DealBundle
	Suite string `json:"suite,omitempty"`
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
		Suite       string `json:"suite,omitempty"`
	}{
		DealerIndex: d.DealerIndex,
		Deals:       d.Deals,
		Public:      points,
		SessionID:   d.SessionID,
		Signature:   d.Signature,
		Suite:       d.Suite,
	})
}

func (d *DealBundle) UnmarshalJSON(bt []byte) error {
	deal := struct {
		DealerIndex uint32
		Deals       []pedersen.Deal
		Public      [][]byte
		SessionID   []byte
		Signature   []byte
		Suite       string `json:"suite,omitempty"`
	}{}
	err := json.Unmarshal(bt, &deal)
	if err != nil {
		return errors.Wrap(err, "Deal UnmarshalJSON")
	}

	public := deal.Public
	points := make([]kyber.Point, len(public))
	suiteStr := deal.Suite
	if suiteStr == "" {
		suiteStr = "ed25519"
	}
	suiteType, _ := SuiteTypeFromString(suiteStr)
	suite := NewSuite(suiteType)
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
	d.Suite = suiteStr

	return nil

}

type KyberPoints struct {
	Public []kyber.Point
	Suite  string `json:"suite,omitempty"`
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

	return json.Marshal(struct {
		Public [][]byte
		Suite  string `json:"suite,omitempty"`
	}{
		Public: points,
		Suite:  d.Suite,
	})
}

func (d *KyberPoints) UnmarshalJSON(bt []byte) error {
	jsonData := struct {
		Public [][]byte
		Suite  string `json:"suite,omitempty"`
	}{}
	err := json.Unmarshal(bt, &jsonData)
	if err != nil {
		public := [][]byte{}
		err2 := json.Unmarshal(bt, &public)
		if err2 != nil {
			return errors.Wrap(err2, "Deal UnmarshalJSON")
		}
		jsonData.Public = public
	}

	points := make([]kyber.Point, len(jsonData.Public))
	suiteStr := jsonData.Suite
	if suiteStr == "" {
		suiteStr = "ed25519"
	}
	suiteType, _ := SuiteTypeFromString(suiteStr)
	suite := NewSuite(suiteType)
	for i, p := range jsonData.Public {
		point := suite.Point()
		err := point.UnmarshalBinary(p)
		if err != nil {
			return errors.Wrap(err, "Deal UnmarshalJSON")
		}

		points[i] = point
	}

	d.Public = points
	d.Suite = suiteStr
	return nil

}

type JustificationBundle struct {
	DealerIndex    uint32
	Justifications []Justification
	SessionID      []byte
	Signature      []byte
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
