package model

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

// =============================================================================
// Test types with idx tags
// =============================================================================

type testProduct struct {
	Name     string `json:"name"`
	Category uint8  `idx:"exact"`                 // 精确索引
	Price    uint64 `idx:"range"`                 // 范围索引
	Stock    uint64 `idx:"range"`                 // 范围索引
	IsActive bool   `json:"isActive" idx:"exact"` // bool 精确索引
}

// =============================================================================
// IndexedStore 单元测试
// =============================================================================

func setupIndexerTest(t *testing.T) (*Txn, func()) {
	os.RemoveAll(dbPath)
	_, err := NewDB()
	require.NoError(t, err)
	txn := DBINS.NewTransaction()
	return txn, func() { DBINS.Close() }
}

// ── Put / Get ────────────────────────────────────────────

func TestIndexedStore_PutAndGet(t *testing.T) {
	txn, cleanup := setupIndexerTest(t)
	defer cleanup()

	store := NewIndexedStore[uint64, testProduct]("test", "prod_")

	// 写入
	err := store.Put(txn, 1, testProduct{Name: "A", Category: 1, Price: 25, Stock: 10, IsActive: true})
	require.NoError(t, err)

	err = store.Put(txn, 2, testProduct{Name: "B", Category: 2, Price: 200, Stock: 50, IsActive: true})
	require.NoError(t, err)

	// 读取
	val, err := store.Primary.Get(txn, 1)
	require.NoError(t, err)
	require.NotNil(t, val)
	require.Equal(t, "A", val.Name)
	require.Equal(t, uint8(1), val.Category)

	val, err = store.Primary.Get(txn, 2)
	require.NoError(t, err)
	require.Equal(t, "B", val.Name)
}

// ── Exact index query ────────────────────────────────────

func TestIndexedStore_QueryExact(t *testing.T) {
	txn, cleanup := setupIndexerTest(t)
	defer cleanup()

	store := NewIndexedStore[uint64, testProduct]("test", "prod_")

	require.NoError(t, store.Put(txn, 1, testProduct{Name: "A", Category: 1, Price: 25}))
	require.NoError(t, store.Put(txn, 2, testProduct{Name: "B", Category: 2, Price: 200}))
	require.NoError(t, store.Put(txn, 3, testProduct{Name: "C", Category: 1, Price: 80}))

	// 查 Category=1
	ids, err := store.Query(txn, map[string]any{"Category": uint64(1)})
	require.NoError(t, err)
	require.ElementsMatch(t, []uint64{1, 3}, ids)

	// 查 Category=2
	ids, err = store.Query(txn, map[string]any{"Category": uint64(2)})
	require.NoError(t, err)
	require.Equal(t, []uint64{2}, ids)

	// 查不存在的 Category
	ids, err = store.Query(txn, map[string]any{"Category": uint64(99)})
	require.NoError(t, err)
	require.Nil(t, ids)
}

// ── Bucket range query ───────────────────────────────────

func TestIndexedStore_QueryBucketRange(t *testing.T) {
	txn, cleanup := setupIndexerTest(t)
	defer cleanup()

	store := NewIndexedStore[uint64, testProduct]("test", "prod_")

	require.NoError(t, store.Put(txn, 1, testProduct{Name: "cheap", Price: 5}))
	require.NoError(t, store.Put(txn, 2, testProduct{Name: "mid", Price: 25}))
	require.NoError(t, store.Put(txn, 3, testProduct{Name: "high", Price: 300}))
	require.NoError(t, store.Put(txn, 4, testProduct{Name: "luxury", Price: 800}))

	// Price ≤ 50 → buckets 10, 50
	ids, err := store.Query(txn, map[string]any{"Price": Range{Min: 0, Max: 50}})
	require.NoError(t, err)
	require.ElementsMatch(t, []uint64{1, 2}, ids)

	// Price ≤ 500 → buckets 10, 50, 100, 500
	ids, err = store.Query(txn, map[string]any{"Price": Range{Min: 0, Max: 500}})
	require.NoError(t, err)
	require.ElementsMatch(t, []uint64{1, 2, 3}, ids)

	// Price ≥ 100 → buckets 100, 500, 1000
	ids, err = store.Query(txn, map[string]any{"Price": Range{Min: 100, Max: 0}})
	require.NoError(t, err)
	require.ElementsMatch(t, []uint64{3, 4}, ids)
}

// ── Combined filter (intersection) ───────────────────────

func TestIndexedStore_QueryCombined(t *testing.T) {
	txn, cleanup := setupIndexerTest(t)
	defer cleanup()

	store := NewIndexedStore[uint64, testProduct]("test", "prod_")

	require.NoError(t, store.Put(txn, 1, testProduct{Name: "A", Category: 1, Price: 25, Stock: 10}))
	require.NoError(t, store.Put(txn, 2, testProduct{Name: "B", Category: 2, Price: 25, Stock: 10}))
	require.NoError(t, store.Put(txn, 3, testProduct{Name: "C", Category: 1, Price: 200, Stock: 10}))
	require.NoError(t, store.Put(txn, 4, testProduct{Name: "D", Category: 1, Price: 25, Stock: 300}))

	// Category=1 AND Price≤50
	ids, err := store.Query(txn, map[string]any{
		"Category": uint64(1),
		"Price":    Range{Min: 0, Max: 50},
	})
	require.NoError(t, err)
	require.ElementsMatch(t, []uint64{1, 4}, ids)

	// Category=1 AND Price≤50 AND Stock≤20
	ids, err = store.Query(txn, map[string]any{
		"Category": uint64(1),
		"Price":    Range{Min: 0, Max: 50},
		"Stock":    Range{Min: 0, Max: 20},
	})
	require.NoError(t, err)
	require.ElementsMatch(t, []uint64{1}, ids)

	// 无交集
	ids, err = store.Query(txn, map[string]any{
		"Category": uint64(2),
		"Price":    Range{Min: 500, Max: 0},
	})
	require.NoError(t, err)
	require.Nil(t, ids)
}

// ── Update ───────────────────────────────────────────────

func TestIndexedStore_Update(t *testing.T) {
	txn, cleanup := setupIndexerTest(t)
	defer cleanup()

	store := NewIndexedStore[uint64, testProduct]("test", "prod_")

	require.NoError(t, store.Put(txn, 1, testProduct{Name: "X", Category: 1, Price: 25}))

	// 更新 Category: 1→2
	err := store.Update(txn, 1, func(cur *testProduct) {
		cur.Category = 2
		cur.Price = 300
	})
	require.NoError(t, err)

	// 旧 Category=1 中不应再有 id=1
	ids, err := store.Query(txn, map[string]any{"Category": uint64(1)})
	require.NoError(t, err)
	require.Nil(t, ids)

	// 新 Category=2 中应有 id=1
	ids, err = store.Query(txn, map[string]any{"Category": uint64(2)})
	require.NoError(t, err)
	require.Equal(t, []uint64{1}, ids)

	// Price 应在高桶中
	ids, err = store.Query(txn, map[string]any{"Price": Range{Min: 100, Max: 0}})
	require.NoError(t, err)
	require.Equal(t, []uint64{1}, ids)

	// 值已更新
	val, err := store.Primary.Get(txn, 1)
	require.NoError(t, err)
	require.Equal(t, uint8(2), val.Category)
	require.Equal(t, uint64(300), val.Price)
}

// ── Update non-existent ──────────────────────────────────

func TestIndexedStore_UpdateNotFound(t *testing.T) {
	txn, cleanup := setupIndexerTest(t)
	defer cleanup()

	store := NewIndexedStore[uint64, testProduct]("test", "prod_")

	err := store.Update(txn, 999, func(cur *testProduct) {
		cur.Category = 2
	})
	require.Error(t, err)
}

// ── Delete ───────────────────────────────────────────────

func TestIndexedStore_Delete(t *testing.T) {
	txn, cleanup := setupIndexerTest(t)
	defer cleanup()

	store := NewIndexedStore[uint64, testProduct]("test", "prod_")

	require.NoError(t, store.Put(txn, 1, testProduct{Name: "X", Category: 1, Price: 25}))
	require.NoError(t, store.Put(txn, 2, testProduct{Name: "Y", Category: 1, Price: 100}))

	// 删除 1
	require.NoError(t, store.Delete(txn, 1))

	// 主存储已删除
	val, err := store.Primary.Get(txn, 1)
	require.NoError(t, err)
	require.Nil(t, val)

	// 索引中不应再有 1
	ids, err := store.Query(txn, map[string]any{"Category": uint64(1)})
	require.NoError(t, err)
	require.Equal(t, []uint64{2}, ids)

	// 2 未受影响
	val, err = store.Primary.Get(txn, 2)
	require.NoError(t, err)
	require.NotNil(t, val)
}

// ── Empty filter ─────────────────────────────────────────

func TestIndexedStore_QueryEmptyFilter(t *testing.T) {
	txn, cleanup := setupIndexerTest(t)
	defer cleanup()

	store := NewIndexedStore[uint64, testProduct]("test", "prod_")

	require.NoError(t, store.Put(txn, 1, testProduct{Name: "A", Category: 1}))
	require.NoError(t, store.Put(txn, 2, testProduct{Name: "B", Category: 2}))

	ids, err := store.Query(txn, map[string]any{})
	require.NoError(t, err)
	require.Nil(t, ids)
}

// ── Bad filter ───────────────────────────────────────────

func TestIndexedStore_QueryBadField(t *testing.T) {
	txn, cleanup := setupIndexerTest(t)
	defer cleanup()

	store := NewIndexedStore[uint64, testProduct]("test", "prod_")

	_, err := store.Query(txn, map[string]any{"NoSuchField": uint64(1)})
	require.Error(t, err)
	require.Contains(t, err.Error(), "no exact/range index")
}

func TestIndexedStore_QueryBadFilterType(t *testing.T) {
	txn, cleanup := setupIndexerTest(t)
	defer cleanup()

	store := NewIndexedStore[uint64, testProduct]("test", "prod_")

	_, err := store.Query(txn, map[string]any{"Category": "not_a_number"})
	require.Error(t, err)
	require.Contains(t, err.Error(), "no string index")
}

// =============================================================================
// IndexedList 单元测试
// =============================================================================

// ── Insert / Len / Get ───────────────────────────────────

func TestIndexedList_InsertLenGet(t *testing.T) {
	txn, cleanup := setupIndexerTest(t)
	defer cleanup()

	list := NewIndexedList[uint64, testProduct]("test", "next_id", "item_")

	// Len 初始为 0
	n, err := list.Len(txn)
	require.NoError(t, err)
	require.Equal(t, uint64(0), n)

	// Insert 返回自增 id
	id1, err := list.Insert(txn, testProduct{Name: "A", Category: 1, Price: 25})
	require.NoError(t, err)
	require.Equal(t, uint64(0), id1)

	id2, err := list.Insert(txn, testProduct{Name: "B", Category: 2, Price: 200})
	require.NoError(t, err)
	require.Equal(t, uint64(1), id2)

	// Len
	n, err = list.Len(txn)
	require.NoError(t, err)
	require.Equal(t, uint64(2), n)

	// Get
	val, err := list.Get(txn, 0)
	require.NoError(t, err)
	require.NotNil(t, val)
	require.Equal(t, "A", val.Name)

	val, err = list.Get(txn, 1)
	require.NoError(t, err)
	require.Equal(t, "B", val.Name)

	// Get 不存在的
	val, err = list.Get(txn, 99)
	require.NoError(t, err)
	require.Nil(t, val)
}

// ── List pagination ──────────────────────────────────────

func TestIndexedList_ListPagination(t *testing.T) {
	txn, cleanup := setupIndexerTest(t)
	defer cleanup()

	list := NewIndexedList[uint64, testProduct]("test", "next_id", "item_")

	for i := 0; i < 10; i++ {
		_, err := list.Insert(txn, testProduct{Name: "Item", Category: 1, Price: uint64(i * 10)})
		require.NoError(t, err)
	}

	// List 前 3 条
	keys, vals, err := list.List(txn, 0, 3)
	require.NoError(t, err)
	require.Equal(t, []uint64{0, 1, 2}, keys)
	require.Len(t, vals, 3)

	// List offset=5 limit=3
	keys, vals, err = list.List(txn, 5, 3)
	require.NoError(t, err)
	require.Equal(t, []uint64{5, 6, 7}, keys)
	require.Len(t, vals, 3)

	// List 超范围
	keys, vals, err = list.List(txn, 8, 5)
	require.NoError(t, err)
	require.Equal(t, []uint64{8, 9}, keys)
	require.Len(t, vals, 2)

	// limit=0
	keys, vals, err = list.List(txn, 0, 0)
	require.NoError(t, err)
	require.Nil(t, keys)
}

// ── DescList pagination ──────────────────────────────────

func TestIndexedList_DescList(t *testing.T) {
	txn, cleanup := setupIndexerTest(t)
	defer cleanup()

	list := NewIndexedList[uint64, testProduct]("test", "next_id", "item_")

	for i := 0; i < 5; i++ {
		_, err := list.Insert(txn, testProduct{Name: "Item", Category: 1, Price: uint64(i)})
		require.NoError(t, err)
	}

	// DescList 从末尾取 2 条
	keys, _, err := list.DescList(txn, nil, 2)
	require.NoError(t, err)
	require.Equal(t, []uint64{4, 3}, keys)

	// DescList 从 startKey=2 向前取 2 条
	start := uint64(2)
	keys, _, err = list.DescList(txn, &start, 2)
	require.NoError(t, err)
	require.Equal(t, []uint64{2, 1}, keys)
}

// ── ListAll ──────────────────────────────────────────────

func TestIndexedList_ListAll(t *testing.T) {
	txn, cleanup := setupIndexerTest(t)
	defer cleanup()

	list := NewIndexedList[uint64, testProduct]("test", "next_id", "item_")

	for i := 0; i < 5; i++ {
		_, err := list.Insert(txn, testProduct{Name: "Item", Category: 1})
		require.NoError(t, err)
	}

	keys, vals, err := list.ListAll(txn)
	require.NoError(t, err)
	require.Len(t, keys, 5)
	require.Len(t, vals, 5)
	require.Equal(t, []uint64{0, 1, 2, 3, 4}, keys)
}

// ── UpdateFn ─────────────────────────────────────────────

func TestIndexedList_UpdateFn(t *testing.T) {
	txn, cleanup := setupIndexerTest(t)
	defer cleanup()

	list := NewIndexedList[uint64, testProduct]("test", "next_id", "item_")

	_, err := list.Insert(txn, testProduct{Name: "X", Category: 1, Price: 25})
	require.NoError(t, err)

	// 更新
	err = list.UpdateFn(txn, 0, func(cur *testProduct) {
		cur.Category = 2
		cur.Price = 300
	})
	require.NoError(t, err)

	// 值已更新
	val, err := list.Get(txn, 0)
	require.NoError(t, err)
	require.Equal(t, uint8(2), val.Category)
	require.Equal(t, uint64(300), val.Price)

	// 索引已更新
	ids, err := list.Query(txn, map[string]any{"Category": uint64(2)})
	require.NoError(t, err)
	require.Equal(t, []uint64{0}, ids)

	ids, err = list.Query(txn, map[string]any{"Category": uint64(1)})
	require.NoError(t, err)
	require.Nil(t, ids)
}

// ── UpdateFn non-existent ────────────────────────────────

func TestIndexedList_UpdateFnNotFound(t *testing.T) {
	txn, cleanup := setupIndexerTest(t)
	defer cleanup()

	list := NewIndexedList[uint64, testProduct]("test", "next_id", "item_")

	err := list.UpdateFn(txn, 999, func(cur *testProduct) {
		cur.Category = 2
	})
	require.Error(t, err)
}

// ── Clear ────────────────────────────────────────────────

func TestIndexedList_Clear(t *testing.T) {
	txn, cleanup := setupIndexerTest(t)
	defer cleanup()

	list := NewIndexedList[uint64, testProduct]("test", "next_id", "item_")

	_, err := list.Insert(txn, testProduct{Name: "X", Category: 1, Price: 25})
	require.NoError(t, err)

	// Clear：索引 + 数据都清除，但 Len 不变
	require.NoError(t, list.Clear(txn, 0))

	val, err := list.Get(txn, 0)
	require.NoError(t, err)
	require.Nil(t, val)

	ids, err := list.Query(txn, map[string]any{"Category": uint64(1)})
	require.NoError(t, err)
	require.Nil(t, ids)

	n, err := list.Len(txn)
	require.NoError(t, err)
	require.Equal(t, uint64(1), n) // Len 不受 Clear 影响
}

// ── Contains ─────────────────────────────────────────────

func TestIndexedList_Contains(t *testing.T) {
	txn, cleanup := setupIndexerTest(t)
	defer cleanup()

	list := NewIndexedList[uint64, testProduct]("test", "next_id", "item_")

	_, err := list.Insert(txn, testProduct{Name: "X", Category: 1})
	require.NoError(t, err)

	ok, err := list.Contains(txn, 0)
	require.NoError(t, err)
	require.True(t, ok)

	ok, err = list.Contains(txn, 99)
	require.NoError(t, err)
	require.False(t, ok)
}

// ── Query (indexed) ──────────────────────────────────────

func TestIndexedList_Query(t *testing.T) {
	txn, cleanup := setupIndexerTest(t)
	defer cleanup()

	list := NewIndexedList[uint64, testProduct]("test", "next_id", "item_")

	_, _ = list.Insert(txn, testProduct{Name: "A", Category: 1, Price: 25})
	_, _ = list.Insert(txn, testProduct{Name: "B", Category: 2, Price: 25})
	_, _ = list.Insert(txn, testProduct{Name: "C", Category: 1, Price: 300})

	ids, err := list.Query(txn, map[string]any{
		"Category": uint64(1),
		"Price":    Range{Min: 0, Max: 50},
	})
	require.NoError(t, err)
	require.ElementsMatch(t, []uint64{0}, ids)
}

// ── StoreList legacy methods ─────────────────────────────

func TestIndexedList_StoreListMethods(t *testing.T) {
	txn, cleanup := setupIndexerTest(t)
	defer cleanup()

	list := NewIndexedList[uint64, testProduct]("test", "next_id", "item_")

	_, _ = list.Insert(txn, testProduct{Name: "A", Category: 1})

	// Update (legacy)
	err := list.StoreList.Update(txn, 0, testProduct{Name: "Z", Category: 3, Price: 999})
	require.NoError(t, err)
	// 注意：legacy Update 不更新索引！Category=3 不在索引中

	val, err := list.Get(txn, 0)
	require.NoError(t, err)
	require.Equal(t, "Z", val.Name)

	// GetOrDefault
	def := testProduct{Name: "default"}
	v, err := list.GetOrDefault(txn, 99, def)
	require.NoError(t, err)
	require.Equal(t, "default", v.Name)
}

// =============================================================================
// 复杂索引测试
// =============================================================================

// ── Bool exact 索引 ──────────────────────────────────────

func TestIndexedList_QueryBool(t *testing.T) {
	txn, cleanup := setupIndexerTest(t)
	defer cleanup()

	list := NewIndexedList[uint64, testProduct]("test", "next_id", "item_")

	_, _ = list.Insert(txn, testProduct{Name: "A", Category: 1, IsActive: true})
	_, _ = list.Insert(txn, testProduct{Name: "B", Category: 2, IsActive: false})
	_, _ = list.Insert(txn, testProduct{Name: "C", Category: 1, IsActive: true})
	_, _ = list.Insert(txn, testProduct{Name: "D", Category: 2, IsActive: false})

	// IsActive=true (1)
	ids, err := list.Query(txn, map[string]any{"IsActive": uint64(1)})
	require.NoError(t, err)
	require.ElementsMatch(t, []uint64{0, 2}, ids)

	// IsActive=false (0)
	ids, err = list.Query(txn, map[string]any{"IsActive": uint64(0)})
	require.NoError(t, err)
	require.ElementsMatch(t, []uint64{1, 3}, ids)

	// bool + exact 组合: Category=1 AND IsActive=true
	ids, err = list.Query(txn, map[string]any{
		"Category": uint64(1),
		"IsActive": uint64(1),
	})
	require.NoError(t, err)
	require.ElementsMatch(t, []uint64{0, 2}, ids)

	// bool + bucket 组合: IsActive=true AND Price≤50
	err = list.UpdateFn(txn, 0, func(cur *testProduct) { cur.Price = 25 })
	require.NoError(t, err)
	err = list.UpdateFn(txn, 2, func(cur *testProduct) { cur.Price = 200 })
	require.NoError(t, err)
	ids, err = list.Query(txn, map[string]any{
		"IsActive": uint64(1),
		"Price":    Range{Min: 0, Max: 50},
	})
	require.NoError(t, err)
	require.ElementsMatch(t, []uint64{0}, ids)
}

// ── 大规模数据索引一致性 ─────────────────────────────────

func TestIndexedList_LargeScaleConsistency(t *testing.T) {
	txn, cleanup := setupIndexerTest(t)
	defer cleanup()

	list := NewIndexedList[uint64, testProduct]("test", "next_id", "item_")

	const n = 500
	for i := 0; i < n; i++ {
		_, err := list.Insert(txn, testProduct{
			Name:     "Item",
			Category: uint8(i % 10),
			Price:    uint64(i * 10),
			Stock:    uint64(i % 100),
			IsActive: i%3 == 0,
		})
		require.NoError(t, err)
	}

	// 验证每个 Category 的计数
	for cat := 0; cat < 10; cat++ {
		ids, err := list.Query(txn, map[string]any{"Category": uint64(cat)})
		require.NoError(t, err)
		require.Len(t, ids, n/10, "Category %d count mismatch", cat)
	}

	// 验证 IsActive=true 的计数 (i%3==0 → 约 1/3)
	ids, err := list.Query(txn, map[string]any{"IsActive": uint64(1)})
	require.NoError(t, err)
	require.Len(t, ids, 167) // 500/3 ≈ 166.67

	// 验证组合查询: Category=5 AND IsActive=true
	ids, err = list.Query(txn, map[string]any{
		"Category": uint64(5),
		"IsActive": uint64(1),
	})
	require.NoError(t, err)
	// i%10==5 的有 50 个，其中 i%3==0 的有 i=15,45,75... 共约 17 个
	require.Len(t, ids, 17)
}

// ── RebuildIndex 历史数据补偿 ────────────────────────────

func TestIndexedList_RebuildIndex(t *testing.T) {
	txn, cleanup := setupIndexerTest(t)
	defer cleanup()

	list := NewIndexedList[uint64, testProduct]("test", "next_id", "item_")

	// 先插入数据（此时 IsActive 有 idx tag，会正常建索引）
	_, _ = list.Insert(txn, testProduct{Name: "A", Category: 1, IsActive: true})
	_, _ = list.Insert(txn, testProduct{Name: "B", Category: 2, IsActive: false})

	// 确认索引正常
	ids, err := list.Query(txn, map[string]any{"IsActive": uint64(1)})
	require.NoError(t, err)
	require.ElementsMatch(t, []uint64{0}, ids)

	// 模拟"新增 idx tag"：删除索引后重建
	// 先通过 Clear + Insert 模拟无索引状态
	// 实际场景: 给已有字段新增 idx tag 后调用 RebuildIndex
	list2 := NewIndexedList[uint64, testProduct]("test", "next_id2", "item2_")
	_, _ = list2.Insert(txn, testProduct{Name: "X", Category: 3, IsActive: true})
	_, _ = list2.Insert(txn, testProduct{Name: "Y", Category: 3, IsActive: true})
	_, _ = list2.Insert(txn, testProduct{Name: "Z", Category: 3, IsActive: false})

	// RebuildIndex 后应能正常查询
	err = list2.RebuildIndex(txn)
	require.NoError(t, err)

	ids, err = list2.Query(txn, map[string]any{"IsActive": uint64(1)})
	require.NoError(t, err)
	require.ElementsMatch(t, []uint64{0, 1}, ids)

	ids, err = list2.Query(txn, map[string]any{"Category": uint64(3)})
	require.NoError(t, err)
	require.ElementsMatch(t, []uint64{0, 1, 2}, ids)
}

// ── 范围边界值查询 ─────────────────────────────────────────
//
// range 索引使用固定宽度字典序 key，零假阳性：
//   - 精确匹配范围边界
//   - 无漏报、无假阳性

func TestIndexedStore_QueryRangeEdgeCases(t *testing.T) {
	txn, cleanup := setupIndexerTest(t)
	defer cleanup()

	store := NewIndexedStore[uint64, testProduct]("test", "prod_")

	require.NoError(t, store.Put(txn, 1, testProduct{Price: 10}))
	require.NoError(t, store.Put(txn, 2, testProduct{Price: 11}))
	require.NoError(t, store.Put(txn, 3, testProduct{Price: 50}))
	require.NoError(t, store.Put(txn, 4, testProduct{Price: 51}))
	require.NoError(t, store.Put(txn, 5, testProduct{Price: 0}))
	require.NoError(t, store.Put(txn, 6, testProduct{Price: 1000}))
	require.NoError(t, store.Put(txn, 7, testProduct{Price: 1001}))

	// Price ≤ 50 → 精确包含 0,10,11,50
	ids, err := store.Query(txn, map[string]any{"Price": Range{Min: 0, Max: 50}})
	require.NoError(t, err)
	require.ElementsMatch(t, []uint64{1, 2, 3, 5}, ids)

	// 精确点查：Price=50
	ids, err = store.Query(txn, map[string]any{"Price": Range{Min: 50, Max: 50}})
	require.NoError(t, err)
	require.ElementsMatch(t, []uint64{3}, ids)

	// 下界查询：Price > 500 (Min=501, Max=0)
	ids, err = store.Query(txn, map[string]any{"Price": Range{Min: 501, Max: 0}})
	require.NoError(t, err)
	require.ElementsMatch(t, []uint64{6, 7}, ids)

	// 超大数据查询：Price > 1000 → 包含 Price=1001
	ids, err = store.Query(txn, map[string]any{"Price": Range{Min: 1001, Max: 0}})
	require.NoError(t, err)
	require.ElementsMatch(t, []uint64{7}, ids)
}

// ── 多字段交集正确性 ─────────────────────────────────────

func TestIndexedStore_QueryMultiFieldIntersection(t *testing.T) {
	txn, cleanup := setupIndexerTest(t)
	defer cleanup()

	store := NewIndexedStore[uint64, testProduct]("test", "prod_")

	// 构造数据矩阵: Category(1,2) × IsActive(T,F) × Price(低,高)
	data := []struct {
		id     uint64
		cat    uint8
		active bool
		price  uint64
	}{
		{1, 1, true, 25},
		{2, 1, true, 200},
		{3, 1, false, 25},
		{4, 1, false, 200},
		{5, 2, true, 25},
		{6, 2, true, 200},
		{7, 2, false, 25},
		{8, 2, false, 200},
	}
	for _, d := range data {
		require.NoError(t, store.Put(txn, d.id, testProduct{
			Category: d.cat,
			IsActive: d.active,
			Price:    d.price,
		}))
	}

	// Category=1 AND IsActive=true → {1,2}
	ids, err := store.Query(txn, map[string]any{
		"Category": uint64(1),
		"IsActive": uint64(1),
	})
	require.NoError(t, err)
	require.ElementsMatch(t, []uint64{1, 2}, ids)

	// Category=1 AND IsActive=true AND Price≤50 → {1}
	ids, err = store.Query(txn, map[string]any{
		"Category": uint64(1),
		"IsActive": uint64(1),
		"Price":    Range{Min: 0, Max: 50},
	})
	require.NoError(t, err)
	require.ElementsMatch(t, []uint64{1}, ids)

	// Category=2 AND IsActive=false AND Price>50 → {8}
	ids, err = store.Query(txn, map[string]any{
		"Category": uint64(2),
		"IsActive": uint64(0),
		"Price":    Range{Min: 51, Max: 0},
	})
	require.NoError(t, err)
	require.ElementsMatch(t, []uint64{8}, ids)

	// range 索引零假阳性：Price>500 无匹配，整个交集应为空
	ids, err = store.Query(txn, map[string]any{
		"Category": uint64(1),
		"IsActive": uint64(1),
		"Price":    Range{Min: 501, Max: 0},
	})
	require.NoError(t, err)
	require.Nil(t, ids)
}

// ── Update 跨桶迁移 ──────────────────────────────────────

func TestIndexedStore_UpdateBucketMigration(t *testing.T) {
	txn, cleanup := setupIndexerTest(t)
	defer cleanup()

	store := NewIndexedStore[uint64, testProduct]("test", "prod_")

	// Price buckets: 10,50,100,500,1000
	require.NoError(t, store.Put(txn, 1, testProduct{Price: 25}))  // bucket 50
	require.NoError(t, store.Put(txn, 2, testProduct{Price: 300})) // bucket 500

	// 更新 id=1 从 bucket 50 → bucket 500
	err := store.Update(txn, 1, func(cur *testProduct) {
		cur.Price = 300
	})
	require.NoError(t, err)

	// 原 bucket 50 中不应再有 id=1
	ids, err := store.Query(txn, map[string]any{"Price": Range{Min: 0, Max: 50}})
	require.NoError(t, err)
	require.Nil(t, ids)

	// 新 bucket 500 中应有 id=1 和 id=2
	ids, err = store.Query(txn, map[string]any{"Price": Range{Min: 51, Max: 500}})
	require.NoError(t, err)
	require.ElementsMatch(t, []uint64{1, 2}, ids)

	// 再次更新 id=1 从 bucket 500 → bucket 1000
	err = store.Update(txn, 1, func(cur *testProduct) {
		cur.Price = 800
	})
	require.NoError(t, err)

	ids, err = store.Query(txn, map[string]any{"Price": Range{Min: 501, Max: 1000}})
	require.NoError(t, err)
	require.ElementsMatch(t, []uint64{1}, ids)

	ids, err = store.Query(txn, map[string]any{"Price": Range{Min: 101, Max: 500}})
	require.NoError(t, err)
	require.ElementsMatch(t, []uint64{2}, ids)
}

// ── 空结果与全匹配 ───────────────────────────────────────

func TestIndexedStore_QueryEmptyAndFullMatch(t *testing.T) {
	txn, cleanup := setupIndexerTest(t)
	defer cleanup()

	store := NewIndexedStore[uint64, testProduct]("test", "prod_")

	// 空 filter → nil
	ids, err := store.Query(txn, map[string]any{})
	require.NoError(t, err)
	require.Nil(t, ids)

	// 写入数据后全匹配
	for i := 1; i <= 5; i++ {
		require.NoError(t, store.Put(txn, uint64(i), testProduct{Category: 1, IsActive: true}))
	}

	// 所有数据的共同特征
	ids, err = store.Query(txn, map[string]any{"Category": uint64(1)})
	require.NoError(t, err)
	require.ElementsMatch(t, []uint64{1, 2, 3, 4, 5}, ids)

	ids, err = store.Query(txn, map[string]any{"IsActive": uint64(1)})
	require.NoError(t, err)
	require.ElementsMatch(t, []uint64{1, 2, 3, 4, 5}, ids)

	// 不存在的值
	ids, err = store.Query(txn, map[string]any{"Category": uint64(99)})
	require.NoError(t, err)
	require.Nil(t, ids)
}

// ── 前缀隔离防碰撞 ───────────────────────────────────────

func TestStoreIndex_PrefixCollision(t *testing.T) {
	txn, cleanup := setupIndexerTest(t)
	defer cleanup()

	// 两个不同 namespace 的索引，使用相同 prefix
	idxA := NewStoreIndex[uint64]("nsA", "idx_cat_")
	idxB := NewStoreIndex[uint64]("nsB", "idx_cat_")

	require.NoError(t, idxA.Add(txn, "1", 100))
	require.NoError(t, idxB.Add(txn, "1", 200))

	// 各自独立查询
	idsA, err := idxA.GetIDs(txn, "1")
	require.NoError(t, err)
	require.Equal(t, []uint64{100}, idsA)

	idsB, err := idxB.GetIDs(txn, "1")
	require.NoError(t, err)
	require.Equal(t, []uint64{200}, idsB)

	// GetAllIDs 也不互相影响
	allA, err := idxA.GetAllIDs(txn)
	require.NoError(t, err)
	require.Equal(t, []uint64{100}, allA)

	allB, err := idxB.GetAllIDs(txn)
	require.NoError(t, err)
	require.Equal(t, []uint64{200}, allB)
}

// ── IndexedList.Update 直接更新 ──────────────────────────

func TestIndexedList_UpdateDirect(t *testing.T) {
	txn, cleanup := setupIndexerTest(t)
	defer cleanup()

	list := NewIndexedList[uint64, testProduct]("test", "next_id", "item_")

	_, _ = list.Insert(txn, testProduct{Name: "A", Category: 1, IsActive: true})
	_, _ = list.Insert(txn, testProduct{Name: "B", Category: 2, IsActive: false})

	// 直接 Update（非 UpdateFn）
	err := list.Update(txn, 0, testProduct{Name: "A2", Category: 3, IsActive: false})
	require.NoError(t, err)

	// 旧索引已清理
	ids, err := list.Query(txn, map[string]any{"Category": uint64(1)})
	require.NoError(t, err)
	require.Nil(t, ids)

	// IsActive=true 的查询应无结果（A 更新为 false，B 初始为 false）
	ids, err = list.Query(txn, map[string]any{"IsActive": uint64(1)})
	require.NoError(t, err)
	require.Nil(t, ids)

	// 新索引已建立
	ids, err = list.Query(txn, map[string]any{"Category": uint64(3)})
	require.NoError(t, err)
	require.ElementsMatch(t, []uint64{0}, ids)
}

// =============================================================================
// Benchmarks
// =============================================================================

func BenchmarkIndexedStore_Put(b *testing.B) {
	os.RemoveAll(dbPath)
	NewDB()
	defer DBINS.Close()

	txn := DBINS.NewTransaction()
	store := NewIndexedStore[uint64, testProduct]("bench", "prod_")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = store.Put(txn, uint64(i), testProduct{
			Name:     "Item",
			Category: uint8(i % 5),
			Price:    uint64(i * 10),
			Stock:    uint64(i % 100),
		})
	}
}

func BenchmarkIndexedStore_Query(b *testing.B) {
	os.RemoveAll(dbPath)
	NewDB()
	defer DBINS.Close()

	txn := DBINS.NewTransaction()
	store := NewIndexedStore[uint64, testProduct]("bench", "prod_")
	for i := 0; i < 1000; i++ {
		_ = store.Put(txn, uint64(i), testProduct{
			Category: uint8(i % 5),
			Price:    uint64(i * 10),
		})
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = store.Query(txn, map[string]any{
			"Category": uint64(2),
			"Price":    Range{Min: 0, Max: 100},
		})
	}
}

func BenchmarkIndexedList_Insert(b *testing.B) {
	os.RemoveAll(dbPath)
	NewDB()
	defer DBINS.Close()

	txn := DBINS.NewTransaction()
	list := NewIndexedList[uint64, testProduct]("bench", "next_id", "item_")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = list.Insert(txn, testProduct{
			Name:     "Item",
			Category: uint8(i % 5),
			Price:    uint64(i * 10),
		})
	}
}

func BenchmarkIndexedList_List(b *testing.B) {
	os.RemoveAll(dbPath)
	NewDB()
	defer DBINS.Close()

	txn := DBINS.NewTransaction()
	list := NewIndexedList[uint64, testProduct]("bench", "next_id", "item_")
	for i := 0; i < 10000; i++ {
		_, _ = list.Insert(txn, testProduct{Name: "Item", Category: 1})
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _, _ = list.List(txn, 0, 20)
	}
}
