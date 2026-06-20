package dkgtypes

import (
	"encoding/json"
	"fmt"

	"github.com/computeflux/tee-provider/pkg/chaintypes"
	"github.com/computeflux/tee-provider/pkg/crypto"
	"github.com/pkg/errors"
	"go.dedis.ch/kyber/v4"
	pedersen "go.dedis.ch/kyber/v4/share/dkg/pedersen"
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
	Sponsor          *chaintypes.Validator
	DealBundles      map[crypto.SuiteType]*DealBundle
	Epoch            uint32
	EpochTime        int64
	ShareCommits     map[crypto.SuiteType]KyberPoints
	OldValidators    []*chaintypes.Validator
	Validators       []*chaintypes.Validator
	ConsensusNodeNum int
}

// jsonConsensusMsg is the JSON-serializable form of ConsensusMsg (string keys).
type jsonConsensusMsg struct {
	Sponsor          *chaintypes.Validator   `json:"Sponsor,omitempty"`
	DealBundles      map[string]*DealBundle  `json:"DealBundles,omitempty"`
	Epoch            uint32                  `json:"Epoch"`
	EpochTime        int64                   `json:"EpochTime"`
	ShareCommits     map[string]KyberPoints  `json:"ShareCommits"`
	OldValidators    []*chaintypes.Validator `json:"OldValidators,omitempty"`
	Validators       []*chaintypes.Validator `json:"Validators,omitempty"`
	ConsensusNodeNum int                     `json:"ConsensusNodeNum"`
}

func (c ConsensusMsg) MarshalJSON() ([]byte, error) {
	stringBundles := make(map[string]*DealBundle, len(c.DealBundles))
	for st, bundle := range c.DealBundles {
		stringBundles[st.String()] = bundle
	}
	stringCommits := make(map[string]KyberPoints, len(c.ShareCommits))
	for st, commits := range c.ShareCommits {
		stringCommits[st.String()] = commits
	}
	return json.Marshal(jsonConsensusMsg{
		Sponsor:          c.Sponsor,
		DealBundles:      stringBundles,
		Epoch:            c.Epoch,
		EpochTime:        c.EpochTime,
		ShareCommits:     stringCommits,
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
	c.DealBundles = make(map[crypto.SuiteType]*DealBundle, len(raw.DealBundles))
	for stStr, bundle := range raw.DealBundles {
		st, err := crypto.SuiteTypeFromString(stStr)
		if err != nil {
			continue // skip unknown suite types
		}
		c.DealBundles[st] = bundle
	}
	c.Epoch = raw.Epoch
	c.EpochTime = raw.EpochTime
	c.ShareCommits = make(map[crypto.SuiteType]KyberPoints, len(raw.ShareCommits))
	for stStr, commits := range raw.ShareCommits {
		st, err := crypto.SuiteTypeFromString(stStr)
		if err != nil {
			continue // skip unknown suite types
		}
		c.ShareCommits[st] = commits
	}
	c.OldValidators = raw.OldValidators
	c.Validators = raw.Validators
	c.ConsensusNodeNum = raw.ConsensusNodeNum
	return nil
}

// ConsensusRespMsg carries responses for multiple suites.
type ConsensusRespMsg struct {
	Responses map[crypto.SuiteType][]byte
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
	c.Responses = make(map[crypto.SuiteType][]byte, len(raw.Responses))
	for stStr, data := range raw.Responses {
		st, err := crypto.SuiteTypeFromString(stStr)
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
	suiteType, _ := crypto.SuiteTypeFromString(suiteStr)
	suite := crypto.NewSuite(suiteType)
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
	suiteType, _ := crypto.SuiteTypeFromString(suiteStr)
	suite := crypto.NewSuite(suiteType)
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
