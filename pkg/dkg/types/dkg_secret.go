package dkgtypes

import (
	"encoding/json"
	"fmt"

	"github.com/computeflux/tee-provider/pkg/types/crypto"
	"go.dedis.ch/kyber/v4"
	"go.dedis.ch/kyber/v4/share"
)

// DistKeyShare
type DistKeyShare struct {
	// Coefficients of the public polynomial holding the public key
	CommitsWrap KyberPoints `json:"Commits"`
	// PriShare of the distributed secret
	PriShareWrap PriShareWithSuite `json:"PriShare"`
}

// DistKeyShare
func (d DistKeyShare) PriShare() *share.PriShare {
	return d.PriShareWrap.PriShare
}

// DistKeyShare
func (d DistKeyShare) Commitments() []kyber.Point {
	return d.CommitsWrap.Public
}

// PriShare
type PriShareWithSuite struct {
	*share.PriShare
	Suite string `json:"suite,omitempty"`
}

// Custom JSON serialization
func (d PriShareWithSuite) MarshalJSON() ([]byte, error) {
	if d.PriShare == nil || d.V == nil {
		return nil, fmt.Errorf("PriShareWithSuite.MarshalJSON: nil private share")
	}
	sbuf, err := d.V.MarshalBinary()
	if err != nil {
		return nil, fmt.Errorf("couldn't marshal private share: %w", err)
	}

	return json.Marshal(struct {
		I     uint32
		V     []byte
		Suite string `json:"suite,omitempty"`
	}{
		I:     d.I,
		V:     sbuf,
		Suite: d.Suite,
	})
}

// Custom JSON deserialization
func (d *PriShareWithSuite) UnmarshalJSON(bt []byte) error {
	jsonData := struct {
		I     uint32
		V     []byte
		Suite string `json:"suite,omitempty"`
	}{}

	err := json.Unmarshal(bt, &jsonData)
	if err != nil {
		return err
	}

	if jsonData.Suite == "" {
		// 旧数据未存储 suite 字段，默认回退到 ed25519
		jsonData.Suite = "ed25519"
	}

	var parseErr error
	st, parseErr := crypto.SuiteTypeFromString(jsonData.Suite)
	if parseErr != nil {
		return fmt.Errorf("PriShareWithSuite.UnmarshalJSON err: invalid suite in PriShareWithSuite JSON: %w", parseErr)
	}

	key := crypto.NewSuite(st).Scalar()
	err = key.UnmarshalBinary(jsonData.V)
	if err != nil {
		return err
	}

	d.PriShare = &share.PriShare{
		I: jsonData.I,
		V: key,
	}

	d.Suite = jsonData.Suite

	return nil
}

func (d *PriShareWithSuite) String() {

}
