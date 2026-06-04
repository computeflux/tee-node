package model

import (
	"encoding/json"
	"math/big"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewU128_Normal(t *testing.T) {
	u := NewU128(big.NewInt(42))
	assert.Equal(t, int64(42), u.Int64())
}

func TestNewU128_Nil(t *testing.T) {
	u := NewU128(nil)
	assert.Equal(t, int64(0), u.Int64())
}

func TestNewU128_Negative(t *testing.T) {
	u := NewU128(big.NewInt(-5))
	assert.Equal(t, int64(0), u.Int64())
}

func TestNewU128_Max(t *testing.T) {
	max := new(big.Int).Lsh(big.NewInt(1), 128)
	max.Sub(max, big.NewInt(1))
	u := NewU128(max)
	assert.Equal(t, 0, u.Cmp(max))
}

func TestNewU128_Overflow(t *testing.T) {
	over := new(big.Int).Lsh(big.NewInt(1), 129)
	u := NewU128(over)
	max := u128Max()
	assert.Equal(t, 0, u.Cmp(max), "should clamp to max")
}

func TestNewU128FromString_Valid(t *testing.T) {
	u := NewU128FromString("12345678901234567890")
	assert.Equal(t, "12345678901234567890", u.String())
}

func TestNewU128FromString_Empty(t *testing.T) {
	u := NewU128FromString("")
	assert.Equal(t, int64(0), u.Int64())
}

func TestNewU128FromString_Invalid(t *testing.T) {
	u := NewU128FromString("not-a-number")
	assert.Equal(t, int64(0), u.Int64())
}

func TestU128_MarshalJSON(t *testing.T) {
	u := NewU128(big.NewInt(999))
	data, err := json.Marshal(u)
	require.NoError(t, err)
	assert.Equal(t, `"999"`, string(data))
}

func TestU128_UnmarshalJSON(t *testing.T) {
	var u U128
	err := json.Unmarshal([]byte(`"12345"`), &u)
	require.NoError(t, err)
	assert.Equal(t, int64(12345), u.Int64())
}

func TestU128_UnmarshalJSON_Empty(t *testing.T) {
	var u U128
	err := json.Unmarshal([]byte(`""`), &u)
	require.NoError(t, err)
	assert.Equal(t, int64(0), u.Int64())
}

func TestU128_UnmarshalJSON_Invalid(t *testing.T) {
	var u U128
	err := json.Unmarshal([]byte(`"abc"`), &u)
	assert.Error(t, err)
}

func TestU128ToBytes(t *testing.T) {
	b := U128ToBytes(big.NewInt(1))
	require.Len(t, b, 16)
	assert.Equal(t, byte(1), b[15]) // big-endian: last byte is LSB
}

func TestU128ToBytes_Nil(t *testing.T) {
	b := U128ToBytes(nil)
	require.Len(t, b, 16)
	assert.Equal(t, make([]byte, 16), b)
}

func TestBytesToU128(t *testing.T) {
	b := []byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1}
	z := BytesToU128(b)
	assert.Equal(t, int64(1), z.Int64())
}

func TestBytesToU128_Short(t *testing.T) {
	b := []byte{0, 0, 1}
	z := BytesToU128(b)
	assert.Equal(t, int64(1), z.Int64())
}

func TestBytesToU128_Long(t *testing.T) {
	b := make([]byte, 32)
	b[31] = 1
	z := BytesToU128(b)
	assert.Equal(t, int64(1), z.Int64())
}

func TestBytesToU128_Empty(t *testing.T) {
	z := BytesToU128([]byte{})
	assert.Equal(t, int64(0), z.Int64())
}

func TestU128_RoundTrip(t *testing.T) {
	original := NewU128FromString("340282366920938463463374607431768211455") // 2^128 - 1
	b := U128ToBytes(&original.Int)
	back := BytesToU128(b)
	assert.Equal(t, original.String(), back.String())
}
