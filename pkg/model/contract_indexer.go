// Package model: IndexedStore / IndexedList 基于 struct tag 自动维护二级索引。
//
// 用法 — IndexedStore（自由 key）：
//
//	type ModelInfo struct {
//	    Name            string `json:"name"`
//	    InputModalities uint8  `idx:"exact"`
//	    ContextLength   uint64 `idx:"range"`
//	    PromptPricing   uint64 `idx:"range"`
//	}
//
//	models := model.NewIndexedStore[uint64, ModelInfo]("gateway", "model_")
//	models.Put(txn, id, info)                                  // 写 + 自动索引
//	models.Update(txn, id, func(cur *ModelInfo) { ... })       // 更新 + 自动比对索引
//	ids, _ := models.Query(txn, QueryCond{Field: "InputModalities", Val: uint64(3)})
//
// 用法 — IndexedList（自增 id）：
//
//	list := model.NewIndexedList[uint64, ModelInfo]("gateway", "model_id", "model_")
//	id, _ := list.Insert(txn, info)                            // 插入 + 自动分配 id + 自动索引
//	list.Update(txn, id, func(cur *ModelInfo) { ... })         // 更新 + 自动比对索引
//	keys, vals, _ := list.List(txn, 0, 10)                     // 原生分页
//	ids, _ := list.Query(txn, QueryCond{Field: "InputModalities", Val: uint64(3)})

package model

import (
	"fmt"
	"reflect"
	"sort"
	"strconv"
	"strings"
)

// ── shared index registry ────────────────────────────────

// idxRegistry 是 IndexedStore 和 IndexedList 共用的索引管理核心。
type idxRegistry[ID IdxID, V any] struct {
	exactIndexes map[string]*StoreIndex[ID]
	strIndexes   map[string]*StoreIndex[ID]
	rangeIndexes map[string]*StoreIndex[ID]
	elemType     reflect.Type
}

func newIdxRegistry[ID IdxID, V any](namespace, prefix string) *idxRegistry[ID, V] {
	var zero V
	elemType := reflect.TypeOf(zero)
	if elemType.Kind() != reflect.Struct {
		panic("IndexedStore/IndexedList: V must be a struct")
	}

	r := &idxRegistry[ID, V]{
		exactIndexes: make(map[string]*StoreIndex[ID]),
		strIndexes:   make(map[string]*StoreIndex[ID]),
		rangeIndexes: make(map[string]*StoreIndex[ID]),
		elemType:     elemType,
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
			switch f.Type.Kind() {
			case reflect.String:
				r.strIndexes[f.Name] = NewStoreIndex[ID](namespace, idxPrefix)
			case reflect.Bool:
				fallthrough
			case reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
				r.exactIndexes[f.Name] = NewStoreIndex[ID](namespace, idxPrefix)
			default:
				panic(fmt.Sprintf("idx tag exact: unsupported field type %s on %s", f.Type.Kind(), f.Name))
			}
		case "range":
			switch f.Type.Kind() {
			case reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
				r.rangeIndexes[f.Name] = NewStoreIndex[ID](namespace, idxPrefix)
			default:
				panic(fmt.Sprintf("idx tag range: unsupported field type %s on %s", f.Type.Kind(), f.Name))
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
		_ = idx.Add(txn, idxUint64Str(v), id)
	}
	for name, idx := range r.strIndexes {
		v := fieldString(rv, name)
		_ = idx.Add(txn, v, id)
	}
	for name, idx := range r.rangeIndexes {
		v := fieldUint64(rv, name)
		_ = idx.Add(txn, idxUint64Pad20(v), id)
	}
}

func (r *idxRegistry[ID, V]) indexRemove(txn *Txn, id ID, val V) {
	rv := reflect.ValueOf(val)
	for name, idx := range r.exactIndexes {
		v := fieldUint64(rv, name)
		_ = idx.Remove(txn, idxUint64Str(v), id)
	}
	for name, idx := range r.strIndexes {
		v := fieldString(rv, name)
		_ = idx.Remove(txn, v, id)
	}
	for name, idx := range r.rangeIndexes {
		v := fieldUint64(rv, name)
		_ = idx.Remove(txn, idxUint64Pad20(v), id)
	}
}

func (r *idxRegistry[ID, V]) indexUpdate(txn *Txn, id ID, old, new V) {
	oldRV := reflect.ValueOf(old)
	newRV := reflect.ValueOf(new)
	for name, idx := range r.exactIndexes {
		oldV := fieldUint64(oldRV, name)
		newV := fieldUint64(newRV, name)
		if oldV != newV {
			_ = idx.Remove(txn, idxUint64Str(oldV), id)
			_ = idx.Add(txn, idxUint64Str(newV), id)
		}
	}
	for name, idx := range r.strIndexes {
		oldV := fieldString(oldRV, name)
		newV := fieldString(newRV, name)
		if oldV != newV {
			_ = idx.Remove(txn, oldV, id)
			_ = idx.Add(txn, newV, id)
		}
	}
	for name, idx := range r.rangeIndexes {
		oldV := fieldUint64(oldRV, name)
		newV := fieldUint64(newRV, name)
		if oldV != newV {
			_ = idx.Remove(txn, idxUint64Pad20(oldV), id)
			_ = idx.Add(txn, idxUint64Pad20(newV), id)
		}
	}
}

func (r *idxRegistry[ID, V]) query(txn *Txn, expr QueryExpr) ([]ID, error) {
	switch e := expr.(type) {
	case QueryCond:
		return r.queryCond(txn, e)
	case QueryAND:
		var candidates []ID
		for _, child := range e {
			ids, err := r.query(txn, child)
			if err != nil {
				return nil, err
			}
			candidates = mergeIDs(candidates, ids)
		}
		return candidates, nil
	case QueryOR:
		var all []ID
		seen := make(map[string]bool)
		for _, child := range e {
			ids, err := r.query(txn, child)
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
	default:
		return nil, fmt.Errorf("query: unsupported expression type %T", expr)
	}
}

func (r *idxRegistry[ID, V]) queryCond(txn *Txn, cond QueryCond) ([]ID, error) {
	field := cond.Field
	val := cond.Val
	switch v := val.(type) {
	case uint64:
		idx, ok := r.exactIndexes[field]
		if ok {
			return idx.GetIDs(txn, idxUint64Str(v))
		}
		// 也可能是 range 索引上的 exact 查询（如 IsActive 实际是 bool）
		idxRange, ok := r.rangeIndexes[field]
		if ok {
			return idxRange.GetIDs(txn, idxUint64Pad20(v))
		}
		return nil, fmt.Errorf("query: no exact/range index for field %q", field)
	case string:
		idx, ok := r.strIndexes[field]
		if !ok {
			return nil, fmt.Errorf("query: no string index for field %q", field)
		}
		return idx.GetIDs(txn, v)
	case Range:
		idx, ok := r.rangeIndexes[field]
		if !ok {
			return nil, fmt.Errorf("query: no range index for field %q", field)
		}
		minVal := v.Min
		maxVal := v.Max
		// Max=0 且 Min>0 表示只有下界（无上限）
		if minVal != 0 && maxVal == 0 {
			maxVal = ^uint64(0)
		}
		minStr := idxUint64Pad20(minVal)
		maxStr := idxUint64Pad20(maxVal)
		return idx.GetIDsByRange(txn, minStr, maxStr)
	default:
		return nil, fmt.Errorf("query: unsupported filter type %T for field %q", val, field)
	}
}

// mergeIDs 将 ids 与 candidates 取交集（若 candidates 为 nil 则直接返回 ids）。
func mergeIDs[ID IdxID](candidates, ids []ID) []ID {
	if candidates == nil {
		return ids
	}
	return IntersectIDs(candidates, ids)
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

func (s *IndexedStore[ID, V]) Query(txn *Txn, expr QueryExpr) ([]ID, error) {
	return s.idx.query(txn, expr)
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

// Update 直接更新值，并自动比对更新索引。
func (l *IndexedList[K, V]) Update(txn *Txn, key K, val V) error {
	cur, err := l.StoreList.Get(txn, key)
	if err != nil || cur == nil {
		return fmt.Errorf("IndexedList.Update: %w", err)
	}
	old := *cur
	if err := l.StoreList.Update(txn, key, val); err != nil {
		return err
	}
	l.idx.indexUpdate(txn, key, old, val)
	return nil
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

// RebuildIndex 遍历所有已有数据并重建索引（用于新增 idx tag 后的历史数据补偿）。
func (l *IndexedList[K, V]) RebuildIndex(txn *Txn) error {
	keys, vals, err := l.StoreList.ListAll(txn)
	if err != nil {
		return err
	}
	for i, key := range keys {
		l.idx.indexAdd(txn, key, vals[i])
	}
	return nil
}

// Query 组合索引查询，返回匹配的 key 列表。
func (l *IndexedList[K, V]) Query(txn *Txn, expr QueryExpr) ([]K, error) {
	return l.idx.query(txn, expr)
}

// ListByQuery 分页列表（升序）：先用 QueryExpr 过滤出匹配的 id，再按 key 升序从 startKey 起取最多 size 条。
func (l *IndexedList[K, V]) ListByQuery(txn *Txn, expr QueryExpr, startKey K, size uint32) ([]K, []V, error) {
	if size == 0 {
		return nil, nil, nil
	}
	ids, err := l.idx.query(txn, expr)
	if err != nil {
		return nil, nil, err
	}
	sort.Slice(ids, func(i, j int) bool {
		return compareListIndex(ids[i], ids[j]) < 0
	})
	var outKeys []K
	var outVals []V
	for _, id := range ids {
		if compareListIndex(id, startKey) < 0 {
			continue
		}
		if len(outKeys) >= int(size) {
			break
		}
		v, err := l.StoreList.Get(txn, id)
		if err != nil {
			return nil, nil, err
		}
		if v != nil {
			outKeys = append(outKeys, id)
			outVals = append(outVals, *v)
		}
	}
	return outKeys, outVals, nil
}

// DescListByQuery 分页列表（降序）：先用 QueryExpr 过滤出匹配的 id，再按 key 降序从 startKey 向前取最多 size 条。
// startKey 为 nil 时表示从最大 id 开始。
func (l *IndexedList[K, V]) DescListByQuery(txn *Txn, expr QueryExpr, startKey *K, size uint32) ([]K, []V, error) {
	if size == 0 {
		return nil, nil, nil
	}
	ids, err := l.idx.query(txn, expr)
	if err != nil {
		return nil, nil, err
	}
	sort.Slice(ids, func(i, j int) bool {
		return compareListIndex(ids[i], ids[j]) > 0
	})
	var outKeys []K
	var outVals []V
	for _, id := range ids {
		if startKey != nil && compareListIndex(id, *startKey) > 0 {
			continue
		}
		if len(outKeys) >= int(size) {
			break
		}
		v, err := l.StoreList.Get(txn, id)
		if err != nil {
			return nil, nil, err
		}
		if v != nil {
			outKeys = append(outKeys, id)
			outVals = append(outVals, *v)
		}
	}
	return outKeys, outVals, nil
}

// ── helpers ──────────────────────────────────────────────

func fieldUint64(rv reflect.Value, name string) uint64 {
	f := rv.FieldByName(name)
	switch f.Kind() {
	case reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return f.Uint()
	case reflect.Bool:
		if f.Bool() {
			return 1
		}
		return 0
	default:
		panic(fmt.Sprintf("idx tag: field %q is not a uint type", name))
	}
}

func fieldString(rv reflect.Value, name string) string {
	f := rv.FieldByName(name)
	switch f.Kind() {
	case reflect.String:
		return f.String()
	default:
		panic(fmt.Sprintf("idx tag: field %q is not a string type", name))
	}
}

func idxUint64Str(v uint64) string {
	return strconv.FormatUint(v, 10)
}

// ── Query types ──────────────────────────────────────────

// Range 表示一个范围查询条件 [Min, Max]。
// Min=0 表示不限制下限，Max=0 表示不限制上限。
type Range struct {
	Min uint64
	Max uint64
}

// QueryExpr 是索引查询表达式接口。
type QueryExpr interface {
	isQueryExpr()
}

// QueryCond 单个字段条件。
type QueryCond struct {
	Field string
	Val   any // uint64 | string | Range
}

func (QueryCond) isQueryExpr() {}

// QueryAND 多个表达式的逻辑与（交集）。
type QueryAND []QueryExpr

func (QueryAND) isQueryExpr() {}

// QueryOR 多个表达式的逻辑或（并集）。
type QueryOR []QueryExpr

func (QueryOR) isQueryExpr() {}
