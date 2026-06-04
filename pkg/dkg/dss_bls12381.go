package dkg

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"

	teecrypto "github.com/computeflux/tee-provider/pkg/crypto"
	"github.com/computeflux/tee-provider/pkg/dkgtypes"
	"github.com/computeflux/tee-provider/pkg/model/bls12381"
	"github.com/ethereum/go-ethereum/crypto"
	"go.dedis.ch/kyber/v4"
	"go.dedis.ch/kyber/v4/pairing"
	"go.dedis.ch/kyber/v4/share"
)

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

func (d *DssSigner) signBLS12381(suite pairing.Suite, long *dkgtypes.DistKeyShare, threshold, n int, msg []byte) ([]byte, error) {
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

func (d *DssSigner) partialSignBLS12381(suite pairing.Suite, long *dkgtypes.DistKeyShare, msg []byte) ([]byte, error) {
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

// BLSSignEIP2537 signs a message with BLS and returns the EIP-2537 formatted
// parameters needed for on-chain verification: the raw signature, the negated
// signature in EIP-2537 format, and the hash-to-curve G1 point in EIP-2537 format.
func (d *DssSigner) BLSSignEIP2537(msg []byte) (signature []byte, negSigEIP []byte, hmG1EIP []byte, err error) {
	if d.suite != teecrypto.SuiteBLS12381 {
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
