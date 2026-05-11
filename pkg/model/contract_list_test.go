package model

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

// =============================================================================
// StoreList 单元测试
// =============================================================================

// TestStoreList_InsertAndGet 测试基本 Insert/Get 操作
func TestStoreList_InsertAndGet(t *testing.T) {
	os.RemoveAll(dbPath)
	NewDB()
	defer DBINS.Close()

	txn := DBINS.NewTransaction()
	list := NewStoreList[uint32, string]("test_ns", "list_next_", "list_items_")

	// 1. 初始长度为 0
	len, err := list.Len(txn)
	require.NoError(t, err)
	require.Equal(t, uint32(0), len)

	// 2. Insert 插入记录
	id, err := list.Insert(txn, "first")
	require.NoError(t, err)
	require.Equal(t, uint32(0), id)

	id, err = list.Insert(txn, "second")
	require.NoError(t, err)
	require.Equal(t, uint32(1), id)

	// 3. 长度更新
	len, err = list.Len(txn)
	require.NoError(t, err)
	require.Equal(t, uint32(2), len)

	// 4. Get 读取值
	val, err := list.Get(txn, 0)
	require.NoError(t, err)
	require.NotNil(t, val)
	require.Equal(t, "first", *val)

	val, err = list.Get(txn, 1)
	require.NoError(t, err)
	require.NotNil(t, val)
	require.Equal(t, "second", *val)

	// 5. 不存在的 key 返回 nil
	val, err = list.Get(txn, 999)
	require.NoError(t, err)
	require.Nil(t, val)
}

// TestStoreList_GetOrDefault 测试 GetOrDefault
func TestStoreList_GetOrDefault(t *testing.T) {
	os.RemoveAll(dbPath)
	NewDB()
	defer DBINS.Close()

	txn := DBINS.NewTransaction()
	list := NewStoreList[uint32, int64]("test_ns", "list_next_", "list_items_")

	// 不存在时返回默认值
	val, err := list.GetOrDefault(txn, 0, 999)
	require.NoError(t, err)
	require.Equal(t, int64(999), val)

	// 存在时返回存储的值
	_, err = list.Insert(txn, 100)
	require.NoError(t, err)

	val, err = list.GetOrDefault(txn, 0, 999)
	require.NoError(t, err)
	require.Equal(t, int64(100), val)
}

// TestStoreList_Contains 测试 Contains
func TestStoreList_Contains(t *testing.T) {
	os.RemoveAll(dbPath)
	NewDB()
	defer DBINS.Close()

	txn := DBINS.NewTransaction()
	list := NewStoreList[uint32, string]("test_ns", "list_next_", "list_items_")

	// 初始不存在
	contains, err := list.Contains(txn, 0)
	require.NoError(t, err)
	require.False(t, contains)

	// 插入后存在
	_, err = list.Insert(txn, "value")
	require.NoError(t, err)

	contains, err = list.Contains(txn, 0)
	require.NoError(t, err)
	require.True(t, contains)
}

// TestStoreList_Update 测试 Update
func TestStoreList_Update(t *testing.T) {
	os.RemoveAll(dbPath)
	NewDB()
	defer DBINS.Close()

	txn := DBINS.NewTransaction()
	list := NewStoreList[uint32, string]("test_ns", "list_next_", "list_items_")

	_, err := list.Insert(txn, "original")
	require.NoError(t, err)

	// 更新值
	err = list.Update(txn, 0, "updated")
	require.NoError(t, err)

	val, err := list.Get(txn, 0)
	require.NoError(t, err)
	require.Equal(t, "updated", *val)
}

// TestStoreList_Clear 测试 Clear
func TestStoreList_Clear(t *testing.T) {
	os.RemoveAll(dbPath)
	NewDB()
	defer DBINS.Close()

	txn := DBINS.NewTransaction()
	list := NewStoreList[uint32, string]("test_ns", "list_next_", "list_items_")

	_, err := list.Insert(txn, "value")
	require.NoError(t, err)

	// 清除
	err = list.Clear(txn, 0)
	require.NoError(t, err)

	// 清除后不存在
	contains, err := list.Contains(txn, 0)
	require.NoError(t, err)
	require.False(t, contains)

	// 长度不变（clear 不改变 next_id）
	len, err := list.Len(txn)
	require.NoError(t, err)
	require.Equal(t, uint32(1), len)
}

// TestStoreList_List 测试分页列表
func TestStoreList_List(t *testing.T) {
	os.RemoveAll(dbPath)
	NewDB()
	defer DBINS.Close()

	txn := DBINS.NewTransaction()
	list := NewStoreList[uint32, string]("test_ns", "list_next_", "list_items_")

	// 插入 5 条记录
	for i := 0; i < 5; i++ {
		_, err := list.Insert(txn, string(rune('a'+i)))
		require.NoError(t, err)
	}

	// 从 0 开始取 3 条
	keys, values, err := list.List(txn, 0, 3)
	require.NoError(t, err)
	require.Len(t, keys, 3)
	require.Len(t, values, 3)
	require.Equal(t, uint32(0), keys[0])
	require.Equal(t, uint32(1), keys[1])
	require.Equal(t, uint32(2), keys[2])

	// 从 2 开始取 3 条（只有 2, 3, 4）
	keys, values, err = list.List(txn, 2, 3)
	require.NoError(t, err)
	require.Len(t, keys, 3)
	require.Equal(t, uint32(2), keys[0])
	require.Equal(t, uint32(3), keys[1])
	require.Equal(t, uint32(4), keys[2])

	// 从 10 开始（超出范围）
	keys, values, err = list.List(txn, 10, 3)
	require.NoError(t, err)
	require.Empty(t, keys)

	// size 为 0
	keys, values, err = list.List(txn, 0, 0)
	require.NoError(t, err)
	require.Empty(t, keys)
}

// TestStoreList_DescList 测试降序列表
func TestStoreList_DescList(t *testing.T) {
	os.RemoveAll(dbPath)
	NewDB()
	defer DBINS.Close()

	txn := DBINS.NewTransaction()
	list := NewStoreList[uint32, string]("test_ns", "list_next_", "list_items_")

	// 插入 5 条记录
	for i := 0; i < 5; i++ {
		_, err := list.Insert(txn, string(rune('a'+i)))
		require.NoError(t, err)
	}

	// 从末尾开始取 3 条
	keys, _, err := list.DescList(txn, nil, 3)
	require.NoError(t, err)
	require.Len(t, keys, 3)
	require.Equal(t, uint32(4), keys[0])
	require.Equal(t, uint32(3), keys[1])
	require.Equal(t, uint32(2), keys[2])

	// 从指定位置开始
	startKey := uint32(2)
	keys, _, err = list.DescList(txn, &startKey, 3)
	require.NoError(t, err)
	require.Len(t, keys, 3)
	require.Equal(t, uint32(2), keys[0])
	require.Equal(t, uint32(1), keys[1])
	require.Equal(t, uint32(0), keys[2])
}

// TestStoreList_ListAll 测试获取所有记录
func TestStoreList_ListAll(t *testing.T) {
	os.RemoveAll(dbPath)
	NewDB()
	defer DBINS.Close()

	txn := DBINS.NewTransaction()
	list := NewStoreList[uint32, string]("test_ns", "list_next_", "list_items_")

	// 空列表
	keys, values, err := list.ListAll(txn)
	require.NoError(t, err)
	require.Empty(t, keys)

	// 插入 5 条记录
	for i := 0; i < 5; i++ {
		_, err := list.Insert(txn, string(rune('a'+i)))
		require.NoError(t, err)
	}

	keys, values, err = list.ListAll(txn)
	require.NoError(t, err)
	require.Len(t, keys, 5)
	require.Len(t, values, 5)
}

// TestStoreList_IndexTypes 测试不同索引类型
func TestStoreList_IndexTypes(t *testing.T) {
	t.Run("uint8", func(t *testing.T) {
		os.RemoveAll(dbPath)
		NewDB()
		defer DBINS.Close()

		txn := DBINS.NewTransaction()
		list := NewStoreList[uint8, string]("test_ns", "list_u8_", "items_")

		id, err := list.Insert(txn, "value")
		require.NoError(t, err)
		require.Equal(t, uint8(0), id)

		val, err := list.Get(txn, 0)
		require.NoError(t, err)
		require.Equal(t, "value", *val)
	})

	t.Run("uint16", func(t *testing.T) {
		os.RemoveAll(dbPath)
		NewDB()
		defer DBINS.Close()

		txn := DBINS.NewTransaction()
		list := NewStoreList[uint16, string]("test_ns", "list_u16_", "items_")

		id, err := list.Insert(txn, "value")
		require.NoError(t, err)
		require.Equal(t, uint16(0), id)
	})

	t.Run("uint64", func(t *testing.T) {
		os.RemoveAll(dbPath)
		NewDB()
		defer DBINS.Close()

		txn := DBINS.NewTransaction()
		list := NewStoreList[uint64, string]("test_ns", "list_u64_", "items_")

		id, err := list.Insert(txn, "value")
		require.NoError(t, err)
		require.Equal(t, uint64(0), id)
	})
}

// TestStoreList_ValueTypes 测试不同值类型
func TestStoreList_ValueTypes(t *testing.T) {
	os.RemoveAll(dbPath)
	NewDB()
	defer DBINS.Close()

	txn := DBINS.NewTransaction()

	t.Run("struct_value", func(t *testing.T) {
		type Data struct {
			Name  string
			Value int64
		}
		list := NewStoreList[uint32, Data]("test_ns", "struct_next_", "struct_items_")

		data := Data{Name: "test", Value: 123}
		id, err := list.Insert(txn, data)
		require.NoError(t, err)

		val, err := list.Get(txn, id)
		require.NoError(t, err)
		require.Equal(t, data, *val)
	})

	t.Run("Amount_value", func(t *testing.T) {
		list := NewStoreList[uint32, Amount]("test_ns", "amount_next_", "amount_items_")

		amount := ZeroAmount
		id, err := list.Insert(txn, amount)
		require.NoError(t, err)

		val, err := list.Get(txn, id)
		require.NoError(t, err)
		require.NotNil(t, val)
	})
}

// TestStoreList_Namespace 测试命名空间隔离
func TestStoreList_Namespace(t *testing.T) {
	os.RemoveAll(dbPath)
	NewDB()
	defer DBINS.Close()

	txn := DBINS.NewTransaction()

	list1 := NewStoreList[uint32, string]("ns1", "next_", "items_")
	list2 := NewStoreList[uint32, string]("ns2", "next_", "items_")

	_, err := list1.Insert(txn, "from_ns1")
	require.NoError(t, err)

	_, err = list2.Insert(txn, "from_ns2")
	require.NoError(t, err)

	val1, err := list1.Get(txn, 0)
	require.NoError(t, err)
	require.Equal(t, "from_ns1", *val1)

	val2, err := list2.Get(txn, 0)
	require.NoError(t, err)
	require.Equal(t, "from_ns2", *val2)
}

// TestStoreList_ClearAndList 测试清除后列表行为
func TestStoreList_ClearAndList(t *testing.T) {
	os.RemoveAll(dbPath)
	NewDB()
	defer DBINS.Close()

	txn := DBINS.NewTransaction()
	list := NewStoreList[uint32, string]("test_ns", "list_next_", "list_items_")

	// 插入 3 条记录
	for i := 0; i < 3; i++ {
		_, err := list.Insert(txn, string(rune('a'+i)))
		require.NoError(t, err)
	}

	// 清除中间的记录
	err := list.Clear(txn, 1)
	require.NoError(t, err)

	// 列表应该跳过已清除的记录
	keys, _, err := list.List(txn, 0, 10)
	require.NoError(t, err)
	require.Len(t, keys, 2) // 只有 0 和 2
	require.Equal(t, uint32(0), keys[0])
	require.Equal(t, uint32(2), keys[1])
}

// TestStoreList_ManyRecords 测试大量记录
func TestStoreList_ManyRecords(t *testing.T) {
	os.RemoveAll(dbPath)
	NewDB()
	defer DBINS.Close()

	txn := DBINS.NewTransaction()
	list := NewStoreList[uint32, string]("test_ns", "list_next_", "list_items_")

	count := 100
	for i := 0; i < count; i++ {
		_, err := list.Insert(txn, "value")
		require.NoError(t, err)
	}

	// 验证长度
	len, err := list.Len(txn)
	require.NoError(t, err)
	require.Equal(t, uint32(count), len)

	// 验证所有记录
	keys, values, err := list.ListAll(txn)
	require.NoError(t, err)
	require.Len(t, keys, count)
	require.Len(t, values, count)
}
