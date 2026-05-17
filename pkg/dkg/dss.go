package dkg

import (
	"bytes"
	"crypto/ed25519"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"os"

	"github.com/centrifuge/go-substrate-rpc-client/v4/types"
	"github.com/computeflux/tee-provider/pkg/model"
	"github.com/computeflux/tee-provider/pkg/model/bls12381"
	"github.com/computeflux/tee-provider/pkg/util"
	"github.com/ethereum/go-ethereum/crypto"
	"go.dedis.ch/kyber/v4"
	"go.dedis.ch/kyber/v4/pairing"
	"go.dedis.ch/kyber/v4/share"
	"go.dedis.ch/kyber/v4/sign/dss"
)

// create dss signer from dkg
func NewDssSigner(dkg *DKG, txIndex int64, suite model.SuiteType) *DssSigner {
	signer := &DssSigner{
		dkg:   dkg,
		suite: suite,
		sigs:  make([][]byte, 0),
	}
	// Try to acquire an independent random share from the pool.
	// If the pool is nil, empty, or txIndex <= 0, fallback to using the long share.
	// BLS12-381 does not use random shares (tbls signs directly with the long-term share).
	if suite == model.SuiteEd25519 && dkg.RandomPools != nil && txIndex > 0 {
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
	sigs        [][]byte            // raw partial signature bytes (format depends on suite)
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
	if pub, ok := d.dkg.NewDkgPubKeys[d.suite]; ok && pub != nil {
		return pub.Byte()
	}
	return nil
}

func (d *DssSigner) AccountID() types.AccountID {
	if pub, ok := d.dkg.DkgPubKeys[d.suite]; ok && pub != nil {
		return pub.AccountID()
	}
	if pub, ok := d.dkg.NewDkgPubKeys[d.suite]; ok && pub != nil {
		return pub.AccountID()
	}
	return types.AccountID{}
}

func (d *DssSigner) Pubkey() *model.PubKey {
	if pub, ok := d.dkg.DkgPubKeys[d.suite]; ok && pub != nil {
		return pub
	}
	if pub, ok := d.dkg.NewDkgPubKeys[d.suite]; ok && pub != nil {
		return pub
	}

	return nil
}

// sign msg aggregated share
func (d *DssSigner) Sign(msg []byte) ([]byte, error) {
	pubs, long, random, threshold := d.PubList()
	suite, ok := d.dkg.Suites[d.suite]
	if !ok {
		return nil, errors.New("suite not found: " + string(d.suite))
	}

	switch d.suite {
	case model.SuiteEd25519:
		return d.signEd25519(suite, pubs, long, random, threshold, msg)
	case model.SuiteBLS12381:
		return d.signBLS12381(suite.(pairing.Suite), long, threshold, len(pubs), msg)
	default:
		return nil, errors.New("unsupported suite: " + d.suite.String())
	}
}

func (d *DssSigner) signEd25519(suite dss.Suite, pubs []kyber.Point, long, random *model.DistKeyShare, threshold int, msg []byte) ([]byte, error) {
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

	for _, bt := range d.sigs {
		sig := &model.PartialSigWrap{}
		if err := json.Unmarshal(bt, sig); err != nil {
			fmt.Println(err)
			continue
		}
		ps := &dss.PartialSig{
			Partial:   sig.Partial.PriShare,
			SessionID: sig.SessionID,
			Signature: sig.Signature,
		}
		if err := dssInst.ProcessPartialSig(ps); err != nil {
			return nil, err
		}
	}

	return dssInst.Signature()
}

// hashToG1BLS12381 computes the hash-to-curve point using MAP_FP_TO_G1 semantics:
// keccak256(msg) -> field element -> SSWU + isogeny + clearCofactor.
// Returns a kyber G1 Point.
func hashToG1BLS12381(suite pairing.Suite, msg []byte) (kyber.Point, error) {
	msgHash := crypto.Keccak256(msg)
	hmG1Compressed, err := bls12381.MapFpToG1Compressed(msgHash)
	if err != nil {
		return nil, fmt.Errorf("map fp to g1: %w", err)
	}
	hmG1 := suite.G1().Point()
	if err := hmG1.UnmarshalBinary(hmG1Compressed); err != nil {
		return nil, fmt.Errorf("unmarshal hmG1: %w", err)
	}
	return hmG1, nil
}

func (d *DssSigner) signBLS12381(suite pairing.Suite, long *model.DistKeyShare, threshold, n int, msg []byte) ([]byte, error) {
	if long == nil {
		return nil, errors.New("long-term share not available")
	}
	hmG1, err := hashToG1BLS12381(suite, msg)
	if err != nil {
		return nil, err
	}

	pubPoly := share.NewPubPoly(suite.G2(), suite.G2().Point().Base(), long.Commitments())
	g2Base := suite.G2().Point().Base()

	var pubShares []*share.PubShare
	for _, sig := range d.sigs {
		if len(sig) < 2 {
			continue
		}
		idx := uint32(binary.BigEndian.Uint16(sig[:2]))
		sigPoint := suite.G1().Point()
		if err := sigPoint.UnmarshalBinary(sig[2:]); err != nil {
			continue
		}
		pubShare := pubPoly.Eval(idx)
		if !suite.ValidatePairing(hmG1, pubShare.V, sigPoint, g2Base) {
			continue
		}
		pubShares = append(pubShares, &share.PubShare{I: idx, V: sigPoint})
		if uint32(len(pubShares)) >= uint32(threshold) {
			break
		}
	}

	if uint32(len(pubShares)) < uint32(threshold) {
		return nil, errors.New("not enough valid partial signatures")
	}

	fullSig, err := share.RecoverCommit(suite.G1(), pubShares, uint32(threshold), uint32(n))
	if err != nil {
		return nil, fmt.Errorf("recover commit: %w", err)
	}

	return fullSig.MarshalBinary()
}

// partial sign msg
func (d *DssSigner) PartialSign(msg []byte) ([]byte, error) {
	pubs, long, random, threshold := d.PubList()
	suite, ok := d.dkg.Suites[d.suite]
	if !ok {
		return nil, errors.New("suite not found: " + string(d.suite))
	}

	switch d.suite {
	case model.SuiteEd25519:
		return d.partialSignEd25519(suite, pubs, long, random, threshold, msg)
	case model.SuiteBLS12381:
		return d.partialSignBLS12381(suite.(pairing.Suite), long, msg)
	default:
		return nil, errors.New("unsupported suite: " + d.suite.String())
	}
}

func (d *DssSigner) partialSignEd25519(suite dss.Suite, pubs []kyber.Point, long, random *model.DistKeyShare, threshold int, msg []byte) ([]byte, error) {
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

func (d *DssSigner) partialSignBLS12381(suite pairing.Suite, long *model.DistKeyShare, msg []byte) ([]byte, error) {
	if long == nil {
		return nil, errors.New("long-term share not available")
	}
	hmG1, err := hashToG1BLS12381(suite, msg)
	if err != nil {
		return nil, err
	}
	sigPoint := suite.G1().Point().Mul(long.PriShare().V, hmG1)
	sigBytes, err := sigPoint.MarshalBinary()
	if err != nil {
		return nil, fmt.Errorf("marshal sig: %w", err)
	}
	// Prepend 2-byte big-endian index (same format as tbls.Sign)
	buf := new(bytes.Buffer)
	if err := binary.Write(buf, binary.BigEndian, uint16(long.PriShare().I)); err != nil {
		return nil, err
	}
	if err := binary.Write(buf, binary.BigEndian, sigBytes); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
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
			pubs = append(pubs, validatorPubForSuite(k, d.suite))
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
		pubs = append(pubs, validatorPubForSuite(k, d.suite))
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

func validatorPubForSuite(v *model.Validator, st model.SuiteType) kyber.Point {
	switch st {
	case model.SuiteBLS12381:
		if p := v.ValidatorIdBLS.Point(); p != nil {
			return p
		}
	case model.SuiteEd25519:
		return v.ValidatorId.Point()
	}

	util.LogError("DKG", "validatorPubForSuite: failed to get pub for validator", st.String())
	os.Exit(1)
	return nil
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
	case model.SuiteBLS12381:
		return d.verifyBLS12381(msg, signature)
	default:
		return false
	}
}

func (d *DssSigner) verifyBLS12381(msg, signature []byte) bool {
	suite, ok := d.dkg.Suites[d.suite]
	if !ok {
		return false
	}
	ps, ok := suite.(pairing.Suite)
	if !ok {
		return false
	}

	pub, ok := d.dkg.DkgPubKeys[d.suite]
	if !ok || pub == nil {
		pub = d.dkg.NewDkgPubKeys[d.suite]
	}
	if pub == nil {
		return false
	}

	hmG1, err := hashToG1BLS12381(ps, msg)
	if err != nil {
		return false
	}

	sigPoint := ps.G1().Point()
	if err := sigPoint.UnmarshalBinary(signature); err != nil {
		return false
	}

	return ps.ValidatePairing(hmG1, pub.Point(), sigPoint, ps.G2().Point().Base())
}

func (d *DssSigner) SignType() uint8 {
	return 0
}

// Suite returns the suite type used by this signer.
func (d *DssSigner) Suite() model.SuiteType {
	return d.suite
}

// BLSSignEIP2537 signs a message with BLS and returns the EIP-2537 formatted
// parameters needed for on-chain verification: the raw signature, the negated
// signature in EIP-2537 format, and the hash-to-curve G1 point in EIP-2537 format.
func (d *DssSigner) BLSSignEIP2537(msg []byte) (signature []byte, negSigEIP []byte, hmG1EIP []byte, err error) {
	if d.suite != model.SuiteBLS12381 {
		return nil, nil, nil, errors.New("not BLS suite")
	}

	// 1. Sign the message
	sig, err := d.Sign(msg)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("bls sign: %w", err)
	}

	// 2. Convert signature to EIP-2537 format and negate
	sigEIP, err := bls12381.G1ToEIP2537(sig)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("g1 to eip2537: %w", err)
	}
	negSig, err := bls12381.NegateG1EIP2537(sigEIP)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("negate g1: %w", err)
	}

	// 3. Compute hmG1 using MAP_FP_TO_G1(keccak256(msg))
	msgHash := crypto.Keccak256(msg)
	hmG1EIP, err = bls12381.MapFpToG1(msgHash)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("map fp to g1: %w", err)
	}

	return sig, negSig, hmG1EIP, nil
}
