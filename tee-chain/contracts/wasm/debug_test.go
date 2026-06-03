package wasm

import (
	"encoding/hex"
	"fmt"
	"math/big"
	"testing"

	"github.com/computeflux/tee-provider/tee-chain/contracts/module/gov"
	"github.com/computeflux/tee-provider/tee-chain/contracts/store"
)

func TestDebugMemberEncode(t *testing.T) {
	v := gov.Member{Account: store.UniAddr{T: 1, V: []byte("alice")}, Balance: store.Amount{Int: big.NewInt(100)}}
	data, _ := store.ScaleEncode(&v)
	fmt.Println("debug encoded:", hex.EncodeToString(data))
}
