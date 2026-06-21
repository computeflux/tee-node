package dkg

import (
	"math/big"
	"testing"

	"github.com/computeflux/tee-provider/pkg/chaintypes"
	crypto "github.com/computeflux/tee-provider/pkg/crypto"
	dkgtypes "github.com/computeflux/tee-provider/pkg/dkg/types"
	"github.com/computeflux/tee-provider/pkg/model/bls12381"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/vm"
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
		Suites: map[crypto.SuiteType]suites.Suite{
			crypto.SuiteBLS12381: circl.NewSuiteBLS12381(),
		},
		DkgPubKeys: map[crypto.SuiteType]*crypto.PubKey{
			crypto.SuiteBLS12381: mustPubKeyFromPoint(groupPub, crypto.SuiteBLS12381),
		},
		DkgKeyShares: map[crypto.SuiteType]*dkgtypes.DistKeyShare{
			crypto.SuiteBLS12381: {
				CommitsWrap:  dkgtypes.KyberPoints{Public: commitments},
				PriShareWrap: dkgtypes.PriShareWithSuite{PriShare: shares[0], Suite: "bls12381"},
			},
		},
		Nodes:     make([]*chaintypes.Validator, n),
		Threshold: threshold,
	}
	for i := 0; i < n; i++ {
		nodePub := pubPoly.Eval(uint32(i))
		dkgInst.Nodes[i] = &chaintypes.Validator{
			ValidatorIdBLS: *mustPubKeyFromPoint(nodePub.V, crypto.SuiteBLS12381),
		}
	}

	msg := []byte("hello BLS12381")

	// Each node produces a partial signature.
	partialSigs := make([][]byte, 0, n)
	for i := 0; i < n; i++ {
		signer := &DssSigner{
			dkg:   dkgInst,
			suite: crypto.SuiteBLS12381,
		}
		// Manually set the long-term share for this node.
		dkgInst.DkgKeyShares[crypto.SuiteBLS12381] = &dkgtypes.DistKeyShare{
			CommitsWrap:  dkgtypes.KyberPoints{Public: commitments},
			PriShareWrap: dkgtypes.PriShareWithSuite{PriShare: shares[i], Suite: "bls12381"},
		}
		sig, err := signer.PartialSign(msg)
		require.NoError(t, err, "node %d partial sign failed", i)
		partialSigs = append(partialSigs, sig)
	}

	// Aggregator (node 0) collects partial signatures.
	aggregator := &DssSigner{
		dkg:   dkgInst,
		suite: crypto.SuiteBLS12381,
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

	// Use the same hash-to-curve method as signBLS12381/partialSignBLS12381
	hmG1, err := hashToG1BLS12381(suite, msg)
	require.NoError(t, err)
	hmCompressed, err := hmG1.MarshalBinary()
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

	// ── 链上验证：调用 Ethereum EIP-2537 pairing precompile (address 0x10) ──
	pairingPrecompile := vm.PrecompiledContractsBLS[common.BytesToAddress([]byte{16})]
	require.NotNil(t, pairingPrecompile, "EIP-2537 pairing precompile not found")

	out, err := pairingPrecompile.Run(pairingInput)
	require.NoError(t, err, "EIP-2537 pairing precompile execution failed")
	require.Len(t, out, 32, "precompile output must be 32 bytes")

	expected := common.LeftPadBytes(big.NewInt(1).Bytes(), 32)
	require.Equal(t, expected, out, "on-chain pairing verification failed")
}

func mustPubKeyFromPoint(pt kyber.Point, st crypto.SuiteType) *crypto.PubKey {
	pk, err := crypto.PubKeyFromPointWithSuite(pt, st)
	if err != nil {
		panic(err)
	}
	return pk
}
