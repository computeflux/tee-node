package store

import (
	"testing"
	"time"

	"github.com/cockroachdb/pebble"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCleanupExpiredByIndex(t *testing.T) {
	db, err := pebble.Open(t.TempDir(), &pebble.Options{})
	require.NoError(t, err)
	defer db.Close()
	DBINS = &DB{DB: db}

	expiredKey := "sub:expired"
	validKey := "sub:valid"

	require.NoError(t, SetKey("GATEWAY", expiredKey, []byte("old-data")))
	require.NoError(t, ScheduleExpiration("GATEWAY", expiredKey, time.Now().Add(-1*time.Hour).Unix()))

	require.NoError(t, SetKey("GATEWAY", validKey, []byte("fresh-data")))
	require.NoError(t, ScheduleExpiration("GATEWAY", validKey, time.Now().Add(1*time.Hour).Unix()))

	deleted := make(map[string]bool)
	require.NoError(t, CleanupExpiredByIndex("GATEWAY", func(subKey string) error {
		deleted[subKey] = true
		return DeleteKey("GATEWAY", subKey)
	}))

	assert.True(t, deleted[expiredKey], "应删除过期索引指向的 key")
	assert.False(t, deleted[validKey], "不应删除未过期的 key")

	_, err = GetKey("GATEWAY", expiredKey)
	assert.ErrorIs(t, err, pebble.ErrNotFound)

	v, err := GetKey("GATEWAY", validKey)
	require.NoError(t, err)
	assert.Equal(t, "fresh-data", string(v))
}

func TestCleanupExpiredByIndex_MissingData(t *testing.T) {
	db, err := pebble.Open(t.TempDir(), &pebble.Options{})
	require.NoError(t, err)
	defer db.Close()
	DBINS = &DB{DB: db}

	// 索引存在但数据已不存在：应只清理索引，不报错
	require.NoError(t, ScheduleExpiration("GATEWAY", "sub:missing", time.Now().Add(-1*time.Hour).Unix()))

	called := false
	require.NoError(t, CleanupExpiredByIndex("GATEWAY", func(subKey string) error {
		called = true
		return nil
	}))
	assert.True(t, called)
}
