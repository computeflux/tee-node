package revive

import (
	"encoding/binary"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// abiEncodeString 按 Solidity ABI 编码 string（动态类型：offset + length + 右填充数据）。
func abiEncodeString(s string) []byte {
	length := len(s)
	padded := (length + 31) / 32 * 32
	data := make([]byte, 64+padded)
	data[31] = 0x20 // offset = 32
	binary.BigEndian.PutUint64(data[56:64], uint64(length))
	copy(data[64:], s)
	return data
}

func TestABIDecodeString(t *testing.T) {
	t.Run("short string", func(t *testing.T) {
		s, err := abiDecodeString(abiEncodeString("USDC"))
		require.NoError(t, err)
		assert.Equal(t, "USDC", s)
	})

	t.Run("long string", func(t *testing.T) {
		name := strings.Repeat("A", 33)
		s, err := abiDecodeString(abiEncodeString(name))
		require.NoError(t, err)
		assert.Equal(t, name, s)
	})

	t.Run("exactly 32 bytes", func(t *testing.T) {
		name := strings.Repeat("B", 32)
		s, err := abiDecodeString(abiEncodeString(name))
		require.NoError(t, err)
		assert.Equal(t, name, s)
	})

	t.Run("empty string", func(t *testing.T) {
		s, err := abiDecodeString(abiEncodeString(""))
		require.NoError(t, err)
		assert.Equal(t, "", s)
	})

	t.Run("too short", func(t *testing.T) {
		_, err := abiDecodeString([]byte{0x01, 0x02})
		require.Error(t, err)
	})

	t.Run("offset out of range", func(t *testing.T) {
		data := make([]byte, 64)
		data[31] = 0x40 // offset=64 > len-32
		_, err := abiDecodeString(data)
		require.Error(t, err)
	})
}

func TestABIDecodeUint8(t *testing.T) {
	t.Run("value right-aligned", func(t *testing.T) {
		// decimals() = 18 → 32 字节 word，值在最后 1 字节
		data := make([]byte, 32)
		data[31] = 18

		v, err := abiDecodeUint8(data)
		require.NoError(t, err)
		assert.Equal(t, uint8(18), v)
	})

	t.Run("too short", func(t *testing.T) {
		_, err := abiDecodeUint8([]byte{0x01})
		require.Error(t, err)
	})
}
