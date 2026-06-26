package dkg

import (
	"crypto/ed25519"
	"errors"

	"github.com/centrifuge/go-substrate-rpc-client/v4/types"
	dkgtypes "github.com/computeflux/tee-provider/pkg/dkg/types"
	"github.com/computeflux/tee-provider/pkg/types/chaintypes"
	"github.com/computeflux/tee-provider/pkg/types/crypto"
	"github.com/computeflux/tee-provider/pkg/util"
	"go.dedis.ch/kyber/v4"
	"go.dedis.ch/kyber/v4/pairing"
	"golang.org/x/crypto/blake2b"
)

// create dss signer from dkg
func NewDssSigner(dkg *DKG, txIndex int64, suite crypto.SuiteType) *DssSigner {
	signer := &DssSigner{
		dkg:   dkg,
		suite: suite,
		sigs:  make([][]byte, 0),
	}
	// Try to acquire an independent random share from the pool.
	// If the pool is nil, empty, or txIndex <= 0, fallback to using the long share.
	// BLS12-381 does not use random shares (tbls signs directly with the long-term share).
	if suite == crypto.SuiteEd25519 && dkg.RandomPools != nil && txIndex > 0 {
		if pool, ok := dkg.RandomPools[suite]; ok && pool != nil {
			random, err := pool.Acquire(txIndex)
			if err == nil {
				signer.randomShare = random
			}
		}
	}
	return signer
}

// signer for dss
type DssSigner struct {
	dkg         *DKG
	suite       crypto.SuiteType
	randomShare *dkgtypes.DistKeyShare // independent random share from RandomPool; nil means fallback to long share
	sigs        [][]byte               // raw partial signature bytes (format depends on suite)
}

// set shares for dss
func (d *DssSigner) SetSigs(btsigs [][]byte) {
	d.sigs = btsigs
}

// get public key for this suite
func (d *DssSigner) Public() []byte {
	if pub, ok := d.dkg.DkgPubKeys[d.suite]; ok && pub != nil {
		return pub.Byte()
	}

	util.LogError("DssSigner", "Public key not found for suite", d.suite.String())
	return nil
}

func (d *DssSigner) AccountID() types.AccountID {
	if pub, ok := d.dkg.DkgPubKeys[d.suite]; ok && pub != nil {
		return pub.AccountID()
	}

	util.LogError("DssSigner", "AccountID not found for suite", d.suite.String())
	return types.AccountID{}
}

func (d *DssSigner) Pubkey() *crypto.PubKey {
	if pub, ok := d.dkg.DkgPubKeys[d.suite]; ok && pub != nil {
		return pub
	}

	util.LogError("DssSigner", "PubKey not found for suite", d.suite.String())
	return nil
}

// pub list returns the participant public keys, the long-term share,
// the random share, and the threshold.
// If an independent random share was acquired from RandomPool, it is used;
// otherwise falls back to the long-term share (preserves backward compatibility).
func (d *DssSigner) PubList() ([]kyber.Point, *dkgtypes.DistKeyShare, *dkgtypes.DistKeyShare, int) {
	d.dkg.mu.RLock()
	defer d.dkg.mu.RUnlock()

	var pubs []kyber.Point
	for _, k := range d.dkg.Nodes {
		pub := validatorPubForSuite(k, d.suite)
		if pub != nil {
			pubs = append(pubs, pub)
		}
	}

	var longShare *dkgtypes.DistKeyShare
	if s, ok := d.dkg.DkgKeyShares[d.suite]; ok {
		longShare = s
	}

	random := d.randomShare
	if random == nil {
		random = longShare
	}

	return pubs, longShare, random, d.dkg.Threshold
}

func validatorPubForSuite(v *chaintypes.Validator, st crypto.SuiteType) kyber.Point {
	switch st {
	case crypto.SuiteBLS12381:
		if p := v.ValidatorIdBLS.Point(); p != nil {
			return p
		}
		return nil
	case crypto.SuiteEd25519:
		return v.ValidatorId.Point()
	}

	util.LogError("DKG", "validatorPubForSuite: failed to get pub for validator", st.String())
	return nil
}

func (d *DssSigner) SignType() uint8 {
	if d.suite == crypto.SuiteEd25519 {
		return 1
	}
	return 0
}

// Suite returns the suite type used by this signer.
func (d *DssSigner) Suite() crypto.SuiteType {
	return d.suite
}

// sign msg aggregated share
func (d *DssSigner) Sign(msg []byte) ([]byte, error) {
	pubs, long, random, threshold := d.PubList()
	suite, ok := d.dkg.Suites[d.suite]
	if !ok {
		return nil, errors.New("suite not found: " + string(d.suite))
	}

	switch d.suite {
	case crypto.SuiteEd25519:
		return d.signEd25519(suite, pubs, long, random, threshold, msg)
	case crypto.SuiteBLS12381:
		return d.signBLS12381(suite.(pairing.Suite), long, threshold, len(pubs), msg)
	default:
		return nil, errors.New("unsupported suite: " + d.suite.String())
	}
}

// partial sign msg
func (d *DssSigner) PartialSign(msg []byte) ([]byte, error) {
	pubs, long, random, threshold := d.PubList()
	suite, ok := d.dkg.Suites[d.suite]
	if !ok {
		return nil, errors.New("suite not found: " + string(d.suite))
	}

	switch d.suite {
	case crypto.SuiteEd25519:
		return d.partialSignEd25519(suite, pubs, long, random, threshold, msg)
	case crypto.SuiteBLS12381:
		return d.partialSignBLS12381(suite.(pairing.Suite), long, msg)
	default:
		return nil, errors.New("unsupported suite: " + d.suite.String())
	}
}

// verify sig
func (d *DssSigner) Verify(msg []byte, signature []byte) bool {
	d.dkg.mu.RLock()
	defer d.dkg.mu.RUnlock()

	pub, ok := d.dkg.DkgPubKeys[d.suite]
	if !ok || pub == nil {
		util.LogError("DssSigner", "Public key not found for suite", d.suite.String())
		return false
	}

	switch d.suite {
	case crypto.SuiteEd25519:
		if len(msg) > 256 {
			h := blake2b.Sum256(msg)
			msg = h[:]
		}
		return ed25519.Verify(pub.Ed25519PublicKey(), msg, signature)
	case crypto.SuiteBLS12381:
		return d.verifyBLS12381(msg, signature)
	default:
		return false
	}
}
