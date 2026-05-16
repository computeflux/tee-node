package dkg

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/centrifuge/go-substrate-rpc-client/v4/types"
	"github.com/computeflux/tee-provider/pkg/model"
	"go.dedis.ch/kyber/v4"
	"go.dedis.ch/kyber/v4/sign/dss"
)

// create dss signer from dkg
func NewDssSigner(dkg *DKG, txIndex int64, suite model.SuiteType) *DssSigner {
	signer := &DssSigner{
		dkg:   dkg,
		suite: suite,
		sigs:  make([]*dss.PartialSig, 0),
	}
	// Try to acquire an independent random share from the pool.
	// If the pool is nil, empty, or txIndex <= 0, fallback to using the long share.
	if dkg.RandomPools != nil && txIndex > 0 {
		if pool, ok := dkg.RandomPools[suite]; ok && pool != nil {
			random, err := pool.Acquire(txIndex)
			if err == nil {
				signer.randomShare = random
			} else {
				fmt.Println("DssSigner: failed to acquire random share:", err)
			}
		}
	}
	return signer
}

// signer for dss
type DssSigner struct {
	dkg         *DKG
	suite       model.SuiteType
	randomShare *model.DistKeyShare // independent random share from RandomPool; nil means fallback to long share
	sigs        []*dss.PartialSig
}

// set shares for dss
func (d *DssSigner) SetSigs(btsigs [][]byte) {
	sigs := make([]*dss.PartialSig, 0, len(btsigs))
	for _, bt := range btsigs {
		sig := &model.PartialSigWrap{}
		err := json.Unmarshal(bt, sig)
		if err != nil {
			fmt.Println(err)
			continue
		}
		sigs = append(sigs, &dss.PartialSig{
			Partial:   sig.Partial.PriShare,
			SessionID: sig.SessionID,
			Signature: sig.Signature,
		})
	}
	d.sigs = sigs
}

// get public key for this suite
func (d *DssSigner) Public() []byte {
	d.dkg.mu.RLock()
	defer d.dkg.mu.RUnlock()
	if pub, ok := d.dkg.DkgPubKeys[d.suite]; ok && pub != nil {
		return pub.Byte()
	}
	if pub, ok := d.dkg.NewDkgPubKeys[d.suite]; ok && pub != nil {
		return pub.Byte()
	}
	return nil
}

func (d *DssSigner) AccountID() types.AccountID {
	d.dkg.mu.RLock()
	defer d.dkg.mu.RUnlock()
	if pub, ok := d.dkg.DkgPubKeys[d.suite]; ok && pub != nil {
		return pub.AccountID()
	}
	if pub, ok := d.dkg.NewDkgPubKeys[d.suite]; ok && pub != nil {
		return pub.AccountID()
	}
	return types.AccountID{}
}

// ChainAccountID returns the chain-agnostic account ID.
func (d *DssSigner) ChainAccountID() model.ChainAccountID {
	return model.ChainAccountID(d.AccountID())
}

// sign msg aggregated share
func (d *DssSigner) Sign(msg []byte) ([]byte, error) {
	pubs, long, random, threshold := d.PubList()
	suite, ok := d.dkg.Suites[d.suite]
	if !ok {
		return nil, errors.New("suite not found: " + string(d.suite))
	}

	dssInst, err := dss.NewDSS(
		suite,
		d.dkg.longtermForSuite(d.suite),
		pubs,
		long,
		random,
		msg,
		uint32(threshold),
	)

	if dssInst == nil || err != nil {
		fmt.Println(err)
		return nil, errors.New("dss.NewDSS failed")
	}

	for _, sig := range d.sigs {
		err = dssInst.ProcessPartialSig(sig)
		if err != nil {
			return nil, err
		}
	}

	return dssInst.Signature()
}

// partial sign msg
func (d *DssSigner) PartialSign(msg []byte) ([]byte, error) {
	pubs, long, random, threshold := d.PubList()
	suite, ok := d.dkg.Suites[d.suite]
	if !ok {
		return nil, errors.New("suite not found: " + string(d.suite))
	}

	dssInst, err := dss.NewDSS(
		suite,
		d.dkg.longtermForSuite(d.suite),
		pubs,
		long,
		random,
		msg,
		uint32(threshold),
	)

	if dssInst == nil || err != nil {
		return nil, errors.New("dss.NewDSS failed")
	}

	sig, err := dssInst.PartialSig()
	if err != nil {
		return nil, errors.New("dss.PartialSig failed")
	}

	sigWrap := model.PartialSigWrap{
		Partial:   model.PriShare{PriShare: sig.Partial},
		SessionID: sig.SessionID,
		Signature: sig.Signature,
	}

	return json.Marshal(sigWrap)
}

// pub list returns the participant public keys, the long-term share,
// the random share, and the threshold.
// If an independent random share was acquired from RandomPool, it is used;
// otherwise falls back to the long-term share (preserves backward compatibility).
func (d *DssSigner) PubList() ([]kyber.Point, *model.DistKeyShare, *model.DistKeyShare, int) {
	d.dkg.mu.RLock()
	defer d.dkg.mu.RUnlock()

	pubs := make([]kyber.Point, 0, len(d.dkg.Nodes))
	if len(d.dkg.NewNodes) > 0 {
		for _, k := range d.dkg.NewNodes {
			pub := k.ValidatorId.Point()
			if d.suite == model.SuiteSecp256k1 {
				pub = k.ValidatorIdSecp.Point()
			}
			pubs = append(pubs, pub)
		}

		var longShare *model.DistKeyShare
		if s, ok := d.dkg.NewDkgKeyShares[d.suite]; ok {
			longShare = s
		}

		random := d.randomShare
		if random == nil {
			random = longShare
		}
		return pubs, longShare, random, len(d.dkg.NewNodes) * 2 / 3
	}

	for _, k := range d.dkg.Nodes {
		pub := k.ValidatorId.Point()
		if d.suite == model.SuiteSecp256k1 {
			pub = k.ValidatorIdSecp.Point()
		}
		pubs = append(pubs, pub)
	}
	var longShare *model.DistKeyShare
	if s, ok := d.dkg.DkgKeyShares[d.suite]; ok {
		longShare = s
	}
	random := d.randomShare
	if random == nil {
		random = longShare
	}
	return pubs, longShare, random, d.dkg.Threshold
}

// verify sig
func (d *DssSigner) Verify(msg []byte, signature []byte) bool {
	d.dkg.mu.RLock()
	defer d.dkg.mu.RUnlock()

	pub, ok := d.dkg.DkgPubKeys[d.suite]
	if !ok || pub == nil {
		pub = d.dkg.NewDkgPubKeys[d.suite]
	}
	if pub == nil {
		return false
	}

	switch d.suite {
	case model.SuiteEd25519:
		return ed25519.Verify(pub.Ed25519PublicKey(), msg, signature)
	case model.SuiteSecp256k1:
		return verifySecp256k1DSSSignature(d.dkg.Suites[d.suite], pub.Point(), msg, signature)
	default:
		return false
	}
}

func (d *DssSigner) SignType() uint8 {
	return 0
}

// verifySecp256k1DSSSignature verifies a DSS signature over the secp256k1 curve.
// The DSS signature format is R_point_bytes || s_scalar_bytes.
// For secp256k1, R is 33 bytes (compressed) and s is 32 bytes → 65 bytes total.
func verifySecp256k1DSSSignature(suite kyber.Group, public kyber.Point, msg, sig []byte) bool {
	// For secp256k1: R (33 bytes compressed) + s (32 bytes) = 65 bytes
	if len(sig) != 65 {
		return false
	}

	R := suite.Point()
	if err := R.UnmarshalBinary(sig[:33]); err != nil {
		return false
	}

	s := suite.Scalar()
	if err := s.UnmarshalBinary(sig[33:]); err != nil {
		return false
	}

	// Reconstruct challenge: h = H(R || Public || Msg)
	hash := sha256.New()
	Rb, _ := R.MarshalBinary()
	Pb, _ := public.MarshalBinary()
	hash.Write(Rb)
	hash.Write(Pb)
	hash.Write(msg)

	h := suite.Scalar().SetBytes(hash.Sum(nil))

	// Verify: s*G == R + h*Public
	// s*G
	sG := suite.Point().Mul(s, nil)
	// h*Public
	hPub := suite.Point().Mul(h, public)
	// R + h*Public
	R.Add(R, hPub)

	return sG.Equal(R)
}
