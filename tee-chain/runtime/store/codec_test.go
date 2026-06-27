package store

import (
	"math"
	"math/big"
	"reflect"
	"testing"
)

func check(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

func eq[T comparable](t *testing.T, want, got T) {
	t.Helper()
	if want != got {
		t.Fatalf("want %v, got %v", want, got)
	}
}

func deepEq(t *testing.T, want, got any) {
	t.Helper()
	if !reflect.DeepEqual(want, got) {
		t.Fatalf("want %v, got %v", want, got)
	}
}

// ── 基础类型 ────────────────────────────────────

func TestCodec_Bool(t *testing.T) {
	for _, v := range []bool{false, true} {
		data, _ := ScaleEncode(v)
		var out bool
		check(t, ScaleDecode(&out, data))
		eq(t, v, out)
	}
}

func TestCodec_Uint8(t *testing.T)  { testCodecInt(t, uint8(0), uint8(255)) }
func TestCodec_Uint16(t *testing.T) { testCodecInt(t, uint16(0), uint16(math.MaxUint16)) }
func TestCodec_Uint32(t *testing.T) { testCodecInt(t, uint32(0), uint32(math.MaxUint32)) }
func TestCodec_Uint64(t *testing.T) { testCodecInt(t, uint64(0), uint64(math.MaxUint64)) }
func TestCodec_Int8(t *testing.T)   { testCodecInt(t, int8(math.MinInt8), int8(math.MaxInt8)) }
func TestCodec_Int64(t *testing.T)  { testCodecInt(t, int64(math.MinInt64), int64(math.MaxInt64)) }

func testCodecInt[T comparable](t *testing.T, minV, maxV T) {
	for _, v := range []T{minV, maxV} {
		data, _ := ScaleEncode(v)
		var out T
		check(t, ScaleDecode(&out, data))
		eq(t, v, out)
	}
}

func TestCodec_String(t *testing.T) {
	for _, v := range []string{"", "hello", "你好"} {
		data, _ := ScaleEncode(v)
		var out string
		check(t, ScaleDecode(&out, data))
		eq(t, v, out)
	}
}

func TestCodec_Bytes(t *testing.T) {
	for _, v := range [][]byte{{}, {0x01, 0x02}} {
		data, _ := ScaleEncode(v)
		var out []byte
		check(t, ScaleDecode(&out, data))
		deepEq(t, v, out)
	}
}

func TestCodec_UniAddr(t *testing.T) {
	for _, v := range []UniAddr{
		{T: 1, V: []byte("alice")}, {T: 42, V: make([]byte, 32)},
	} {
		data, _ := ScaleEncode(v)
		var out UniAddr
		check(t, ScaleDecode(&out, data))
		eq(t, v.T, out.T)
		deepEq(t, v.V, out.V)
	}
}

func TestCodec_U256(t *testing.T) {
	for _, val := range []*big.Int{
		big.NewInt(0), big.NewInt(1), big.NewInt(1000000),
	} {
		v := U256{Int: val}
		data, _ := ScaleEncode(v)
		var out U256
		out.Int = new(big.Int)
		check(t, ScaleDecode(&out, data))
		eq(t, v.Int.String(), out.Int.String())
	}
}

// ── Compact ─────────────────────────────────────

func TestCodec_Compact(t *testing.T) {
	for _, v := range []uint64{0, 1, 0x3F, 0x40, 0x3FFF, 0x4000, 0x3FFFFFFF, math.MaxUint64} {
		data := EncodeCompact(nil, v)
		dec, sz := DecodeCompact(data)
		eq(t, v, dec)
		eq(t, len(data), sz)
	}
}

// ── Option ──────────────────────────────────────
func TestCodec_OptionNone(t *testing.T) {
	v := NewNone[uint32]()
	data, _ := ScaleEncode(v)
	deepEq(t, []byte{0x00}, data)
	var out Option[uint32]
	check(t, ScaleDecode(&out, data))
	eq(t, false, out.IsSome())
}

func TestCodec_OptionSome(t *testing.T) {
	v := NewSome(uint32(42))
	data, _ := ScaleEncode(v)
	eq(t, byte(0x01), data[0])
	var out Option[uint32]
	check(t, ScaleDecode(&out, data))
	eq(t, true, out.IsSome())
	val, _ := out.UnWrap()
	eq(t, uint32(42), val)
}

// ── Slice ───────────────────────────────────────

func TestCodec_SliceUint32(t *testing.T) {
	v := []uint32{0, 1, math.MaxUint32}
	data, _ := ScaleEncode(v)
	var out []uint32
	check(t, ScaleDecode(&out, data))
	deepEq(t, v, out)
}

func TestCodec_SliceString(t *testing.T) {
	v := []string{"a", "bb", "ccc"}
	data, _ := ScaleEncode(v)
	var out []string
	check(t, ScaleDecode(&out, data))
	deepEq(t, v, out)
}

func TestCodec_SliceBytes(t *testing.T) {
	v := [][]byte{{0x01}, {}, {0xff, 0xfe}}
	data, _ := ScaleEncode(v)
	var out [][]byte
	check(t, ScaleDecode(&out, data))
	eq(t, len(v), len(out))
	for i := range v {
		deepEq(t, v[i], out[i])
	}
}

// ── *[]T (pointer to slice) ─────────────────────

func TestCodec_PtrSliceUint32(t *testing.T) {
	v := []uint32{0, 1, 65536}
	data, _ := ScaleEncode(v)
	var out []uint32
	check(t, ScaleDecode(&out, data))
	deepEq(t, v, out)
}

func TestCodec_PtrSliceString(t *testing.T) {
	v := []string{"x", "yy"}
	data, _ := ScaleEncode(v)
	var out []string
	check(t, ScaleDecode(&out, data))
	deepEq(t, v, out)
}

// ── ScaleEncoder ────────────────────────────────

type TestItem struct {
	ID   uint32
	Name string
}

func (t *TestItem) Encode() ([]byte, error) {
	b := EncodeCompact(nil, uint64(t.ID))
	b = EncodeCompact(b, uint64(len(t.Name)))
	b = append(b, []byte(t.Name)...)
	return b, nil
}

func (t *TestItem) Decode(data []byte) (int, error) {
	off := 0
	id, sz := DecodeCompact(data[off:])
	off += sz
	t.ID = uint32(id)
	nlen, sz := DecodeCompact(data[off:])
	off += sz
	t.Name = string(data[off : off+int(nlen)])
	off += int(nlen)
	return off, nil
}

func TestCodec_ScaleEncoderSingle(t *testing.T) {
	v := &TestItem{ID: 42, Name: "hello"}
	data, _ := ScaleEncode(v)
	var out TestItem
	check(t, ScaleDecode(&out, data))
	eq(t, uint32(42), out.ID)
	eq(t, "hello", out.Name)
}

func TestCodec_ScaleEncoderSlice(t *testing.T) {
	v := []*TestItem{{ID: 1, Name: "a"}, {ID: 2, Name: "bb"}}
	data, _ := ScaleEncode(v)
	var out []*TestItem
	check(t, ScaleDecode(&out, data))
	eq(t, 2, len(out))
	eq(t, uint32(1), out[0].ID)
	eq(t, "a", out[0].Name)
	eq(t, uint32(2), out[1].ID)
	eq(t, "bb", out[1].Name)
}

// 模拟 *[]BatchCall 场景：value type 实现 ScaleDecoder
type TestValue struct {
	X uint32
}

func (t *TestValue) Encode() ([]byte, error) {
	return EncodeCompact(nil, uint64(t.X)), nil
}

func (t *TestValue) Decode(data []byte) (int, error) {
	v, sz := DecodeCompact(data)
	t.X = uint32(v)
	return sz, nil
}

func TestCodec_ValueSlice(t *testing.T) {
	v := []TestValue{{X: 10}, {X: 20}}
	data, _ := ScaleEncode(v)
	var out []TestValue
	check(t, ScaleDecode(&out, data))
	eq(t, 2, len(out))
	eq(t, uint32(10), out[0].X)
	eq(t, uint32(20), out[1].X)
}

// ── 边界 ────────────────────────────────────────

func TestCodec_EmptySlice(t *testing.T) {
	v := []uint32{}
	data, _ := ScaleEncode(v)
	eq(t, byte(0x00), data[0]) // compact 编码 0
	var out []uint32
	check(t, ScaleDecode(&out, data))
	eq(t, 0, len(out))
}

func TestCodec_NilSlice(t *testing.T) {
	var v []uint32 = nil
	data, _ := ScaleEncode(v)
	var out []uint32
	check(t, ScaleDecode(&out, data))
	eq(t, 0, len(out))
}

// ── DecodeScaleArgBytes 路径（*[]T 解码）──
// 模拟 DecodeScaleArgBytes[[]T] → ScaleDecode(&z, data) 场景
func TestCodec_DecodePtrSlice(t *testing.T) {
	// 构造编码数据
	items := []TestValue{{X: 10}, {X: 20}}
	data, err := ScaleEncode(items)
	check(t, err)

	// 模拟 DecodeScaleArgBytes[[]TestValue] 的调用方式
	var out []TestValue
	err = ScaleDecode(&out, data)
	check(t, err)
	eq(t, 2, len(out))
	eq(t, uint32(10), out[0].X)
	eq(t, uint32(20), out[1].X)

	// 空切片
	emptyData, _ := ScaleEncode([]TestValue{})
	var emptyOut []TestValue
	check(t, ScaleDecode(&emptyOut, emptyData))
	eq(t, 0, len(emptyOut))

	// nil 切片
	var nilData []byte
	nilData, _ = ScaleEncode([]TestValue(nil))
	var nilOut []TestValue
	check(t, ScaleDecode(&nilOut, nilData))
	eq(t, 0, len(nilOut))
}
