package model

import (
	"bytes"
	"testing"

	"github.com/computeflux/tee-provider/pkg/model/protoio"
	"github.com/stretchr/testify/require"
)

func TestProtoioMarshalDeterministic(t *testing.T) {
	msg := &TeeCall{
		ChainId: 1,
		Caller:  []byte("caller"),
		Time:    1234567890,
		Tx:      &TeeCall_Text{Text: []byte("deterministic test")},
	}

	var first []byte
	for i := 0; i < 1000; i++ {
		data, err := protoio.Marshal(msg)
		if err != nil {
			t.Fatalf("marshal %d: %v", i, err)
		}
		if i == 0 {
			first = data
			continue
		}
		if !bytes.Equal(first, data) {
			t.Fatalf("marshal %d differs from first", i)
		}
	}
}

func TestProtoioRoundTrip(t *testing.T) {
	msg := &TeeCall{
		ChainId: 7,
		Caller:  []byte("round"),
		Time:    42,
		Tx:      &TeeCall_Text{Text: []byte("trip")},
	}

	data, err := protoio.Marshal(msg)
	require.NoError(t, err)
	require.NotEmpty(t, data)

	restored := &TeeCall{}
	require.NoError(t, protoio.Unmarshal(data, restored))
	require.Equal(t, msg.ChainId, restored.ChainId)
	require.Equal(t, msg.Caller, restored.Caller)
	require.Equal(t, msg.Time, restored.Time)
	require.Equal(t, "trip", string(restored.GetText()))
}

func TestProtoioWriteMessageRoundTrip(t *testing.T) {
	msg := &TeeCall{ChainId: 9, Tx: &TeeCall_Text{Text: []byte("write")}}

	var buf bytes.Buffer
	require.NoError(t, protoio.WriteMessage(msg, &buf))

	restored := &TeeCall{}
	require.NoError(t, protoio.Unmarshal(buf.Bytes(), restored))
	require.Equal(t, uint32(9), restored.ChainId)
}

func assertVTMarshaler[T protoio.VTMarshaler](_ T) {}

// TestProtoioMarshalVT_OneofWrapper 回归：vtproto 不对 oneof 包装消息生成
// MarshalVT()（只有 SizeVT + MarshalToVT），VTMarshaler 接口若要求它，
// SgxIssue/SgxVerify 等对 call.Tx（如 *TeeCall_Text）的 protoio.MarshalVT
// 会报 "unsupported tee tx type"。
func TestProtoioMarshalVT_OneofWrapper(t *testing.T) {
	// 编译期：TeeCall 全部 oneof 包装类型须满足 VTMarshaler。
	assertVTMarshaler((*TeeCall_PodStart)(nil))
	assertVTMarshaler((*TeeCall_PodMint)(nil))
	assertVTMarshaler((*TeeCall_BridgeCall)(nil))
	assertVTMarshaler((*TeeCall_Text)(nil))
	assertVTMarshaler((*TeeCall_UploadSecret)(nil))
	assertVTMarshaler((*TeeCall_InitDisk)(nil))

	w := &TeeCall_Text{Text: []byte("abc")}
	data, err := protoio.MarshalVT(w)
	require.NoError(t, err)
	// field 8、length-delimited：[0x42][varint 长度][text]
	require.Equal(t, []byte{0x42, 0x03, 'a', 'b', 'c'}, data)

	// 与外层 TeeCall 的 oneof 段字节一致（quote 签名两端编码必须逐字节相同）。
	full, err := protoio.MarshalVT(&TeeCall{Tx: w})
	require.NoError(t, err)
	require.Equal(t, full, data)
}

func TestProtoioErrors(t *testing.T) {
	_, err := protoio.Marshal(nil)
	require.Error(t, err)

	data, err := protoio.MarshalVT(nil)
	require.NoError(t, err)
	require.Nil(t, data)

	_, err = protoio.MarshalVT("not a message")
	require.Error(t, err)

	err = protoio.Unmarshal([]byte{1}, "not a vt message")
	require.Error(t, err)
}
