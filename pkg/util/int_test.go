package util

import (
	"math"
	"testing"
)

func TestInt64ToBytes_Layout(t *testing.T) {
	// 大端序：最高字节在前
	got := Int64ToBytes(0x0102030405060708)
	want := []byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08}
	if len(got) != 8 {
		t.Fatalf("Int64ToBytes() len = %d, want 8", len(got))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("Int64ToBytes() byte %d = %#x, want %#x", i, got[i], want[i])
		}
	}
}

func TestInt64ToBytes_RoundTrip(t *testing.T) {
	values := []int64{0, 1, -1, math.MaxInt64, math.MinInt64, 42, 1234567890}
	for _, v := range values {
		got := BytesToInt64(Int64ToBytes(v))
		if got != v {
			t.Errorf("roundtrip(%d) = %d", v, got)
		}
	}
}

func TestBytesToInt64_ShortInput(t *testing.T) {
	// 短于 8 字节的输入不应 panic，应返回 0
	cases := [][]byte{
		nil,
		{},
		{0x01},
		{0x01, 0x02, 0x03, 0x04},
		{0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00},
	}
	for _, b := range cases {
		if got := BytesToInt64(b); got != 0 {
			t.Errorf("BytesToInt64(%v) = %d, want 0", b, got)
		}
	}
}

func TestBytesToInt64_ExtraBytes(t *testing.T) {
	// 超过 8 字节时取前 8 字节（与 binary.BigEndian.Uint64 语义一致）
	b := append([]byte{0, 0, 0, 0, 0, 0, 0, 42}, 0xFF, 0xFF)
	if got := BytesToInt64(b); got != 42 {
		t.Errorf("BytesToInt64(extra) = %d, want 42", got)
	}
}
