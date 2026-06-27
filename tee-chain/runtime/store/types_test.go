package store

import (
	"testing"
)

// ── Option ─────────────────────────────────────────────────

func TestOption_None_EncodeDecode(t *testing.T) {
	o := NewNone[uint64]()
	data, err := o.Encode()
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	if len(data) != 1 || data[0] != 0x00 {
		t.Fatalf("expected [0x00], got % x", data)
	}

	var got Option[uint64]
	n, err := got.Decode(data)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if n != 1 {
		t.Fatalf("expected consume 1 byte, got %d", n)
	}
	if !got.IsNone() || got.IsSome() {
		t.Fatalf("expected None, got some=%v", got.IsSome())
	}
}

func TestOption_Some_EncodeDecode(t *testing.T) {
	o := NewSome[uint64](0xdeadbeef)
	data, err := o.Encode()
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	if data[0] != 0x01 {
		t.Fatalf("expected variant 0x01, got 0x%02x", data[0])
	}

	var got Option[uint64]
	if _, err := got.Decode(data); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !got.IsSome() {
		t.Fatalf("expected Some")
	}
	v, err := got.UnWrap()
	if err != nil {
		t.Fatalf("unwrap: %v", err)
	}
	if v != 0xdeadbeef {
		t.Fatalf("expected 0xdeadbeef, got 0x%x", v)
	}
}

func TestOption_None_UnwrapErrors(t *testing.T) {
	o := NewNone[string]()
	if _, err := o.UnWrap(); err == nil {
		t.Fatalf("expected error on None unwrap")
	}
}

// ── Result ─────────────────────────────────────────────────

func TestResult_Ok_EncodeDecode(t *testing.T) {
	r := NewOk[uint64, string](42)
	data, err := r.Encode()
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	if data[0] != 0x00 {
		t.Fatalf("expected variant 0x00 for Ok, got 0x%02x", data[0])
	}

	var got Result[uint64, string]
	if _, err := got.Decode(data); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !got.IsOk() {
		t.Fatalf("expected IsOk=true, got IsErr=%v", got.IsErr())
	}
	v, err := got.UnWrap()
	if err != nil {
		t.Fatalf("unwrap: %v", err)
	}
	if v != 42 {
		t.Fatalf("expected 42, got %d", v)
	}
}

func TestResult_Err_EncodeDecode(t *testing.T) {
	r := NewErr[uint64, string]("boom")
	data, err := r.Encode()
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	if data[0] != 0x01 {
		t.Fatalf("expected variant 0x01 for Err, got 0x%02x", data[0])
	}

	var got Result[uint64, string]
	if _, err := got.Decode(data); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !got.IsErr() {
		t.Fatalf("expected IsErr=true, got IsOk=%v", got.IsOk())
	}
	if got.E != "boom" {
		t.Fatalf("expected Err=\"boom\", got %q", got.E)
	}
	if _, err := got.UnWrap(); err == nil {
		t.Fatalf("expected UnWrap to error on Err")
	}
}

func TestResult_Decode_EmptyData(t *testing.T) {
	var r Result[uint64, string]
	if _, err := r.Decode(nil); err == nil {
		t.Fatalf("expected error on empty data")
	}
}

func TestResult_Decode_UnknownVariant(t *testing.T) {
	var r Result[uint64, string]
	if _, err := r.Decode([]byte{0x02}); err == nil {
		t.Fatalf("expected error on unknown variant")
	}
}

// ── 复合场景 ─────────────────────────────────────────────

func TestResult_OkWithStringErrType_RoundTrip(t *testing.T) {
	r := NewOk[uint32, string](7)
	data, err := r.Encode()
	if err != nil {
		t.Fatalf("encode: %v", err)
	}

	var got Result[uint32, string]
	if _, err := got.Decode(data); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !got.IsOk() {
		t.Fatalf("expected Ok")
	}
	v, err := got.UnWrap()
	if err != nil {
		t.Fatalf("unwrap: %v", err)
	}
	if v != 7 {
		t.Fatalf("expected 7, got %d", v)
	}
}
