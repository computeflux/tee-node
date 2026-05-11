package model

import (
	"bytes"
	"encoding/hex"
	"testing"

	"github.com/centrifuge/go-substrate-rpc-client/v4/types/codec"
	"github.com/stretchr/testify/require"
)

func TestUniAddrScale(t *testing.T) {
	// 测试用例：编码 -> 解码 -> 验证
	t.Run("encode_decode_roundtrip", func(t *testing.T) {
		addr := "1234567890123456789012345678901234567890123456789012345678901234"
		bt, _ := hex.DecodeString(addr)

		original := UniAddr{
			T: 1,
			V: bt,
		}

		// 编码
		encoded, err := codec.Encode(original)
		require.NoError(t, err)
		t.Logf("Encoded bytes: %x (len=%d)", encoded, len(encoded))

		// 解码
		var decoded UniAddr
		err = codec.Decode(encoded, &decoded)
		require.NoError(t, err)

		// 验证
		require.Equal(t, original.T, decoded.T)
		require.Equal(t, original.V, decoded.V)
	})

	// 测试空 V
	t.Run("empty_V", func(t *testing.T) {
		original := UniAddr{
			T: 0,
			V: []byte{},
		}

		encoded, err := codec.Encode(original)
		require.NoError(t, err)

		var decoded UniAddr
		err = codec.Decode(encoded, &decoded)
		require.NoError(t, err)

		require.Equal(t, original.T, decoded.T)
		// SCALE 解码空字节切片返回 nil，而非空切片
		require.Empty(t, decoded.V)
	})

	// 测试较大值
	t.Run("large_values", func(t *testing.T) {
		original := UniAddr{
			T: ^uint32(0), // max uint32
			V: bytes.Repeat([]byte{0xff}, 32),
		}

		encoded, err := codec.Encode(original)
		require.NoError(t, err)

		var decoded UniAddr
		err = codec.Decode(encoded, &decoded)
		require.NoError(t, err)

		require.Equal(t, original.T, decoded.T)
		require.Equal(t, original.V, decoded.V)
	})

	// 测试不同类型 T 的值
	t.Run("different_T_values", func(t *testing.T) {
		for _, tVal := range []uint32{0, 1, 2, 100, 255, 256, 65535, 65536} {
			original := UniAddr{
				T: tVal,
				V: []byte{0xde, 0xad, 0xbe, 0xef},
			}

			encoded, err := codec.Encode(original)
			require.NoError(t, err)

			var decoded UniAddr
			err = codec.Decode(encoded, &decoded)
			require.NoError(t, err)

			require.Equal(t, original.T, decoded.T, "T mismatch for tVal=%d", tVal)
			require.Equal(t, original.V, decoded.V, "V mismatch for tVal=%d", tVal)
		}
	})
}
