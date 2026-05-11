// Package model: StoreMapping 提供类似智能合约 mapping 的键值存储能力。
// 泛型 StoreMapping[K, V] 的 K 为 key 类型（string、[]byte、uint64、uint32），V 为 codec 值类型。
package model

import (
	"encoding/hex"
	"errors"
	"strconv"

	"github.com/centrifuge/go-substrate-rpc-client/v4/types/codec"
	"github.com/cockroachdb/pebble"
)

// MappingKey 约束 mapping 的 key 类型：string、[]byte、uint64、uint32、uint16、uint8。
type MappingKey interface {
	string | []byte | uint64 | uint32 | uint16 | uint8 | UniAddr
}

// StoreMapping 类似 Solidity mapping(key => value)。
// K 为 key 类型；V 用于 codec 辅助方法，不影响 raw bytes 读写能力。
type StoreMapping[K MappingKey, V any] struct {
	Namespace string // 命名空间，如 "dao"
	KeyPrefix string // 键前缀，如 "member_"、"allowance_"
}

func (m *StoreMapping[K, V]) fullKey(suffix string) []byte {
	return ComboNamespaceKey(m.Namespace, m.KeyPrefix+suffix)
}

// keySuffix 将 key 编码为存储用的字符串后缀。
func (m *StoreMapping[K, V]) keySuffix(key K) string {
	switch v := any(key).(type) {
	case string:
		return v
	case []byte:
		return hex.EncodeToString(v)
	case uint64:
		return strconv.FormatUint(v, 10)
	case uint32:
		return strconv.FormatUint(uint64(v), 10)
	case uint16:
		return strconv.FormatUint(uint64(v), 10)
	case uint8:
		return strconv.FormatUint(uint64(v), 10)
	case UniAddr:
		k, _ := codec.EncodeToHex(v)
		return k
	default:
		panic("unsupported MappingKey type")
	}
}

// parseKeySuffix 将字符串后缀解码回 K 类型。
func parseKeySuffix[K MappingKey](suffix string) K {
	var zero K
	switch any(zero).(type) {
	case string:
		return any(suffix).(K)
	case []byte:
		b, _ := hex.DecodeString(suffix)
		return any(b).(K)
	case uint64:
		v, _ := strconv.ParseUint(suffix, 10, 64)
		return any(v).(K)
	case uint32:
		v, _ := strconv.ParseUint(suffix, 10, 32)
		return any(uint32(v)).(K)
	case uint16:
		v, _ := strconv.ParseUint(suffix, 10, 16)
		return any(uint16(v)).(K)
	case uint8:
		v, _ := strconv.ParseUint(suffix, 10, 8)
		return any(uint8(v)).(K)
	case UniAddr:
		var addr UniAddr
		codec.DecodeFromHex(suffix, &addr)
		return any(addr).(K)
	default:
		panic("unsupported MappingKey type")
	}
}

// StorageKey 返回该 key 对应的完整存储 key。
func (m *StoreMapping[K, V]) StorageKey(key K) []byte {
	return m.fullKey(m.keySuffix(key))
}

func (m *StoreMapping[K, V]) getByFullKey(txn *Txn, fullKey []byte) ([]byte, error) {
	v, err := txn.Get(fullKey)
	if err != nil {
		if errors.Is(err, pebble.ErrNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return v, nil
}

// Get 按 key 读取 raw 值；不存在时返回 (nil, nil)。
func (m *StoreMapping[K, V]) get(txn *Txn, key K) ([]byte, error) {
	return m.getByFullKey(txn, m.StorageKey(key))
}

// Set 按 key 写入 raw 值。
func (m *StoreMapping[K, V]) set(txn *Txn, key K, value []byte) error {
	return txn.Set(m.StorageKey(key), value)
}

// Delete 按 key 删除。
func (m *StoreMapping[K, V]) Delete(txn *Txn, key K) error {
	return txn.Delete(m.StorageKey(key))
}

// Contains 按 key 判断是否存在。
func (m *StoreMapping[K, V]) Contains(txn *Txn, key K) (bool, error) {
	v, err := m.get(txn, key)
	if err != nil {
		return false, err
	}
	return len(v) > 0, nil
}

// DeleteByPrefix 删除该 mapping 下以 prefix 开头的所有 key（慎用，会扫前缀）。
// prefix 为字符串后缀前缀，与 key 同编码方式，如 "0" 表示所有数字 key 中以 0 开头的。
func (m *StoreMapping[K, V]) DeleteByPrefix(txn *Txn, prefix string) error {
	return txn.DeletekeysByPrefix(m.fullKey(prefix))
}

// GetCodec 按 key 读取并使用 codec.Decode 反序列化为 V；不存在时返回 (nil, nil)。
func (m *StoreMapping[K, V]) Get(txn *Txn, key K) (*V, error) {
	bt, err := m.get(txn, key)
	if err != nil {
		return nil, err
	}
	if len(bt) == 0 {
		return nil, nil
	}

	val := new(V)
	if err := codec.Decode(bt, val); err != nil {
		return nil, err
	}
	return val, nil
}

// GetOrDefault 按 key 读取；不存在时返回 defaultVal 的副本（非 nil）。
func (m *StoreMapping[K, V]) GetOrDefault(txn *Txn, key K, defaultVal V) (V, error) {
	val, err := m.Get(txn, key)
	if err != nil {
		var zero V
		return zero, err
	}
	if val == nil {
		return defaultVal, nil
	}
	return *val, nil
}

// SetCodec 使用 codec.Encode 序列化 v 并按 key 写入。
func (m *StoreMapping[K, V]) Set(txn *Txn, key K, v V) error {
	bt, err := codec.Encode(v)
	if err != nil {
		return err
	}
	return m.set(txn, key, bt)
}

// ListCodec 读取该 mapping 下所有值并使用 codec.Decode 反序列化为 V。
func (m *StoreMapping[K, V]) List(txn *Txn) ([]K, []V, error) {
	keyPrefix := m.fullKey("")
	keys, list, err := txn.ListByPrefix(keyPrefix)
	if err != nil {
		return nil, nil, err
	}

	outKeys := make([]K, 0, len(list))
	out := make([]V, 0, len(list))
	for i, bt := range list {
		val := new(V)
		if err := codec.Decode(bt, val); err != nil {
			return nil, nil, err
		}
		out = append(out, *val)
		// 解析 key 后缀为 K 类型
		suffix := string(keys[i][len(keyPrefix):])
		outKeys = append(outKeys, parseKeySuffix[K](suffix))
	}

	return outKeys, out, nil
}
