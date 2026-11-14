package blscontract

import (
	"math/big"
	"testing"

	"github.com/computeflux/tee-provider/pkg/model/bls12381"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/stretchr/testify/require"
	"go.dedis.ch/kyber/v4"
	"go.dedis.ch/kyber/v4/pairing/bls12381/circl"
	"go.dedis.ch/kyber/v4/share"
	pedersen "go.dedis.ch/kyber/v4/share/dkg/pedersen"
)

// TestSignerPathEquivalence asserts that the production signer primitives in
// pkg/dkg/dss_bls12381.go (partialSignBLS12381 + signBLS12381 + BLSSignEIP2537)
// reproduce the full on-chain verification pipeline end-to-end:
//   - fresh Pedersen DKG (same suite as production)
//   - msg = target calldata; msgHash = keccak(msg) (submit rule) /
//     keccak(abi.encode(datas)) (batch rule) — production rules
//   - hmG1 = MapFpToG1Compressed(msgHash) (hashToG1BLS12381)
//   - sig_i = share_i.V * hmG1 with pubPoly pairing check (partialSignBLS12381)
//   - RecoverCommit over t shares (signBLS12381)
//   - BLSSignEIP2537: sig -> EIP2537 -> negate; hmG1EIP = MapFpToG1(msgHash)
//   - pairing input e(hmG1EIP, aggPubkeyEIP) == e(negSig, g2Gen) — exactly
//     BaseBLS._verifyBLS / BLS._verifyBLS on-chain.
func TestSignerPathEquivalence(t *testing.T) {
	n, threshold := 5, 3
	shares, groupPub, err := Setup(n, threshold)
	require.NoError(t, err)

	ps := circl.NewSuite()
	aggPubEIP, err := bls12381.G2ToEIP2537FromKyber(groupPub)
	require.NoError(t, err)
	require.Len(t, aggPubEIP, 256)

	cases := []struct {
		name string
		msgs [][]byte
	}{
		{"submit-single", [][]byte{mockTargetExecute([]byte{0xde, 0xad, 0xbe, 0xef}, 42)}},
		{"batchSubmit-batch", [][]byte{
			mockTargetExecute([]byte{0x01}, 1),
			mockTargetExecute([]byte{0x02, 0x03}, 2),
			mockTargetExecute([]byte{0x04, 0x05, 0x06}, 3),
		}},
	}

	bytesArrType, err := abi.NewType("bytes[]", "", nil)
	require.NoError(t, err)

	for _, c := range cases {
		// Production msg-hash rules (SignAndSubmit / BLSSignEIP2537).
		var msg []byte
		if len(c.msgs) == 1 {
			msg = c.msgs[0]
		} else {
			packed, err := abi.Arguments{{Type: bytesArrType}}.Pack(c.msgs)
			require.NoError(t, err, c.name)
			msg = packed
		}
		msgHash := crypto.Keccak256(msg)

		hmCompressed, err := bls12381.MapFpToG1Compressed(msgHash)
		require.NoError(t, err, c.name)
		hmG1 := ps.G1().Point()
		require.NoError(t, hmG1.UnmarshalBinary(hmCompressed), c.name)

		pubPoly := share.NewPubPoly(ps.G2(), ps.G2().Point().Base(), shares[0].Commitments())
		pubShares := make([]*share.PubShare, 0, threshold)
		for i := 0; i < threshold; i++ {
			shareIdx := shares[i].PriShare().I
			sigPoint := ps.G1().Point().Mul(shares[i].PriShare().V, hmG1)
			require.True(t, ps.ValidatePairing(
				hmG1, pubPoly.Eval(shareIdx).V,
				sigPoint, ps.G2().Point().Base()),
				"%s: partial %d pairing", c.name, shareIdx)
			pubShares = append(pubShares, &share.PubShare{I: shareIdx, V: sigPoint})
		}

		sigPoint, err := share.RecoverCommit(ps.G1(), pubShares, uint32(threshold), uint32(n))
		require.NoError(t, err, c.name)
		sig, err := sigPoint.MarshalBinary()
		require.NoError(t, err, c.name)

		// BLSSignEIP2537 step 2: sig -> EIP-2537 -> negated.
		sigEIP, err := bls12381.G1ToEIP2537(sig)
		require.NoError(t, err, c.name)
		negSig, err := bls12381.NegateG1EIP2537(sigEIP)
		require.NoError(t, err, c.name)

		// BLSSignEIP2537 step 3: hmG1EIP = MapFpToG1(msgHash).
		hmG1EIP, err := bls12381.MapFpToG1(msgHash)
		require.NoError(t, err, c.name)

		// On-chain verification formula (BaseBLS._verifyBLS).
		input := append(append(append(append([]byte{}, hmG1EIP...), aggPubEIP...), negSig...), bls12381.G2GeneratorEIP2537()...)
		require.Len(t, input, 768, c.name)
		require.NoError(t, VerifyPairing(input), c.name)

		t.Logf("%s: signer-path pairing OK (n=%d t=%d)", c.name, n, threshold)
	}

	// Negative control: a signature over msg A must not verify against the
	// hash-to-curve point of msg B (forgery / replay of published sig).
	origHash := crypto.Keccak256(cases[0].msgs[0])
	origSig := sigOf(t, ps, shares, threshold, n, origHash)
	tamperedHash := crypto.Keccak256([]byte("tampered-msg"))
	hmTampered, err := bls12381.MapFpToG1(tamperedHash)
	require.NoError(t, err)
	sigEIP, err := bls12381.G1ToEIP2537(origSig)
	require.NoError(t, err)
	negSig, err := bls12381.NegateG1EIP2537(sigEIP)
	require.NoError(t, err)
	badInput := append(append(append(append([]byte{}, hmTampered...), aggPubEIP...), negSig...), bls12381.G2GeneratorEIP2537()...)
	require.Error(t, VerifyPairing(badInput), "published sig must not verify on tampered msg")
}

// sigOf re-runs the production aggregation for an arbitrary msgHash.
func sigOf(t *testing.T, ps circl.Suite, shares []*pedersen.DistKeyShare, threshold, n int, msgHash []byte) []byte {
	t.Helper()
	hmCompressed, err := bls12381.MapFpToG1Compressed(msgHash)
	require.NoError(t, err)
	hmG1 := ps.G1().Point()
	require.NoError(t, hmG1.UnmarshalBinary(hmCompressed))
	pubShares := make([]*share.PubShare, 0, threshold)
	for i := 0; i < threshold; i++ {
		shareIdx := shares[i].PriShare().I
		sigPoint := ps.G1().Point().Mul(shares[i].PriShare().V, hmG1)
		pubShares = append(pubShares, &share.PubShare{I: shareIdx, V: sigPoint})
	}
	sigPoint, err := share.RecoverCommit(ps.G1(), pubShares, uint32(threshold), uint32(n))
	require.NoError(t, err)
	sig, err := sigPoint.MarshalBinary()
	require.NoError(t, err)
	return sig
}

var _ kyber.Point
var _ = big.NewInt
