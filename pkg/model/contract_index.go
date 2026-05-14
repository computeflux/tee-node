// Package model: StoreIndex 提供通用二级倒排索引。
//
// 设计目标：
//   1. 零额外依赖 — 基于 StoreMapping[K, []ID] 实现
//   2. 任何合约可复用 — 只需声明索引字段即可
//   3. 支持三种查询模式：精确匹配、分桶范围、前缀匹配
//
// 使用示例（Gateway 合约）：
//
//   type Gateway struct {
//       models    *StoreMapping[uint64, ModelInfo]
//       idxModal  *StoreIndex[uint8, uint64]   // modality → modelIDs
//       idxCtx    *StoreIndex[uint64, uint64]   // context bucket → modelIDs
//   }
//
//   // 写入时维护索引
//   func (g *Gateway) RegisterModel(...) {
//       g.models.Set(txn, id, info)
//       g.idxModal.Add(txn, info.Modalities, id)
//       g.idxCtx.Add(txn, Bucketize(info.ContextLen, ContextBuckets), id)
//   }
//
//   // 查询时使用索引
//   func (g *Gateway) QueryModels(filter) {
//       candidates := g.idxModal.GetIDs(txn, filter.Modality)
//       candidates = intersect(candidates, g.idxCtx.GetIDsByKeys(txn, buckets))
//       // paginate & fetch
//   }

package model

// ── Index key & ID type constraints ──────────────────────

// IdxKey 索引键类型：与 MappingKey 一致。
type IdxKey = MappingKey

// IdxID 索引引用的主键类型（通常是自增 ID）。
type IdxID interface {
	uint64 | uint32 | uint16 | uint8 | string | []byte
}

// ── StoreIndex ───────────────────────────────────────────

// StoreIndex[K, ID] 是一个倒排索引：将字段值 K 映射到一组主键 ID。
//
//	K  — 被索引的字段值（如 modality bitmask、context bucket）
//	ID — 主键类型（如 modelID uint64）
type StoreIndex[K IdxKey, ID IdxID] struct {
	inner *StoreMapping[K, []ID]
}

// NewStoreIndex 创建一个新的索引。
// namespace: 命名空间（如 "gateway"）
// prefix: 键前缀（如 "idx_mod_"、"idx_ctx_"）
func NewStoreIndex[K IdxKey, ID IdxID](namespace, prefix string) *StoreIndex[K, ID] {
	return &StoreIndex[K, ID]{
		inner: &StoreMapping[K, []ID]{
			Namespace: namespace,
			KeyPrefix: prefix,
		},
	}
}

// Add 向索引中添加一条 (key → id) 记录。
func (idx *StoreIndex[K, ID]) Add(txn *Txn, key K, id ID) error {
	ids, err := idx.inner.Get(txn, key)
	if err != nil {
		return err
	}
	if ids == nil {
		ids = &[]ID{}
	}
	*ids = append(*ids, id)
	return idx.inner.Set(txn, key, *ids)
}

// Remove 从索引中移除一条 (key → id) 记录。
func (idx *StoreIndex[K, ID]) Remove(txn *Txn, key K, id ID) error {
	ids, err := idx.inner.Get(txn, key)
	if err != nil || ids == nil {
		return err
	}
	filtered := make([]ID, 0, len(*ids))
	for _, x := range *ids {
		if !idxIDEqual(x, id) {
			filtered = append(filtered, x)
		}
	}
	if len(filtered) == 0 {
		return idx.inner.Delete(txn, key)
	}
	return idx.inner.Set(txn, key, filtered)
}

// GetIDs 查询单个键对应的所有 ID。
func (idx *StoreIndex[K, ID]) GetIDs(txn *Txn, key K) ([]ID, error) {
	ids, err := idx.inner.Get(txn, key)
	if err != nil || ids == nil {
		return nil, err
	}
	return *ids, nil
}

// GetIDsByKeys 查询多个键对应的 ID 并集（用于范围查询的分桶合并）。
func (idx *StoreIndex[K, ID]) GetIDsByKeys(txn *Txn, keys []K) ([]ID, error) {
	seen := make(map[any]bool)
	var out []ID
	for _, k := range keys {
		ids, err := idx.GetIDs(txn, k)
		if err != nil {
			return nil, err
		}
		for _, id := range ids {
			if !seen[id] {
				seen[id] = true
				out = append(out, id)
			}
		}
	}
	return out, nil
}

// GetAllIDs 返回索引中所有 ID（遍历所有键）。
// 注意：这会在内部做前缀扫描，数据量大时谨慎使用。
func (idx *StoreIndex[K, ID]) GetAllIDs(txn *Txn) ([]ID, error) {
	_, lists, err := idx.inner.List(txn)
	if err != nil {
		return nil, err
	}
	seen := make(map[any]bool)
	var out []ID
	for _, list := range lists {
		for _, id := range list {
			if !seen[id] {
				seen[id] = true
				out = append(out, id)
			}
		}
	}
	return out, nil
}

// ── ID equality helper ──────────────────────────────────

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

// ── Bucket helpers ──────────────────────────────────────

// Bucketize 将值归入最接近的桶上限。
// buckets 必须升序排列，如 {4000, 8000, 16000, 32000, 64000, 128000, 256000}。
func Bucketize(value uint64, buckets []uint64) uint64 {
	for _, b := range buckets {
		if value <= b {
			return b
		}
	}
	return buckets[len(buckets)-1]
}

// OverlappingBuckets 返回与 [min, max] 范围有重叠的所有桶。
// 若 min==0 && max==0 表示不限制范围，返回 nil。
func OverlappingBuckets(minVal, maxVal uint64, buckets []uint64) []uint64 {
	if minVal == 0 && maxVal == 0 {
		return nil
	}
	if maxVal == 0 {
		maxVal = ^uint64(0)
	}
	var out []uint64
	var prevB uint64 // 桶的下界，初始为 0
	for _, b := range buckets {
		// 桶的范围是 (prevB, b]
		if b >= minVal && prevB < maxVal {
			out = append(out, b)
		}
		prevB = b
	}
	return out
}

// ── Predefined bucket sets ───────────────────────────────

// ContextBuckets 上下文长度分桶（单位：token）。
var ContextBuckets = []uint64{
	4_000,
	8_000,
	16_000,
	32_000,
	64_000,
	128_000,
	256_000,
	512_000,
	1_000_000,
	^uint64(0), // 兜底：超过 1M 的都在这里
}

// PricingBuckets 每次提示定价分桶（单位：credit）。
var PricingBuckets = []uint64{
	1,
	5,
	10,
	50,
	100,
	500,
	1_000,
	5_000,
	10_000,
	^uint64(0),
}

// ── Set operations for query combining ───────────────────

// IntersectIDs 取两个 ID 列表的交集。
func IntersectIDs[ID IdxID](a, b []ID) []ID {
	if len(a) == 0 {
		return b
	}
	if len(b) == 0 {
		return a
	}
	set := make(map[any]bool, len(a))
	for _, id := range a {
		set[id] = true
	}
	var out []ID
	for _, id := range b {
		if set[id] {
			out = append(out, id)
		}
	}
	return out
}
