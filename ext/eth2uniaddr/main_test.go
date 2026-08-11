package main

import (
	"strings"
	"testing"
)

func TestEthAddrToUniAddrHex(t *testing.T) {
	// 标准测试地址：编码为 T=2(uint32 LE) + compact(20=0x50) + 20 字节地址
	const want = "0200000050" + "ab5801a7d398351b8be11c439e05c5b3259aec9b"

	tests := []struct {
		name    string
		in      string
		want    string
		wantErr bool
	}{
		{name: "with_0x_prefix", in: "0xAb5801a7D398351b8bE11C439e05C5B3259aeC9B", want: want},
		{name: "without_prefix", in: "Ab5801a7D398351b8bE11C439e05C5B3259aeC9B", want: want},
		{name: "lowercase", in: "0xab5801a7d398351b8be11c439e05c5b3259aec9b", want: want},
		{name: "uppercase_hex_digits", in: "0xAB5801A7D398351B8BE11C439E05C5B3259AEC9B", want: want},

		{name: "invalid_hex_char", in: "0xzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzz", wantErr: true},
		{name: "odd_length", in: "0xabc", wantErr: true},
		{name: "too_short_19_bytes", in: "0xab5801a7d398351b8be11c439e05c5b3259aec", wantErr: true},
		{name: "too_long_21_bytes", in: "0xab5801a7d398351b8be11c439e05c5b3259aec9b01", wantErr: true},
		{name: "empty", in: "", wantErr: true},
		{name: "only_prefix", in: "0x", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ethAddrToUniAddrHex(tt.in)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got %q", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Fatalf("got %q, want %q", got, tt.want)
			}
			// 输出必须是小写 hex
			if got != strings.ToLower(got) {
				t.Fatalf("output must be lowercase hex: %q", got)
			}
		})
	}
}
