package model

import (
	"bytes"
	"testing"

	"github.com/computeflux/tee-provider/pkg/model/protoio"
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
