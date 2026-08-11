package util

import (
	"encoding/hex"
	"strings"
	"testing"
)

func TestToTeeChainNodeID_Deterministic(t *testing.T) {
	pub := []byte("some-32-byte-public-key-bytes")
	a := ToTeeChainNodeID(pub)
	b := ToTeeChainNodeID(pub)
	if a != b {
		t.Errorf("ToTeeChainNodeID not deterministic: %s vs %s", a, b)
	}
}

func TestToTeeChainNodeID_HexFormat(t *testing.T) {
	pub := []byte("another-public-key-bytes-123")
	id := ToTeeChainNodeID(pub)
	// 输出应为小写十六进制字符串
	if _, err := hex.DecodeString(string(id)); err != nil {
		t.Errorf("ToTeeChainNodeID output %q is not valid hex: %v", id, err)
	}
	if strings.ToLower(string(id)) != string(id) {
		t.Errorf("ToTeeChainNodeID output %q should be lowercase hex", id)
	}
}

func TestToTeeChainNodeID_DifferentInput(t *testing.T) {
	a := ToTeeChainNodeID([]byte("input-a"))
	b := ToTeeChainNodeID([]byte("input-b"))
	if a == b {
		t.Errorf("different inputs produced the same node id %q", a)
	}
}
