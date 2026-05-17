package bls12381

import (
	"bytes"
	"fmt"

	bls12381 "github.com/cloudflare/circl/ecc/bls12381"
)

// VerifyLocal verifies a BLS12-381 signature locally using standard
// compressed formats (48B G1, 96B G2). It checks e(HM, pubkey) == e(sig, G2).
func VerifyLocal(hmCompressed, sigCompressed, pubkeyCompressed []byte) error {
	if len(hmCompressed) != G1CompressedSize {
		return fmt.Errorf("invalid HM size: %d", len(hmCompressed))
	}
	if len(sigCompressed) != G1CompressedSize {
		return fmt.Errorf("invalid sig size: %d", len(sigCompressed))
	}
	if len(pubkeyCompressed) != G2CompressedSize {
		return fmt.Errorf("invalid pubkey size: %d", len(pubkeyCompressed))
	}

	var g1HM, g1Sig bls12381.G1
	var g2Pub bls12381.G2
	if err := g1HM.SetBytes(hmCompressed); err != nil {
		return fmt.Errorf("parse HM: %w", err)
	}
	if err := g1Sig.SetBytes(sigCompressed); err != nil {
		return fmt.Errorf("parse sig: %w", err)
	}
	if err := g2Pub.SetBytes(pubkeyCompressed); err != nil {
		return fmt.Errorf("parse pubkey: %w", err)
	}

	g1NegSig := g1Sig
	g1NegSig.Neg()

	var g2Base bls12381.G2
	if err := g2Base.SetBytes(g2BaseCompressed()); err != nil {
		return fmt.Errorf("parse G2 base: %w", err)
	}

	result := bls12381.ProdPairFrac(
		[]*bls12381.G1{&g1HM, &g1NegSig},
		[]*bls12381.G2{&g2Pub, &g2Base},
		[]int{1, 1},
	)
	if !result.IsIdentity() {
		return fmt.Errorf("pairing check failed")
	}
	return nil
}

// VerifyNegationRoundTrip checks that NegateG1EIP2537 produces the
// same result as negating the standard uncompressed point.
func VerifyNegationRoundTrip(sigCompressed, eipNegSig []byte) error {
	if len(sigCompressed) != G1CompressedSize {
		return fmt.Errorf("invalid sig size: %d", len(sigCompressed))
	}
	if len(eipNegSig) != EIP2537G1Size {
		return fmt.Errorf("invalid eipNegSig size: %d", len(eipNegSig))
	}

	var verifyNeg bls12381.G1
	if err := verifyNeg.SetBytes(sigCompressed); err != nil {
		return fmt.Errorf("parse sig: %w", err)
	}
	verifyNeg.Neg()
	expectedNeg := verifyNeg.Bytes()

	actualNegX := eipNegSig[16:64]
	actualNegY := eipNegSig[80:128]
	if !bytes.Equal(actualNegX, expectedNeg[:48]) || !bytes.Equal(actualNegY, expectedNeg[48:]) {
		return fmt.Errorf("EIP-2537 negation round-trip mismatch")
	}
	return nil
}
