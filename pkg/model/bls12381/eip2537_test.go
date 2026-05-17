package bls12381

import (
	"testing"

	"go.dedis.ch/kyber/v4"
	"go.dedis.ch/kyber/v4/pairing/bls12381/circl"
	"go.dedis.ch/kyber/v4/share"
	"go.dedis.ch/kyber/v4/sign/bls"
	"go.dedis.ch/kyber/v4/sign/tbls"
	pedersen "go.dedis.ch/kyber/v4/share/dkg/pedersen"
	"go.dedis.ch/kyber/v4/sign/schnorr"
)

// TestEIP2537RoundTrip verifies G1/G2 conversion and negation round-trip.
func TestEIP2537RoundTrip(t *testing.T) {
	sigSuite := circl.NewSuite()
	msg := []byte("round-trip test")

	hm, err := HashToG1(msg)
	if err != nil {
		t.Fatalf("HashToG1: %v", err)
	}

	// Generate a random keypair and create a valid BLS signature:
	// sig = sk * H(m)
	scalar := sigSuite.G1().Scalar().Pick(sigSuite.RandomStream())
	hmPoint := sigSuite.G1().Point()
	if err := hmPoint.UnmarshalBinary(hm); err != nil {
		t.Fatalf("unmarshal HM: %v", err)
	}
	sigPoint := sigSuite.G1().Point().Mul(scalar, hmPoint)
	sig, err := sigPoint.MarshalBinary()
	if err != nil {
		t.Fatalf("marshal sig: %v", err)
	}

	// G1 round-trip
	eipSig, err := G1ToEIP2537(sig)
	if err != nil {
		t.Fatalf("G1ToEIP2537: %v", err)
	}
	if len(eipSig) != EIP2537G1Size {
		t.Fatalf("expected %d bytes, got %d", EIP2537G1Size, len(eipSig))
	}

	eipNegSig, err := NegateG1EIP2537(eipSig)
	if err != nil {
		t.Fatalf("NegateG1EIP2537: %v", err)
	}
	if err := VerifyNegationRoundTrip(sig, eipNegSig); err != nil {
		t.Fatalf("VerifyNegationRoundTrip: %v", err)
	}

	// G2 round-trip (pubkey)
	pubPoint := sigSuite.G2().Point().Mul(scalar, nil)
	eipPub, err := G2ToEIP2537FromKyber(pubPoint)
	if err != nil {
		t.Fatalf("G2ToEIP2537FromKyber: %v", err)
	}
	if len(eipPub) != EIP2537G2Size {
		t.Fatalf("expected %d bytes, got %d", EIP2537G2Size, len(eipPub))
	}

	// G2 generator
	gen := G2GeneratorEIP2537()
	if len(gen) != EIP2537G2Size {
		t.Fatalf("expected %d bytes, got %d", EIP2537G2Size, len(gen))
	}

	// HM round-trip
	eipHM, err := G1ToEIP2537(hm)
	if err != nil {
		t.Fatalf("G1ToEIP2537(HM): %v", err)
	}
	if len(eipHM) != EIP2537G1Size {
		t.Fatalf("expected %d bytes, got %d", EIP2537G1Size, len(eipHM))
	}

	// Local pairing verification
	pubCompressed, err := pubPoint.MarshalBinary()
	if err != nil {
		t.Fatalf("marshal pubkey: %v", err)
	}
	if err := VerifyLocal(hm, sig, pubCompressed); err != nil {
		t.Fatalf("VerifyLocal: %v", err)
	}

	t.Log("All round-trip checks passed")
}

// TestEIP2537WithDKG verifies the full DKG → tbls → aggregate → EIP-2537 flow.
func TestEIP2537WithDKG(t *testing.T) {
	dkgSuite := circl.NewSuiteBLS12381()
	sigSuite := circl.NewSuite()

	n := 5
	thr := 3
	msg := []byte("eip-2537 dkg test")

	partPubs := make([]kyber.Point, n)
	partSec := make([]kyber.Scalar, n)
	for i := 0; i < n; i++ {
		sec := dkgSuite.Scalar().Pick(dkgSuite.RandomStream())
		partSec[i] = sec
		partPubs[i] = dkgSuite.Point().Mul(sec, nil)
	}

	shares := genDistSecretPedersen(t, dkgSuite, partSec, partPubs, thr)
	pubPoly := share.NewPubPoly(dkgSuite, dkgSuite.Point().Base(), shares[0].Commitments())
	groupPub := pubPoly.Commit()

	tblsScheme := tbls.NewThresholdSchemeOnG1(sigSuite)
	partialSigs := make([][]byte, 0, n)
	for i := 0; i < n; i++ {
		s, err := tblsScheme.Sign(shares[i].PriShare(), msg)
		if err != nil {
			t.Fatalf("Sign[%d]: %v", i, err)
		}
		partialSigs = append(partialSigs, s)
	}

	sig, err := tblsScheme.Recover(pubPoly, msg, partialSigs, uint32(thr), uint32(n))
	if err != nil {
		t.Fatalf("Recover: %v", err)
	}

	if err := bls.NewSchemeOnG1(sigSuite).Verify(groupPub, msg, sig); err != nil {
		t.Fatalf("bls.Verify: %v", err)
	}

	// Convert to EIP-2537 and verify round-trip
	hm, err := HashToG1(msg)
	if err != nil {
		t.Fatalf("HashToG1: %v", err)
	}
	pubkeyCompressed, err := groupPub.MarshalBinary()
	if err != nil {
		t.Fatalf("marshal pubkey: %v", err)
	}

	if err := VerifyLocal(hm, sig, pubkeyCompressed); err != nil {
		t.Fatalf("VerifyLocal: %v", err)
	}

	eipPub, err := G2ToEIP2537FromKyber(groupPub)
	if err != nil {
		t.Fatalf("G2ToEIP2537FromKyber: %v", err)
	}
	eipHM, err := G1ToEIP2537(hm)
	if err != nil {
		t.Fatalf("G1ToEIP2537(HM): %v", err)
	}
	eipSig, err := G1ToEIP2537(sig)
	if err != nil {
		t.Fatalf("G1ToEIP2537(sig): %v", err)
	}
	eipNegSig, err := NegateG1EIP2537(eipSig)
	if err != nil {
		t.Fatalf("NegateG1EIP2537: %v", err)
	}
	if err := VerifyNegationRoundTrip(sig, eipNegSig); err != nil {
		t.Fatalf("VerifyNegationRoundTrip: %v", err)
	}

	// Build exact pairing input that Solidity sends to 0x0f
	input := make([]byte, 0, 768)
	input = append(input, eipHM...)
	input = append(input, eipPub...)
	input = append(input, eipNegSig...)
	input = append(input, G2GeneratorEIP2537()...)

	if len(input) != 768 {
		t.Fatalf("expected 768 bytes pairing input, got %d", len(input))
	}

	t.Logf("PUBKEY: 0x%x", eipPub)
	t.Logf("HM:     0x%x", eipHM)
	t.Logf("NEGSIG: 0x%x", eipNegSig)
}

// genDistSecretPedersen runs a Pedersen DKG among n participants.
func genDistSecretPedersen(
	t *testing.T,
	suite pedersen.Suite,
	partSec []kyber.Scalar,
	partPubs []kyber.Point,
	thr int,
) []*pedersen.DistKeyShare {
	t.Helper()
	n := len(partSec)
	nonce := pedersen.GetNonce()
	nodes := make([]pedersen.Node, 0, n)
	for i := 0; i < n; i++ {
		nodes = append(nodes, pedersen.Node{Index: uint32(i), Public: partPubs[i]})
	}
	dkgs := make([]*pedersen.DistKeyGenerator, n)
	for i := 0; i < n; i++ {
		conf := &pedersen.Config{
			Suite: suite, Longterm: partSec[i], NewNodes: nodes,
			Threshold: uint32(thr), Nonce: nonce,
			Auth: schnorr.NewScheme(suite), FastSync: true,
		}
		dkg, err := pedersen.NewDistKeyHandler(conf)
		if err != nil {
			t.Fatalf("NewDistKeyHandler[%d]: %v", i, err)
		}
		dkgs[i] = dkg
	}
	deals := make([]*pedersen.DealBundle, 0, n)
	for _, dkg := range dkgs {
		b, err := dkg.Deals()
		if err != nil {
			t.Fatalf("Deals: %v", err)
		}
		deals = append(deals, b)
	}
	resps := make([]*pedersen.ResponseBundle, 0, n)
	for _, dkg := range dkgs {
		rb, err := dkg.ProcessDeals(deals)
		if err != nil {
			t.Fatalf("ProcessDeals: %v", err)
		}
		if rb != nil {
			resps = append(resps, rb)
		}
	}
	var justifs []*pedersen.JustificationBundle
	var results []*pedersen.Result
	for _, dkg := range dkgs {
		res, just, err := dkg.ProcessResponses(resps)
		if err != nil {
			t.Fatalf("ProcessResponses: %v", err)
		}
		if res != nil {
			results = append(results, res)
		}
		if just != nil {
			justifs = append(justifs, just)
		}
	}
	if len(results) == 0 {
		for _, dkg := range dkgs {
			res, err := dkg.ProcessJustifications(justifs)
			if err != nil {
				t.Fatalf("ProcessJustifications: %v", err)
			}
			if res != nil {
				results = append(results, res)
			}
		}
	} else if len(justifs) > 0 {
		results = nil
		for _, dkg := range dkgs {
			res, err := dkg.ProcessJustifications(justifs)
			if err != nil {
				t.Fatalf("ProcessJustifications: %v", err)
			}
			if res != nil {
				results = append(results, res)
			}
		}
	}
	out := make([]*pedersen.DistKeyShare, 0, len(results))
	for _, r := range results {
		out = append(out, r.Key)
	}
	return out
}
