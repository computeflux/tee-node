// Package model: StoreList 提供类似智能合约顺序列表存储的能力。
// 自增 id（K = uint8/uint16/uint32/uint64）作为 key。
// 支持 insert/get/update 和分页 list/desc_list。
package model

import (
	"github.com/centrifuge/go-substrate-rpc-client/v4/types/codec"
)

// ListIndex 约束列表索引类型：支持自增/自减。
// 用于 StoreList 的 id 类型。
type ListIndex interface {
	uint8 | uint16 | uint32 | uint64
}

// checkedNext 自增 1，溢出返回零值和 false。
func checkedNext[K ListIndex](v K) (K, bool) {
	switch val := any(v).(type) {
	case uint8:
		if val == ^uint8(0) {
			return 0, false
		}
		return any(val + 1).(K), true
	case uint16:
		if val == ^uint16(0) {
			return 0, false
		}
		return any(val + 1).(K), true
	case uint32:
		if val == ^uint32(0) {
			return 0, false
		}
		return any(val + 1).(K), true
	case uint64:
		if val == ^uint64(0) {
			return 0, false
		}
		return any(val + 1).(K), true
	}
	var zero K
	return zero, false
}

// checkedPrev 自减 1，下溢返回零值和 false。
func checkedPrev[K ListIndex](v K) (K, bool) {
	switch val := any(v).(type) {
	case uint8:
		if val == 0 {
			return 0, false
		}
		return any(val - 1).(K), true
	case uint16:
		if val == 0 {
			return 0, false
		}
		return any(val - 1).(K), true
	case uint32:
		if val == 0 {
			return 0, false
		}
		return any(val - 1).(K), true
	case uint64:
		if val == 0 {
			return 0, false
		}
		return any(val - 1).(K), true
	}
	var zero K
	return zero, false
}

// StoreList 列表存储：`next_id` 存当前长度（下一个将分配的 id），
// `items` 为 id -> value 的映射。K 为 id 类型（uint8/uint16/uint32/uint64）。
type StoreList[K ListIndex, V any] struct {
	Namespace      string // 命名空间
	KeyPrefixID    string // next_id 存储前缀
	KeyPrefixItems string // items 存储前缀
}

// NewStoreList 创建 StoreList 实例。
// prefixNextID: next_id 存储前缀
// prefixItems: items 存储前缀
func NewStoreList[K ListIndex, V any](namespace, prefixNextID, prefixItems string) *StoreList[K, V] {
	return &StoreList[K, V]{
		Namespace:      namespace,
		KeyPrefixID:    prefixNextID,
		KeyPrefixItems: prefixItems,
	}
}

// nextIDStorage 返回 next_id 的 StoreValue。
func (l *StoreList[K, V]) nextIDStorage() *StoreValue[K] {
	return &StoreValue[K]{Namespace: l.Namespace, Key: l.KeyPrefixID}
}

// itemsMapping 返回 items 的 StoreMapping。
func (l *StoreList[K, V]) itemsMapping() *StoreMapping[K, V] {
	return &StoreMapping[K, V]{Namespace: l.Namespace, KeyPrefix: l.KeyPrefixItems}
}

// Len 当前长度（即下一个将分配的 id）。
func (l *StoreList[K, V]) Len(txn *Txn) (K, error) {
	var zero K
	val, err := l.nextIDStorage().Get(txn)
	if err != nil {
		return zero, err
	}
	if val == nil {
		return zero, nil
	}
	return *val, nil
}

// Insert 插入一条记录，返回分配的 id。
// 如果索引溢出（如 uint8 达到 256），返回零值和 false。
func (l *StoreList[K, V]) Insert(txn *Txn, value V) (K, error) {
	var zero K
	k, err := l.Len(txn)
	if err != nil {
		return zero, err
	}
	next, ok := checkedNext(k)
	if !ok {
		return zero, nil // 溢出
	}
	if err := l.nextIDStorage().Set(txn, next); err != nil {
		return zero, err
	}
	if err := l.itemsMapping().Set(txn, k, value); err != nil {
		return zero, err
	}
	return k, nil
}

// Contains 是否存在该 key。
func (l *StoreList[K, V]) Contains(txn *Txn, key K) (bool, error) {
	return l.itemsMapping().Contains(txn, key)
}

// Get 按 key 取值。
func (l *StoreList[K, V]) Get(txn *Txn, key K) (*V, error) {
	return l.itemsMapping().Get(txn, key)
}

// GetOrDefault 按 key 取值；不存在时返回 defaultVal。
func (l *StoreList[K, V]) GetOrDefault(txn *Txn, key K, defaultVal V) (V, error) {
	return l.itemsMapping().GetOrDefault(txn, key, defaultVal)
}

// Update 更新 key 对应的值。
func (l *StoreList[K, V]) Update(txn *Txn, key K, value V) error {
	_, err := l.itemsMapping().Get(txn, key)
	if err != nil {
		return err
	}
	return l.itemsMapping().Set(txn, key, value)
}

// Clear 清除 key 对应的值（不改变 len/next_id）。
func (l *StoreList[K, V]) Clear(txn *Txn, key K) error {
	return l.itemsMapping().Delete(txn, key)
}

// List 分页列表（升序）：从 startKey 起取最多 size 条。
func (l *StoreList[K, V]) List(txn *Txn, startKey K, size uint32) ([]K, []V, error) {
	totalLen, err := l.Len(txn)
	if err != nil {
		return nil, nil, err
	}
	if size == 0 {
		return nil, nil, nil
	}

	var outKeys []K
	var outVals []V
	k := startKey
	for i := uint32(0); i < size; i++ {
		// 比较 k >= totalLen
		if compareListIndex(k, totalLen) >= 0 {
			break
		}
		v, err := l.Get(txn, k)
		if err != nil {
			return nil, nil, err
		}
		if v != nil {
			outKeys = append(outKeys, k)
			outVals = append(outVals, *v)
		}
		next, ok := checkedNext(k)
		if !ok {
			break
		}
		k = next
	}
	return outKeys, outVals, nil
}

// DescList 分页列表（降序）：从 startKey 起向前取最多 size 条。
// startKey 为 nil 时表示从末尾开始。
func (l *StoreList[K, V]) DescList(txn *Txn, startKey *K, size uint32) ([]K, []V, error) {
	totalLen, err := l.Len(txn)
	if err != nil {
		return nil, nil, err
	}
	if size == 0 {
		return nil, nil, nil
	}

	var k K
	if startKey != nil {
		k = *startKey
	} else {
		// 从末尾开始：totalLen - 1
		if totalLen == 0 {
			return nil, nil, nil
		}
		k, _ = checkedPrev(totalLen)
	}

	var outKeys []K
	var outVals []V
	for i := uint32(0); i < size; i++ {
		v, err := l.Get(txn, k)
		if err != nil {
			return nil, nil, err
		}
		if v != nil {
			outKeys = append(outKeys, k)
			outVals = append(outVals, *v)
		}
		prev, ok := checkedPrev(k)
		if !ok {
			break
		}
		k = prev
	}
	return outKeys, outVals, nil
}

// ListAll 返回所有 (key, value) 对。
func (l *StoreList[K, V]) ListAll(txn *Txn) ([]K, []V, error) {
	return l.List(txn, 0, ^uint32(0))
}

// compareListIndex 比较两个 ListIndex 类型的值。
// 返回 -1, 0, 1 分别表示 a < b, a == b, a > b。
func compareListIndex[K ListIndex](a, b K) int {
	switch any(a).(type) {
	case uint8:
		ai, bi := any(a).(uint8), any(b).(uint8)
		if ai < bi {
			return -1
		} else if ai > bi {
			return 1
		}
	case uint16:
		ai, bi := any(a).(uint16), any(b).(uint16)
		if ai < bi {
			return -1
		} else if ai > bi {
			return 1
		}
	case uint32:
		ai, bi := any(a).(uint32), any(b).(uint32)
		if ai < bi {
			return -1
		} else if ai > bi {
			return 1
		}
	case uint64:
		ai, bi := any(a).(uint64), any(b).(uint64)
		if ai < bi {
			return -1
		} else if ai > bi {
			return 1
		}
	}
	return 0
}

// compile-time check
var _ = codec.Encode
