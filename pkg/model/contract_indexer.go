// Package model: IndexedStore / IndexedList 基于 struct tag 自动维护二级索引。
//
// 用法 — IndexedStore（自由 key）：
//
//	type ModelInfo struct {
//	    Name            string `json:"name"`
//	    InputModalities uint8  `idx:"exact"`
//	    ContextLength   uint64 `idx:"bucket:4000,8000,16000,32000"`
//	    PromptPricing   uint64 `idx:"bucket:1,5,10,50,100,500"`
//	}
//
//	models := model.NewIndexedStore[uint64, ModelInfo]("gateway", "model_")
//	models.Put(txn, id, info)                                  // 写 + 自动索引
//	models.Update(txn, id, func(cur *ModelInfo) { ... })       // 更新 + 自动比对索引
//	ids, _ := models.Query(txn, map[string]any{"InputModalities": uint64(3)})
//
// 用法 — IndexedList（自增 id）：
//
//	list := model.NewIndexedList[uint64, ModelInfo]("gateway", "model_id", "model_")
//	id, _ := list.Insert(txn, info)                            // 插入 + 自动分配 id + 自动索引
//	list.Update(txn, id, func(cur *ModelInfo) { ... })         // 更新 + 自动比对索引
//	keys, vals, _ := list.List(txn, 0, 10)                     // 原生分页
//	ids, _ := list.Query(txn, map[string]any{"InputModalities": uint64(3)})

package model

import (
	"fmt"
	"reflect"
	"strconv"
	"strings"
)

// ── shared index registry ────────────────────────────────

// idxRegistry 是 IndexedStore 和 IndexedList 共用的索引管理核心。
type idxRegistry[ID IdxID, V any] struct {
	exactIndexes  map[string]*StoreIndex[uint64, ID]
	bucketIndexes map[string]*bucketIdx[ID]
	elemType      reflect.Type
}

type bucketIdx[ID IdxID] struct {
	idx     *StoreIndex[uint64, ID]
	buckets []uint64
}

func newIdxRegistry[ID IdxID, V any](namespace, prefix string) *idxRegistry[ID, V] {
	var zero V
	elemType := reflect.TypeOf(zero)
	if elemType.Kind() != reflect.Struct {
		panic("IndexedStore/IndexedList: V must be a struct")
	}

	r := &idxRegistry[ID, V]{
		exactIndexes:  make(map[string]*StoreIndex[uint64, ID]),
		bucketIndexes: make(map[string]*bucketIdx[ID]),
		elemType:      elemType,
	}

	for i := 0; i < elemType.NumField(); i++ {
		f := elemType.Field(i)
		tag := f.Tag.Get("idx")
		if tag == "" {
			continue
		}

		idxPrefix := prefix + "idx_" + f.Name + "_"
		parts := strings.SplitN(tag, ":", 2)
		switch parts[0] {
		case "exact":
			r.exactIndexes[f.Name] = NewStoreIndex[uint64, ID](namespace, idxPrefix)
		case "bucket":
			buckets := parseBuckets(parts[1])
			r.bucketIndexes[f.Name] = &bucketIdx[ID]{
				idx:     NewStoreIndex[uint64, ID](namespace, idxPrefix),
				buckets: buckets,
			}
		default:
			panic(fmt.Sprintf("idx tag: unknown type %q on field %s", parts[0], f.Name))
		}
	}
	return r
}

func (r *idxRegistry[ID, V]) indexAdd(txn *Txn, id ID, val V) {
	rv := reflect.ValueOf(val)
	for name, idx := range r.exactIndexes {
		v := fieldUint64(rv, name)
		_ = idx.Add(txn, v, id)
	}
	for name, bi := range r.bucketIndexes {
		v := fieldUint64(rv, name)
		b := Bucketize(v, bi.buckets)
		_ = bi.idx.Add(txn, b, id)
	}
}

func (r *idxRegistry[ID, V]) indexRemove(txn *Txn, id ID, val V) {
	rv := reflect.ValueOf(val)
	for name, idx := range r.exactIndexes {
		v := fieldUint64(rv, name)
		_ = idx.Remove(txn, v, id)
	}
	for name, bi := range r.bucketIndexes {
		v := fieldUint64(rv, name)
		b := Bucketize(v, bi.buckets)
		_ = bi.idx.Remove(txn, b, id)
	}
}

func (r *idxRegistry[ID, V]) indexUpdate(txn *Txn, id ID, old, new V) {
	oldRV := reflect.ValueOf(old)
	newRV := reflect.ValueOf(new)
	for name, idx := range r.exactIndexes {
		oldV := fieldUint64(oldRV, name)
		newV := fieldUint64(newRV, name)
		if oldV != newV {
			_ = idx.Remove(txn, oldV, id)
			_ = idx.Add(txn, newV, id)
		}
	}
	for name, bi := range r.bucketIndexes {
		oldV := fieldUint64(oldRV, name)
		newV := fieldUint64(newRV, name)
		oldB := Bucketize(oldV, bi.buckets)
		newB := Bucketize(newV, bi.buckets)
		if oldB != newB {
			_ = bi.idx.Remove(txn, oldB, id)
			_ = bi.idx.Add(txn, newB, id)
		}
	}
}

func (r *idxRegistry[ID, V]) query(txn *Txn, filter map[string]any) ([]ID, error) {
	var candidates []ID
	for field, val := range filter {
		switch v := val.(type) {
		case uint64:
			idx, ok := r.exactIndexes[field]
			if !ok {
				return nil, fmt.Errorf("query: no exact index for field %q", field)
			}
			ids, err := idx.GetIDs(txn, v)
			if err != nil {
				return nil, err
			}
			if candidates == nil {
				candidates = ids
			} else {
				candidates = IntersectIDs(candidates, ids)
			}
		case Range:
			bi, ok := r.bucketIndexes[field]
			if !ok {
				return nil, fmt.Errorf("query: no bucket index for field %q", field)
			}
			buckets := OverlappingBuckets(v.Min, v.Max, bi.buckets)
			ids, err := bi.idx.GetIDsByKeys(txn, buckets)
			if err != nil {
				return nil, err
			}
			if candidates == nil {
				candidates = ids
			} else {
				candidates = IntersectIDs(candidates, ids)
			}
		default:
			return nil, fmt.Errorf("query: unsupported filter type %T for field %q", val, field)
		}
	}
	return candidates, nil
}

// ── IndexedStore (free-key) ──────────────────────────────

// IndexedStore[ID, V] 是对 StoreMapping 的封装。
type IndexedStore[ID IdxID, V any] struct {
	Primary *StoreMapping[ID, V]
	idx     *idxRegistry[ID, V]
}

func NewIndexedStore[ID IdxID, V any](namespace, prefix string) *IndexedStore[ID, V] {
	return &IndexedStore[ID, V]{
		Primary: &StoreMapping[ID, V]{Namespace: namespace, KeyPrefix: prefix},
		idx:     newIdxRegistry[ID, V](namespace, prefix),
	}
}

func (s *IndexedStore[ID, V]) Put(txn *Txn, id ID, val V) error {
	if err := s.Primary.Set(txn, id, val); err != nil {
		return err
	}
	s.idx.indexAdd(txn, id, val)
	return nil
}

func (s *IndexedStore[ID, V]) Update(txn *Txn, id ID, fn func(cur *V)) error {
	cur, err := s.Primary.Get(txn, id)
	if err != nil || cur == nil {
		return fmt.Errorf("IndexedStore.Update: %w", err)
	}
	old := *cur
	fn(cur)
	if err := s.Primary.Set(txn, id, *cur); err != nil {
		return err
	}
	s.idx.indexUpdate(txn, id, old, *cur)
	return nil
}

func (s *IndexedStore[ID, V]) Delete(txn *Txn, id ID) error {
	cur, err := s.Primary.Get(txn, id)
	if err != nil || cur == nil {
		return err
	}
	s.idx.indexRemove(txn, id, *cur)
	return s.Primary.Delete(txn, id)
}

func (s *IndexedStore[ID, V]) Query(txn *Txn, filter map[string]any) ([]ID, error) {
	return s.idx.query(txn, filter)
}

// ── IndexedList (auto-increment key) ─────────────────────

// IndexedList[K, V] 是对 StoreList 的封装，保持其完整 API，同时增加 tag 驱动的索引。
// K 为自增 id 类型（uint8/uint16/uint32/uint64）。
type IndexedList[K ListIndex, V any] struct {
	*StoreList[K, V]
	idx *idxRegistry[K, V]
}

// NewIndexedList 创建带索引的列表存储。
// namespace: 命名空间，如 "gateway"
// prefixNextID: next_id key，如 "next_model_id"
// prefixItems: items key 前缀，如 "model_"
func NewIndexedList[K ListIndex, V any](namespace, prefixNextID, prefixItems string) *IndexedList[K, V] {
	return &IndexedList[K, V]{
		StoreList: NewStoreList[K, V](namespace, prefixNextID, prefixItems),
		idx:       newIdxRegistry[K, V](namespace, prefixItems),
	}
}

// Insert 插入一条记录，自动分配 id 并维护索引。
func (l *IndexedList[K, V]) Insert(txn *Txn, val V) (K, error) {
	id, err := l.StoreList.Insert(txn, val)
	if err != nil {
		var zero K
		return zero, err
	}
	l.idx.indexAdd(txn, id, val)
	return id, nil
}

// UpdateFn 读取当前值，调用 fn 修改，写回并自动比对更新索引。
func (l *IndexedList[K, V]) UpdateFn(txn *Txn, key K, fn func(cur *V)) error {
	cur, err := l.StoreList.Get(txn, key)
	if err != nil || cur == nil {
		return fmt.Errorf("IndexedList.UpdateFn: %w", err)
	}
	old := *cur
	fn(cur)
	if err := l.StoreList.Update(txn, key, *cur); err != nil {
		return err
	}
	l.idx.indexUpdate(txn, key, old, *cur)
	return nil
}

// Clear 清除记录并清理索引。
func (l *IndexedList[K, V]) Clear(txn *Txn, key K) error {
	cur, err := l.StoreList.Get(txn, key)
	if err != nil || cur == nil {
		return err
	}
	l.idx.indexRemove(txn, key, *cur)
	return l.StoreList.Clear(txn, key)
}

// Query 组合索引查询，返回匹配的 key 列表。
func (l *IndexedList[K, V]) Query(txn *Txn, filter map[string]any) ([]K, error) {
	return l.idx.query(txn, filter)
}

// ── helpers ──────────────────────────────────────────────

func fieldUint64(rv reflect.Value, name string) uint64 {
	f := rv.FieldByName(name)
	switch f.Kind() {
	case reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return f.Uint()
	default:
		panic(fmt.Sprintf("idx tag: field %q is not a uint type", name))
	}
}

func parseBuckets(s string) []uint64 {
	parts := strings.Split(s, ",")
	var out []uint64
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		v, err := strconv.ParseUint(p, 10, 64)
		if err != nil {
			panic(fmt.Sprintf("idx tag: invalid bucket value %q", p))
		}
		out = append(out, v)
	}
	return out
}

// ── Query types ──────────────────────────────────────────

// Range 表示一个范围查询条件 [Min, Max]。
// Min=0 表示不限制下限，Max=0 表示不限制上限。
type Range struct {
	Min uint64
	Max uint64
}
