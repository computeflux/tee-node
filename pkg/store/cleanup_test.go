package store

import (
	"fmt"
	"testing"
	"time"

	"github.com/cockroachdb/pebble"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFindGatewayTokenByKey_LazyDeletesExpired(t *testing.T) {
	db, err := NewDBWithPath(t.TempDir())
	require.NoError(t, err)
	defer db.Close()
	SECRET_DB = db

	key := "auth_tee_test_lazy_cleanup"
	token := &GatewayTokenInfo{
		ID:        1,
		Owner:     "user@example.com",
		Key:       key,
		IsActive:  true,
		CreatedAt: uint64(time.Now().Add(-48 * time.Hour).Unix()),
		ExpiresAt: uint64(time.Now().Add(-24 * time.Hour).Unix()),
	}
	require.NoError(t, SetGatewayTokenByKey(token))

	// Find 应返回 nil 并自动删除过期记录
	found := FindGatewayTokenByKey(key)
	assert.Nil(t, found)

	_, err = GetGatewayTokenByKey(key)
	assert.True(t, err == nil || err.Error() == "db not initialized" || err == pebble.ErrNotFound)
}

func TestCleanupExpiredGatewayTokens(t *testing.T) {
	db, err := NewDBWithPath(t.TempDir())
	require.NoError(t, err)
	defer db.Close()
	SECRET_DB = db

	expiredKey := "auth_tee_expired"
	validKey := "auth_tee_valid"
	expiredToken := &GatewayTokenInfo{
		ID: 1, Owner: "a", Key: expiredKey, IsActive: true,
		CreatedAt: uint64(time.Now().Add(-48 * time.Hour).Unix()),
		ExpiresAt: uint64(time.Now().Add(-1 * time.Hour).Unix()),
	}
	require.NoError(t, SetGatewayTokenByKey(expiredToken))
	require.NoError(t, ScheduleExpiration("GATEWAY", "token_by_key:"+expiredKey, int64(expiredToken.ExpiresAt)))

	validToken := &GatewayTokenInfo{
		ID: 2, Owner: "b", Key: validKey, IsActive: true,
		CreatedAt: uint64(time.Now().Unix()),
		ExpiresAt: uint64(time.Now().Add(24 * time.Hour).Unix()),
	}
	require.NoError(t, SetGatewayTokenByKey(validToken))
	require.NoError(t, ScheduleExpiration("GATEWAY", "token_by_key:"+validKey, int64(validToken.ExpiresAt)))

	require.NoError(t, CleanupExpiredGatewayTokens())

	assert.Nil(t, FindGatewayTokenByKey(expiredKey))
	assert.NotNil(t, FindGatewayTokenByKey(validKey))
}

func TestFindRefreshToken_LazyDeletesExpired(t *testing.T) {
	db, err := NewDBWithPath(t.TempDir())
	require.NoError(t, err)
	defer db.Close()
	SECRET_DB = db

	token := "refresh_tee_test_lazy_cleanup"
	require.NoError(t, SetRefreshToken(token, &RefreshTokenInfo{
		Owner:     "user@example.com",
		Address:   "0x1234",
		IsActive:  true,
		CreatedAt: uint64(time.Now().Add(-31 * 24 * time.Hour).Unix()),
		ExpiresAt: uint64(time.Now().Add(-24 * time.Hour).Unix()),
	}))

	assert.Nil(t, FindRefreshToken(token))

	_, err = GetRefreshToken(token)
	assert.True(t, err == nil || err == pebble.ErrNotFound)
}

func TestCleanupExpiredRefreshTokens(t *testing.T) {
	db, err := NewDBWithPath(t.TempDir())
	require.NoError(t, err)
	defer db.Close()
	SECRET_DB = db

	expired := "refresh_tee_expired"
	valid := "refresh_tee_valid"
	expiredInfo := &RefreshTokenInfo{
		Owner: "a", Address: "0x1", IsActive: true,
		CreatedAt: uint64(time.Now().Add(-31 * 24 * time.Hour).Unix()),
		ExpiresAt: uint64(time.Now().Add(-1 * time.Hour).Unix()),
	}
	require.NoError(t, SetRefreshToken(expired, expiredInfo))
	require.NoError(t, ScheduleExpiration("GATEWAY", "refresh_token:"+expired, int64(expiredInfo.ExpiresAt)))

	validInfo := &RefreshTokenInfo{
		Owner: "b", Address: "0x2", IsActive: true,
		CreatedAt: uint64(time.Now().Unix()),
		ExpiresAt: uint64(time.Now().Add(30 * 24 * time.Hour).Unix()),
	}
	require.NoError(t, SetRefreshToken(valid, validInfo))
	require.NoError(t, ScheduleExpiration("GATEWAY", "refresh_token:"+valid, int64(validInfo.ExpiresAt)))

	require.NoError(t, CleanupExpiredRefreshTokens())

	assert.Nil(t, FindRefreshToken(expired))
	assert.NotNil(t, FindRefreshToken(valid))
}

// TestCleanupExpiredByIndex_KeepsIndexOnDeleteFailure verifies that if deleteFn fails,
// the expiration index is preserved so the next cleanup can retry.
func TestCleanupExpiredByIndex_KeepsIndexOnDeleteFailure(t *testing.T) {
	db, err := NewDBWithPath(t.TempDir())
	require.NoError(t, err)
	defer db.Close()
	SECRET_DB = db

	key := "sub:protected"
	require.NoError(t, SECRET_DB.SetKey("GATEWAY", key, []byte("data")))
	require.NoError(t, ScheduleExpiration("GATEWAY", key, time.Now().Add(-1*time.Hour).Unix()))

	// 第一次清理：deleteFn 失败，索引应保留
	require.NoError(t, CleanupExpiredByIndex("GATEWAY", func(subKey string) error {
		return fmt.Errorf("simulated delete failure")
	}))

	// 数据应仍存在
	v, err := SECRET_DB.GetKey("GATEWAY", key)
	require.NoError(t, err)
	assert.Equal(t, "data", string(v))

	// 索引也应仍存在
	require.NoError(t, CleanupExpiredByIndex("GATEWAY", func(subKey string) error {
		return SECRET_DB.DeleteKey("GATEWAY", subKey)
	}))

	_, err = SECRET_DB.GetKey("GATEWAY", key)
	assert.ErrorIs(t, err, pebble.ErrNotFound)
}
