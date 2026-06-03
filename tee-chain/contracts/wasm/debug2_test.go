package wasm

import (
	"encoding/hex"
	"fmt"
	"math/big"
	"testing"

	"github.com/computeflux/tee-provider/tee-chain/contracts/module/gov"
	"github.com/computeflux/tee-provider/tee-chain/contracts/store"
)

func testRoundTripDebug[T any](t *testing.T, v *T) {
	data1, _ := store.ScaleEncode(v)
	fmt.Println("data1:", hex.EncodeToString(data1))

	var out T
	store.ScaleDecode(&out, data1)

	data2, _ := store.ScaleEncode(&out)
	fmt.Println("data2:", hex.EncodeToString(data2))
}

func TestDebugMemberRoundTrip(t *testing.T) {
	v := gov.Member{Account: store.UniAddr{T: 1, V: []byte("alice")}, Balance: store.Amount{Int: big.NewInt(100)}}
	testRoundTripDebug(t, &v)
}
