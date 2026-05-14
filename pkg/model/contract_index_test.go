package model

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

// =============================================================================
// StoreIndex 单元测试
// =============================================================================

func setupIndexTest(t *testing.T) (*Txn, func()) {
	os.RemoveAll(dbPath)
	_, err := NewDB()
	require.NoError(t, err)
	txn := DBINS.NewTransaction()
	return txn, func() { DBINS.Close() }
}

// ── Add / GetIDs ─────────────────────────────────────────

func TestStoreIndex_AddAndGetIDs(t *testing.T) {
	txn, cleanup := setupIndexTest(t)
	defer cleanup()

	idx := NewStoreIndex[uint64, uint64]("test", "idx_")

	// 空索引查询
	ids, err := idx.GetIDs(txn, 10)
	require.NoError(t, err)
	require.Nil(t, ids)

	// 添加
	require.NoError(t, idx.Add(txn, 10, 1))
	require.NoError(t, idx.Add(txn, 10, 2))
	require.NoError(t, idx.Add(txn, 10, 3))

	ids, err = idx.GetIDs(txn, 10)
	require.NoError(t, err)
	require.Equal(t, []uint64{1, 2, 3}, ids)

	// 另一个 key
	require.NoError(t, idx.Add(txn, 20, 4))
	ids, err = idx.GetIDs(txn, 20)
	require.NoError(t, err)
	require.Equal(t, []uint64{4}, ids)

	// key 10 不受影响
	ids, err = idx.GetIDs(txn, 10)
	require.NoError(t, err)
	require.Equal(t, []uint64{1, 2, 3}, ids)
}

// ── Remove ───────────────────────────────────────────────

func TestStoreIndex_Remove(t *testing.T) {
	txn, cleanup := setupIndexTest(t)
	defer cleanup()

	idx := NewStoreIndex[uint64, uint64]("test", "idx_")

	require.NoError(t, idx.Add(txn, 10, 1))
	require.NoError(t, idx.Add(txn, 10, 2))
	require.NoError(t, idx.Add(txn, 10, 3))

	// 移除中间元素
	require.NoError(t, idx.Remove(txn, 10, 2))
	ids, err := idx.GetIDs(txn, 10)
	require.NoError(t, err)
	require.Equal(t, []uint64{1, 3}, ids)

	// 移除首元素
	require.NoError(t, idx.Remove(txn, 10, 1))
	ids, err = idx.GetIDs(txn, 10)
	require.NoError(t, err)
	require.Equal(t, []uint64{3}, ids)

	// 移除最后一个 — key 应被删除
	require.NoError(t, idx.Remove(txn, 10, 3))
	ids, err = idx.GetIDs(txn, 10)
	require.NoError(t, err)
	require.Nil(t, ids)

	// 移除不存在的元素 — 不报错
	require.NoError(t, idx.Remove(txn, 10, 99))
}

// ── GetIDsByKeys (union) ─────────────────────────────────

func TestStoreIndex_GetIDsByKeys(t *testing.T) {
	txn, cleanup := setupIndexTest(t)
	defer cleanup()

	idx := NewStoreIndex[uint64, uint64]("test", "idx_")

	require.NoError(t, idx.Add(txn, 1, 10))
	require.NoError(t, idx.Add(txn, 1, 20))
	require.NoError(t, idx.Add(txn, 2, 20))
	require.NoError(t, idx.Add(txn, 2, 30))
	require.NoError(t, idx.Add(txn, 3, 40))

	// 单 key
	ids, err := idx.GetIDsByKeys(txn, []uint64{1})
	require.NoError(t, err)
	require.ElementsMatch(t, []uint64{10, 20}, ids)

	// 多 key 并集且去重
	ids, err = idx.GetIDsByKeys(txn, []uint64{1, 2})
	require.NoError(t, err)
	require.ElementsMatch(t, []uint64{10, 20, 30}, ids)

	// 三 key 并集
	ids, err = idx.GetIDsByKeys(txn, []uint64{1, 2, 3})
	require.NoError(t, err)
	require.ElementsMatch(t, []uint64{10, 20, 30, 40}, ids)

	// 空 keys
	ids, err = idx.GetIDsByKeys(txn, nil)
	require.NoError(t, err)
	require.Nil(t, ids)
}

// ── GetAllIDs ────────────────────────────────────────────

func TestStoreIndex_GetAllIDs(t *testing.T) {
	txn, cleanup := setupIndexTest(t)
	defer cleanup()

	idx := NewStoreIndex[uint64, uint64]("test", "idx_")

	// 空索引
	ids, err := idx.GetAllIDs(txn)
	require.NoError(t, err)
	require.Nil(t, ids)

	require.NoError(t, idx.Add(txn, 1, 100))
	require.NoError(t, idx.Add(txn, 1, 200))
	require.NoError(t, idx.Add(txn, 2, 200))
	require.NoError(t, idx.Add(txn, 3, 300))

	ids, err = idx.GetAllIDs(txn)
	require.NoError(t, err)
	require.ElementsMatch(t, []uint64{100, 200, 300}, ids)
}

// ── String key type ──────────────────────────────────────

func TestStoreIndex_StringKey(t *testing.T) {
	txn, cleanup := setupIndexTest(t)
	defer cleanup()

	idx := NewStoreIndex[string, uint64]("test", "idx_str_")

	require.NoError(t, idx.Add(txn, "active", 1))
	require.NoError(t, idx.Add(txn, "active", 2))
	require.NoError(t, idx.Add(txn, "inactive", 3))

	ids, err := idx.GetIDs(txn, "active")
	require.NoError(t, err)
	require.Equal(t, []uint64{1, 2}, ids)

	ids, err = idx.GetIDs(txn, "inactive")
	require.NoError(t, err)
	require.Equal(t, []uint64{3}, ids)
}

// ── Namespace isolation ──────────────────────────────────

func TestStoreIndex_NamespaceIsolation(t *testing.T) {
	txn, cleanup := setupIndexTest(t)
	defer cleanup()

	idx1 := NewStoreIndex[uint64, uint64]("ns1", "idx_")
	idx2 := NewStoreIndex[uint64, uint64]("ns2", "idx_")

	require.NoError(t, idx1.Add(txn, 10, 1))
	require.NoError(t, idx2.Add(txn, 10, 2))

	// 不同 namespace 不互相影响
	ids, err := idx1.GetIDs(txn, 10)
	require.NoError(t, err)
	require.Equal(t, []uint64{1}, ids)

	ids, err = idx2.GetIDs(txn, 10)
	require.NoError(t, err)
	require.Equal(t, []uint64{2}, ids)
}

// ── Prefix isolation ─────────────────────────────────────

func TestStoreIndex_PrefixIsolation(t *testing.T) {
	txn, cleanup := setupIndexTest(t)
	defer cleanup()

	idx1 := NewStoreIndex[uint64, uint64]("test", "idx_a_")
	idx2 := NewStoreIndex[uint64, uint64]("test", "idx_b_")

	require.NoError(t, idx1.Add(txn, 10, 1))
	require.NoError(t, idx2.Add(txn, 10, 2))

	ids, err := idx1.GetIDs(txn, 10)
	require.NoError(t, err)
	require.Equal(t, []uint64{1}, ids)

	ids, err = idx2.GetIDs(txn, 10)
	require.NoError(t, err)
	require.Equal(t, []uint64{2}, ids)
}

// ── Benchmarks ───────────────────────────────────────────

func BenchmarkStoreIndex_Add(b *testing.B) {
	os.RemoveAll(dbPath)
	NewDB()
	defer DBINS.Close()

	txn := DBINS.NewTransaction()
	idx := NewStoreIndex[uint64, uint64]("bench", "idx_")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = idx.Add(txn, uint64(i%10), uint64(i))
	}
}

func BenchmarkStoreIndex_GetIDs(b *testing.B) {
	os.RemoveAll(dbPath)
	NewDB()
	defer DBINS.Close()

	txn := DBINS.NewTransaction()
	idx := NewStoreIndex[uint64, uint64]("bench", "idx_")
	for i := 0; i < 1000; i++ {
		_ = idx.Add(txn, uint64(i%10), uint64(i))
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = idx.GetIDs(txn, 5)
	}
}
