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
