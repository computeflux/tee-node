package bls12381

import (
	"encoding/hex"
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum/crypto"
)

func TestProbeMapFpVectors(t *testing.T) {
	hashes := []string{
		"74c6e080131281b6589a90097acb6f160767245563da120fe1f5dd460cd26bb1",
		"969e0b9fadb27af9f05f8d9d032a71361ea61b2b939b4cc6ce43832302b26e8b",
		"8b66f2f622881a33b089e7bbd48bb77d832044c70c305087d919a3be4c8affa9",
	}
	for _, h := range hashes {
		b, err := hex.DecodeString(h)
		if err != nil { t.Fatal(err) }
		out, err := MapFpToG1(b)
		if err != nil { t.Fatalf("MapFpToG1(%s): %v", h, err) }
		t.Logf("%s => %s", h, hex.EncodeToString(out))
		_ = strings.TrimSpace
		_ = crypto.Keccak256
	}
}
