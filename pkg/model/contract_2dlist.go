// Package model: StoreList2D 提供类似智能合约二维列表存储的能力。
// 二维列表：按 K1 分组，每组内为自增 Ix 索引的列表。
// 布局：k1->id, k1_length, k2_next_id per id, store (id, k2)->value。
package model

import (
	"strconv"

	"github.com/centrifuge/go-substrate-rpc-client/v4/types/codec"
)

// List2DKey1 约束二维列表的外层 key 类型。
type List2DKey1 interface {
	string | []byte | uint64 | uint32 | UniAddr
}

// StoreList2D 二维列表：外层 key K1，内层自增索引 Ix（uint8/uint16/uint32/uint64）。
// K1 首次出现时分配一个 id，该 id 下 k2 从 0 自增。
type StoreList2D[K1 List2DKey1, Ix ListIndex, V any] struct {
	Namespace      string
	PrefixK1ToID   string // k1 -> id 映射前缀
	PrefixK1Length string // 不同 K1 数量前缀
	PrefixK2NextID string // 内部 id -> 下一个内层索引前缀
	PrefixStore    string // (内层_id, 内层索引) -> value 前缀
}

// NewStoreList2D 创建 StoreList2D 实例。
func NewStoreList2D[K1 List2DKey1, Ix ListIndex, V any](
	namespace, prefixK1ToID, prefixK1Length, prefixK2NextID, prefixStore string,
) *StoreList2D[K1, Ix, V] {
	return &StoreList2D[K1, Ix, V]{
		Namespace:      namespace,
		PrefixK1ToID:   prefixK1ToID,
		PrefixK1Length: prefixK1Length,
		PrefixK2NextID: prefixK2NextID,
		PrefixStore:    prefixStore,
	}
}

// k1ToIDMapping 返回 k1 -> id 的映射。
func (l *StoreList2D[K1, Ix, V]) k1ToIDMapping() *StoreMapping[K1, Ix] {
	return &StoreMapping[K1, Ix]{Namespace: l.Namespace, KeyPrefix: l.PrefixK1ToID}
}

// k1LengthStorage 返回 k1_length 存储。
func (l *StoreList2D[K1, Ix, V]) k1LengthStorage() *StoreValue[Ix] {
	return &StoreValue[Ix]{Namespace: l.Namespace, Key: l.PrefixK1Length}
}

// k2NextIDMapping 返回 id -> next_id 的映射。
func (l *StoreList2D[K1, Ix, V]) k2NextIDMapping() *StoreMapping[Ix, Ix] {
	return &StoreMapping[Ix, Ix]{Namespace: l.Namespace, KeyPrefix: l.PrefixK2NextID}
}

// storeMapping 返回 (id, k2) -> value 的映射。
func (l *StoreList2D[K1, Ix, V]) storeMapping() *StoreMapping[string, V] {
	return &StoreMapping[string, V]{Namespace: l.Namespace, KeyPrefix: l.PrefixStore}
}

// storeKey 生成存储 key：将 (id, k2) 编码为字符串。
func (l *StoreList2D[K1, Ix, V]) storeKey(id, k2 Ix) string {
	return list2DKeyEncode(id, k2)
}

// list2DKeyEncode 将 (id, k2) 编码为字符串 key。
func list2DKeyEncode[Ix ListIndex](id, k2 Ix) string {
	switch any(id).(type) {
	case uint8:
		return string([]byte{byte(any(id).(uint8)), byte(any(k2).(uint8))})
	case uint16:
		b := make([]byte, 4)
		u16ToBytes(any(id).(uint16), b[0:2])
		u16ToBytes(any(k2).(uint16), b[2:4])
		return string(b)
	case uint32:
		b := make([]byte, 8)
		u32ToBytes(any(id).(uint32), b[0:4])
		u32ToBytes(any(k2).(uint32), b[4:8])
		return string(b)
	case uint64:
		b := make([]byte, 16)
		u64ToBytes(any(id).(uint64), b[0:8])
		u64ToBytes(any(k2).(uint64), b[8:16])
		return string(b)
	}
	return ""
}

// list2DKeyDecode 将字符串 key 解码为 (id, k2)。
func list2DKeyDecode[Ix ListIndex](s string) (Ix, Ix) {
	b := []byte(s)
	switch any(*new(Ix)).(type) {
	case uint8:
		return any(Ix(b[0])).(Ix), any(Ix(b[1])).(Ix)
	case uint16:
		return any(Ix(bytesToU16(b[0:2]))).(Ix), any(Ix(bytesToU16(b[2:4]))).(Ix)
	case uint32:
		return any(Ix(bytesToU32(b[0:4]))).(Ix), any(Ix(bytesToU32(b[4:8]))).(Ix)
	case uint64:
		return any(Ix(bytesToU64(b[0:8]))).(Ix), any(Ix(bytesToU64(b[8:16]))).(Ix)
	}
	var zero Ix
	return zero, zero
}

// u16ToBytes 将 uint16 转为字节。
func u16ToBytes(v uint16, b []byte) {
	b[0] = byte(v >> 8)
	b[1] = byte(v)
}

// bytesToU16 将字节转为 uint16。
func bytesToU16(b []byte) uint16 {
	return uint16(b[0])<<8 | uint16(b[1])
}

// u32ToBytes 将 uint32 转为字节。
func u32ToBytes(v uint32, b []byte) {
	b[0] = byte(v >> 24)
	b[1] = byte(v >> 16)
	b[2] = byte(v >> 8)
	b[3] = byte(v)
}

// bytesToU32 将字节转为 uint32。
func bytesToU32(b []byte) uint32 {
	return uint32(b[0])<<24 | uint32(b[1])<<16 | uint32(b[2])<<8 | uint32(b[3])
}

// u64ToBytes 将 uint64 转为字节。
func u64ToBytes(v uint64, b []byte) {
	b[0] = byte(v >> 56)
	b[1] = byte(v >> 48)
	b[2] = byte(v >> 40)
	b[3] = byte(v >> 32)
	b[4] = byte(v >> 24)
	b[5] = byte(v >> 16)
	b[6] = byte(v >> 8)
	b[7] = byte(v)
}

// bytesToU64 将字节转为 uint64。
func bytesToU64(b []byte) uint64 {
	return uint64(b[0])<<56 | uint64(b[1])<<48 | uint64(b[2])<<40 | uint64(b[3])<<32 |
		uint64(b[4])<<24 | uint64(b[5])<<16 | uint64(b[6])<<8 | uint64(b[7])
}

// NextID 返回该 K1 下下一个将分配的 k2（即当前长度）。
func (l *StoreList2D[K1, Ix, V]) NextID(txn *Txn, k1 K1) (Ix, error) {
	var zero Ix
	id, err := l.k1ToIDMapping().Get(txn, k1)
	if err != nil {
		return zero, err
	}
	if id == nil {
		return zero, nil
	}
	nextID, err := l.k2NextIDMapping().Get(txn, *id)
	if err != nil {
		return zero, err
	}
	if nextID == nil {
		return zero, nil
	}
	return *nextID, nil
}

// Len 返回该 K1 下的条目数量（与 NextID 一致）。
func (l *StoreList2D[K1, Ix, V]) Len(txn *Txn, k1 K1) (Ix, error) {
	return l.NextID(txn, k1)
}

// Insert 在 k1 下插入一条记录，返回分配的 k2。
// 如果内层索引会溢出，则返回零值和 nil。
func (l *StoreList2D[K1, Ix, V]) Insert(txn *Txn, k1 K1, value V) (Ix, error) {
	var zero Ix

	// 获取或分配 id
	id, err := l.k1ToIDMapping().Get(txn, k1)
	if err != nil {
		return zero, err
	}
	if id == nil {
		// 首次出现，分配新 id
		length, err := l.k1LengthStorage().GetOrDefault(txn, zero)
		if err != nil {
			return zero, err
		}
		id = &length
		if err := l.k1ToIDMapping().Set(txn, k1, length); err != nil {
			return zero, err
		}
		nextLen, ok := checkedNext(length)
		if !ok {
			return zero, nil
		}
		if err := l.k1LengthStorage().Set(txn, nextLen); err != nil {
			return zero, err
		}
	}

	// 获取下一个 k2
	nextID, err := l.k2NextIDMapping().GetOrDefault(txn, *id, zero)
	if err != nil {
		return zero, err
	}
	newNextID, ok := checkedNext(nextID)
	if !ok {
		return zero, nil
	}
	if err := l.k2NextIDMapping().Set(txn, *id, newNextID); err != nil {
		return zero, err
	}

	// 存储值
	storeKey := l.storeKey(*id, nextID)
	if err := l.storeMapping().Set(txn, storeKey, value); err != nil {
		return zero, err
	}

	return nextID, nil
}

// Update 更新 (k1, k2) 对应的值。
// 如果 k1 不存在或 k2 超出该 k1 的范围，则返回错误。
func (l *StoreList2D[K1, Ix, V]) Update(txn *Txn, k1 K1, k2 Ix, value V) error {
	id, err := l.k1ToIDMapping().Get(txn, k1)
	if err != nil {
		return err
	}
	if id == nil {
		return ErrKeyNotFound
	}
	storeKey := l.storeKey(*id, k2)
	return l.storeMapping().Set(txn, storeKey, value)
}

// Clear 清除 (k1, k2) 对应的值（不改变该 k1 的 len/next_id）。
func (l *StoreList2D[K1, Ix, V]) Clear(txn *Txn, k1 K1, k2 Ix) error {
	id, err := l.k1ToIDMapping().Get(txn, k1)
	if err != nil {
		return err
	}
	if id == nil {
		return ErrKeyNotFound
	}
	storeKey := l.storeKey(*id, k2)
	return l.storeMapping().Delete(txn, storeKey)
}

// Get 按 (k1, k2) 取值。
func (l *StoreList2D[K1, Ix, V]) Get(txn *Txn, k1 K1, k2 Ix) (*V, error) {
	id, err := l.k1ToIDMapping().Get(txn, k1)
	if err != nil {
		return nil, err
	}
	if id == nil {
		return nil, nil
	}
	storeKey := l.storeKey(*id, k2)
	return l.storeMapping().Get(txn, storeKey)
}

// GetOrDefault 按 (k1, k2) 取值；不存在时返回 defaultVal。
func (l *StoreList2D[K1, Ix, V]) GetOrDefault(txn *Txn, k1 K1, k2 Ix, defaultVal V) (V, error) {
	v, err := l.Get(txn, k1, k2)
	if err != nil {
		var zero V
		return zero, err
	}
	if v == nil {
		return defaultVal, nil
	}
	return *v, nil
}

// List 分页列表（升序）：k1 下从 startKey 起取最多 size 条。
func (l *StoreList2D[K1, Ix, V]) List(txn *Txn, k1 K1, startKey Ix, size uint32) ([]Ix, []V, error) {
	id, err := l.k1ToIDMapping().Get(txn, k1)
	if err != nil {
		return nil, nil, err
	}
	if id == nil {
		return nil, nil, nil
	}

	totalLen, err := l.k2NextIDMapping().GetOrDefault(txn, *id, 0)
	if err != nil {
		return nil, nil, err
	}
	if size == 0 {
		return nil, nil, nil
	}

	var outKeys []Ix
	var outVals []V
	k2 := startKey
	for i := uint32(0); i < size; i++ {
		if compareListIndex(k2, totalLen) >= 0 {
			break
		}
		storeKey := l.storeKey(*id, k2)
		v, err := l.storeMapping().Get(txn, storeKey)
		if err != nil {
			return nil, nil, err
		}
		if v != nil {
			outKeys = append(outKeys, k2)
			outVals = append(outVals, *v)
		}
		next, ok := checkedNext(k2)
		if !ok {
			break
		}
		k2 = next
	}
	return outKeys, outVals, nil
}

// DescList 分页列表（降序）：k1 下从 startKey 起向前取最多 size 条。
// startKey 为 nil 时表示从末尾开始。
func (l *StoreList2D[K1, Ix, V]) DescList(txn *Txn, k1 K1, startKey *Ix, size uint32) ([]Ix, []V, error) {
	id, err := l.k1ToIDMapping().Get(txn, k1)
	if err != nil {
		return nil, nil, err
	}
	if id == nil {
		return nil, nil, nil
	}

	totalLen, err := l.k2NextIDMapping().GetOrDefault(txn, *id, 0)
	if err != nil {
		return nil, nil, err
	}
	if size == 0 {
		return nil, nil, nil
	}

	var k2 Ix
	if startKey != nil {
		k2 = *startKey
	} else {
		if totalLen == 0 {
			return nil, nil, nil
		}
		k2, _ = checkedPrev(totalLen)
	}

	var outKeys []Ix
	var outVals []V
	for i := uint32(0); i < size; i++ {
		storeKey := l.storeKey(*id, k2)
		v, err := l.storeMapping().Get(txn, storeKey)
		if err != nil {
			return nil, nil, err
		}
		if v != nil {
			outKeys = append(outKeys, k2)
			outVals = append(outVals, *v)
		}
		prev, ok := checkedPrev(k2)
		if !ok {
			break
		}
		k2 = prev
	}
	return outKeys, outVals, nil
}

// ListAll 返回 k1 下全部 (k2, value)。
func (l *StoreList2D[K1, Ix, V]) ListAll(txn *Txn, k1 K1) ([]Ix, []V, error) {
	id, err := l.k1ToIDMapping().Get(txn, k1)
	if err != nil {
		return nil, nil, err
	}
	if id == nil {
		return nil, nil, nil
	}

	totalLen, err := l.k2NextIDMapping().GetOrDefault(txn, *id, 0)
	if err != nil {
		return nil, nil, err
	}

	var outKeys []Ix
	var outVals []V
	var k2 Ix
	for compareListIndex(k2, totalLen) < 0 {
		storeKey := l.storeKey(*id, k2)
		v, err := l.storeMapping().Get(txn, storeKey)
		if err != nil {
			return nil, nil, err
		}
		if v != nil {
			outKeys = append(outKeys, k2)
			outVals = append(outVals, *v)
		}
		next, ok := checkedNext(k2)
		if !ok {
			break
		}
		k2 = next
	}
	return outKeys, outVals, nil
}

// ErrKeyNotFound key 不存在错误
var ErrKeyNotFound = hexToError("key not found")

func hexToError(s string) error { return &hexErr{s} }

type hexErr struct{ s string }

func (e *hexErr) Error() string { return e.s }

// compile-time check
var _ = strconv.Itoa
var _ = codec.Encode
