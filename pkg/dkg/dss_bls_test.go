package dkg

import (
	"testing"

	"github.com/computeflux/tee-provider/pkg/model"
	"github.com/computeflux/tee-provider/pkg/model/bls12381"
	"github.com/stretchr/testify/require"
	"go.dedis.ch/kyber/v4"
	"go.dedis.ch/kyber/v4/pairing/bls12381/circl"
	"go.dedis.ch/kyber/v4/share"
	"go.dedis.ch/kyber/v4/sign/tbls"
	"go.dedis.ch/kyber/v4/suites"
)

func TestDssSigner_BLS12381(t *testing.T) {
	suite := circl.NewSuite()

	n := 5
	threshold := 3

	// Generate a shared secret and key shares.
	secret := suite.G2().Scalar().Pick(suite.RandomStream())
	poly := share.NewPriPoly(suite.G2(), uint32(threshold), secret, suite.RandomStream())
	shares := poly.Shares(uint32(n))
	pubPoly := poly.Commit(suite.G2().Point().Base())
	groupPub := pubPoly.Commit()
	_, commitments := pubPoly.Info()

	// Build a minimal DKG with just the BLS share.
	dkgInst := &DKG{
		Suites: map[model.SuiteType]suites.Suite{
			model.SuiteBLS12381: circl.NewSuiteBLS12381(),
		},
		DkgPubKeys: map[model.SuiteType]*model.PubKey{
			model.SuiteBLS12381: mustPubKeyFromPoint(groupPub, model.SuiteBLS12381),
		},
		DkgKeyShares: map[model.SuiteType]*model.DistKeyShare{
			model.SuiteBLS12381: {
				CommitsWrap:  model.KyberPoints{Public: commitments},
				PriShareWrap: model.PriShareWithSuite{PriShare: shares[0], Suite: "bls12381"},
			},
		},
		Nodes:     make([]*model.Validator, n),
		Threshold: threshold,
	}
	for i := 0; i < n; i++ {
		dkgInst.Nodes[i] = &model.Validator{}
	}

	msg := []byte("hello BLS12381")

	// Each node produces a partial signature.
	partialSigs := make([][]byte, 0, n)
	for i := 0; i < n; i++ {
		signer := &DssSigner{
			dkg:   dkgInst,
			suite: model.SuiteBLS12381,
		}
		// Manually set the long-term share for this node.
		dkgInst.DkgKeyShares[model.SuiteBLS12381] = &model.DistKeyShare{
			CommitsWrap:  model.KyberPoints{Public: commitments},
			PriShareWrap: model.PriShareWithSuite{PriShare: shares[i], Suite: "bls12381"},
		}
		sig, err := signer.PartialSign(msg)
		require.NoError(t, err, "node %d partial sign failed", i)
		partialSigs = append(partialSigs, sig)
	}

	// Aggregator (node 0) collects partial signatures.
	aggregator := &DssSigner{
		dkg:   dkgInst,
		suite: model.SuiteBLS12381,
	}
	// Use only threshold signatures.
	aggregator.SetSigs(partialSigs[:threshold])

	aggregated, err := aggregator.Sign(msg)
	require.NoError(t, err, "aggregate signature failed")

	// Verify locally with tbls.
	ok := aggregator.Verify(msg, aggregated)
	if !ok {
		// Debug: try verifying with the scheme directly
		scheme := tbls.NewThresholdSchemeOnG1(suite)
		verr := scheme.VerifyRecovered(groupPub, msg, aggregated)
		t.Logf("direct VerifyRecovered error: %v", verr)
	}
	require.True(t, ok, "aggregated signature should verify")

	// Verify with the group public key using circl.
	sigCompressed := aggregated
	pubkeyCompressed, err := groupPub.MarshalBinary()
	require.NoError(t, err)

	hmCompressed, err := bls12381.HashToG1(msg)
	require.NoError(t, err)

	err = bls12381.VerifyLocal(hmCompressed, sigCompressed, pubkeyCompressed)
	require.NoError(t, err, "local pairing verification failed")

	// Convert to EIP-2537 format and verify negation round-trip.
	eipHM, err := bls12381.G1ToEIP2537(hmCompressed)
	require.NoError(t, err)
	eipPubkey, err := bls12381.G2ToEIP2537(pubkeyCompressed)
	require.NoError(t, err)
	eipSig, err := bls12381.G1ToEIP2537(sigCompressed)
	require.NoError(t, err)

	require.Len(t, eipHM, 128, "HM EIP-2537 size")
	require.Len(t, eipPubkey, 256, "pubkey EIP-2537 size")
	require.Len(t, eipSig, 128, "sig EIP-2537 size")

	eipNegSig, err := bls12381.NegateG1EIP2537(eipSig)
	require.NoError(t, err)

	err = bls12381.VerifyNegationRoundTrip(sigCompressed, eipNegSig)
	require.NoError(t, err, "EIP-2537 negation round-trip failed")

	// Build PAIRING_INPUT (768B).
	pairingInput := append(eipHM, eipPubkey...)
	pairingInput = append(pairingInput, eipNegSig...)
	pairingInput = append(pairingInput, bls12381.G2GeneratorEIP2537()...)
	require.Len(t, pairingInput, 768, "PAIRING_INPUT must be 768 bytes")

	t.Logf("PAIRING_INPUT: %x", pairingInput)
}

func mustPubKeyFromPoint(pt kyber.Point, st model.SuiteType) *model.PubKey {
	pk, err := model.PubKeyFromPointWithSuite(pt, st)
	if err != nil {
		panic(err)
	}
	return pk
}
