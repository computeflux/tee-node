// Package store: 自包含 SCALE codec，不再依赖 go-substrate-rpc-client。
//
// 编码策略：
//  1. 若 *T 实现了 ScaleEncoder / ScaleDecoder（由 scale-codec-gen 生成），使用生成方法
//  2. 否则通过类型断言处理基础类型（int, uint, bool, string, []byte 等）
package store

import (
	"encoding/binary"
	"fmt"
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
	// 1) 尝试生成方法的接口
	if enc, ok := any(&v).(ScaleEncoder); ok {
		return enc.Encode()
	}
	// 2) 基础类型分发
	return encodeAny(v)
}

// ScaleDecode SCALE 解码到指针。
func ScaleDecode[T any](v *T, data []byte) error {
	// 1) 尝试生成方法的接口
	if dec, ok := any(v).(ScaleDecoder); ok {
		_, err := dec.Decode(data)
		return err
	}
	// 2) 基础类型分发
	return decodeAny(v, data)
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
	case UniAddr:
		return addrEncode(val), nil
	case U256:
		b := val.Int.Bytes()
		buf := make([]byte, 32)
		for i := 0; i < len(b); i++ {
			buf[i] = b[len(b)-1-i]
		}
		return buf, nil
	default:
		return nil, fmt.Errorf("storeEncode: unsupported type %T", v)
	}
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
	case *UniAddr:
		return addrDecode(data, val)
	case *U256:
		if len(data) < 32 {
			return fmt.Errorf("decode U256: need 32 bytes, got %d", len(data))
		}
		val.Int.SetBytes(ReverseBytes(data[:32]))
		return nil
	default:
		return fmt.Errorf("storeDecode: unsupported type %T", v)
	}
	return nil
}

// ── UniAddr 编解码 ────────────────────────────────────

func addrEncode(addr UniAddr) []byte {
	var buf []byte
	buf = binary.LittleEndian.AppendUint32(buf, addr.T)
	buf = EncodeCompact(buf, uint64(len(addr.V)))
	buf = append(buf, addr.V...)
	return buf
}

func addrDecode(data []byte, addr *UniAddr) error {
	if len(data) < 4 {
		return fmt.Errorf("decode UniAddr.T: need 4 bytes")
	}
	addr.T = binary.LittleEndian.Uint32(data[:4])
	n, sz := DecodeCompact(data[4:])
	if len(data) < 4+sz+int(n) {
		return fmt.Errorf("decode UniAddr.V: need %d bytes", 4+sz+int(n))
	}
	addr.V = make([]byte, n)
	copy(addr.V, data[4+sz:4+sz+int(n)])
	return nil
}

func uniAddrEncodeToHex(addr UniAddr) string {
	return hexEncodeToString(addrEncode(addr))
}

func uniAddrDecodeFromHex(s string) (UniAddr, error) {
	var addr UniAddr
	if err := addrDecode([]byte(s), &addr); err != nil {
		return UniAddr{}, err
	}
	return addr, nil
}

// ── compact 编码 ──────────────────────────────────────

// EncodeCompact SCALE compact 编码。
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

// DecodeCompact SCALE compact 解码。返回值和消费字节数。
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

// ── 工具 ──────────────────────────────────────────────

// ReverseBytes 字节反转（U256 大端 ↔ LE）。
func ReverseBytes(b []byte) []byte {
	for i, j := 0, len(b)-1; i < j; i, j = i+1, j-1 {
		b[i], b[j] = b[j], b[i]
	}
	return b
}

func hexEncodeToString(b []byte) string {
	const hex = "0123456789abcdef"
	out := make([]byte, len(b)*2)
	for i, c := range b {
		out[i*2] = hex[c>>4]
		out[i*2+1] = hex[c&0xf]
	}
	return string(out)
}
