// Package bls12381 provides BLS12-381 threshold signature helpers with
// EIP-2537 format conversion for on-chain verification.
package bls12381

import (
	"fmt"
	"sync"

	bls12381 "github.com/cloudflare/circl/ecc/bls12381"
	"go.dedis.ch/kyber/v4"
	"go.dedis.ch/kyber/v4/pairing/bls12381/circl"
)

const (
	// G1CompressedSize is the size of a compressed G1 point (48 bytes).
	G1CompressedSize = 48
	// G1UncompressedSize is the size of an uncompressed G1 point (96 bytes).
	G1UncompressedSize = 96
	// G2CompressedSize is the size of a compressed G2 point (96 bytes).
	G2CompressedSize = 96
	// G2UncompressedSize is the size of an uncompressed G2 point (192 bytes).
	G2UncompressedSize = 192

	// EIP2537G1Size is the EIP-2537 zero-padded G1 size (128 bytes).
	EIP2537G1Size = 128
	// EIP2537G2Size is the EIP-2537 zero-padded G2 size (256 bytes).
	EIP2537G2Size = 256
)

var (
	g2GenOnce   sync.Once
	g2GenCached []byte
)

// G1ToEIP2537 converts a 48-byte compressed G1 point to 128-byte EIP-2537 format.
func G1ToEIP2537(compressed []byte) ([]byte, error) {
	if len(compressed) != G1CompressedSize {
		return nil, fmt.Errorf("invalid G1 compressed size: %d, expected %d", len(compressed), G1CompressedSize)
	}
	var g1 bls12381.G1
	if err := g1.SetBytes(compressed); err != nil {
		return nil, fmt.Errorf("parse G1 compressed: %w", err)
	}
	raw := g1.Bytes()
	return pad48To64(raw[:48], raw[48:96]), nil
}

// G2ToEIP2537 converts a 96-byte compressed G2 point to 256-byte EIP-2537 format.
// EIP-2537 expects G2 coordinates in x0||x1||y0||y1 order (c0 before c1).
// Circl's G2.Bytes() returns x1||x0||y1||y0 order.
func G2ToEIP2537(compressed []byte) ([]byte, error) {
	if len(compressed) != G2CompressedSize {
		return nil, fmt.Errorf("invalid G2 compressed size: %d, expected %d", len(compressed), G2CompressedSize)
	}
	var g2 bls12381.G2
	if err := g2.SetBytes(compressed); err != nil {
		return nil, fmt.Errorf("parse G2 compressed: %w", err)
	}
	raw := g2.Bytes()
	// raw[:48] = x1, raw[48:96] = x0, raw[96:144] = y1, raw[144:192] = y0
	return pad48To64(raw[48:96], raw[:48], raw[144:192], raw[96:144]), nil
}

// G2ToEIP2537FromKyber converts a kyber G2 point directly to EIP-2537 format.
func G2ToEIP2537FromKyber(pubkey kyber.Point) ([]byte, error) {
	compressed, err := pubkey.MarshalBinary()
	if err != nil {
		return nil, fmt.Errorf("marshal pubkey: %w", err)
	}
	return G2ToEIP2537(compressed)
}

// NegateG1EIP2537 negates a G1 point in EIP-2537 format and returns the result.
func NegateG1EIP2537(eipG1 []byte) ([]byte, error) {
	if len(eipG1) != EIP2537G1Size {
		return nil, fmt.Errorf("invalid EIP-2537 G1 size: %d, expected %d", len(eipG1), EIP2537G1Size)
	}
	// EIP-2537 format: 16B pad + x(48B) + 16B pad + y(48B) = 128B
	// Standard uncompressed: x(48B) + y(48B) = 96B
	var standard [G1UncompressedSize]byte
	copy(standard[:48], eipG1[16:64])
	copy(standard[48:], eipG1[80:128])

	var g1 bls12381.G1
	if err := g1.SetBytes(standard[:]); err != nil {
		return nil, fmt.Errorf("parse standard G1: %w", err)
	}
	g1.Neg()
	raw := g1.Bytes()
	return pad48To64(raw[:48], raw[48:96]), nil
}

// G2GeneratorEIP2537 returns the G2 generator in EIP-2537 padded format (256 bytes).
// The result is computed once and cached.
func G2GeneratorEIP2537() []byte {
	g2GenOnce.Do(func() {
		g2GenCached = computeG2GeneratorEIP2537()
	})
	// Return a copy to prevent external mutation.
	out := make([]byte, EIP2537G2Size)
	copy(out, g2GenCached)
	return out
}

func computeG2GeneratorEIP2537() []byte {
	var g2 bls12381.G2
	if err := g2.SetBytes(g2BaseCompressed()); err != nil {
		// The G2 base is known-good; any error is a programming bug.
		panic(fmt.Sprintf("bls12381: failed to parse G2 base: %v", err))
	}
	raw := g2.Bytes()
	return pad48To64(raw[48:96], raw[:48], raw[144:192], raw[96:144])
}

func g2BaseCompressed() []byte {
	b, err := circl.NewSuite().G2().Point().Base().MarshalBinary()
	if err != nil {
		panic(fmt.Sprintf("bls12381: failed to marshal G2 base: %v", err))
	}
	return b
}

// pad48To64 left-zero-pads each 48-byte part to 64 bytes and concatenates them.
func pad48To64(parts ...[]byte) []byte {
	out := make([]byte, 64*len(parts))
	for i, p := range parts {
		copy(out[i*64+16:(i+1)*64], p)
	}
	return out
}
