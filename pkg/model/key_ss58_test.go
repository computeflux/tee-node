package model

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSS58_RoundTrip(t *testing.T) {
	pubkey := bytes.Repeat([]byte{0xAB}, 32)
	format := uint16(42)

	encoded := SS58Encode(pubkey, format)
	require.NotEmpty(t, encoded)

	decodedFormat, decodedPubkey, err := SS58Decode(encoded)
	require.NoError(t, err)
	assert.Equal(t, format, decodedFormat)
	assert.True(t, bytes.Equal(pubkey, decodedPubkey))
}

func TestSS58_RoundTrip_ShortFormat(t *testing.T) {
	pubkey := bytes.Repeat([]byte{0xCD}, 32)
	format := uint16(0) // Substrate default

	encoded := SS58Encode(pubkey, format)
	decodedFormat, decodedPubkey, err := SS58Decode(encoded)
	require.NoError(t, err)
	assert.Equal(t, format, decodedFormat)
	assert.True(t, bytes.Equal(pubkey, decodedPubkey))
}

func TestSS58_RoundTrip_LongFormat(t *testing.T) {
	pubkey := bytes.Repeat([]byte{0xEF}, 32)
	format := uint16(1000) // 2-byte format, avoid boundary 63

	encoded := SS58Encode(pubkey, format)
	decodedFormat, decodedPubkey, err := SS58Decode(encoded)
	require.NoError(t, err)
	assert.Equal(t, format, decodedFormat)
	assert.True(t, bytes.Equal(pubkey, decodedPubkey))
}

func TestSS58Decode_InvalidChecksum(t *testing.T) {
	// Corrupt a valid SS58 address
	pubkey := bytes.Repeat([]byte{0x12}, 32)
	encoded := SS58Encode(pubkey, 42)
	corrupted := encoded[:len(encoded)-1] + "X"

	_, _, err := SS58Decode(corrupted)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "checksum")
}

func TestSS58Decode_TooShort(t *testing.T) {
	_, _, err := SS58Decode("ab")
	assert.Error(t, err)
}

func TestSS58Encode_DifferentPubkeys(t *testing.T) {
	format := uint16(42)
	pub1 := bytes.Repeat([]byte{0x01}, 32)
	pub2 := bytes.Repeat([]byte{0x02}, 32)

	enc1 := SS58Encode(pub1, format)
	enc2 := SS58Encode(pub2, format)

	assert.NotEqual(t, enc1, enc2)
}

func TestSS58_HashConsistency(t *testing.T) {
	// Same input should always produce same hash
	data := []byte{0x01, 0x02, 0x03}
	h1 := ss58hash(data)
	h2 := ss58hash(data)
	assert.Equal(t, h1, h2)
}
