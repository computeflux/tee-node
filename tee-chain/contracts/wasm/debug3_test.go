package wasm

import (
	"encoding/hex"
	"fmt"
	"math/big"
	"reflect"
	"testing"

	"github.com/computeflux/tee-provider/tee-chain/contracts/module/gov"
	"github.com/computeflux/tee-provider/tee-chain/contracts/store"
	"github.com/stretchr/testify/require"
)

func testRoundTripDebug2[T any](t *testing.T, v *T) {
	t.Helper()
	data1, err := store.ScaleEncode(v)
	require.NoError(t, err)
	fmt.Println("data1:", hex.EncodeToString(data1))
	var out T
	err = store.ScaleDecode(&out, data1)
	require.NoError(t, err, "decode %T", out)
	data2, err := store.ScaleEncode(&out)
	require.NoError(t, err, "re-encode %T", out)
	fmt.Println("data2:", hex.EncodeToString(data2))
	if !reflect.DeepEqual(data1, data2) {
		t.Errorf("round-trip mismatch for %T:\n  original: %x\n  decoded:  %x", *v, data1, data2)
	}
}

func TestDebugMemberRoundTrip2(t *testing.T) {
	v := gov.Member{Account: store.UniAddr{T: 1, V: []byte("alice")}, Balance: store.Amount{Int: big.NewInt(100)}}
	testRoundTripDebug2(t, &v)
}
