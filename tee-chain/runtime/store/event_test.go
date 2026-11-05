package store

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// ── IsValidEventType ──────────────────────────────────────────────

func TestIsValidEventType_Valid(t *testing.T) {
	for _, name := range []string{
		"a",
		"credit_transfer",
		"balance_changed",
		"order_created_v2",
		"ev_123",
	} {
		assert.True(t, IsValidEventType(name), "expected valid: %s", name)
	}
}

func TestIsValidEventType_Invalid(t *testing.T) {
	for _, name := range []string{
		"",                    // 空
		"Credit",              // 大写开头
		"credit-transfer",     // 连字符
		"credit.transfer",     // 点号
		"1abc",                // 数字开头
		"_abc",                // 下划线开头
		string(make([]byte, 65)), // 超长（>64）
	} {
		assert.False(t, IsValidEventType(name), "expected invalid: %q", name)
	}
}

func TestIsValidEventType_Reserved(t *testing.T) {
	// 系统管道事件保留名
	assert.False(t, IsValidEventType("relay_synced"))
	assert.False(t, IsValidEventType("contract_call"))
	// TxIndexer 保留前缀 "tx"（tx.height / tx.hash 保留 tag）
	assert.False(t, IsValidEventType("tx"))
}

// ── IsValidEventAttr ──────────────────────────────────────────────

func TestIsValidEventAttr_Valid(t *testing.T) {
	assert.True(t, IsValidEventAttr("user", "0xaaa"))
	// 边界值：key=64 字节、value=256 字节（用可打印字符 'a' 填充）
	assert.True(t, IsValidEventAttr(strings.Repeat("a", 64), strings.Repeat("a", 256)))
	assert.True(t, IsValidEventAttr("key_with_underscore", "value"))
}

func TestIsValidEventAttr_Invalid(t *testing.T) {
	assert.False(t, IsValidEventAttr("", "v"), "empty key")
	assert.False(t, IsValidEventAttr("bad.key", "v"), "dot in key")
	assert.False(t, IsValidEventAttr("bad\x00key", "v"), "control char in key")
	assert.False(t, IsValidEventAttr(string(make([]byte, 65)), "v"), "key too long")
	assert.False(t, IsValidEventAttr("k", string(make([]byte, 257))), "value too long")
}

// ── IsValidEvent ──────────────────────────────────────────────────

func TestIsValidEvent_Valid(t *testing.T) {
	attrs := make(map[string]string)
	for i := 0; i < MaxAttrsPerEvent; i++ {
		attrs[fmt.Sprintf("k%d", i)] = "v"
	}
	assert.True(t, IsValidEvent("ev", attrs))
	assert.True(t, IsValidEvent("ev", nil))
	assert.True(t, IsValidEvent("ev", map[string]string{}))
}

func TestIsValidEvent_InvalidType(t *testing.T) {
	assert.False(t, IsValidEvent("tx", map[string]string{"h": "1"}))
	assert.False(t, IsValidEvent("", map[string]string{"h": "1"}))
}

func TestIsValidEvent_InvalidAttr(t *testing.T) {
	assert.False(t, IsValidEvent("ev", map[string]string{"bad.key": "v"}))
	assert.False(t, IsValidEvent("ev", map[string]string{"": "v"}))
}

func TestIsValidEvent_TooManyAttrs(t *testing.T) {
	attrs := make(map[string]string)
	for i := 0; i < MaxAttrsPerEvent+1; i++ {
		attrs[fmt.Sprintf("k%d", i)] = "v"
	}
	assert.False(t, IsValidEvent("ev", attrs), "attrs beyond MaxAttrsPerEvent must be rejected")
}

// ── 保留 tag 场景（reviewer 指出的攻击向量）────────────────────────
// EmitEvent("tx", {"height":"1"}) 会让 kv TxIndexer 拼出保留 tag "tx.height" 并报错。
// 现在 "tx" 在保留名单内，整个事件被拒绝。

func TestReservedTagAttackVector(t *testing.T) {
	// type="tx" + key="height" → 保留 tag tx.height（必须拒绝）
	assert.False(t, IsValidEvent("tx", map[string]string{"height": "1"}))
	// type="tx" + key="hash" → 保留 tag tx.hash（必须拒绝）
	assert.False(t, IsValidEvent("tx", map[string]string{"hash": "abc"}))
	// 即使 key 合法，type="tx" 整体被拒
	assert.False(t, IsValidEvent("tx", map[string]string{"foo": "bar"}))
}
