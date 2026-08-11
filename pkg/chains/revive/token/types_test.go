package token

import (
	"bytes"
	"testing"

	"github.com/centrifuge/go-substrate-rpc-client/v4/scale"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// encodeError 将 Error 编码为字节
func encodeError(t *testing.T, e Error) []byte {
	t.Helper()
	var buf bytes.Buffer
	enc := scale.NewEncoder(&buf)
	require.NoError(t, e.Encode(*enc))
	return buf.Bytes()
}

// decodeError 从字节解码 Error
func decodeError(t *testing.T, b []byte) (Error, error) {
	t.Helper()
	var e Error
	dec := scale.NewDecoder(bytes.NewReader(b))
	err := e.Decode(*dec)
	return e, err
}

// TestError_EncodeDecode_RoundTrip 验证所有枚举变体的 encode/decode 往返一致
func TestError_EncodeDecode_RoundTrip(t *testing.T) {
	cases := []struct {
		name    string
		variant func() Error
		check   func(Error) bool
	}{
		{"OnlySubnet", func() Error { return Error{OnlySubnet: new(bool)} }, func(e Error) bool { return e.OnlySubnet != nil }},
		{"OnlyOwner", func() Error { return Error{OnlyOwner: new(bool)} }, func(e Error) bool { return e.OnlyOwner != nil }},
		{"AmountMustBeGreaterThanZero", func() Error { return Error{AmountMustBeGreaterThanZero: new(bool)} }, func(e Error) bool { return e.AmountMustBeGreaterThanZero != nil }},
		{"InsufficientBalance", func() Error { return Error{InsufficientBalance: new(bool)} }, func(e Error) bool { return e.InsufficientBalance != nil }},
		{"TransferFailed", func() Error { return Error{TransferFailed: new(bool)} }, func(e Error) bool { return e.TransferFailed != nil }},
		{"ZeroAddress", func() Error { return Error{ZeroAddress: new(bool)} }, func(e Error) bool { return e.ZeroAddress != nil }},
		{"ERC20NotSupported", func() Error { return Error{ERC20NotSupported: new(bool)} }, func(e Error) bool { return e.ERC20NotSupported != nil }},
		{"ERC20Inactive", func() Error { return Error{ERC20Inactive: new(bool)} }, func(e Error) bool { return e.ERC20Inactive != nil }},
		{"ERC20TransferFailed", func() Error { return Error{ERC20TransferFailed: new(bool)} }, func(e Error) bool { return e.ERC20TransferFailed != nil }},
		{"NativeDisabled", func() Error { return Error{NativeDisabled: new(bool)} }, func(e Error) bool { return e.NativeDisabled != nil }},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			orig := tc.variant()
			b := encodeError(t, orig)
			// 每个变体编码为单个字节的变体索引
			assert.Len(t, b, 1)

			got, err := decodeError(t, b)
			require.NoError(t, err)
			assert.True(t, tc.check(got), "decoded variant mismatch")
			assert.Equal(t, orig.Error(), got.Error())
		})
	}
}

// TestError_Encode_Unknown 空枚举（无任何变体）编码应报错
func TestError_Encode_Unknown(t *testing.T) {
	var buf bytes.Buffer
	enc := scale.NewEncoder(&buf)
	err := (Error{}).Encode(*enc)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unrecognized enum")
}

// TestError_Decode_UnknownVariant 未知变体索引解码应报错
func TestError_Decode_UnknownVariant(t *testing.T) {
	_, err := decodeError(t, []byte{0xFF})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unrecognized enum")
}

// TestError_Error_String 空枚举的 Error() 应返回 Unknown
func TestError_Error_String(t *testing.T) {
	var empty Error
	assert.Equal(t, "Unknown", empty.Error())
	onlySubnet := Error{OnlySubnet: new(bool)}
	assert.Equal(t, "OnlySubnet", onlySubnet.Error())
}

// hex20 生成 20 字节的 hex 串（40 字符）
func hex20(prefix string) string {
	b := make([]byte, 40)
	for i := range b {
		b[i] = '0'
	}
	copy(b, prefix)
	return string(b)
}

// TestInitTokenContract 地址解析：合法 hex（带/不带 0x）、非法 hex、短地址补零
func TestInitTokenContract(t *testing.T) {
	addr40 := hex20("11") // 0x11 开头、其余为 0

	t.Run("valid_with_0x", func(t *testing.T) {
		tok, err := InitTokenContract(nil, "0x"+addr40)
		require.NoError(t, err)
		assert.Equal(t, "0x"+addr40, tok.Address.Hex())
	})

	t.Run("valid_without_0x", func(t *testing.T) {
		tok, err := InitTokenContract(nil, addr40)
		require.NoError(t, err)
		assert.Equal(t, "0x"+addr40, tok.Address.Hex())
	})

	t.Run("short_address_padded", func(t *testing.T) {
		// ink.go 的 HexToH160 对短地址补零而非报错（行为记录，非缺陷修复）
		tok, err := InitTokenContract(nil, "0x11")
		require.NoError(t, err)
		assert.Equal(t, byte(0x11), tok.Address[0])
		assert.Equal(t, byte(0), tok.Address[19])
	})

	t.Run("invalid_hex", func(t *testing.T) {
		_, err := InitTokenContract(nil, "zz")
		require.Error(t, err)
	})
}

// TestToken_Getters Client/ContractAddress 返回构造值
func TestToken_Getters(t *testing.T) {
	addr40 := hex20("11")
	tok, err := InitTokenContract(nil, "0x"+addr40)
	require.NoError(t, err)
	assert.Nil(t, tok.Client())
	assert.Equal(t, "0x"+addr40, tok.ContractAddress().Hex())
}
