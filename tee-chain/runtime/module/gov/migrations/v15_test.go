package migrations_test

import (
	"testing"

	"github.com/computeflux/tee-provider/tee-chain/runtime/module/gateway"
	govmod "github.com/computeflux/tee-provider/tee-chain/runtime/module/gov"
	"github.com/computeflux/tee-provider/tee-chain/runtime/store"
	"github.com/computeflux/tee-provider/tee-chain/runtime/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestV15_MigrateAPIKeys(t *testing.T) {
	rt := testutil.NewMockRuntime()
	rt.System = true
	base := gateway.NewGateway(rt)
	txn := rt.GetTxn()

	// Write old data
	d1 := gateway.APIKeyInfo{
		OwnerKey:  1,
		Owner:     store.UniAddr{T: 1, V: []byte{0xaa}},
		KeyHash:   []byte("hash-aaaaaaaaaaaaaaaaaaaaaaaa"),
		RateLimit: 60,
		IsActive:  true,
		CreatedAt: 1000,
	}
	require.NoError(t, base.APIKeys.Put(txn, "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", d1))

	d2 := gateway.APIKeyInfo{
		OwnerKey:  2,
		Owner:     store.UniAddr{T: 2, V: []byte{0xbb}},
		KeyHash:   []byte("hash-bbbbbbbbbbbbbbbbbbbbbb"),
		RateLimit: 120,
		IsActive:  false,
		CreatedAt: 2000,
	}
	require.NoError(t, base.APIKeys.Put(txn, "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb", d2))

	// Run migration
	gm := govmod.GovMutation{Gov: *govmod.NewGov(rt)}
	require.NoError(t, gm.Migration(15))

	// Verify
	v2, err := base.APIKeysV2.Primary.Get(txn, "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa")
	require.NoError(t, err)
	require.NotNil(t, v2)
	assert.Equal(t, d1.OwnerKey, v2.OwnerKey)
	assert.Equal(t, "", v2.Prefix)
	assert.Equal(t, d1.Owner, v2.Owner)
	assert.Equal(t, d1.RateLimit, v2.RateLimit)
	assert.Equal(t, d1.IsActive, v2.IsActive)
	assert.Equal(t, d1.CreatedAt, v2.CreatedAt)

	// Empty store should not error
	rt2 := testutil.NewMockRuntime()
	rt2.System = true
	gm2 := govmod.GovMutation{Gov: *govmod.NewGov(rt2)}
	require.NoError(t, gm2.Migration(15))
}
