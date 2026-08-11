package types

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTypesTestDB(t *testing.T) *DB {
	t.Helper()
	db, err := NewDBWithPath(t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	return db
}

func TestAllocNextID_Sequential(t *testing.T) {
	newTypesTestDB(t)

	id, err := AllocNextID("gateway_token")
	require.NoError(t, err)
	require.Equal(t, uint64(0), id)
	id, err = AllocNextID("gateway_token")
	require.NoError(t, err)
	require.Equal(t, uint64(1), id)
	id, err = AllocNextID("gateway_token")
	require.NoError(t, err)
	require.Equal(t, uint64(2), id)

	// 不同 key 互不影响
	id, err = AllocNextID("other_counter")
	require.NoError(t, err)
	require.Equal(t, uint64(0), id)
	id, err = AllocNextID("other_counter")
	require.NoError(t, err)
	require.Equal(t, uint64(1), id)

	// 持久化：再次读取仍能继续递增
	id, err = AllocNextID("gateway_token")
	require.NoError(t, err)
	require.Equal(t, uint64(3), id)
}

func TestAllocNextID_ConcurrentUnique(t *testing.T) {
	newTypesTestDB(t)

	const workers = 32
	const perWorker = 16

	var wg sync.WaitGroup
	ids := make(chan uint64, workers*perWorker)
	errs := make(chan error, workers*perWorker)
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < perWorker; i++ {
				id, err := AllocNextID("gateway_token")
				if err != nil {
					errs <- err
					continue
				}
				ids <- id
			}
		}()
	}
	wg.Wait()
	close(ids)
	close(errs)

	for err := range errs {
		t.Errorf("AllocNextID failed: %v", err)
	}

	seen := make(map[uint64]struct{}, workers*perWorker)
	for id := range ids {
		_, dup := seen[id]
		assert.False(t, dup, "duplicate ID allocated: %d", id)
		seen[id] = struct{}{}
	}
	assert.Len(t, seen, workers*perWorker, "all allocated IDs must be unique")
}
