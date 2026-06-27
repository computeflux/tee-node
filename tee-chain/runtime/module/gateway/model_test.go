package gateway

import (
	"strings"
	"testing"
)

func TestMaskAPIKey(t *testing.T) {
	longKey := "sk-cp-yFrjdXWNwolTyg4MDKxrOk50KOEuEKMdOkF2Tf94Id3OcigGSHkZ4BCVq68Sq1qIRHjKZyWkkc-YrCNQj3qh2Z9T_Pg_Yo2c1rmIqaaAT-GY5H1-DzF"
	t.Logf("key len=%d", len(longKey))
	masked := maskAPIKey(longKey)
	t.Logf("masked long key: %s", masked)
	if !strings.HasPrefix(masked, "sk-cp-y") {
		t.Errorf("expected prefix sk-cp-y, got %s", masked)
	}

	// 统计明文长度（非 *）
	visible := len(masked) - strings.Count(masked, "*")
	if visible > len(longKey)/8 {
		t.Errorf("visible chars = %d, want <= %d", visible, len(longKey)/8)
	}

	// 无 '-' 的情况
	noDash := "abcdefghabcdefgh"
	maskedNoDash := maskAPIKey(noDash)
	t.Logf("masked no dash: %s", maskedNoDash)
	if !strings.HasPrefix(maskedNoDash, "ab") {
		t.Errorf("expected prefix ab, got %s", maskedNoDash)
	}

	// 短 key
	short := "ab"
	if maskAPIKey(short) != "**" {
		t.Errorf("short key masking failed: %s", maskAPIKey(short))
	}
}
