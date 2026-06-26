package gateway

import (
	"testing"
	"time"

	"github.com/computeflux/tee-provider/pkg/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupGatewayCacheTest(t *testing.T) func() {
	db, err := types.NewDBWithPath(t.TempDir())
	require.NoError(t, err)

	return func() {
		types.SECRET_DB = nil
		db.Close()
	}
}

func TestFindTokenByKey_ExpiredTokenRejected(t *testing.T) {
	cleanup := setupGatewayCacheTest(t)
	defer cleanup()

	key := "expired-api-key-001"
	tk := &types.GatewayTokenInfo{
		Owner:     "owner1",
		Key:       key,
		IsActive:  true,
		ExpiresAt: uint64(time.Now().Add(-1 * time.Hour).Unix()),
	}
	require.NoError(t, types.SetGatewayTokenByKey(tk))

	// Expired token should be rejected even if IsActive is true.
	got := FindTokenByKey(key)
	assert.Nil(t, got)
}

func TestFindTokenByKey_ActiveNonExpiredTokenAccepted(t *testing.T) {
	cleanup := setupGatewayCacheTest(t)
	defer cleanup()

	key := "active-api-key-002"
	tk := &types.GatewayTokenInfo{
		Owner:     "owner2",
		Key:       key,
		IsActive:  true,
		ExpiresAt: uint64(time.Now().Add(1 * time.Hour).Unix()),
	}
	require.NoError(t, types.SetGatewayTokenByKey(tk))

	got := FindTokenByKey(key)
	require.NotNil(t, got)
	assert.Equal(t, key, got.Key)
	assert.True(t, got.IsActive)
}

func TestFindTokenByKey_ZeroExpiryAccepted(t *testing.T) {
	cleanup := setupGatewayCacheTest(t)
	defer cleanup()

	key := "noexpiry-api-key-003"
	tk := &types.GatewayTokenInfo{
		Owner:     "owner3",
		Key:       key,
		IsActive:  true,
		ExpiresAt: 0,
	}
	require.NoError(t, types.SetGatewayTokenByKey(tk))

	got := FindTokenByKey(key)
	require.NotNil(t, got)
	assert.Equal(t, key, got.Key)
}

func TestFindTokenByKey_InactiveTokenRejected(t *testing.T) {
	cleanup := setupGatewayCacheTest(t)
	defer cleanup()

	key := "inactive-api-key-004"
	tk := &types.GatewayTokenInfo{
		Owner:     "owner4",
		Key:       key,
		IsActive:  false,
		ExpiresAt: uint64(time.Now().Add(1 * time.Hour).Unix()),
	}
	require.NoError(t, types.SetGatewayTokenByKey(tk))

	got := FindTokenByKey(key)
	assert.Nil(t, got)
}
