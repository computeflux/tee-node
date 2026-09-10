package teetypes

import (
	"testing"

	"github.com/computeflux/tee-provider/pkg/model"
	"github.com/computeflux/tee-provider/pkg/model/protoio"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
)

func TestMarshalVT_TeeCallTxRoundtrip(t *testing.T) {
	tests := []struct {
		name string
		call *model.TeeCall
	}{
		{
			name: "text",
			call: &model.TeeCall{
				Tx: &model.TeeCall_Text{Text: []byte("hello world")},
			},
		},
		{
			name: "pod_start",
			call: &model.TeeCall{
				Tx: &model.TeeCall_PodStart{
					PodStart: &model.PodStart{
						Id:        42,
						AppId:     []byte("app-id"),
						NameSpace: []byte("namespace"),
						PubKey:    []byte("pubkey"),
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.NotNil(t, tt.call.Tx)

			data, err := protoio.MarshalVT(tt.call)
			require.NoError(t, err)

			// The wrapper bytes are a single field of TeeCall, so they can be
			// unmarshaled back into a TeeCall with only Tx set.
			decoded := &model.TeeCall{}
			err = proto.Unmarshal(data, decoded)
			require.NoError(t, err)

			assert.True(t, proto.Equal(tt.call, decoded))
		})
	}
}

func TestMarshalVT_TeeCallTxDeterministic(t *testing.T) {
	call := &model.TeeCall{
		Tx: &model.TeeCall_Text{Text: []byte("deterministic test")},
	}

	var first []byte
	for i := 0; i < 10; i++ {
		data, err := protoio.MarshalVT(call)
		require.NoError(t, err)
		if i == 0 {
			first = data
			continue
		}
		assert.Equal(t, first, data, "marshal %d differs from first", i)
	}
}
