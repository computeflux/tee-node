// package store: StoreIndex 提供通用二级倒排索引。
//
// 设计目标：
//   1. 零额外依赖 — 基于 Txn prefix 扫描实现
//   2. 任何合约可复用 — 只需声明索引字段即可
//   3. 支持精确匹配、范围查询
//
// 索引 key 格式：{namespace}_{prefix}{fieldValue}_{id}
// 例如：gateway_idx_IsActive_1_42 表示 IsActive=1 的 model_id=42
//
// 查询时通过 prefix 扫描直接获取所有匹配的 id，无需反序列化 ID 列表。

package store

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/cockroachdb/pebble"
)

// ── Index key & ID type constraints ──────────────────────

// IdxID 索引引用的主键类型（通常是自增 ID）。
type IdxID interface {
	uint64 | uint32 | uint16 | uint8 | string | []byte
}

// ── StoreIndex ───────────────────────────────────────────

// StoreIndex[ID] 是一个基于 prefix 扫描的倒排索引。
//
//	namespace — 命名空间（如 "gateway"）
//	prefix    — 键前缀（如 "idx_IsActive_"）
type StoreIndex[ID IdxID] struct {
	namespace string
	prefix    string
}

// NewStoreIndex 创建一个新的索引。
func NewStoreIndex[ID IdxID](namespace, prefix string) *StoreIndex[ID] {
	return &StoreIndex[ID]{namespace: namespace, prefix: prefix}
}

// Add 向索引中添加一条 (fieldValue → id) 记录。
func (idx *StoreIndex[ID]) Add(txn *Txn, fieldValue string, id ID) error {
	key := idx.makeKey(fieldValue, id)
	return txn.Set([]byte(key), []byte{1})
}

// Remove 从索引中移除一条 (fieldValue → id) 记录。
func (idx *StoreIndex[ID]) Remove(txn *Txn, fieldValue string, id ID) error {
	key := idx.makeKey(fieldValue, id)
	return txn.Delete([]byte(key))
}

// GetIDs 查询单个 fieldValue 对应的所有 ID。
func (idx *StoreIndex[ID]) GetIDs(txn *Txn, fieldValue string) ([]ID, error) {
	prefix := idx.namespace + "_" + idx.prefix + fieldValue + "_"
	return idx.scanPrefix(txn, prefix)
}

// GetIDsByKeys 查询多个 fieldValue 的并集（去重）。
func (idx *StoreIndex[ID]) GetIDsByKeys(txn *Txn, fieldValues []string) ([]ID, error) {
	if len(fieldValues) == 0 {
		return nil, nil
	}
	var all []ID
	seen := make(map[string]bool)
	for _, fv := range fieldValues {
		ids, err := idx.GetIDs(txn, fv)
		if err != nil {
			return nil, err
		}
		for _, id := range ids {
			sk := idxIDToString(id)
			if !seen[sk] {
				seen[sk] = true
				all = append(all, id)
			}
		}
	}
	return all, nil
}

// GetIDsByRange 范围扫描查询：返回 [minStr, maxStr] 区间内所有 ID（包含边界）。
// fieldValue 必须是固定宽度的字典序字符串（如左填充零的数值）。
func (idx *StoreIndex[ID]) GetIDsByRange(txn *Txn, minStr, maxStr string) ([]ID, error) {
	lower := idx.namespace + "_" + idx.prefix + minStr + "_"
	upper := idx.namespace + "_" + idx.prefix + maxStr + "_"
	// upper 的 keyUpperBound 保证覆盖 maxStr 的所有后缀
	ub := keyUpperBound([]byte(upper))
	return idx.scanRange(txn, string(lower), string(ub))
}

// GetAllIDs 返回索引中所有 ID（遍历所有键）。
// 注意：这会在内部做前缀扫描，数据量大时谨慎使用。
func (idx *StoreIndex[ID]) GetAllIDs(txn *Txn) ([]ID, error) {
	prefix := idx.namespace + "_" + idx.prefix
	return idx.scanPrefix(txn, prefix)
}

// scanPrefix 扫描指定 prefix 的所有 key，解析出 ID（自动去重）。
// key 格式为 {namespace}_{prefix}{fieldValue}_{id}，suffix 可能是纯 id
// 或 fieldValue_id；统一取最后一个 "_" 后的部分作为 ID。
func (idx *StoreIndex[ID]) scanPrefix(txn *Txn, prefix string) ([]ID, error) {
	keys, _, err := txn.ListByPrefix([]byte(prefix))
	if err != nil {
		return nil, err
	}
	return idx.parseKeys(keys, len(prefix)), nil
}

// scanRange 扫描 [lower, upper) 范围内的所有 key，解析出 ID。
func (idx *StoreIndex[ID]) scanRange(txn *Txn, lower, upper string) ([]ID, error) {
	iter, err := txn.NewIter(&pebble.IterOptions{
		LowerBound: []byte(lower),
		UpperBound: []byte(upper),
	})
	if err != nil {
		return nil, err
	}
	defer iter.Close()

	prefixLen := len(lower) // lower = namespace + "_" + prefix + minStr + "_"
	var out []ID
	seen := make(map[string]bool)
	for iter.First(); iter.Valid(); iter.Next() {
		k := iter.Key()
		if len(k) <= prefixLen {
			continue
		}
		suffix := string(k[prefixLen:])
		lastIdx := strings.LastIndex(suffix, "_")
		var idStr string
		if lastIdx < 0 {
			idStr = suffix
		} else {
			idStr = suffix[lastIdx+1:]
		}
		id, err := idxParseIDFromKey[ID]([]byte(idStr))
		if err != nil {
			continue
		}
		sk := idxIDToString(id)
		if seen[sk] {
			continue
		}
		seen[sk] = true
		out = append(out, id)
	}
	return out, nil
}

// parseKeys 从 key 列表中解析 ID。
func (idx *StoreIndex[ID]) parseKeys(keys [][]byte, prefixLen int) []ID {
	var out []ID
	seen := make(map[string]bool)
	for _, k := range keys {
		if len(k) <= prefixLen {
			continue
		}
		suffix := string(k[prefixLen:])
		lastIdx := strings.LastIndex(suffix, "_")
		var idStr string
		if lastIdx < 0 {
			idStr = suffix
		} else {
			idStr = suffix[lastIdx+1:]
		}
		id, err := idxParseIDFromKey[ID]([]byte(idStr))
		if err != nil {
			continue
		}
		sk := idxIDToString(id)
		if seen[sk] {
			continue
		}
		seen[sk] = true
		out = append(out, id)
	}
	return out
}

// makeKey 构造索引存储 key。
func (idx *StoreIndex[ID]) makeKey(fieldValue string, id ID) string {
	return idx.namespace + "_" + idx.prefix + fieldValue + "_" + idxIDToString(id)
}

// ── ID serialization helpers ─────────────────────────────

func idxIDToString[ID IdxID](id ID) string {
	switch v := any(id).(type) {
	case uint64:
		return strconv.FormatUint(v, 10)
	case uint32:
		return strconv.FormatUint(uint64(v), 10)
	case uint16:
		return strconv.FormatUint(uint64(v), 10)
	case uint8:
		return strconv.FormatUint(uint64(v), 10)
	case string:
		return v
	case []byte:
		return fmt.Sprintf("%x", v)
	}
	return ""
}

func idxParseIDFromKey[ID IdxID](suffix []byte) (ID, error) {
	var zero ID
	s := string(suffix)
	switch any(zero).(type) {
	case uint64:
		v, err := strconv.ParseUint(s, 10, 64)
		if err != nil {
			return zero, err
		}
		return any(v).(ID), nil
	case uint32:
		v, err := strconv.ParseUint(s, 10, 32)
		if err != nil {
			return zero, err
		}
		return any(uint32(v)).(ID), nil
	case uint16:
		v, err := strconv.ParseUint(s, 10, 16)
		if err != nil {
			return zero, err
		}
		return any(uint16(v)).(ID), nil
	case uint8:
		v, err := strconv.ParseUint(s, 10, 8)
		if err != nil {
			return zero, err
		}
		return any(uint8(v)).(ID), nil
	case string:
		return any(s).(ID), nil
	case []byte:
		return any([]byte(s)).(ID), nil
	}
	return zero, fmt.Errorf("unsupported ID type")
}

func idxIDEqual[ID IdxID](a, b ID) bool {
	switch va := any(a).(type) {
	case uint64:
		return va == any(b).(uint64)
	case uint32:
		return va == any(b).(uint32)
	case uint16:
		return va == any(b).(uint16)
	case uint8:
		return va == any(b).(uint8)
	case string:
		return va == any(b).(string)
	case []byte:
		return string(va) == string(any(b).([]byte))
	}
	return false
}

// ── Range helpers ────────────────────────────────────────

// idxUint64Pad20 将 uint64 左填充零至 20 位，保证字典序 = 数值序。
func idxUint64Pad20(v uint64) string {
	return fmt.Sprintf("%020d", v)
}

// ── Set operations for query combining ───────────────────

// IntersectIDs 取两个 ID 列表的交集。
// 若 a 或 b 为空，返回空切片（空集与任何集的交集为空）。
func IntersectIDs[ID IdxID](a, b []ID) []ID {
	if len(a) == 0 || len(b) == 0 {
		return []ID{}
	}
	set := make(map[string]bool, len(a))
	for _, id := range a {
		set[idxIDToString(id)] = true
	}
	var out []ID
	for _, id := range b {
		if set[idxIDToString(id)] {
			out = append(out, id)
		}
	}
	return out
}
