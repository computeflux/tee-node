package model

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

// =============================================================================
// StoreList2D 单元测试
// =============================================================================

// TestStoreList2D_InsertAndGet 测试基本 Insert/Get 操作
func TestStoreList2D_InsertAndGet(t *testing.T) {
	os.RemoveAll(dbPath)
	NewDB()
	defer DBINS.Close()

	txn := DBINS.NewTransaction()
	list := NewStoreList2D[string, uint32, string](
		"test_ns",
		"k1_to_id_",
		"k1_len_",
		"k2_next_",
		"store_",
	)

	// 1. 初始长度为 0
	len, err := list.Len(txn, "group1")
	require.NoError(t, err)
	require.Equal(t, uint32(0), len)

	// 2. 在 group1 下插入记录
	id, err := list.Insert(txn, "group1", "first")
	require.NoError(t, err)
	require.Equal(t, uint32(0), id)

	id, err = list.Insert(txn, "group1", "second")
	require.NoError(t, err)
	require.Equal(t, uint32(1), id)

	// 3. 长度更新
	len, err = list.Len(txn, "group1")
	require.NoError(t, err)
	require.Equal(t, uint32(2), len)

	// 4. Get 读取值
	val, err := list.Get(txn, "group1", 0)
	require.NoError(t, err)
	require.NotNil(t, val)
	require.Equal(t, "first", *val)

	val, err = list.Get(txn, "group1", 1)
	require.NoError(t, err)
	require.NotNil(t, val)
	require.Equal(t, "second", *val)

	// 5. 不存在的组返回 nil
	val, err = list.Get(txn, "nonexistent", 0)
	require.NoError(t, err)
	require.Nil(t, val)
}

// TestStoreList2D_MultipleGroups 测试多个组
func TestStoreList2D_MultipleGroups(t *testing.T) {
	os.RemoveAll(dbPath)
	NewDB()
	defer DBINS.Close()

	txn := DBINS.NewTransaction()
	list := NewStoreList2D[string, uint32, string](
		"test_ns",
		"k1_to_id_",
		"k1_len_",
		"k2_next_",
		"store_",
	)

	// 在不同组下插入记录
	_, err := list.Insert(txn, "group1", "g1_first")
	require.NoError(t, err)
	_, err = list.Insert(txn, "group1", "g1_second")
	require.NoError(t, err)

	_, err = list.Insert(txn, "group2", "g2_first")
	require.NoError(t, err)

	// 验证 group1 长度
	len1, err := list.Len(txn, "group1")
	require.NoError(t, err)
	require.Equal(t, uint32(2), len1)

	// 验证 group2 长度
	len2, err := list.Len(txn, "group2")
	require.NoError(t, err)
	require.Equal(t, uint32(1), len2)

	// 验证各组值独立
	val, err := list.Get(txn, "group1", 0)
	require.NoError(t, err)
	require.Equal(t, "g1_first", *val)

	val, err = list.Get(txn, "group2", 0)
	require.NoError(t, err)
	require.Equal(t, "g2_first", *val)
}

// TestStoreList2D_GetOrDefault 测试 GetOrDefault
func TestStoreList2D_GetOrDefault(t *testing.T) {
	os.RemoveAll(dbPath)
	NewDB()
	defer DBINS.Close()

	txn := DBINS.NewTransaction()
	list := NewStoreList2D[string, uint32, int64](
		"test_ns",
		"k1_to_id_",
		"k1_len_",
		"k2_next_",
		"store_",
	)

	// 不存在时返回默认值
	val, err := list.GetOrDefault(txn, "group1", 0, 999)
	require.NoError(t, err)
	require.Equal(t, int64(999), val)

	// 存在时返回存储的值
	_, err = list.Insert(txn, "group1", 100)
	require.NoError(t, err)

	val, err = list.GetOrDefault(txn, "group1", 0, 999)
	require.NoError(t, err)
	require.Equal(t, int64(100), val)
}

// TestStoreList2D_Update 测试 Update
func TestStoreList2D_Update(t *testing.T) {
	os.RemoveAll(dbPath)
	NewDB()
	defer DBINS.Close()

	txn := DBINS.NewTransaction()
	list := NewStoreList2D[string, uint32, string](
		"test_ns",
		"k1_to_id_",
		"k1_len_",
		"k2_next_",
		"store_",
	)

	_, err := list.Insert(txn, "group1", "original")
	require.NoError(t, err)

	// 更新值
	err = list.Update(txn, "group1", 0, "updated")
	require.NoError(t, err)

	val, err := list.Get(txn, "group1", 0)
	require.NoError(t, err)
	require.Equal(t, "updated", *val)
}

// TestStoreList2D_Clear 测试 Clear
func TestStoreList2D_Clear(t *testing.T) {
	os.RemoveAll(dbPath)
	NewDB()
	defer DBINS.Close()

	txn := DBINS.NewTransaction()
	list := NewStoreList2D[string, uint32, string](
		"test_ns",
		"k1_to_id_",
		"k1_len_",
		"k2_next_",
		"store_",
	)

	_, err := list.Insert(txn, "group1", "value")
	require.NoError(t, err)

	// 清除
	err = list.Clear(txn, "group1", 0)
	require.NoError(t, err)

	// 清除后不存在
	val, err := list.Get(txn, "group1", 0)
	require.NoError(t, err)
	require.Nil(t, val)

	// 长度不变
	len, err := list.Len(txn, "group1")
	require.NoError(t, err)
	require.Equal(t, uint32(1), len)
}

// TestStoreList2D_List 测试分页列表
func TestStoreList2D_List(t *testing.T) {
	os.RemoveAll(dbPath)
	NewDB()
	defer DBINS.Close()

	txn := DBINS.NewTransaction()
	list := NewStoreList2D[string, uint32, string](
		"test_ns",
		"k1_to_id_",
		"k1_len_",
		"k2_next_",
		"store_",
	)

	// 在 group1 下插入 5 条记录
	for i := 0; i < 5; i++ {
		_, err := list.Insert(txn, "group1", string(rune('a'+i)))
		require.NoError(t, err)
	}

	// 从 0 开始取 3 条
	keys, values, err := list.List(txn, "group1", 0, 3)
	require.NoError(t, err)
	require.Len(t, keys, 3)
	require.Len(t, values, 3)
	require.Equal(t, uint32(0), keys[0])
	require.Equal(t, uint32(1), keys[1])
	require.Equal(t, uint32(2), keys[2])

	// 从 2 开始取 3 条
	keys, values, err = list.List(txn, "group1", 2, 3)
	require.NoError(t, err)
	require.Len(t, keys, 3)
	require.Equal(t, uint32(2), keys[0])
	require.Equal(t, uint32(3), keys[1])
	require.Equal(t, uint32(4), keys[2])

	// 不存在的组
	keys, _, err = list.List(txn, "nonexistent", 0, 3)
	require.NoError(t, err)
	require.Empty(t, keys)
}

// TestStoreList2D_DescList 测试降序列表
func TestStoreList2D_DescList(t *testing.T) {
	os.RemoveAll(dbPath)
	NewDB()
	defer DBINS.Close()

	txn := DBINS.NewTransaction()
	list := NewStoreList2D[string, uint32, string](
		"test_ns",
		"k1_to_id_",
		"k1_len_",
		"k2_next_",
		"store_",
	)

	// 在 group1 下插入 5 条记录
	for i := 0; i < 5; i++ {
		_, err := list.Insert(txn, "group1", string(rune('a'+i)))
		require.NoError(t, err)
	}

	// 从末尾开始取 3 条
	keys, _, err := list.DescList(txn, "group1", nil, 3)
	require.NoError(t, err)
	require.Len(t, keys, 3)
	require.Equal(t, uint32(4), keys[0])
	require.Equal(t, uint32(3), keys[1])
	require.Equal(t, uint32(2), keys[2])

	// 从指定位置开始
	startKey := uint32(2)
	keys, _, err = list.DescList(txn, "group1", &startKey, 3)
	require.NoError(t, err)
	require.Len(t, keys, 3)
	require.Equal(t, uint32(2), keys[0])
	require.Equal(t, uint32(1), keys[1])
	require.Equal(t, uint32(0), keys[2])
}

// TestStoreList2D_ListAll 测试获取所有记录
func TestStoreList2D_ListAll(t *testing.T) {
	os.RemoveAll(dbPath)
	NewDB()
	defer DBINS.Close()

	txn := DBINS.NewTransaction()
	list := NewStoreList2D[string, uint32, string](
		"test_ns",
		"k1_to_id_",
		"k1_len_",
		"k2_next_",
		"store_",
	)

	// 空组
	keys, values, err := list.ListAll(txn, "group1")
	require.NoError(t, err)
	require.Empty(t, keys)

	// 插入 5 条记录
	for i := 0; i < 5; i++ {
		_, err := list.Insert(txn, "group1", string(rune('a'+i)))
		require.NoError(t, err)
	}

	keys, values, err = list.ListAll(txn, "group1")
	require.NoError(t, err)
	require.Len(t, keys, 5)
	require.Len(t, values, 5)
}

// TestStoreList2D_IndexTypes 测试不同索引类型
func TestStoreList2D_IndexTypes(t *testing.T) {
	t.Run("uint8_index", func(t *testing.T) {
		os.RemoveAll(dbPath)
		NewDB()
		defer DBINS.Close()

		txn := DBINS.NewTransaction()
		list := NewStoreList2D[string, uint8, string](
			"test_ns",
			"k1_",
			"len_",
			"next_",
			"store_",
		)

		id, err := list.Insert(txn, "group1", "value")
		require.NoError(t, err)
		require.Equal(t, uint8(0), id)
	})

	t.Run("uint16_index", func(t *testing.T) {
		os.RemoveAll(dbPath)
		NewDB()
		defer DBINS.Close()

		txn := DBINS.NewTransaction()
		list := NewStoreList2D[string, uint16, string](
			"test_ns",
			"k1_",
			"len_",
			"next_",
			"store_",
		)

		id, err := list.Insert(txn, "group1", "value")
		require.NoError(t, err)
		require.Equal(t, uint16(0), id)
	})

	t.Run("uint64_index", func(t *testing.T) {
		os.RemoveAll(dbPath)
		NewDB()
		defer DBINS.Close()

		txn := DBINS.NewTransaction()
		list := NewStoreList2D[string, uint64, string](
			"test_ns",
			"k1_",
			"len_",
			"next_",
			"store_",
		)

		id, err := list.Insert(txn, "group1", "value")
		require.NoError(t, err)
		require.Equal(t, uint64(0), id)
	})
}

// TestStoreList2D_KeyTypes 测试不同外层 key 类型
func TestStoreList2D_KeyTypes(t *testing.T) {
	t.Run("string_key", func(t *testing.T) {
		os.RemoveAll(dbPath)
		NewDB()
		defer DBINS.Close()

		txn := DBINS.NewTransaction()
		list := NewStoreList2D[string, uint32, string](
			"test_ns", "k1_", "len_", "next_", "store_",
		)

		_, err := list.Insert(txn, "key1", "value")
		require.NoError(t, err)
	})

	t.Run("uint64_key", func(t *testing.T) {
		os.RemoveAll(dbPath)
		NewDB()
		defer DBINS.Close()

		txn := DBINS.NewTransaction()
		list := NewStoreList2D[uint64, uint32, string](
			"test_ns", "k1_", "len_", "next_", "store_",
		)

		_, err := list.Insert(txn, 123, "value")
		require.NoError(t, err)

		val, err := list.Get(txn, 123, 0)
		require.NoError(t, err)
		require.Equal(t, "value", *val)
	})

	t.Run("uint32_key", func(t *testing.T) {
		os.RemoveAll(dbPath)
		NewDB()
		defer DBINS.Close()

		txn := DBINS.NewTransaction()
		list := NewStoreList2D[uint32, uint32, string](
			"test_ns", "k1_", "len_", "next_", "store_",
		)

		_, err := list.Insert(txn, 456, "value")
		require.NoError(t, err)
	})

	t.Run("UniAddr_key", func(t *testing.T) {
		os.RemoveAll(dbPath)
		NewDB()
		defer DBINS.Close()

		txn := DBINS.NewTransaction()
		list := NewStoreList2D[UniAddr, uint32, string](
			"test_ns", "k1_", "len_", "next_", "store_",
		)

		addr := UniAddr{T: 1, V: []byte{0x01, 0x02}}
		_, err := list.Insert(txn, addr, "value")
		require.NoError(t, err)

		val, err := list.Get(txn, addr, 0)
		require.NoError(t, err)
		require.Equal(t, "value", *val)
	})
}

// TestStoreList2D_ValueTypes 测试不同值类型
func TestStoreList2D_ValueTypes(t *testing.T) {
	os.RemoveAll(dbPath)
	NewDB()
	defer DBINS.Close()

	txn := DBINS.NewTransaction()

	t.Run("struct_value", func(t *testing.T) {
		type Data struct {
			Name  string
			Value int64
		}
		list := NewStoreList2D[string, uint32, Data](
			"test_ns", "k1_", "len_", "next_", "store_",
		)

		data := Data{Name: "test", Value: 123}
		id, err := list.Insert(txn, "group1", data)
		require.NoError(t, err)

		val, err := list.Get(txn, "group1", id)
		require.NoError(t, err)
		require.Equal(t, data, *val)
	})

	t.Run("Amount_value", func(t *testing.T) {
		list := NewStoreList2D[string, uint32, Amount](
			"test_ns", "k1_", "len_", "next_", "store_",
		)

		id, err := list.Insert(txn, "group1", ZeroAmount)
		require.NoError(t, err)

		val, err := list.Get(txn, "group1", id)
		require.NoError(t, err)
		require.NotNil(t, val)
	})
}

// TestStoreList2D_Namespace 测试命名空间隔离
func TestStoreList2D_Namespace(t *testing.T) {
	os.RemoveAll(dbPath)
	NewDB()
	defer DBINS.Close()

	txn := DBINS.NewTransaction()

	list1 := NewStoreList2D[string, uint32, string](
		"ns1", "k1_", "len_", "next_", "store_",
	)
	list2 := NewStoreList2D[string, uint32, string](
		"ns2", "k1_", "len_", "next_", "store_",
	)

	_, err := list1.Insert(txn, "group1", "from_ns1")
	require.NoError(t, err)

	_, err = list2.Insert(txn, "group1", "from_ns2")
	require.NoError(t, err)

	val1, err := list1.Get(txn, "group1", 0)
	require.NoError(t, err)
	require.Equal(t, "from_ns1", *val1)

	val2, err := list2.Get(txn, "group1", 0)
	require.NoError(t, err)
	require.Equal(t, "from_ns2", *val2)
}

// TestStoreList2D_ClearAndList 测试清除后列表行为
func TestStoreList2D_ClearAndList(t *testing.T) {
	os.RemoveAll(dbPath)
	NewDB()
	defer DBINS.Close()

	txn := DBINS.NewTransaction()
	list := NewStoreList2D[string, uint32, string](
		"test_ns", "k1_", "len_", "next_", "store_",
	)

	// 插入 3 条记录
	for i := 0; i < 3; i++ {
		_, err := list.Insert(txn, "group1", string(rune('a'+i)))
		require.NoError(t, err)
	}

	// 清除中间的记录
	err := list.Clear(txn, "group1", 1)
	require.NoError(t, err)

	// 列表应该跳过已清除的记录
	keys, _, err := list.List(txn, "group1", 0, 10)
	require.NoError(t, err)
	require.Len(t, keys, 2) // 只有 0 和 2
	require.Equal(t, uint32(0), keys[0])
	require.Equal(t, uint32(2), keys[1])
}

// TestStoreList2D_ManyRecords 测试大量记录
func TestStoreList2D_ManyRecords(t *testing.T) {
	os.RemoveAll(dbPath)
	NewDB()
	defer DBINS.Close()

	txn := DBINS.NewTransaction()
	list := NewStoreList2D[string, uint32, string](
		"test_ns", "k1_", "len_", "next_", "store_",
	)

	count := 100
	for i := 0; i < count; i++ {
		_, err := list.Insert(txn, "group1", "value")
		require.NoError(t, err)
	}

	// 验证长度
	len, err := list.Len(txn, "group1")
	require.NoError(t, err)
	require.Equal(t, uint32(count), len)

	// 验证所有记录
	keys, values, err := list.ListAll(txn, "group1")
	require.NoError(t, err)
	require.Len(t, keys, count)
	require.Len(t, values, count)
}

// TestStoreList2D_UpdateNonExistent 测试更新不存在的记录
func TestStoreList2D_UpdateNonExistent(t *testing.T) {
	os.RemoveAll(dbPath)
	NewDB()
	defer DBINS.Close()

	txn := DBINS.NewTransaction()
	list := NewStoreList2D[string, uint32, string](
		"test_ns", "k1_", "len_", "next_", "store_",
	)

	// 更新不存在的组应该返回错误
	err := list.Update(txn, "nonexistent", 0, "value")
	require.Error(t, err)
	require.Equal(t, ErrKeyNotFound, err)
}

// TestStoreList2D_ClearNonExistent 测试清除不存在的记录
func TestStoreList2D_ClearNonExistent(t *testing.T) {
	os.RemoveAll(dbPath)
	NewDB()
	defer DBINS.Close()

	txn := DBINS.NewTransaction()
	list := NewStoreList2D[string, uint32, string](
		"test_ns", "k1_", "len_", "next_", "store_",
	)

	// 清除不存在的组应该返回错误
	err := list.Clear(txn, "nonexistent", 0)
	require.Error(t, err)
	require.Equal(t, ErrKeyNotFound, err)
}

// TestStoreList2D_NextID 测试 NextID
func TestStoreList2D_NextID(t *testing.T) {
	os.RemoveAll(dbPath)
	NewDB()
	defer DBINS.Close()

	txn := DBINS.NewTransaction()
	list := NewStoreList2D[string, uint32, string](
		"test_ns", "k1_", "len_", "next_", "store_",
	)

	// 初始为 0
	nextID, err := list.NextID(txn, "group1")
	require.NoError(t, err)
	require.Equal(t, uint32(0), nextID)

	// 插入后 nextID 更新
	_, err = list.Insert(txn, "group1", "value")
	require.NoError(t, err)

	nextID, err = list.NextID(txn, "group1")
	require.NoError(t, err)
	require.Equal(t, uint32(1), nextID)
}
