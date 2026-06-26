package store

import (
	"testing"
	"time"

	"github.com/cockroachdb/pebble"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCacheCleanupExpiredByIndex(t *testing.T) {
	db, err := NewDBWithPath(t.TempDir())
	require.NoError(t, err)
	defer db.Close()
	SECRET_DB = db

	expiredKey := "sub:expired"
	validKey := "sub:valid"

	require.NoError(t, SECRET_DB.CacheKey("GATEWAY", expiredKey, []byte("old-data"), time.Now().Add(-1*time.Hour).Unix()))
	require.NoError(t, SECRET_DB.CacheKey("GATEWAY", validKey, []byte("fresh-data"), time.Now().Add(1*time.Hour).Unix()))

	deleted := make(map[string]bool)
	require.NoError(t, cacheCleanupExpiredByIndex("GATEWAY", func(subKey string) error {
		deleted[subKey] = true
		return SECRET_DB.DeleteKey("GATEWAY", subKey)
	}))

	assert.True(t, deleted[expiredKey], "应删除过期索引指向的 key")
	assert.False(t, deleted[validKey], "不应删除未过期的 key")

	_, err = SECRET_DB.GetKey("GATEWAY", expiredKey)
	assert.ErrorIs(t, err, pebble.ErrNotFound)

	v, err := SECRET_DB.GetKey("GATEWAY", validKey)
	require.NoError(t, err)
	assert.Equal(t, "fresh-data", string(v))
}

func TestCacheCleanupExpiredByIndex_MissingData(t *testing.T) {
	db, err := NewDBWithPath(t.TempDir())
	require.NoError(t, err)
	defer db.Close()
	SECRET_DB = db

	// 索引存在但数据已不存在：应只清理索引，不报错
	require.NoError(t, SECRET_DB.CacheKey("GATEWAY", "sub:missing", []byte("gone"), time.Now().Add(-1*time.Hour).Unix()))

	called := false
	require.NoError(t, cacheCleanupExpiredByIndex("GATEWAY", func(subKey string) error {
		called = true
		return nil
	}))
	assert.True(t, called)
}

func TestCacheByteKey_Simple(t *testing.T) {
	db, err := NewDBWithPath(t.TempDir())
	require.NoError(t, err)
	defer db.Close()
	SECRET_DB = db

	key := ComboNamespaceKey("GATEWAY", "cache:k1")
	require.NoError(t, SECRET_DB.CacheByteKey(key, []byte("hello"), time.Now().Add(1*time.Hour).Unix()))

	// 验证数据可读
	v, err := SECRET_DB.GetKey("GATEWAY", "cache:k1")
	require.NoError(t, err)
	assert.Equal(t, "hello", string(v))
}

func TestCacheByteKey_NoExpire(t *testing.T) {
	db, err := NewDBWithPath(t.TempDir())
	require.NoError(t, err)
	defer db.Close()
	SECRET_DB = db

	key := ComboNamespaceKey("GATEWAY", "cache:k2")
	// expiresAt=0 不注册过期索引，但数据仍写入
	require.NoError(t, SECRET_DB.CacheByteKey(key, []byte("persist"), 0))

	v, err := SECRET_DB.GetKey("GATEWAY", "cache:k2")
	require.NoError(t, err)
	assert.Equal(t, "persist", string(v))
}

func TestDeleteCacheByteKey_Simple(t *testing.T) {
	db, err := NewDBWithPath(t.TempDir())
	require.NoError(t, err)
	defer db.Close()
	SECRET_DB = db

	key := ComboNamespaceKey("GATEWAY", "cache:k3")
	require.NoError(t, SECRET_DB.CacheByteKey(key, []byte("tmp"), time.Now().Add(1*time.Hour).Unix()))

	require.NoError(t, SECRET_DB.DeleteCacheByteKey(key))

	_, err = SECRET_DB.GetKey("GATEWAY", "cache:k3")
	assert.ErrorIs(t, err, pebble.ErrNotFound)
}

func TestDeleteCacheByteKey_ExpiredIndexCleaned(t *testing.T) {
	db, err := NewDBWithPath(t.TempDir())
	require.NoError(t, err)
	defer db.Close()
	SECRET_DB = db

	key := ComboNamespaceKey("GATEWAY", "cache:k4")
	require.NoError(t, SECRET_DB.CacheByteKey(key, []byte("data"), time.Now().Add(-1*time.Hour).Unix()))

	// 显式删除数据（索引变为孤项）
	require.NoError(t, SECRET_DB.DeleteCacheByteKey(key))

	// Cleanup：数据已不存在，但 deleteFn 返回 ErrNotFound → flushExpiredBatch 应清理索引
	require.NoError(t, cacheCleanupExpiredByIndex("GATEWAY", func(subKey string) error {
		return SECRET_DB.DeleteKey("GATEWAY", subKey)
	}))

	// 确保数据确实已删
	_, err = SECRET_DB.GetKey("GATEWAY", "cache:k4")
	assert.ErrorIs(t, err, pebble.ErrNotFound)
}

func TestSplitComboKey(t *testing.T) {
	ns, sub, err := splitComboKey([]byte("GATEWAY_cache:k1"))
	require.NoError(t, err)
	assert.Equal(t, "GATEWAY", ns)
	assert.Equal(t, "cache:k1", sub)

	ns, sub, err = splitComboKey([]byte("NS_a:b:c"))
	require.NoError(t, err)
	assert.Equal(t, "NS", ns)
	assert.Equal(t, "a:b:c", sub)

	_, _, err = splitComboKey([]byte("nounderscore"))
	assert.Error(t, err)
}

func TestCacheByteKey_InvalidKey(t *testing.T) {
	err := SECRET_DB.CacheByteKey([]byte("nounderscore"), []byte("v"), 0)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not in comboKey format")
}
