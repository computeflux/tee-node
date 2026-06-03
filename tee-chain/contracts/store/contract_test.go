package store

import (
	"bytes"
	"encoding/hex"
	"math/big"
	"testing"

	gsrpcTypes "github.com/centrifuge/go-substrate-rpc-client/v4/types"
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

// ---------------------------------------------------------------------------
// 新旧 codec 交叉兼容性测试 —— 确保自研 codec 与 go-substrate-rpc-client codec
// 对 UniAddr / U256 的编解码结果完全一致。
// ---------------------------------------------------------------------------

func TestUniAddrCodecCompat(t *testing.T) {
	testCases := []UniAddr{
		{T: 0, V: nil},
		{T: 0, V: []byte{}},
		{T: 1, V: []byte("alice")},
		{T: 42, V: make([]byte, 32)},
		{T: ^uint32(0), V: bytes.Repeat([]byte{0xff}, 32)},
		{T: 256, V: []byte{0xde, 0xad, 0xbe, 0xef}},
	}

	for _, original := range testCases {
		// 1) 旧 codec 编码 → 新 codec 解码
		oldEnc, err := codec.Encode(original)
		require.NoError(t, err, "old codec encode UniAddr")

		var newDec UniAddr
		err = ScaleDecode(&newDec, oldEnc)
		require.NoError(t, err, "new codec decode UniAddr from old bytes")
		require.Equal(t, original.T, newDec.T, "T mismatch (old→new)")
		// 旧 codec 空 []byte 解码为 nil，允许兼容
		if len(original.V) == 0 {
			require.Empty(t, newDec.V, "V mismatch (old→new)")
		} else {
			require.Equal(t, original.V, newDec.V, "V mismatch (old→new)")
		}

		// 2) 新 codec 编码 → 旧 codec 解码
		newEnc, err := ScaleEncode(original)
		require.NoError(t, err, "new codec encode UniAddr")

		var oldDec UniAddr
		err = codec.Decode(newEnc, &oldDec)
		require.NoError(t, err, "old codec decode UniAddr from new bytes")
		require.Equal(t, original.T, oldDec.T, "T mismatch (new→old)")
		if len(original.V) == 0 {
			require.Empty(t, oldDec.V, "V mismatch (new→old)")
		} else {
			require.Equal(t, original.V, oldDec.V, "V mismatch (new→old)")
		}

		// 3) 两者编码结果逐字节一致
		require.Equal(t, oldEnc, newEnc, "old and new codec produce different bytes for UniAddr")
	}
}

func TestU256CodecCompat(t *testing.T) {
	// 使用旧 types.U256 生成参考字节，再用新 codec 解码为 store.U256
	// 然后反方向：新 codec 编码 store.U256，旧 codec 解码为 types.U256
	testVals := []*big.Int{
		big.NewInt(0),
		big.NewInt(1),
		big.NewInt(1000000),
		new(big.Int).Sub(new(big.Int).Lsh(big.NewInt(1), 255), big.NewInt(1)), // 2^255 - 1
	}

	for _, val := range testVals {
		// ── 方向 A：旧 types.U256 → 旧 codec 编码 → 新 codec 解码为 store.U256 ──
		oldU := gsrpcTypes.NewU256(*val)
		oldEnc, err := codec.Encode(oldU)
		require.NoError(t, err, "old codec encode types.U256 val=%s", val)

		var newU U256
		newU.Int = new(big.Int)
		err = ScaleDecode(&newU, oldEnc)
		require.NoError(t, err, "new codec decode store.U256 from old bytes val=%s", val)
		require.Equal(t, 0, val.Cmp(newU.Int), "value mismatch after old-encode new-decode")

		// ── 方向 B：新 store.U256 → 新 codec 编码 → 旧 codec 解码为 types.U256 ──
		storeU := U256{Int: new(big.Int).Set(val)}
		newEnc, err := ScaleEncode(storeU)
		require.NoError(t, err, "new codec encode store.U256 val=%s", val)

		var oldDec gsrpcTypes.U256
		err = codec.Decode(newEnc, &oldDec)
		require.NoError(t, err, "old codec decode types.U256 from new bytes val=%s", val)
		require.NotNil(t, oldDec.Int, "decoded types.U256.Int is nil")
		require.Equal(t, 0, val.Cmp(oldDec.Int), "value mismatch after new-encode old-decode")

		// ── 3) 两者编码结果逐字节一致 ──
		require.Equal(t, 32, len(oldEnc), "old codec produced unexpected byte length")
		require.Equal(t, 32, len(newEnc), "new codec produced unexpected byte length")
		require.Equal(t, oldEnc, newEnc, "old and new codec produce different bytes for U256=%s", val)
	}
}
