package dkg

import (
	"testing"

	crypto "github.com/computeflux/tee-provider/pkg/crypto"
	dkgtypes "github.com/computeflux/tee-provider/pkg/dkg/types"
	"github.com/computeflux/tee-provider/pkg/store"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupTestDB(t *testing.T) {
	t.Helper()
	db, err := store.NewDBWithPath(t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
}

func TestRandomSharePool_AddShareAndAcquire(t *testing.T) {
	setupTestDB(t)

	pk, _ := crypto.PrivateKeyFromHex("7512939e37970b04c2b9a6060b16654473cf0721b71f8e56126ee314cbd0a7e9fe9125a81688d932ea792e9722c777f7696117363f86a107ab9d3681a8c922c8")
	pool := NewRandomSharePool(pk, 10)

	// Initially empty
	assert.True(t, pool.IsExhausted())
	assert.Equal(t, 0, pool.Len())

	// Add shares
	pool.AddShare(0, makeTestDistKeyShare(t, 0))
	pool.AddShare(1, makeTestDistKeyShare(t, 1))

	assert.Equal(t, 2, pool.Len())
	assert.False(t, pool.IsExhausted())

	// Acquire existing
	share, err := pool.Acquire(0)
	require.NoError(t, err)
	assert.NotNil(t, share)

	share1, err := pool.Acquire(1)
	require.NoError(t, err)
	assert.NotNil(t, share1)

	// Acquire non-existing
	_, err = pool.Acquire(99)
	assert.Error(t, err)
}

func TestRandomSharePool_PeekAt(t *testing.T) {
	setupTestDB(t)

	pk, _ := crypto.PrivateKeyFromHex("7512939e37970b04c2b9a6060b16654473cf0721b71f8e56126ee314cbd0a7e9fe9125a81688d932ea792e9722c777f7696117363f86a107ab9d3681a8c922c8")
	pool := NewRandomSharePool(pk, 10)

	pool.AddShare(5, makeTestDistKeyShare(t, 5))

	share, err := pool.PeekAt(5)
	require.NoError(t, err)
	assert.NotNil(t, share)

	_, err = pool.PeekAt(99)
	assert.Error(t, err)
}

func TestRandomSharePool_SetShares(t *testing.T) {
	setupTestDB(t)

	pk, _ := crypto.PrivateKeyFromHex("7512939e37970b04c2b9a6060b16654473cf0721b71f8e56126ee314cbd0a7e9fe9125a81688d932ea792e9722c777f7696117363f86a107ab9d3681a8c922c8")
	pool := NewRandomSharePool(pk, 10)

	pool.SetShares(map[uint32]*dkgtypes.DistKeyShare{
		0: makeTestDistKeyShare(t, 0),
		1: makeTestDistKeyShare(t, 1),
		2: makeTestDistKeyShare(t, 2),
	})

	assert.Equal(t, 3, pool.Len())
	assert.Equal(t, 3, pool.Remaining())
	assert.Equal(t, 10, pool.Capacity())
}

func TestRandomSharePool_SaveAndLoad(t *testing.T) {
	setupTestDB(t)

	pk, _ := crypto.PrivateKeyFromHex("7512939e37970b04c2b9a6060b16654473cf0721b71f8e56126ee314cbd0a7e9fe9125a81688d932ea792e9722c777f7696117363f86a107ab9d3681a8c922c8")
	pool1 := NewRandomSharePool(pk, 10)

	pool1.AddShare(0, makeTestDistKeyShare(t, 0))
	pool1.AddShare(1, makeTestDistKeyShare(t, 1))
	require.Equal(t, 2, pool1.Len())

	// Create a new pool with the same signer — should auto-load from DB
	pool2 := NewRandomSharePool(pk, 10)
	assert.Equal(t, 2, pool2.Len())

	// Verify shares are accessible after load
	share, err := pool2.Acquire(0)
	require.NoError(t, err)
	assert.NotNil(t, share)
}

func TestRandomSharePool_TxMapping(t *testing.T) {
	setupTestDB(t)

	pk, _ := crypto.PrivateKeyFromHex("7512939e37970b04c2b9a6060b16654473cf0721b71f8e56126ee314cbd0a7e9fe9125a81688d932ea792e9722c777f7696117363f86a107ab9d3681a8c922c8")
	pool := NewRandomSharePool(pk, 10)

	// TxMapping not set by default for simple AddShare
	_, ok := pool.TxMapping(0)
	assert.False(t, ok)
}

func TestRandomSharePool_CapacityDefault(t *testing.T) {
	setupTestDB(t)

	pk, _ := crypto.PrivateKeyFromHex("7512939e37970b04c2b9a6060b16654473cf0721b71f8e56126ee314cbd0a7e9fe9125a81688d932ea792e9722c777f7696117363f86a107ab9d3681a8c922c8")
	pool := NewRandomSharePool(pk, 0) // 0 should default to DefaultRandomSharePoolCapacity
	assert.Equal(t, DefaultRandomSharePoolCapacity, pool.Capacity())
}

func TestRandomSharePool_ConcurrentAccess(t *testing.T) {
	setupTestDB(t)

	pk, _ := crypto.PrivateKeyFromHex("7512939e37970b04c2b9a6060b16654473cf0721b71f8e56126ee314cbd0a7e9fe9125a81688d932ea792e9722c777f7696117363f86a107ab9d3681a8c922c8")
	pool := NewRandomSharePool(pk, 100)

	// Pre-populate
	for i := uint32(0); i < 50; i++ {
		pool.AddShare(i, makeTestDistKeyShare(t, i))
	}

	// Concurrent reads
	done := make(chan struct{}, 10)
	for i := 0; i < 10; i++ {
		go func(idx int) {
			for j := 0; j < 100; j++ {
				pool.Acquire(int64(j % 50))
			}
			done <- struct{}{}
		}(i)
	}

	for i := 0; i < 10; i++ {
		<-done
	}

	// All shares should still be there
	assert.Equal(t, 50, pool.Len())
}
