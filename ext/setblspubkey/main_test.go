package main

import (
	"testing"
)

func TestHexDecode(t *testing.T) {
	tests := []struct {
		name    string
		in      string
		want    []byte
		wantErr bool
	}{
		{name: "even_length", in: "deadbeef", want: []byte{0xde, 0xad, 0xbe, 0xef}},
		{name: "odd_length_pads_zero", in: "abc", want: []byte{0x0a, 0xbc}},
		{name: "empty", in: "", want: []byte{}},
		{name: "uppercase", in: "DEADBEEF", want: []byte{0xde, 0xad, 0xbe, 0xef}},
		{name: "mixed_case", in: "DeAdBeEf", want: []byte{0xde, 0xad, 0xbe, 0xef}},
		{name: "single_char_pads", in: "f", want: []byte{0x0f}},
		{name: "invalid_char_g", in: "12g4", wantErr: true},
		{name: "invalid_char_special", in: "12!4", wantErr: true},
		{name: "invalid_odd_last", in: "12g", wantErr: true}, // g 非法，即使 g 是最后一个字符也会先被 hexValue 拒绝
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := hexDecode(tt.in)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got %v", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(got) != len(tt.want) {
				t.Fatalf("len got %d, want %d", len(got), len(tt.want))
			}
			for i := range tt.want {
				if got[i] != tt.want[i] {
					t.Fatalf("byte %d: got %02x, want %02x", i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestHexValue(t *testing.T) {
	tests := []struct {
		in   byte
		want int
	}{
		{'0', 0}, {'1', 1}, {'9', 9},
		{'a', 10}, {'b', 11}, {'f', 15},
		{'A', 10}, {'B', 11}, {'F', 15},
		{'g', -1}, {'G', -1}, {'z', -1},
		{' ', -1}, {'/', -1}, {':', -1}, {'@', -1}, {0xff, -1},
	}

	for _, tt := range tests {
		if got := hexValue(tt.in); got != tt.want {
			t.Errorf("hexValue(%q) = %d, want %d", tt.in, got, tt.want)
		}
	}
}
