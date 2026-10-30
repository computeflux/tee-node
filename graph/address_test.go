package graph

import (
	"encoding/hex"
	"testing"
)

// TestGovBalanceOfSelector 回归：balance_of 查询选择器必须按 mutates=false 生成，
// 与 gov 合约查询分发（gov_gen.go / abis/gov.json）一致，否则 ContractQuery 会落到
// default 分支返回 "gov: unknown method"，余额查询对所有地址都失败。
func TestGovBalanceOfSelector(t *testing.T) {
	got := hex.EncodeToString(govBalanceOfSelector[:])
	const want = "87e0ff32"
	if got != want {
		t.Fatalf("govBalanceOfSelector = %s, want %s", got, want)
	}
}

// TestClampPage 回归：分页区间收敛必须防 (page-1)*size 整数溢出为负导致 all[start:end]
// 负下标 panic。page 来自 GraphQL Int 且无上限。
func TestClampPage(t *testing.T) {
	cases := []struct {
		name      string
		page      int
		size      int
		total     int
		wantStart int
		wantEnd   int
	}{
		{"first page", 1, 20, 100, 0, 20},
		{"middle page", 3, 20, 100, 40, 60},
		{"last partial page", 5, 20, 100, 80, 100},
		{"page beyond total -> empty page", 99, 20, 100, 100, 100},
		{"huge page no overflow -> empty", 1 << 62, 20, 100, 100, 100},
		{"max int page no overflow", 1<<31 - 1, 100, 50, 0, 50},
		{"empty total", 1, 20, 0, 0, 0},
		{"empty total huge page", 1 << 62, 20, 0, 0, 0},
		{"size zero guarded", 1, 0, 10, 0, 1},
		{"page zero clamps to 1", 0, 20, 100, 0, 20},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			start, end := clampPage(c.page, c.size, c.total)
			if start != c.wantStart || end != c.wantEnd {
				t.Fatalf("clampPage(%d,%d,%d) = [%d,%d), want [%d,%d)",
					c.page, c.size, c.total, start, end, c.wantStart, c.wantEnd)
			}
			// 不变量：区间必须合法（非负、不越界、start<=end）
			if start < 0 || end < 0 || start > end || end > c.total {
				t.Fatalf("clampPage(%d,%d,%d) = [%d,%d) violates invariants (total=%d)",
					c.page, c.size, c.total, start, end, c.total)
			}
		})
	}
}
