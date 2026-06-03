package wasm

import (
	"math/big"
	"reflect"
	"testing"

	"github.com/computeflux/tee-provider/tee-chain/contracts/module/auth"
	"github.com/computeflux/tee-provider/tee-chain/contracts/module/batch"
	"github.com/computeflux/tee-provider/tee-chain/contracts/module/gateway"
	"github.com/computeflux/tee-provider/tee-chain/contracts/module/gov"
	"github.com/computeflux/tee-provider/tee-chain/contracts/store"
	"github.com/stretchr/testify/require"
)

// 生成代码 U256 往返 bug：Encode/Decode 字节序不一致（WriteZeroPadReverse vs ReversePad）
func TestCodec_Gov_Member(t *testing.T) {
	v := gov.Member{Account: store.UniAddr{T: 1, V: []byte("alice")}, Balance: store.Amount{Int: big.NewInt(100)}}
	testRoundTrip(t, &v)
}

func TestCodec_Gov_ProposalStatusQuery(t *testing.T) {
	v := gov.ProposalStatusQuery{State: gov.ProposalStatusOngoing, BlockHeight: 100}
	testRoundTrip(t, &v)
}

func TestCodec_Gov_ProposalResult(t *testing.T) {
	v := gov.ProposalResult{Result: store.NewNone[[]byte](), ExecError: store.NewSome([]byte("err"))}
	testRoundTrip(t, &v)
}

func TestCodec_Gov_Vote(t *testing.T) {
	v := gov.Vote{Index: 1, ProposalID: 42, Caller: store.UniAddr{T: 1, V: []byte("alice")},
		Pledge: store.Amount{Int: big.NewInt(1000)}, VoteWeight: store.Amount{Int: big.NewInt(500)},
		OpinionYes: true, VoteBlock: 500}
	testRoundTrip(t, &v)
}

func TestCodec_Gov_Spend(t *testing.T) {
	v := gov.Spend{ID: 1, Caller: store.UniAddr{T: 1, V: []byte("alice")},
		To: store.UniAddr{T: 2, V: []byte("bob")}, Amount: store.Amount{Int: big.NewInt(500)}}
	testRoundTrip(t, &v)
}

func TestCodec_Auth_UserInfo(t *testing.T) {
	v := auth.UserInfo{EmailHash: []byte("hash"), ShareRoot: make([]byte, 32), RegisteredAt: 100}
	testRoundTrip(t, &v)
}

func TestCodec_Auth_RecoveryConfig(t *testing.T) {
	v := auth.RecoveryConfig{Guardians: [][]byte{{1, 2, 3}}, Threshold: 3}
	testRoundTrip(t, &v)
}

func TestCodec_Auth_AuthConfig(t *testing.T) {
	v := auth.AuthConfig{SMTPHost: "smtp.test.com", SMTPPort: 465, MockEmail: true}
	testRoundTrip(t, &v)
}

func TestCodec_Gateway_APIKeyInfo(t *testing.T) {
	v := gateway.APIKeyInfo{
		OwnerKey: 12345, Owner: store.UniAddr{T: 1, V: []byte("alice")},
		KeyHash: make([]byte, 32), RateLimit: 100, IsActive: true, CreatedAt: 1234567890,
	}
	testRoundTrip(t, &v)
}

func TestCodec_Gateway_RechargeRecord(t *testing.T) {
	v := gateway.RechargeRecord{User: store.UniAddr{T: 1, V: []byte("alice")}, Amount: 1000, Time: 1234567890}
	testRoundTrip(t, &v)
}

func TestCodec_Gateway_ModelInfo(t *testing.T) {
	v := gateway.ModelInfo{Name: "gpt-4", ProviderID: 1, InPrice: 100}
	testRoundTrip(t, &v)
}

func TestCodec_Gateway_ProviderInfo(t *testing.T) {
	v := gateway.ProviderInfo{Name: "openai", Owner: store.UniAddr{T: 1, V: []byte("alice")}, IsActive: true}
	testRoundTrip(t, &v)
}

func TestCodec_Batch_BatchCall(t *testing.T) {
	v := batch.BatchCall{Name: []byte("gov"), Method: []byte{1, 2, 3, 4}, Args: [][]byte{{10, 20}}}
	testRoundTrip(t, &v)
}

func TestCodec_OptionOfTrackData(t *testing.T) {
	v := store.NewSome(gov.TrackData{
		Name: "custom", PreparePeriod: 50,
		DecisionDeposit: store.Amount{Int: big.NewInt(100)},
		MaxBalance:      store.Amount{Int: big.NewInt(10000)},
	})
	data, _ := store.ScaleEncode(v)
	var out store.Option[gov.TrackData]
	require.NoError(t, store.ScaleDecode(&out, data))
	require.True(t, out.IsSome())
}

func TestCodec_SliceOfAPIKeyInfo(t *testing.T) {
	v := []gateway.APIKeyInfo{
		{OwnerKey: 1, IsActive: true, KeyHash: make([]byte, 32)},
		{OwnerKey: 2, IsActive: false, KeyHash: make([]byte, 32)},
	}
	data, _ := store.ScaleEncode(v)
	var out []gateway.APIKeyInfo
	require.NoError(t, store.ScaleDecode(&out, data))
	require.Len(t, out, 2)
}

func TestCodec_SliceOfBatchCall(t *testing.T) {
	v := []batch.BatchCall{
		{Name: []byte("gov"), Method: []byte{1, 2, 3, 4}},
		{Name: []byte("auth"), Method: []byte{5, 6, 7, 8}},
	}
	data, _ := store.ScaleEncode(v)
	var out []batch.BatchCall
	require.NoError(t, store.ScaleDecode(&out, data))
	require.Len(t, out, 2)
}

func TestCodec_Int(t *testing.T) {
	for _, v := range []int{0, -1, 1, 65536} {
		data, _ := store.ScaleEncode(v)
		var out int
		require.NoError(t, store.ScaleDecode(&out, data))
		require.Equal(t, v, out)
	}
}

// testRoundTrip 编码→解码→再编码，对比两次编码结果。
func testRoundTrip[T any](t *testing.T, v *T) {
	t.Helper()
	data1, err := store.ScaleEncode(v)
	require.NoError(t, err)

	var out T
	err = store.ScaleDecode(&out, data1)
	require.NoError(t, err, "decode %T", out)

	data2, err := store.ScaleEncode(&out)
	require.NoError(t, err, "re-encode %T", out)

	if !reflect.DeepEqual(data1, data2) {
		t.Errorf("round-trip mismatch for %T:\n  original: %x\n  decoded:  %x", *v, data1, data2)
		t.Logf("len(data1)=%d len(data2)=%d", len(data1), len(data2))
		if len(data1) >= 14 && len(data2) >= 14 {
			t.Logf("data1[10:14]=%x data2[10:14]=%x", data1[10:14], data2[10:14])
			t.Logf("data1[38:42]=%x data2[38:42]=%x", data1[38:42], data2[38:42])
		}
	}
}
