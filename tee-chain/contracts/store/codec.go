// Package store: 自包含 SCALE codec，不再依赖 go-substrate-rpc-client。
//
// 编码策略：
//  1. 若 *T 实现了 ScaleEncoder / ScaleDecoder（由 scale-codec-gen 生成），使用生成方法
//  2. 否则通过类型断言处理基础类型（int, uint, bool, string, []byte 等）
//  3. 对 slice 类型，若元素实现了 ScaleEncoder，则递归编码每个元素
package store

import (
	"encoding/binary"
	"fmt"
	"math/big"
	"reflect"
	"strings"
)

// ScaleEncoder 生成类型实现的编码接口。
type ScaleEncoder interface {
	Encode() ([]byte, error)
}

// ScaleDecoder 生成类型实现的解码接口。
type ScaleDecoder interface {
	Decode([]byte) (int, error)
}

// ── 公开 API ────────────────────────────────────────────

// ScaleEncode SCALE 编码任意值。
func ScaleEncode[T any](v T) ([]byte, error) {
	if enc, ok := any(&v).(ScaleEncoder); ok {
		return enc.Encode()
	}
	// 若 T 是指针，检查其指向类型是否实现 ScaleEncoder
	rv := reflect.ValueOf(v)
	if rv.Kind() == reflect.Ptr && !rv.IsNil() {
		if enc, ok := rv.Elem().Addr().Interface().(ScaleEncoder); ok {
			return enc.Encode()
		}
	}
	return encodeAny(v)
}

// ScaleDecode SCALE 解码到指针。
func ScaleDecode[T any](v *T, data []byte) error {
	// 初始化 nil *big.Int 字段（生成代码不检查 nil）
	initBigIntFields(v)

	if dec, ok := any(v).(ScaleDecoder); ok {
		_, err := dec.Decode(data)
		return err
	}
	rv := reflect.ValueOf(v)
	if rv.Kind() == reflect.Ptr {
		if rv.Elem().Kind() == reflect.Ptr && !rv.Elem().IsNil() {
			if dec, ok := rv.Elem().Interface().(ScaleDecoder); ok {
				_, err := dec.Decode(data)
				return err
			}
		}
	}
	return decodeAny(v, data)
}

// initBigIntFields 递归初始化结构体中所有 nil *big.Int 字段。
func initBigIntFields(v any) {
	rv := reflect.ValueOf(v)
	if rv.Kind() == reflect.Ptr {
		rv = rv.Elem()
	}
	if rv.Kind() != reflect.Struct {
		return
	}
	for i := 0; i < rv.NumField(); i++ {
		f := rv.Field(i)
		if f.Kind() == reflect.Ptr && f.Type() == reflect.TypeFor[*big.Int]() && f.IsNil() {
			f.Set(reflect.ValueOf(new(big.Int)))
		} else if f.Kind() == reflect.Struct {
			initBigIntFields(f.Addr().Interface())
		}
	}
}

// ── 基础类型编码 —─────────────────────────────────────

func encodeAny(v any) ([]byte, error) {
	switch val := v.(type) {
	case uint8:
		return []byte{val}, nil
	case uint16:
		var b [2]byte
		binary.LittleEndian.PutUint16(b[:], val)
		return b[:], nil
	case uint32:
		var b [4]byte
		binary.LittleEndian.PutUint32(b[:], val)
		return b[:], nil
	case uint64:
		var b [8]byte
		binary.LittleEndian.PutUint64(b[:], val)
		return b[:], nil
	case int:
		var b [8]byte
		binary.LittleEndian.PutUint64(b[:], uint64(val))
		return b[:], nil
	case int8:
		return []byte{uint8(val)}, nil
	case int16:
		var b [2]byte
		binary.LittleEndian.PutUint16(b[:], uint16(val))
		return b[:], nil
	case int32:
		var b [4]byte
		binary.LittleEndian.PutUint32(b[:], uint32(val))
		return b[:], nil
	case int64:
		var b [8]byte
		binary.LittleEndian.PutUint64(b[:], uint64(val))
		return b[:], nil
	case bool:
		if val {
			return []byte{1}, nil
		}
		return []byte{0}, nil
	case string:
		b := EncodeCompact(nil, uint64(len(val)))
		b = append(b, []byte(val)...)
		return b, nil
	case []byte:
		b := EncodeCompact(nil, uint64(len(val)))
		b = append(b, val...)
		return b, nil
	default:
		// 尝试 Option<T> 编码
		if data, ok := encodeOption(v); ok {
			return data, nil
		}

		// 尝试 slice<ScaleEncoder> 编码
		if data, ok := encodeSlice(v); ok {
			return data, nil
		}
		return nil, fmt.Errorf("storeEncode: unsupported type %T", v)
	}
}

// encodeSlice 用 reflect 检测 slice，若元素实现了 ScaleEncoder 则递归编码。
func encodeSlice(v any) ([]byte, bool) {
	rv := reflect.ValueOf(v)
	if rv.Kind() != reflect.Slice {
		return nil, false
	}
	b := EncodeCompact(nil, uint64(rv.Len()))
	for i := 0; i < rv.Len(); i++ {
		elem := rv.Index(i)
		elemPtr := elem
		if elem.Kind() == reflect.Ptr {
			if elem.IsNil() {
				continue
			}
			elemPtr = elem.Elem()
		}
		if enc, ok := elemPtr.Addr().Interface().(ScaleEncoder); ok {
			data, err := enc.Encode()
			if err != nil {
				return nil, false
			}
			b = append(b, data...)
		} else if enc, ok := elem.Interface().(ScaleEncoder); ok {
			data, err := enc.Encode()
			if err != nil {
				return nil, false
			}
			b = append(b, data...)
		} else {
			// 元素不支持 ScaleEncoder，回退到 encodeAny
			data, err := encodeAny(elem.Interface())
			if err != nil {
				return nil, false
			}
			b = append(b, data...)
		}
	}
	return b, true
}

// encodeOption 用 reflect 检测 Option<T>，SCALE 编码：None=0x00，Some(v)=0x01+ScaleEncode(v)。
func encodeOption(v any) ([]byte, bool) {
	rv := reflect.ValueOf(v)
	if !strings.HasPrefix(rv.Type().String(), "store.Option[") {
		return nil, false
	}
	isSome := rv.FieldByName("isSome").Bool()
	if !isSome {
		return []byte{0x00}, true
	}
	vField := rv.FieldByName("V")
	data, err := encodeAny(vField.Interface())
	if err != nil {
		return nil, false
	}
	return append([]byte{0x01}, data...), true
}

// decodeSlice 用 reflect 检测 *[]T，递归解码每个元素。
func decodeSlice(v any, data []byte) error {
	rv := reflect.ValueOf(v)
	if rv.Kind() != reflect.Ptr {
		return fmt.Errorf("not a pointer")
	}
	elem := rv.Elem()
	if elem.Kind() != reflect.Slice {
		return fmt.Errorf("not a slice")
	}
	count, sz := DecodeCompact(data)
	if count == 0 {
		elem.Set(reflect.MakeSlice(elem.Type(), 0, 0))
		return nil
	}
	sl := reflect.MakeSlice(elem.Type(), int(count), int(count))
	off := sz
	for i := 0; i < int(count); i++ {
		item := sl.Index(i)
		itemPtr := item
		if item.Kind() == reflect.Ptr {
			// 为指针元素分配新实例
			item.Set(reflect.New(item.Type().Elem()))
			itemPtr = item
		}
		if dec, ok := itemPtr.Addr().Interface().(ScaleDecoder); ok {
			n, err := dec.Decode(data[off:])
			if err != nil {
				return fmt.Errorf("decodeSlice[%d]: %w", i, err)
			}
			off += n
		} else if dec, ok := itemPtr.Interface().(ScaleDecoder); ok {
			n, err := dec.Decode(data[off:])
			if err != nil {
				return fmt.Errorf("decodeSlice[%d]: %w", i, err)
			}
			off += n
		} else {
			// 基础类型：解码到 itemPtr，消费固定大小
			if err := decodeAny(itemPtr.Addr().Interface(), data[off:]); err != nil {
				return fmt.Errorf("decodeSlice[%d]: %w", i, err)
			}
			// 估算消费字节
			consumed := 0
			switch itemPtr.Kind() {
			case reflect.Uint8, reflect.Int8, reflect.Bool:
				consumed = 1
			case reflect.Uint16, reflect.Int16:
				consumed = 2
			case reflect.Uint32, reflect.Int32, reflect.Float32:
				consumed = 4
			case reflect.Uint64, reflect.Int64, reflect.Float64:
				consumed = 8
			case reflect.String:
				n, sz := DecodeCompact(data[off:])
				consumed = sz + int(n)
			case reflect.Slice:
				if itemPtr.Type().Elem().Kind() == reflect.Uint8 {
					n, sz := DecodeCompact(data[off:])
					consumed = sz + int(n)
				} else {
					elem.Set(reflect.MakeSlice(elem.Type(), i+1, i+1))
					for j := 0; j <= i; j++ {
						elem.Index(j).Set(sl.Index(j))
					}
					return nil
				}
			default:
				elem.Set(reflect.MakeSlice(elem.Type(), i+1, i+1))
				for j := 0; j <= i; j++ {
					elem.Index(j).Set(sl.Index(j))
				}
				return nil
			}
			off += consumed
		}
	}
	elem.Set(sl)
	return nil
}

func decodeAny(v any, data []byte) error {
	switch val := v.(type) {
	case *uint8:
		if len(data) < 1 {
			return fmt.Errorf("decode uint8: need 1 byte, got %d", len(data))
		}
		*val = data[0]
	case *uint16:
		if len(data) < 2 {
			return fmt.Errorf("decode uint16: need 2 bytes, got %d", len(data))
		}
		*val = binary.LittleEndian.Uint16(data)
	case *uint32:
		if len(data) < 4 {
			return fmt.Errorf("decode uint32: need 4 bytes, got %d", len(data))
		}
		*val = binary.LittleEndian.Uint32(data)
	case *uint64:
		if len(data) < 8 {
			return fmt.Errorf("decode uint64: need 8 bytes, got %d", len(data))
		}
		*val = binary.LittleEndian.Uint64(data)
	case *int8:
		if len(data) < 1 {
			return fmt.Errorf("decode int8: need 1 byte, got %d", len(data))
		}
		*val = int8(data[0])
	case *int:
		if len(data) < 8 {
			return fmt.Errorf("decode int: need 8 bytes, got %d", len(data))
		}
		*val = int(binary.LittleEndian.Uint64(data))
	case *int16:
		if len(data) < 2 {
			return fmt.Errorf("decode int16: need 2 bytes, got %d", len(data))
		}
		*val = int16(binary.LittleEndian.Uint16(data))
	case *int32:
		if len(data) < 4 {
			return fmt.Errorf("decode int32: need 4 bytes, got %d", len(data))
		}
		*val = int32(binary.LittleEndian.Uint32(data))
	case *int64:
		if len(data) < 8 {
			return fmt.Errorf("decode int64: need 8 bytes, got %d", len(data))
		}
		*val = int64(binary.LittleEndian.Uint64(data))
	case *bool:
		if len(data) < 1 {
			return fmt.Errorf("decode bool: need 1 byte, got %d", len(data))
		}
		*val = data[0] != 0
	case *string:
		n, sz := DecodeCompact(data)
		if len(data) < sz+int(n) {
			return fmt.Errorf("decode string: need %d bytes, got %d", sz+int(n), len(data))
		}
		*val = string(data[sz : sz+int(n)])
	case *[]byte:
		n, sz := DecodeCompact(data)
		if len(data) < sz+int(n) {
			return fmt.Errorf("decode []byte: need %d bytes, got %d", sz+int(n), len(data))
		}
		*val = make([]byte, n)
		copy(*val, data[sz:sz+int(n)])
	default:
		// 尝试 *slice<T> 解码
		if err := decodeSlice(v, data); err == nil {
			return nil
		}
		return fmt.Errorf("storeDecode: unsupported type %T", v)
	}
	return nil
}

// ── compact 编码 ──────────────────────────────────────

func EncodeCompact(buf []byte, v uint64) []byte {
	switch {
	case v <= 0x3F:
		return append(buf, byte(v)<<2)
	case v <= 0x3FFF:
		return append(buf, byte(v)<<2|1, byte(v>>6))
	case v <= 0x3FFFFFFF:
		return append(buf, byte(v)<<2|2, byte(v>>6), byte(v>>14), byte(v>>22))
	default:
		n := byte(0)
		t := v
		for t > 0 {
			n++
			t >>= 8
		}
		if n < 4 {
			n = 4
		}
		buf = append(buf, byte(n-4)<<2|3)
		for i := byte(0); i < n; i++ {
			buf = append(buf, byte(v>>(i*8)))
		}
		return buf
	}
}

func DecodeCompact(data []byte) (uint64, int) {
	if len(data) == 0 {
		return 0, 0
	}
	switch data[0] & 3 {
	case 0:
		return uint64(data[0]) >> 2, 1
	case 1:
		return uint64(data[0])>>2 | uint64(data[1])<<6, 2
	case 2:
		return uint64(data[0])>>2 | uint64(data[1])<<6 | uint64(data[2])<<14 | uint64(data[3])<<22, 4
	default:
		return uint64(data[1]) | uint64(data[2])<<8 | uint64(data[3])<<16 | uint64(data[4])<<24 |
			uint64(data[5])<<32 | uint64(data[6])<<40 | uint64(data[7])<<48 | uint64(data[8])<<56, 9
	}
}

func ReverseBytes(b []byte) []byte {
	for i, j := 0, len(b)-1; i < j; i, j = i+1, j-1 {
		b[i], b[j] = b[j], b[i]
	}
	return b
}
