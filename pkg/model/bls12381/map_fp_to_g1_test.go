package bls12381

import (
	"bytes"
	"crypto"
	"testing"

	bls12381 "github.com/cloudflare/circl/ecc/bls12381"
	"github.com/cloudflare/circl/expander"
)

func TestMapFpToG1MatchesEncode(t *testing.T) {
	msg := []byte("test message for map_fp_to_g1")

	// circl's Encode does: expander(64B) → SetBytes(64B) → sswu → evalIsog → clearCofactor
	var gExpected bls12381.G1
	gExpected.Encode(msg, nil)

	// To match, MapFpToG1 must receive the SAME field element that Encode uses.
	pseudo := expander.NewExpanderMD(crypto.SHA256, nil).Expand(msg, 64)

	// Our MapFpToG1 uses the field element directly
	eipOut, err := MapFpToG1(pseudo[:64])
	if err != nil {
		t.Fatalf("MapFpToG1 failed: %v", err)
	}

	circlOut := gExpected.Bytes()

	// Compare x and y coordinates
	if !bytes.Equal(eipOut[16:64], circlOut[:48]) {
		t.Errorf("x coordinate mismatch\ngot:      %x\nexpected: %x", eipOut[16:64], circlOut[:48])
	}
	if !bytes.Equal(eipOut[80:128], circlOut[48:96]) {
		t.Errorf("y coordinate mismatch\ngot:      %x\nexpected: %x", eipOut[80:128], circlOut[48:96])
	}
}

func TestMapFpToG1ValidPoint(t *testing.T) {
	fpElement := make([]byte, 32)
	fpElement[0] = 0x01

	eipOut, err := MapFpToG1(fpElement)
	if err != nil {
		t.Fatalf("MapFpToG1 failed: %v", err)
	}
	if len(eipOut) != 128 {
		t.Errorf("expected 128 bytes, got %d", len(eipOut))
	}

	// Construct uncompressed bytes for SetBytes
	// Format: 96 bytes total, x[0] contains header bits
	uncompressed := make([]byte, 96)
	copy(uncompressed[0:48], eipOut[16:64])
	copy(uncompressed[48:96], eipOut[80:128])

	var g bls12381.G1
	if err := g.SetBytes(uncompressed); err != nil {
		t.Fatalf("SetBytes failed: %v", err)
	}

	if !g.IsOnG1() {
		t.Errorf("MapFpToG1 result is not on G1")
	}
}
