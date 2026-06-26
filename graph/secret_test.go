package graph

import (
	"context"
	"encoding/hex"
	"testing"

	"github.com/computeflux/tee-provider/pkg/types"
	"github.com/computeflux/tee-provider/tee-chain/contracts/store"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// knownTestSS58 is a fixed SS58 address used in tests (pkg/crypto/key_ss58_test.go).
// Its H160 address is 0x82ca18a28793a7149247d1f20dc4d26393e56986.
const knownTestSS58 = "5DyxQhChcTfL7JmyJ9NMznNL5fx8zHG3xDeCkxGXaiKJYyAb"

func mustHex(s string) []byte {
	b, err := hex.DecodeString(s)
	if err != nil {
		panic(err)
	}
	return b
}

func TestCheckSecretOwnership(t *testing.T) {
	targetAddr := store.UniAddr{T: 2, V: mustHex("82ca18a28793a7149247d1f20dc4d26393e56986")}
	otherAddr := store.UniAddr{T: 2, V: mustHex("1111111111111111111111111111111111111111")}

	t.Run("missing token", func(t *testing.T) {
		ctx := context.Background()
		_, err := checkSecretOwnership(ctx, knownTestSS58)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "authentication required")
	})

	t.Run("mismatched owner address", func(t *testing.T) {
		token := &types.GatewayTokenInfo{Addr: otherAddr}
		ctx := context.WithValue(context.Background(), gatewayTokenCtxKey, token)
		_, err := checkSecretOwnership(ctx, knownTestSS58)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "does not match authenticated token owner")
	})

	t.Run("matching owner address", func(t *testing.T) {
		token := &types.GatewayTokenInfo{Addr: targetAddr}
		ctx := context.WithValue(context.Background(), gatewayTokenCtxKey, token)
		pub, err := checkSecretOwnership(ctx, knownTestSS58)
		require.NoError(t, err)
		require.NotNil(t, pub)
	})
}
