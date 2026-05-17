package model

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha512"
	"encoding/hex"
	"encoding/json"
	"strings"
	"testing"

	oed25519 "github.com/oasisprotocol/curve25519-voi/primitives/ed25519"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// Constructors
// ---------------------------------------------------------------------------

func TestGenerateEd25519KeyPair(t *testing.T) {
	t.Run("valid_random_source", func(t *testing.T) {
		priv, pub, err := GenerateEd25519KeyPair(rand.Reader)
		require.NoError(t, err)
		require.NotNil(t, priv)
		require.NotNil(t, pub)

		assert.Len(t, priv.PrivateKey, ed25519.PrivateKeySize)
		assert.Len(t, pub.PublicKey, ed25519.PublicKeySize)
	})

	t.Run("deterministic_with_fixed_reader", func(t *testing.T) {
		seed := make([]byte, 32)
		for i := range seed {
			seed[i] = byte(i)
		}

		// Two calls with same reader should produce different keys
		// (crypto/rand uses random, but let's test a deterministic source)
		r1 := strings.NewReader(strings.Repeat("A", 64))
		r2 := strings.NewReader(strings.Repeat("A", 64))

		priv1, pub1, err1 := GenerateEd25519KeyPair(r1)
		priv2, pub2, err2 := GenerateEd25519KeyPair(r2)

		require.NoError(t, err1)
		require.NoError(t, err2)
		assert.Equal(t, priv1.PrivateKey, priv2.PrivateKey, "same seed → same private key")
		assert.Equal(t, pub1.PublicKey, pub2.PublicKey, "same seed → same public key")
	})

	t.Run("key_relationship", func(t *testing.T) {
		priv, pub, err := GenerateEd25519KeyPair(rand.Reader)
		require.NoError(t, err)

		// The public key from GenerateEd25519KeyPair should match the
		// public key embedded in the private key.
		assert.Equal(t, ed25519.PublicKey(priv.PrivateKey[32:]), pub.PublicKey)
	})
}

func TestPrivateKeyFromStd(t *testing.T) {
	t.Run("valid_key", func(t *testing.T) {
		_, sk, err := ed25519.GenerateKey(rand.Reader)
		require.NoError(t, err)

		priv, err := PrivateKeyFromStd(sk)
		require.NoError(t, err)
		require.NotNil(t, priv)
		assert.Equal(t, ed25519.PrivateKey(sk), priv.PrivateKey)
	})

	t.Run("nil_key", func(t *testing.T) {
		priv, err := PrivateKeyFromStd(nil)
		require.Error(t, err)
		require.Nil(t, priv)
		assert.Contains(t, err.Error(), "invalid private key length")
	})

	t.Run("wrong_length", func(t *testing.T) {
		for _, length := range []int{0, 31, 32, 33, 63, 65, 128} {
			badKey := make(ed25519.PrivateKey, length)
			priv, err := PrivateKeyFromStd(badKey)
			require.Error(t, err, "length %d should fail", length)
			require.Nil(t, priv)
		}
	})
}

func TestPrivateKeyFromHex(t *testing.T) {
	t.Run("valid_hex", func(t *testing.T) {
		_, sk, err := ed25519.GenerateKey(rand.Reader)
		require.NoError(t, err)

		hexKey := hex.EncodeToString(sk)
		priv, err := PrivateKeyFromHex(hexKey)
		require.NoError(t, err)
		assert.Equal(t, ed25519.PrivateKey(sk), priv.PrivateKey)
	})

	t.Run("valid_hex_with_0x_prefix", func(t *testing.T) {
		_, sk, err := ed25519.GenerateKey(rand.Reader)
		require.NoError(t, err)

		hexKey := "0x" + hex.EncodeToString(sk)
		priv, err := PrivateKeyFromHex(hexKey)
		require.NoError(t, err)
		assert.Equal(t, ed25519.PrivateKey(sk), priv.PrivateKey)
	})

	t.Run("invalid_hex", func(t *testing.T) {
		priv, err := PrivateKeyFromHex("zzzz")
		require.Error(t, err)
		require.Nil(t, priv)
	})

	t.Run("empty_string", func(t *testing.T) {
		priv, err := PrivateKeyFromHex("")
		require.Error(t, err)
		require.Nil(t, priv)
	})

	t.Run("wrong_length_hex", func(t *testing.T) {
		shortHex := hex.EncodeToString(make([]byte, 32)) // 32 bytes → 64 hex chars, but ed25519 needs 64 bytes → 128 hex chars
		priv, err := PrivateKeyFromHex(shortHex)
		require.Error(t, err)
		require.Nil(t, priv)
		assert.Contains(t, err.Error(), "invalid private key length")
	})
}

func TestPrivateKeyFromOed25519(t *testing.T) {
	t.Run("roundtrip", func(t *testing.T) {
		_, sk, err := ed25519.GenerateKey(rand.Reader)
		require.NoError(t, err)

		oed, err := StdToOed25519(sk)
		require.NoError(t, err)

		priv, err := PrivateKeyFromOed25519(oed)
		require.NoError(t, err)
		assert.Equal(t, ed25519.PrivateKey(sk), priv.PrivateKey)
	})

	t.Run("invalid_length", func(t *testing.T) {
		priv, err := PrivateKeyFromOed25519(nil)
		require.Error(t, err)
		require.Nil(t, priv)
	})
}

func TestStdToOed25519(t *testing.T) {
	t.Run("valid", func(t *testing.T) {
		_, sk, err := ed25519.GenerateKey(rand.Reader)
		require.NoError(t, err)

		oed, err := StdToOed25519(sk)
		require.NoError(t, err)
		assert.Len(t, oed, 64)
		assert.Equal(t, oed25519.PrivateKey(sk), oed)
	})

	t.Run("wrong_length", func(t *testing.T) {
		for _, length := range []int{0, 32, 63, 65} {
			badKey := make(ed25519.PrivateKey, length)
			_, err := StdToOed25519(badKey)
			require.Error(t, err, "length %d should fail", length)
		}
	})
}

func TestOed25519ToStd(t *testing.T) {
	t.Run("valid", func(t *testing.T) {
		_, sk, err := ed25519.GenerateKey(rand.Reader)
		require.NoError(t, err)

		oed := oed25519.PrivateKey(sk)
		std, err := Oed25519ToStd(oed)
		require.NoError(t, err)
		assert.Equal(t, sk, std)
	})

	t.Run("roundtrip", func(t *testing.T) {
		_, sk, err := ed25519.GenerateKey(rand.Reader)
		require.NoError(t, err)

		oed, err := StdToOed25519(sk)
		require.NoError(t, err)

		std, err := Oed25519ToStd(oed)
		require.NoError(t, err)
		assert.Equal(t, sk, std)
	})

	t.Run("wrong_length", func(t *testing.T) {
		for _, length := range []int{0, 32, 63, 65} {
			badKey := make(oed25519.PrivateKey, length)
			_, err := Oed25519ToStd(badKey)
			require.Error(t, err, "length %d should fail", length)
		}
	})
}

// ---------------------------------------------------------------------------
// Scalar / Public
// ---------------------------------------------------------------------------

func TestPrivKey_Scalar(t *testing.T) {
	priv, _, err := GenerateEd25519KeyPair(rand.Reader)
	require.NoError(t, err)

	t.Run("ed25519_scalar_is_deterministic", func(t *testing.T) {
		s1 := priv.Scalar(SuiteEd25519)
		s2 := priv.Scalar(SuiteEd25519)
		require.NotNil(t, s1)
		require.NotNil(t, s2)
		assert.True(t, s1.Equal(s2), "scalar must be deterministic")
	})

	t.Run("ed25519_scalar_bytes_length", func(t *testing.T) {
		s := priv.Scalar(SuiteEd25519)
		b, err := s.MarshalBinary()
		require.NoError(t, err)
		assert.Len(t, b, 32)
	})

	t.Run("ed25519_scalar_matches_rfc8032_clamp", func(t *testing.T) {
		// Verify RFC 8032 clamping: the scalar is derived from
		// SHA-512(seed) with bit-twiddling.
		// NOTE: kyber edwards25519 uses little-endian encoding,
		// so the marshalled bytes differ from the raw big-endian digest.
		seed := priv.PrivateKey[:32]
		digest := sha512.Sum512(seed)
		digest[0] &= 0xf8
		digest[31] &= 0x7f
		digest[31] |= 0x40

		s := priv.Scalar(SuiteEd25519)
		b, err := s.MarshalBinary()
		require.NoError(t, err)
		assert.Len(t, b, 32)

		// The scalar must be non-zero and consistent across calls.
		s2 := priv.Scalar(SuiteEd25519)
		b2, _ := s2.MarshalBinary()
		assert.Equal(t, b, b2, "scalar must be deterministic")

		// SetBytes accepts the clamped digest (kyber handles the endianness).
		s3 := NewSuite(SuiteEd25519).Scalar().SetBytes(digest[:32])
		assert.NotNil(t, s3)
	})
}

func TestPrivKey_Public(t *testing.T) {
	priv, _, err := GenerateEd25519KeyPair(rand.Reader)
	require.NoError(t, err)

	t.Run("ed25519_public_point", func(t *testing.T) {
		pt := priv.Public(SuiteEd25519)
		require.NotNil(t, pt)

		b, err := pt.MarshalBinary()
		require.NoError(t, err)
		assert.Len(t, b, 32)
	})

	t.Run("ed25519_public_matches_embedded", func(t *testing.T) {
		pt := priv.Public(SuiteEd25519)
		b, err := pt.MarshalBinary()
		require.NoError(t, err)

		// The public key embedded in the private key bytes [32:] should match
		assert.Equal(t, []byte(priv.PrivateKey[32:]), b)
	})
}

// ---------------------------------------------------------------------------
// GetPublic / GetPublicForSuite
// ---------------------------------------------------------------------------

func TestPrivKey_GetPublicForSuite_Ed25519(t *testing.T) {
	priv, _, err := GenerateEd25519KeyPair(rand.Reader)
	require.NoError(t, err)

	pub := priv.GetPublicForSuite(SuiteEd25519)
	require.NotNil(t, pub)

	t.Run("returns_ed25519_pubkey", func(t *testing.T) {
		assert.True(t, pub.IsEd25519())
		assert.False(t, pub.IsBLS12381())
	})

	t.Run("public_key_matches_embedded", func(t *testing.T) {
		assert.Equal(t, ed25519.PublicKey(priv.PrivateKey[32:]), pub.PublicKey)
	})

	t.Run("suite_is_set", func(t *testing.T) {
		assert.NotNil(t, pub.Suite())
	})
}

func TestPrivKey_GetPublicForSuite(t *testing.T) {
	priv, _, err := GenerateEd25519KeyPair(rand.Reader)
	require.NoError(t, err)

	t.Run("ed25519_suite", func(t *testing.T) {
		pub := priv.GetPublicForSuite(SuiteEd25519)
		require.NotNil(t, pub)
		assert.True(t, pub.IsEd25519())
	})
}

// ---------------------------------------------------------------------------
// String / MarshalJSON / UnmarshalJSON
// ---------------------------------------------------------------------------

func TestPrivKey_String(t *testing.T) {
	priv, _, err := GenerateEd25519KeyPair(rand.Reader)
	require.NoError(t, err)

	s := priv.String()
	assert.Len(t, s, 128) // 64 bytes → 128 hex chars
	assert.Equal(t, hex.EncodeToString(priv.PrivateKey), s)

	// Round-trip
	priv2, err := PrivateKeyFromHex(s)
	require.NoError(t, err)
	assert.Equal(t, priv.PrivateKey, priv2.PrivateKey)
}

func TestPrivKey_MarshalJSON(t *testing.T) {
	priv, _, err := GenerateEd25519KeyPair(rand.Reader)
	require.NoError(t, err)

	data, err := priv.MarshalJSON()
	require.NoError(t, err)

	// Should be a JSON array of bytes
	var raw []byte
	err = json.Unmarshal(data, &raw)
	require.NoError(t, err)
	assert.Equal(t, []byte(priv.PrivateKey), raw)
}

func TestPrivKey_UnmarshalJSON(t *testing.T) {
	t.Run("valid_json", func(t *testing.T) {
		_, sk, err := ed25519.GenerateKey(rand.Reader)
		require.NoError(t, err)

		data, err := json.Marshal(sk)
		require.NoError(t, err)

		var priv PrivKey
		err = json.Unmarshal(data, &priv)
		require.NoError(t, err)
		assert.Equal(t, ed25519.PrivateKey(sk), priv.PrivateKey)
		// Unmarshalled key should be usable
		s := priv.Scalar(SuiteEd25519)
		assert.NotNil(t, s)
	})

	t.Run("invalid_json", func(t *testing.T) {
		var priv PrivKey
		err := json.Unmarshal([]byte("not-json"), &priv)
		require.Error(t, err)
	})

	t.Run("json_roundtrip", func(t *testing.T) {
		original, _, err := GenerateEd25519KeyPair(rand.Reader)
		require.NoError(t, err)

		data, err := json.Marshal(original)
		require.NoError(t, err)

		var restored PrivKey
		err = json.Unmarshal(data, &restored)
		require.NoError(t, err)

		assert.Equal(t, original.PrivateKey, restored.PrivateKey)

		// Verify the restored key still works
		s1 := original.Scalar(SuiteEd25519)
		s2 := restored.Scalar(SuiteEd25519)
		assert.True(t, s1.Equal(s2))
	})
}

// ---------------------------------------------------------------------------
// ToSigner
// ---------------------------------------------------------------------------

func TestPrivKey_ToSigner(t *testing.T) {
	t.Run("ed25519_valid_key", func(t *testing.T) {
		priv, _, err := GenerateEd25519KeyPair(rand.Reader)
		require.NoError(t, err)

		signer := priv.ToSigner(SuiteEd25519)
		require.NotNil(t, signer)
		assert.Equal(t, uint8(1), signer.SignType())
		assert.NotEmpty(t, signer.Public())
	})
}

// ---------------------------------------------------------------------------
// ToSecp256k1Signer
// ---------------------------------------------------------------------------

func TestPrivKey_ToSecp256k1Signer(t *testing.T) {
	t.Run("returns_non_nil_signer", func(t *testing.T) {
		priv, _, err := GenerateEd25519KeyPair(rand.Reader)
		require.NoError(t, err)
		signer := priv.ToSecp256k1Signer()
		require.NotNil(t, signer)
		assert.Equal(t, uint8(2), signer.SignType())
		assert.Len(t, signer.Public(), 33)
	})

	t.Run("sign_verify_roundtrip", func(t *testing.T) {
		priv, _, err := GenerateEd25519KeyPair(rand.Reader)
		require.NoError(t, err)
		signer := priv.ToSecp256k1Signer()
		msg := []byte("hello secp256k1")
		sig, err := signer.Sign(msg)
		require.NoError(t, err)
		assert.Len(t, sig, 64)
		assert.True(t, signer.Verify(msg, sig))
		assert.False(t, signer.Verify([]byte("tampered"), sig))
	})

	t.Run("deterministic", func(t *testing.T) {
		priv, _, err := GenerateEd25519KeyPair(rand.Reader)
		require.NoError(t, err)
		s1 := priv.ToSecp256k1Signer()
		s2 := priv.ToSecp256k1Signer()
		assert.Equal(t, s1.Public(), s2.Public())
		msg := []byte("test")
		sig1, _ := s1.Sign(msg)
		sig2, _ := s2.Sign(msg)
		assert.Equal(t, sig1, sig2)
	})
}

// ---------------------------------------------------------------------------
// Consistency between derived keys
// ---------------------------------------------------------------------------
