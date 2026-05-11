package model

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

// =============================================================================
// StoreValue 单元测试
// =============================================================================

// TestStoreValue_SetAndGet 测试基本 Set/Get 操作
func TestStoreValue_SetAndGet(t *testing.T) {
	os.RemoveAll(dbPath)
	NewDB()
	defer DBINS.Close()

	txn := DBINS.NewTransaction()
	sv := StoreValue[string]{
		Namespace: "test_ns",
		Key:       "test_key",
	}

	// 1. 初始读取应返回 nil
	val, err := sv.Get(txn)
	require.NoError(t, err)
	require.Nil(t, val)

	// 2. Set 写入值
	err = sv.Set(txn, "hello world")
	require.NoError(t, err)

	// 3. Get 读取刚写入的值
	val, err = sv.Get(txn)
	require.NoError(t, err)
	require.NotNil(t, val)
	require.Equal(t, "hello world", *val)

	// 4. 覆盖写入
	err = sv.Set(txn, "new value")
	require.NoError(t, err)

	val, err = sv.Get(txn)
	require.NoError(t, err)
	require.Equal(t, "new value", *val)
}

// TestStoreValue_GetOrDefault 测试 GetOrDefault 方法
func TestStoreValue_GetOrDefault(t *testing.T) {
	os.RemoveAll(dbPath)
	NewDB()
	defer DBINS.Close()

	txn := DBINS.NewTransaction()
	sv := StoreValue[int64]{
		Namespace: "test_ns",
		Key:       "counter",
	}

	// 1. 不存在时返回默认值
	val, err := sv.GetOrDefault(txn, 100)
	require.NoError(t, err)
	require.Equal(t, int64(100), val)

	// 2. 存在时返回存储的值
	err = sv.Set(txn, 200)
	require.NoError(t, err)

	val, err = sv.GetOrDefault(txn, 100)
	require.NoError(t, err)
	require.Equal(t, int64(200), val)
}

// TestStoreValue_GetOrDefault_Struct 测试结构体类型的 GetOrDefault
func TestStoreValue_GetOrDefault_Struct(t *testing.T) {
	os.RemoveAll(dbPath)
	NewDB()
	defer DBINS.Close()

	type Config struct {
		Name  string
		Value int64
	}

	txn := DBINS.NewTransaction()
	sv := StoreValue[Config]{
		Namespace: "test_ns",
		Key:       "config",
	}

	// 1. 不存在时返回默认值
	defaultConfig := Config{Name: "default", Value: 0}
	val, err := sv.GetOrDefault(txn, defaultConfig)
	require.NoError(t, err)
	require.Equal(t, defaultConfig, val)

	// 2. 存在时返回存储的值
	storedConfig := Config{Name: "production", Value: 42}
	err = sv.Set(txn, storedConfig)
	require.NoError(t, err)

	val, err = sv.GetOrDefault(txn, defaultConfig)
	require.NoError(t, err)
	require.Equal(t, storedConfig, val)
}

// TestStoreValue_Exists 测试 Exists 方法
func TestStoreValue_Exists(t *testing.T) {
	os.RemoveAll(dbPath)
	NewDB()
	defer DBINS.Close()

	txn := DBINS.NewTransaction()
	sv := StoreValue[string]{
		Namespace: "test_ns",
		Key:       "exists_test",
	}

	// 1. 初始不存在
	exists, err := sv.Exists(txn)
	require.NoError(t, err)
	require.False(t, exists)

	// 2. 写入后存在
	err = sv.Set(txn, "value")
	require.NoError(t, err)

	exists, err = sv.Exists(txn)
	require.NoError(t, err)
	require.True(t, exists)
}

// TestStoreValue_Delete 测试 Delete 方法
func TestStoreValue_Delete(t *testing.T) {
	os.RemoveAll(dbPath)
	NewDB()
	defer DBINS.Close()

	txn := DBINS.NewTransaction()
	sv := StoreValue[string]{
		Namespace: "test_ns",
		Key:       "delete_test",
	}

	// 1. 写入值
	err := sv.Set(txn, "to be deleted")
	require.NoError(t, err)

	exists, err := sv.Exists(txn)
	require.NoError(t, err)
	require.True(t, exists)

	// 2. 删除
	err = sv.Delete(txn)
	require.NoError(t, err)

	// 3. 删除后不存在
	exists, err = sv.Exists(txn)
	require.NoError(t, err)
	require.False(t, exists)

	// 4. 删除不存在的值不应报错
	err = sv.Delete(txn)
	require.NoError(t, err)
}

// TestStoreValue_GetRawAndSetRaw 测试原始字节操作
func TestStoreValue_GetRawAndSetRaw(t *testing.T) {
	os.RemoveAll(dbPath)
	NewDB()
	defer DBINS.Close()

	txn := DBINS.NewTransaction()
	sv := StoreValue[string]{
		Namespace: "test_ns",
		Key:       "raw_test",
	}

	// 1. 初始读取返回 nil
	raw, err := sv.GetRaw(txn)
	require.NoError(t, err)
	require.Nil(t, raw)

	// 2. SetRaw 写入原始字节
	err = sv.SetRaw(txn, []byte{0x01, 0x02, 0x03})
	require.NoError(t, err)

	// 3. GetRaw 读取原始字节
	raw, err = sv.GetRaw(txn)
	require.NoError(t, err)
	require.Equal(t, []byte{0x01, 0x02, 0x03}, raw)

	// 4. Get 应该能解码（如果字节格式正确）
	// 注意：这里的字节可能不是有效的 codec 编码，所以只测试原始读写
}

// TestStoreValue_DifferentTypes 测试不同泛型类型
func TestStoreValue_DifferentTypes(t *testing.T) {
	os.RemoveAll(dbPath)
	NewDB()
	defer DBINS.Close()

	txn := DBINS.NewTransaction()

	// 测试 uint64 类型
	t.Run("uint64", func(t *testing.T) {
		sv := StoreValue[uint64]{
			Namespace: "test_ns",
			Key:       "uint64_key",
		}
		err := sv.Set(txn, uint64(123456789))
		require.NoError(t, err)

		val, err := sv.Get(txn)
		require.NoError(t, err)
		require.NotNil(t, val)
		require.Equal(t, uint64(123456789), *val)
	})

	// 测试 bool 类型
	t.Run("bool", func(t *testing.T) {
		sv := StoreValue[bool]{
			Namespace: "test_ns",
			Key:       "bool_key",
		}
		err := sv.Set(txn, true)
		require.NoError(t, err)

		val, err := sv.Get(txn)
		require.NoError(t, err)
		require.NotNil(t, val)
		require.True(t, *val)
	})

	// 测试 []byte 类型
	t.Run("bytes", func(t *testing.T) {
		sv := StoreValue[[]byte]{
			Namespace: "test_ns",
			Key:       "bytes_key",
		}
		original := []byte{0xDE, 0xAD, 0xBE, 0xEF}
		err := sv.Set(txn, original)
		require.NoError(t, err)

		val, err := sv.Get(txn)
		require.NoError(t, err)
		require.NotNil(t, val)
		require.Equal(t, original, *val)
	})

	// 测试 Amount 类型
	t.Run("Amount", func(t *testing.T) {
		sv := StoreValue[Amount]{
			Namespace: "test_ns",
			Key:       "amount_key",
		}
		amount := AmountAdd(AmountSub(ZeroAmount, ZeroAmount), ZeroAmount) // 零值
		err := sv.Set(txn, amount)
		require.NoError(t, err)

		val, err := sv.Get(txn)
		require.NoError(t, err)
		require.NotNil(t, val)
	})

	// 测试 UniAddr 类型
	t.Run("UniAddr", func(t *testing.T) {
		sv := StoreValue[UniAddr]{
			Namespace: "test_ns",
			Key:       "uniaddr_key",
		}
		addr := UniAddr{T: 1, V: []byte{0x01, 0x02, 0x03}}
		err := sv.Set(txn, addr)
		require.NoError(t, err)

		val, err := sv.Get(txn)
		require.NoError(t, err)
		require.NotNil(t, val)
		require.Equal(t, uint32(1), val.T)
		require.Equal(t, []byte{0x01, 0x02, 0x03}, val.V)
	})
}

// TestStoreValue_Namespace 测试不同命名空间
func TestStoreValue_Namespace(t *testing.T) {
	os.RemoveAll(dbPath)
	NewDB()
	defer DBINS.Close()

	txn := DBINS.NewTransaction()

	// 相同 Key，不同 Namespace
	sv1 := StoreValue[string]{
		Namespace: "ns1",
		Key:       "same_key",
	}
	sv2 := StoreValue[string]{
		Namespace: "ns2",
		Key:       "same_key",
	}

	err := sv1.Set(txn, "value_from_ns1")
	require.NoError(t, err)

	err = sv2.Set(txn, "value_from_ns2")
	require.NoError(t, err)

	val1, err := sv1.Get(txn)
	require.NoError(t, err)
	require.Equal(t, "value_from_ns1", *val1)

	val2, err := sv2.Get(txn)
	require.NoError(t, err)
	require.Equal(t, "value_from_ns2", *val2)
}

// TestStoreValue_ZeroValue 测试零值处理
func TestStoreValue_ZeroValue(t *testing.T) {
	os.RemoveAll(dbPath)
	NewDB()
	defer DBINS.Close()

	txn := DBINS.NewTransaction()
	sv := StoreValue[int64]{
		Namespace: "test_ns",
		Key:       "zero_test",
	}

	// 写入零值
	err := sv.Set(txn, 0)
	require.NoError(t, err)

	// 零值应该能正确读取
	val, err := sv.Get(txn)
	require.NoError(t, err)
	require.NotNil(t, val)
	require.Equal(t, int64(0), *val)

	// 零值应该存在
	exists, err := sv.Exists(txn)
	require.NoError(t, err)
	require.True(t, exists)
}

// TestStoreValue_EmptyString 测试空字符串处理
func TestStoreValue_EmptyString(t *testing.T) {
	os.RemoveAll(dbPath)
	NewDB()
	defer DBINS.Close()

	txn := DBINS.NewTransaction()
	sv := StoreValue[string]{
		Namespace: "test_ns",
		Key:       "empty_string_test",
	}

	// 写入空字符串
	err := sv.Set(txn, "")
	require.NoError(t, err)

	// 空字符串应该能正确读取
	val, err := sv.Get(txn)
	require.NoError(t, err)
	require.NotNil(t, val)
	require.Equal(t, "", *val)

	// 空字符串应该存在
	exists, err := sv.Exists(txn)
	require.NoError(t, err)
	require.True(t, exists)
}
