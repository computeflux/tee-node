package protoio

import (
	"bytes"
	"testing"

	"github.com/computeflux/tee-provider/pkg/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
)

func newTestVTTx() *model.Tx {
	return &model.Tx{
		Caller:     []byte("caller-pub-key-0000000000000000"),
		CallerType: 1,
		Call:       []byte("test-call-data"),
		Signature:  []byte("signature-0000000000000000000000000000000000000000000000000000000000"),
	}
}

func newTestVTSysCall() *model.SysCall {
	return &model.SysCall{
		Payload: &model.SysCall_Contract{
			Contract: &model.ContractCall{
				Name:   []byte("test-contract"),
				Method: []byte("test-method"),
				Args:   [][]byte{[]byte("arg1"), []byte("arg2")},
				Nonce:  42,
			},
		},
	}
}

// TestVTProtoMarshalDeterministicConsistency checks that repeated MarshalVT
// produces the same bytes for project messages.
func TestVTProtoMarshalDeterministicConsistency(t *testing.T) {
	tests := []struct {
		name string
		msg  proto.Message
	}{
		{"Tx", newTestVTTx()},
		{"SysCall", newTestVTSysCall()},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var first []byte
			for i := 0; i < 10; i++ {
				data, err := MarshalDeterministic(tt.msg)
				require.NoError(t, err)
				if i == 0 {
					first = data
					continue
				}
				assert.Equal(t, first, data, "serialization %d differs from first", i)
			}
		})
	}
}

// TestVTProtoMarshalUsesFastPath checks that MarshalDeterministic returns the
// same bytes as the vtproto MarshalVT fast path for project messages.
func TestVTProtoMarshalUsesFastPath(t *testing.T) {
	tests := []struct {
		name string
		msg  interface{ MarshalVT() ([]byte, error) }
	}{
		{"Tx", newTestVTTx()},
		{"SysCall", newTestVTSysCall()},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fast, err := tt.msg.MarshalVT()
			require.NoError(t, err)

			viaDeterministic, err := MarshalDeterministic(tt.msg.(proto.Message))
			require.NoError(t, err)

			assert.Equal(t, fast, viaDeterministic)
		})
	}
}

// TestVTProtoRoundtrip checks that WriteMessage/ReadMessage round-trips project
// messages using vtproto marshal/unmarshal.
func TestVTProtoRoundtrip(t *testing.T) {
	tests := []struct {
		name     string
		original proto.Message
		decode   func() proto.Message
	}{
		{"Tx", newTestVTTx(), func() proto.Message { return &model.Tx{} }},
		{"SysCall", newTestVTSysCall(), func() proto.Message { return &model.SysCall{} }},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			err := WriteMessage(tt.original, &buf)
			require.NoError(t, err)

			decoded := tt.decode()
			err = ReadMessage(&buf, decoded)
			require.NoError(t, err)

			assert.True(t, proto.Equal(tt.original, decoded))
		})
	}
}
