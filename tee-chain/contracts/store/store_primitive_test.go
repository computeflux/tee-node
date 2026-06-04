package store

import (
	"encoding/binary"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// memStoreTx is an in-memory StoreTx for unit testing.
type memStoreTx struct{ data map[string][]byte }

func newMemStoreTx() *memStoreTx { return &memStoreTx{data: make(map[string][]byte)} }

// Test structs for indexed storage
type testModel struct {
	Name       string `json:"name" idx:"exact"`
	ProviderID uint64 `json:"providerId" idx:"exact"`
	IsActive   bool   `json:"isActive" idx:"exact"`
}

func (m *testModel) Encode() ([]byte, error) {
	b := EncodeCompact(nil, uint64(len(m.Name)))
	b = append(b, []byte(m.Name)...)
	var tmp [8]byte
	binary.LittleEndian.PutUint64(tmp[:], m.ProviderID)
	b = append(b, tmp[:]...)
	if m.IsActive {
		b = append(b, 1)
	} else {
		b = append(b, 0)
	}
	return b, nil
}

func (m *testModel) Decode(data []byte) (int, error) {
	off := 0
	nameLen, sz := DecodeCompact(data[off:])
	off += sz
	m.Name = string(data[off : off+int(nameLen)])
	off += int(nameLen)
	m.ProviderID = binary.LittleEndian.Uint64(data[off:])
	off += 8
	m.IsActive = data[off] != 0
	off++
	return off, nil
}

type testProvider struct {
	Name     string `json:"name" idx:"exact"`
	IsActive bool   `json:"isActive" idx:"exact"`
}

func (p *testProvider) Encode() ([]byte, error) {
	b := EncodeCompact(nil, uint64(len(p.Name)))
	b = append(b, []byte(p.Name)...)
	if p.IsActive {
		b = append(b, 1)
	} else {
		b = append(b, 0)
	}
	return b, nil
}

func (p *testProvider) Decode(data []byte) (int, error) {
	off := 0
	nameLen, sz := DecodeCompact(data[off:])
	off += sz
	p.Name = string(data[off : off+int(nameLen)])
	off += int(nameLen)
	p.IsActive = data[off] != 0
	off++
	return off, nil
}

func (m *memStoreTx) Get(key []byte) ([]byte, error)     { return m.data[string(key)], nil }
func (m *memStoreTx) Set(key, value []byte) error        { m.data[string(key)] = value; return nil }
func (m *memStoreTx) Delete(key []byte) error            { delete(m.data, string(key)); return nil }
func (m *memStoreTx) Commit() error                      { return nil }
func (m *memStoreTx) Rollback() error                    { return nil }
func (m *memStoreTx) DeletekeysByPrefix(prefix []byte) error {
	for k := range m.data {
		if strings.HasPrefix(k, string(prefix)) {
			delete(m.data, k)
		}
	}
	return nil
}
func (m *memStoreTx) ListByPrefix(prefix []byte) ([][]byte, [][]byte, error) {
	var keys, vals [][]byte
	for k, v := range m.data {
		if strings.HasPrefix(k, string(prefix)) {
			keys = append(keys, []byte(k))
			vals = append(vals, v)
		}
	}
	return keys, vals, nil
}

// ── StoreValue ───────────────────────────────────────────

func TestStoreValue_GetSetDelete(t *testing.T) {
	txn := newMemStoreTx()
	sv := &StoreValue[uint64]{Namespace: "test", Key: "counter"}

	// Get non-existent
	v, err := sv.Get(txn)
	require.NoError(t, err)
	assert.Nil(t, v)

	// Set and Get
	require.NoError(t, sv.Set(txn, uint64(42)))
	v, err = sv.Get(txn)
	require.NoError(t, err)
	require.NotNil(t, v)
	assert.Equal(t, uint64(42), *v)

	// Exists
	exists, err := sv.Exists(txn)
	require.NoError(t, err)
	assert.True(t, exists)

	// Delete
	require.NoError(t, sv.Delete(txn))
	v, err = sv.Get(txn)
	require.NoError(t, err)
	assert.Nil(t, v)
}

func TestStoreValue_GetOrDefault(t *testing.T) {
	txn := newMemStoreTx()
	sv := &StoreValue[string]{Namespace: "test", Key: "name"}

	v, err := sv.GetOrDefault(txn, "default")
	require.NoError(t, err)
	assert.Equal(t, "default", v)

	require.NoError(t, sv.Set(txn, "hello"))
	v, err = sv.GetOrDefault(txn, "default")
	require.NoError(t, err)
	assert.Equal(t, "hello", v)
}

// ── StoreMapping ─────────────────────────────────────────

func TestStoreMapping_StringKey(t *testing.T) {
	txn := newMemStoreTx()
	m := &StoreMapping[string, uint64]{Namespace: "test", KeyPrefix: "map_"}

	require.NoError(t, m.Set(txn, "alice", uint64(100)))
	require.NoError(t, m.Set(txn, "bob", uint64(200)))

	v, err := m.Get(txn, "alice")
	require.NoError(t, err)
	assert.Equal(t, uint64(100), *v)

	keys, vals, err := m.List(txn)
	require.NoError(t, err)
	assert.Len(t, keys, 2)
	assert.Len(t, vals, 2)
}

func TestStoreMapping_BytesKey(t *testing.T) {
	txn := newMemStoreTx()
	m := &StoreMapping[[]byte, string]{Namespace: "test", KeyPrefix: "bmap_"}

	key := []byte{0x01, 0x02, 0x03}
	require.NoError(t, m.Set(txn, key, "value"))

	v, err := m.Get(txn, key)
	require.NoError(t, err)
	assert.Equal(t, "value", *v)

	require.NoError(t, m.Delete(txn, key))
	v, err = m.Get(txn, key)
	require.NoError(t, err)
	assert.Nil(t, v)
}

func TestStoreMapping_Uint64Key(t *testing.T) {
	txn := newMemStoreTx()
	m := &StoreMapping[uint64, string]{Namespace: "test", KeyPrefix: "u64_"}

	require.NoError(t, m.Set(txn, uint64(123), "item123"))

	v, err := m.Get(txn, uint64(123))
	require.NoError(t, err)
	assert.Equal(t, "item123", *v)
}

func TestStoreMapping_Uint8Key(t *testing.T) {
	txn := newMemStoreTx()
	m := &StoreMapping[uint8, string]{Namespace: "test", KeyPrefix: "u8_"}

	require.NoError(t, m.Set(txn, uint8(255), "max"))

	v, err := m.Get(txn, uint8(255))
	require.NoError(t, err)
	assert.Equal(t, "max", *v)
}

func TestStoreMapping_UniAddrKey(t *testing.T) {
	txn := newMemStoreTx()
	m := &StoreMapping[UniAddr, uint64]{Namespace: "test", KeyPrefix: "addr_"}

	addr := UniAddr{T: 1, V: []byte{0xAA, 0xBB}}
	require.NoError(t, m.Set(txn, addr, uint64(1000)))

	v, err := m.Get(txn, addr)
	require.NoError(t, err)
	assert.Equal(t, uint64(1000), *v)

	contains, err := m.Contains(txn, addr)
	require.NoError(t, err)
	assert.True(t, contains)
}

func TestStoreMapping_DeleteByPrefix(t *testing.T) {
	txn := newMemStoreTx()
	m := &StoreMapping[string, uint64]{Namespace: "test", KeyPrefix: "del_"}

	require.NoError(t, m.Set(txn, "a", uint64(1)))
	require.NoError(t, m.Set(txn, "b", uint64(2)))

	require.NoError(t, m.DeleteByPrefix(txn, ""))

	keys, _, err := m.List(txn)
	require.NoError(t, err)
	assert.Len(t, keys, 0)
}

// ── StoreList ────────────────────────────────────────────

func TestStoreList_InsertAndGet(t *testing.T) {
	txn := newMemStoreTx()
	l := NewStoreList[uint64, string]("test", "next_id", "items_")

	id, err := l.Insert(txn, "first")
	require.NoError(t, err)
	assert.Equal(t, uint64(0), id)

	id2, err := l.Insert(txn, "second")
	require.NoError(t, err)
	assert.Equal(t, uint64(1), id2)

	v, err := l.Get(txn, 0)
	require.NoError(t, err)
	assert.Equal(t, "first", *v)

	v2, err := l.Get(txn, 1)
	require.NoError(t, err)
	assert.Equal(t, "second", *v2)
}

func TestStoreList_Len(t *testing.T) {
	txn := newMemStoreTx()
	l := NewStoreList[uint64, string]("test", "next_id", "items_")

	len0, err := l.Len(txn)
	require.NoError(t, err)
	assert.Equal(t, uint64(0), len0)

	l.Insert(txn, "a")
	l.Insert(txn, "b")

	len2, err := l.Len(txn)
	require.NoError(t, err)
	assert.Equal(t, uint64(2), len2)
}

func TestStoreList_UpdateAndClear(t *testing.T) {
	txn := newMemStoreTx()
	l := NewStoreList[uint64, string]("test", "next_id", "items_")

	l.Insert(txn, "old")
	require.NoError(t, l.Update(txn, 0, "new"))

	v, err := l.Get(txn, 0)
	require.NoError(t, err)
	assert.Equal(t, "new", *v)

	require.NoError(t, l.Clear(txn, 0))
	v, err = l.Get(txn, 0)
	require.NoError(t, err)
	assert.Nil(t, v)
}

func TestStoreList_ListPagination(t *testing.T) {
	txn := newMemStoreTx()
	l := NewStoreList[uint64, string]("test", "next_id", "items_")

	for i := 0; i < 5; i++ {
		l.Insert(txn, string(rune('a'+i)))
	}

	keys, vals, err := l.List(txn, 1, 2)
	require.NoError(t, err)
	assert.Equal(t, []uint64{1, 2}, keys)
	assert.Equal(t, []string{"b", "c"}, vals)
}

func TestStoreList_DescList(t *testing.T) {
	txn := newMemStoreTx()
	l := NewStoreList[uint64, string]("test", "next_id", "items_")

	for i := 0; i < 5; i++ {
		l.Insert(txn, string(rune('a'+i)))
	}

	keys, vals, err := l.DescList(txn, nil, 3)
	require.NoError(t, err)
	assert.Equal(t, []uint64{4, 3, 2}, keys)
	assert.Equal(t, []string{"e", "d", "c"}, vals)
}

func TestStoreList_Overflow(t *testing.T) {
	txn := newMemStoreTx()
	l := NewStoreList[uint8, string]("test", "next_id", "items_")

	// Fill to max uint8
	for i := 0; i < 256; i++ {
		_, err := l.Insert(txn, "x")
		require.NoError(t, err)
	}

	// Next insert should overflow
	id, err := l.Insert(txn, "overflow")
	require.NoError(t, err)
	assert.Equal(t, uint8(0), id)
}

// ── StoreIndex ───────────────────────────────────────────

func TestStoreIndex_AddRemoveGetIDs(t *testing.T) {
	txn := newMemStoreTx()
	idx := NewStoreIndex[uint64]("test", "idx_active_")

	require.NoError(t, idx.Add(txn, "1", uint64(10)))
	require.NoError(t, idx.Add(txn, "1", uint64(20)))
	require.NoError(t, idx.Add(txn, "0", uint64(30)))

	ids, err := idx.GetIDs(txn, "1")
	require.NoError(t, err)
	assert.ElementsMatch(t, []uint64{10, 20}, ids)

	require.NoError(t, idx.Remove(txn, "1", uint64(10)))
	ids, err = idx.GetIDs(txn, "1")
	require.NoError(t, err)
	assert.ElementsMatch(t, []uint64{20}, ids)
}

func TestStoreIndex_GetIDsByRange(t *testing.T) {
	txn := newMemStoreTx()
	idx := NewStoreIndex[uint64]("test", "idx_price_")

	require.NoError(t, idx.Add(txn, idxUint64Pad20(100), uint64(1)))
	require.NoError(t, idx.Add(txn, idxUint64Pad20(200), uint64(2)))
	require.NoError(t, idx.Add(txn, idxUint64Pad20(300), uint64(3)))

	ids, err := idx.GetIDsByRange(txn, idxUint64Pad20(150), idxUint64Pad20(350))
	require.NoError(t, err)
	assert.ElementsMatch(t, []uint64{2, 3}, ids)
}

// ── IndexedStore ─────────────────────────────────────────

func TestIndexedStore_PutUpdateDeleteQuery(t *testing.T) {
	txn := newMemStoreTx()
	s := NewIndexedStore[string, testModel]("test", "model_")

	info := testModel{Name: "gpt-4", ProviderID: 1, IsActive: true}
	require.NoError(t, s.Put(txn, "m1", info))

	// Query by exact index
	ids, err := s.Query(txn, QueryCond{Field: "Name", Val: "gpt-4"})
	require.NoError(t, err)
	assert.ElementsMatch(t, []string{"m1"}, ids)

	// Update
	require.NoError(t, s.Update(txn, "m1", func(cur *testModel) {
		cur.Name = "gpt-4o"
	}))

	// Old index removed
	ids, err = s.Query(txn, QueryCond{Field: "Name", Val: "gpt-4"})
	require.NoError(t, err)
	assert.Len(t, ids, 0)

	// New index added
	ids, err = s.Query(txn, QueryCond{Field: "Name", Val: "gpt-4o"})
	require.NoError(t, err)
	assert.ElementsMatch(t, []string{"m1"}, ids)

	// Delete
	require.NoError(t, s.Delete(txn, "m1"))
	ids, err = s.Query(txn, QueryCond{Field: "Name", Val: "gpt-4o"})
	require.NoError(t, err)
	assert.Len(t, ids, 0)
}

func TestIndexedStore_QueryAND(t *testing.T) {
	txn := newMemStoreTx()
	s := NewIndexedStore[string, testModel]("test", "model_")

	s.Put(txn, "m1", testModel{Name: "gpt-4", ProviderID: 1, IsActive: true})
	s.Put(txn, "m2", testModel{Name: "gpt-4", ProviderID: 2, IsActive: true})
	s.Put(txn, "m3", testModel{Name: "claude", ProviderID: 1, IsActive: true})

	ids, err := s.Query(txn, QueryAND{
		QueryCond{Field: "Name", Val: "gpt-4"},
		QueryCond{Field: "ProviderID", Val: uint64(1)},
	})
	require.NoError(t, err)
	assert.ElementsMatch(t, []string{"m1"}, ids)
}

// ── IndexedList ──────────────────────────────────────────


func TestIndexedList_InsertUpdateQuery(t *testing.T) {
	txn := newMemStoreTx()
	l := NewIndexedList[uint64, testProvider]("test", "next_prov", "prov_")

	id, err := l.Insert(txn, testProvider{Name: "OpenAI", IsActive: true})
	require.NoError(t, err)
	assert.Equal(t, uint64(0), id)

	id2, err := l.Insert(txn, testProvider{Name: "Claude", IsActive: true})
	require.NoError(t, err)
	assert.Equal(t, uint64(1), id2)

	// Query by string index
	ids, err := l.Query(txn, QueryCond{Field: "Name", Val: "OpenAI"})
	require.NoError(t, err)
	assert.ElementsMatch(t, []uint64{0}, ids)

	// Query by bool index (exact)
	ids, err = l.Query(txn, QueryCond{Field: "IsActive", Val: uint64(1)})
	require.NoError(t, err)
	assert.ElementsMatch(t, []uint64{0, 1}, ids)

	// UpdateFn
	require.NoError(t, l.UpdateFn(txn, 0, func(p *testProvider) {
		p.IsActive = false
	}))

	ids, err = l.Query(txn, QueryCond{Field: "IsActive", Val: uint64(1)})
	require.NoError(t, err)
	assert.ElementsMatch(t, []uint64{1}, ids)
}

func TestIndexedList_ListByQuery(t *testing.T) {
	txn := newMemStoreTx()
	l := NewIndexedList[uint64, testProvider]("test", "next_prov", "prov_")

	l.Insert(txn, testProvider{Name: "A", IsActive: true})
	l.Insert(txn, testProvider{Name: "B", IsActive: false})
	l.Insert(txn, testProvider{Name: "C", IsActive: true})

	keys, vals, err := l.ListByQuery(txn, QueryCond{Field: "IsActive", Val: uint64(1)}, 0, 10)
	require.NoError(t, err)
	assert.Len(t, keys, 2)
	assert.True(t, vals[0].IsActive)
	assert.True(t, vals[1].IsActive)
}

func TestIndexedList_DescListByQuery(t *testing.T) {
	txn := newMemStoreTx()
	l := NewIndexedList[uint64, testProvider]("test", "next_prov", "prov_")

	l.Insert(txn, testProvider{Name: "A", IsActive: true})
	l.Insert(txn, testProvider{Name: "B", IsActive: true})
	l.Insert(txn, testProvider{Name: "C", IsActive: true})

	keys, vals, err := l.DescListByQuery(txn, QueryCond{Field: "IsActive", Val: uint64(1)}, nil, 2)
	require.NoError(t, err)
	assert.Len(t, keys, 2)
	assert.Equal(t, uint64(2), keys[0])
	assert.Equal(t, "C", vals[0].Name)
}

func TestIndexedList_ClearAndDelete(t *testing.T) {
	txn := newMemStoreTx()
	l := NewIndexedList[uint64, testProvider]("test", "next_prov", "prov_")

	l.Insert(txn, testProvider{Name: "A", IsActive: true})

	require.NoError(t, l.Clear(txn, 0))
	v, err := l.Get(txn, 0)
	require.NoError(t, err)
	assert.Nil(t, v)

	// Index should be removed
	ids, err := l.Query(txn, QueryCond{Field: "Name", Val: "A"})
	require.NoError(t, err)
	assert.Len(t, ids, 0)
}

// ── StoreList2D ──────────────────────────────────────────

func TestStoreList2D_InsertAndGet(t *testing.T) {
	txn := newMemStoreTx()
	l := NewStoreList2D[string, uint32, string]("test", "k1id_", "k1len_", "k2next_", "store_")

	ix, err := l.Insert(txn, "user1", "record-a")
	require.NoError(t, err)
	assert.Equal(t, uint32(0), ix)

	ix2, err := l.Insert(txn, "user1", "record-b")
	require.NoError(t, err)
	assert.Equal(t, uint32(1), ix2)

	ix3, err := l.Insert(txn, "user2", "record-x")
	require.NoError(t, err)
	assert.Equal(t, uint32(0), ix3)

	v, err := l.Get(txn, "user1", 0)
	require.NoError(t, err)
	assert.Equal(t, "record-a", *v)

	v2, err := l.Get(txn, "user2", 0)
	require.NoError(t, err)
	assert.Equal(t, "record-x", *v2)
}

func TestStoreList2D_ListAndDescList(t *testing.T) {
	txn := newMemStoreTx()
	l := NewStoreList2D[string, uint32, string]("test", "k1id_", "k1len_", "k2next_", "store_")

	l.Insert(txn, "user1", "a")
	l.Insert(txn, "user1", "b")
	l.Insert(txn, "user1", "c")

	keys, vals, err := l.List(txn, "user1", 0, 10)
	require.NoError(t, err)
	assert.Equal(t, []uint32{0, 1, 2}, keys)
	assert.Equal(t, []string{"a", "b", "c"}, vals)

	keys, vals, err = l.DescList(txn, "user1", nil, 2)
	require.NoError(t, err)
	assert.Equal(t, []uint32{2, 1}, keys)
	assert.Equal(t, []string{"c", "b"}, vals)
}

func TestStoreList2D_UpdateAndClear(t *testing.T) {
	txn := newMemStoreTx()
	l := NewStoreList2D[string, uint32, string]("test", "k1id_", "k1len_", "k2next_", "store_")

	l.Insert(txn, "user1", "old")
	require.NoError(t, l.Update(txn, "user1", 0, "new"))

	v, err := l.Get(txn, "user1", 0)
	require.NoError(t, err)
	assert.Equal(t, "new", *v)

	require.NoError(t, l.Clear(txn, "user1", 0))
	v, err = l.Get(txn, "user1", 0)
	require.NoError(t, err)
	assert.Nil(t, v)
}

func TestStoreList2D_KeyEncodeDecode(t *testing.T) {
	// uint8
	require.Equal(t, string([]byte{5, 7}), list2DKeyEncode(uint8(5), uint8(7)))
	a, b := list2DKeyDecode[uint8](string([]byte{5, 7}))
	assert.Equal(t, uint8(5), a)
	assert.Equal(t, uint8(7), b)

	// uint16
	require.Equal(t, string([]byte{0, 1, 0, 2}), list2DKeyEncode(uint16(1), uint16(2)))
	a16, b16 := list2DKeyDecode[uint16](string([]byte{0, 1, 0, 2}))
	assert.Equal(t, uint16(1), a16)
	assert.Equal(t, uint16(2), b16)

	// uint32
	require.Equal(t, string([]byte{0, 0, 0, 1, 0, 0, 0, 2}), list2DKeyEncode(uint32(1), uint32(2)))
	a32, b32 := list2DKeyDecode[uint32](string([]byte{0, 0, 0, 1, 0, 0, 0, 2}))
	assert.Equal(t, uint32(1), a32)
	assert.Equal(t, uint32(2), b32)

	// uint64
	require.Equal(t, string([]byte{0, 0, 0, 0, 0, 0, 0, 1, 0, 0, 0, 0, 0, 0, 0, 2}), list2DKeyEncode(uint64(1), uint64(2)))
	a64, b64 := list2DKeyDecode[uint64](string([]byte{0, 0, 0, 0, 0, 0, 0, 1, 0, 0, 0, 0, 0, 0, 0, 2}))
	assert.Equal(t, uint64(1), a64)
	assert.Equal(t, uint64(2), b64)
}
