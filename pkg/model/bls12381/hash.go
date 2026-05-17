package bls12381

import (
	"fmt"

	"go.dedis.ch/kyber/v4"
	"go.dedis.ch/kyber/v4/pairing/bls12381/circl"
)

// HashToG1 hashes a message to G1 using the standard BLS12-381 hash-to-curve.
// Returns a 48-byte compressed G1 point.
func HashToG1(msg []byte) ([]byte, error) {
	suite := circl.NewSuite()
	hm := suite.G1().Point().(kyber.HashablePoint).Hash(msg)
	b, err := hm.MarshalBinary()
	if err != nil {
		return nil, fmt.Errorf("marshal hashed point: %w", err)
	}
	if len(b) != G1CompressedSize {
		return nil, fmt.Errorf("unexpected G1 compressed size: %d", len(b))
	}
	return b, nil
}
