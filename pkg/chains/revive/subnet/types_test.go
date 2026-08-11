package subnet

import (
	"bytes"
	"testing"

	"github.com/centrifuge/go-substrate-rpc-client/v4/scale"
	"github.com/centrifuge/go-substrate-rpc-client/v4/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func encodeInto[T any](t *testing.T, v T, encode func(T, scale.Encoder) error) []byte {
	t.Helper()
	var buf bytes.Buffer
	enc := scale.NewEncoder(&buf)
	require.NoError(t, encode(v, *enc))
	return buf.Bytes()
}

// ── Error 枚举 ─────────────────────────────────────────────

// TestError_EncodeDecode_RoundTrip 验证所有错误变体的 encode/decode 往返一致
func TestError_EncodeDecode_RoundTrip(t *testing.T) {
	cases := []struct {
		name    string
		variant func() Error
		check   func(Error) bool
	}{
		{"NotEnoughBalance", func() Error { return Error{NotEnoughBalance: new(bool)} }, func(e Error) bool { return e.NotEnoughBalance != nil }},
		{"MustCallByMainContract", func() Error { return Error{MustCallByMainContract: new(bool)} }, func(e Error) bool { return e.MustCallByMainContract != nil }},
		{"WorkerNotExist", func() Error { return Error{WorkerNotExist: new(bool)} }, func(e Error) bool { return e.WorkerNotExist != nil }},
		{"WorkerNotOwnedByCaller", func() Error { return Error{WorkerNotOwnedByCaller: new(bool)} }, func(e Error) bool { return e.WorkerNotOwnedByCaller != nil }},
		{"WorkerStatusNotReady", func() Error { return Error{WorkerStatusNotReady: new(bool)} }, func(e Error) bool { return e.WorkerStatusNotReady != nil }},
		{"WorkerMortgageNotExist", func() Error { return Error{WorkerMortgageNotExist: new(bool)} }, func(e Error) bool { return e.WorkerMortgageNotExist != nil }},
		{"TransferFailed", func() Error { return Error{TransferFailed: new(bool)} }, func(e Error) bool { return e.TransferFailed != nil }},
		{"WorkerIsUseByUser", func() Error { return Error{WorkerIsUseByUser: new(bool)} }, func(e Error) bool { return e.WorkerIsUseByUser != nil }},
		{"NodeNotExist", func() Error { return Error{NodeNotExist: new(bool)} }, func(e Error) bool { return e.NodeNotExist != nil }},
		{"SecretNodeAlreadyExists", func() Error { return Error{SecretNodeAlreadyExists: new(bool)} }, func(e Error) bool { return e.SecretNodeAlreadyExists != nil }},
		{"SetCodeFailed", func() Error { return Error{SetCodeFailed: new(bool)} }, func(e Error) bool { return e.SetCodeFailed != nil }},
		{"EpochNotExpired", func() Error { return Error{EpochNotExpired: new(bool)} }, func(e Error) bool { return e.EpochNotExpired != nil }},
		{"InvalidTeeChainSignature", func() Error { return Error{InvalidTeeChainSignature: new(bool)} }, func(e Error) bool { return e.InvalidTeeChainSignature != nil }},
		{"NodeIsRunning", func() Error { return Error{NodeIsRunning: new(bool)} }, func(e Error) bool { return e.NodeIsRunning != nil }},
		{"InvalidTeeChainCaller", func() Error { return Error{InvalidTeeChainCaller: new(bool)} }, func(e Error) bool { return e.InvalidTeeChainCaller != nil }},
		{"RegionNotExist", func() Error { return Error{RegionNotExist: new(bool)} }, func(e Error) bool { return e.RegionNotExist != nil }},
		{"AssetNotExist", func() Error { return Error{AssetNotExist: new(bool)} }, func(e Error) bool { return e.AssetNotExist != nil }},
		{"DepositNotEnough", func() Error { return Error{DepositNotEnough: new(bool)} }, func(e Error) bool { return e.DepositNotEnough != nil }},
		{"MortgageNotEnough", func() Error { return Error{MortgageNotEnough: new(bool)} }, func(e Error) bool { return e.MortgageNotEnough != nil }},
		{"SlashAmountTooLarge", func() Error { return Error{SlashAmountTooLarge: new(bool)} }, func(e Error) bool { return e.SlashAmountTooLarge != nil }},
		{"CloudContractNotSet", func() Error { return Error{CloudContractNotSet: new(bool)} }, func(e Error) bool { return e.CloudContractNotSet != nil }},
		{"ResourceNotEnough", func() Error { return Error{ResourceNotEnough: new(bool)} }, func(e Error) bool { return e.ResourceNotEnough != nil }},
		{"InvalidBlsKey", func() Error { return Error{InvalidBlsKey: new(bool)} }, func(e Error) bool { return e.InvalidBlsKey != nil }},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			orig := tc.variant()
			b := encodeInto(t, orig, Error.Encode)
			assert.Len(t, b, 1)

			var got Error
			dec := scale.NewDecoder(bytes.NewReader(b))
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
	neb := Error{NotEnoughBalance: new(bool)}
	assert.Equal(t, "NotEnoughBalance", neb.Error())
	ibl := Error{InvalidBlsKey: new(bool)}
	assert.Equal(t, "InvalidBlsKey", ibl.Error())
}

// ── AssetInfo 枚举 ─────────────────────────────────────────

func TestAssetInfo_RoundTrip(t *testing.T) {
	t.Run("native", func(t *testing.T) {
		native := []byte{1, 2, 3, 4}
		orig := AssetInfo{Native: &native}

		b := encodeInto(t, orig, AssetInfo.Encode)
		// 变体字节 0 + 长度前缀 + 数据
		assert.Equal(t, byte(0), b[0])

		var got AssetInfo
		dec := scale.NewDecoder(bytes.NewReader(b))
		require.NoError(t, got.Decode(*dec))
		require.NotNil(t, got.Native)
		assert.Equal(t, native, *got.Native)
	})

	t.Run("erc20", func(t *testing.T) {
		orig := AssetInfo{ERC20: &struct {
			F0 []byte
			F1 types.H256
		}{
			F0: []byte{9, 9},
			F1: types.H256([32]byte{7}),
		}}

		b := encodeInto(t, orig, AssetInfo.Encode)
		assert.Equal(t, byte(1), b[0])

		var got AssetInfo
		dec := scale.NewDecoder(bytes.NewReader(b))
		require.NoError(t, got.Decode(*dec))
		require.NotNil(t, got.ERC20)
		assert.Equal(t, []byte{9, 9}, got.ERC20.F0)
		assert.Equal(t, types.H256([32]byte{7}), got.ERC20.F1)
	})
}

// ── InitSubnetContract ─────────────────────────────────────

func TestInitSubnetContract(t *testing.T) {
	addr40 := make([]byte, 40)
	for i := range addr40 {
		addr40[i] = '0'
	}
	addr40[0], addr40[1] = '1', '1'
	addrStr := string(addr40)

	t.Run("valid_with_0x", func(t *testing.T) {
		s, err := InitSubnetContract(nil, "0x"+addrStr)
		require.NoError(t, err)
		assert.Equal(t, "0x"+addrStr, s.Address.Hex())
	})

	t.Run("valid_without_0x", func(t *testing.T) {
		s, err := InitSubnetContract(nil, addrStr)
		require.NoError(t, err)
		assert.Equal(t, "0x"+addrStr, s.Address.Hex())
	})

	t.Run("invalid_hex", func(t *testing.T) {
		_, err := InitSubnetContract(nil, "zz")
		require.Error(t, err)
	})
}
