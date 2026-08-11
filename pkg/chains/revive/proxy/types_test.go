package proxy

import (
	"bytes"
	"testing"

	"github.com/centrifuge/go-substrate-rpc-client/v4/scale"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestError_EncodeDecode_RoundTrip 验证所有错误变体的 encode/decode 往返一致
func TestError_EncodeDecode_RoundTrip(t *testing.T) {
	cases := []struct {
		name    string
		variant func() Error
		check   func(Error) bool
	}{
		{"Unauthorized", func() Error { return Error{Unauthorized: new(bool)} }, func(e Error) bool { return e.Unauthorized != nil }},
		{"AddressNotFound", func() Error { return Error{AddressNotFound: new(bool)} }, func(e Error) bool { return e.AddressNotFound != nil }},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			orig := tc.variant()

			var buf bytes.Buffer
			enc := scale.NewEncoder(&buf)
			require.NoError(t, orig.Encode(*enc))
			assert.Len(t, buf.Bytes(), 1)

			var got Error
			dec := scale.NewDecoder(bytes.NewReader(buf.Bytes()))
			require.NoError(t, got.Decode(*dec))
			assert.True(t, tc.check(got), "decoded variant mismatch")
			assert.Equal(t, orig.Error(), got.Error())
		})
	}
}

func TestError_Encode_Unknown(t *testing.T) {
	var buf bytes.Buffer
	enc := scale.NewEncoder(&buf)
	err := (Error{}).Encode(*enc)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unrecognized enum")
}

func TestError_Decode_UnknownVariant(t *testing.T) {
	var got Error
	dec := scale.NewDecoder(bytes.NewReader([]byte{0xFF}))
	require.Error(t, got.Decode(*dec))
}

func TestError_Error_String(t *testing.T) {
	var empty Error
	assert.Equal(t, "Unknown", empty.Error())
	unauth := Error{Unauthorized: new(bool)}
	assert.Equal(t, "Unauthorized", unauth.Error())
	notFound := Error{AddressNotFound: new(bool)}
	assert.Equal(t, "AddressNotFound", notFound.Error())
}

func TestInitProxyContract(t *testing.T) {
	addr40 := make([]byte, 40)
	for i := range addr40 {
		addr40[i] = '0'
	}
	addr40[0], addr40[1] = '1', '1'
	addrStr := string(addr40)

	t.Run("valid_with_0x", func(t *testing.T) {
		p, err := InitProxyContract(nil, "0x"+addrStr)
		require.NoError(t, err)
		assert.Equal(t, "0x"+addrStr, p.Address.Hex())
	})

	t.Run("valid_without_0x", func(t *testing.T) {
		p, err := InitProxyContract(nil, addrStr)
		require.NoError(t, err)
		assert.Equal(t, "0x"+addrStr, p.Address.Hex())
	})

	t.Run("invalid_hex", func(t *testing.T) {
		_, err := InitProxyContract(nil, "zz")
		require.Error(t, err)
	})
}
