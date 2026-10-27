package notify

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestTemplateFor 验证模板兜底（零 LLM 依赖）：各事件类型产出非空文案，占位符被替换。
func TestTemplateFor(t *testing.T) {
	tt, cc := TemplateFor(EventTypeModelOffline, map[string]string{"model": "gpt-4"})
	assert.Equal(t, "Model temporarily unavailable", tt)
	assert.Contains(t, cc, "gpt-4")
	assert.NotContains(t, cc, "{model}")

	tt2, cc2 := TemplateFor(EventTypeSettlement, map[string]string{"calls": "10", "tokens": "1000"})
	assert.Equal(t, "Usage settlement", tt2)
	assert.Contains(t, cc2, "10")
	assert.Contains(t, cc2, "1000")

	// 模型上架/下架（目录变更，推送给模型主人）
	ttL, ccL := TemplateFor(EventTypeModelListed, map[string]string{"model": "claude-3"})
	assert.Equal(t, "New model listed", ttL)
	assert.Contains(t, ccL, "claude-3")
	assert.NotContains(t, ccL, "{model}")

	ttD, ccD := TemplateFor(EventTypeModelDelisted, map[string]string{"model": "gpt-4o"})
	assert.Equal(t, "Model delisted", ttD)
	assert.Contains(t, ccD, "gpt-4o")
	assert.NotContains(t, ccD, "{model}")

	// 未知类型 → system 兜底
	tt3, cc3 := TemplateFor("unknown", nil)
	assert.Equal(t, "System notification", tt3)
	assert.NotEmpty(t, cc3)
}

// TestGenerate_NoLLM 未配置 LLM 时 Generate 直接走模板（全链路无 LLM 依赖）。
func TestGenerate_NoLLM(t *testing.T) {
	n := New(Config{Enabled: true})
	title, content := n.Generate(context.Background(), EventTypeModelOffline, map[string]string{"model": "gpt-4"})
	assert.Equal(t, "Model temporarily unavailable", title)
	assert.Contains(t, content, "gpt-4")
}

// TestGenerate_LLMUnavailable 配置了 LLM 但端点不可访问（模型下架）→ 自动降级到模板。
func TestGenerate_LLMUnavailable(t *testing.T) {
	// 指向一个立即拒绝的端点（模拟模型下架/不可访问）
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "model offline", http.StatusServiceUnavailable)
	}))
	defer srv.Close()

	n := New(Config{
		Enabled:    true,
		LLMBaseURL: srv.URL,
		LLMModel:   "gpt-4",
		LLMTimeout: 2 * time.Second,
	})
	title, content := n.Generate(context.Background(), EventTypeModelOffline, map[string]string{"model": "gpt-4"})
	// 降级到模板
	assert.Equal(t, "Model temporarily unavailable", title)
	assert.Contains(t, content, "gpt-4")
}

// TestGenerate_LLMOK 配置了 LLM 且可用 → 使用 LLM 输出。
func TestGenerate_LLMOK(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/chat/completions", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"Custom title\nCustom body text"}}]}`))
	}))
	defer srv.Close()

	n := New(Config{
		Enabled:    true,
		LLMBaseURL: srv.URL,
		LLMModel:   "gpt-4",
		LLMTimeout: 2 * time.Second,
	})
	title, content := n.Generate(context.Background(), EventTypeSystem, nil)
	assert.Equal(t, "Custom title", title)
	assert.Equal(t, "Custom body text", content)
}

// TestPush_TemplateFallback 核心验收：模型完全不可访问时，Push 仍能产生通知并成功推送到 actives。
func TestPush_TemplateFallback(t *testing.T) {
	var gotSecret, gotBody string
	// 模拟 actives /messages/push
	actives := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/messages/push", r.URL.Path)
		gotSecret = r.Header.Get("X-Actives-Push-Secret")
		buf := make([]byte, r.ContentLength)
		_, _ = r.Body.Read(buf)
		gotBody = string(buf)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true,"id":"m-1"}`))
	}))
	defer actives.Close()

	// LLM 端点不可访问（模型下架）
	deadLLM := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "offline", http.StatusServiceUnavailable)
	}))
	defer deadLLM.Close()

	n := New(Config{
		Enabled:    true,
		ActivesURL: actives.URL,
		PushSecret: "s3cret",
		LLMBaseURL: deadLLM.URL,
		LLMModel:   "gpt-4",
		LLMTimeout: 2 * time.Second,
	})

	err := n.Push(context.Background(), "0xABC", EventTypeModelOffline, map[string]string{"model": "gpt-4"})
	require.NoError(t, err)
	assert.Equal(t, "s3cret", gotSecret)
	assert.Contains(t, gotBody, `"wallet":"0xabc"`) // wallet 归一化小写
	assert.Contains(t, gotBody, `"type":"model_offline"`)
	assert.Contains(t, gotBody, "Model temporarily unavailable") // 模板兜底文案
	assert.Contains(t, gotBody, "gpt-4")
}

// TestPush_Disabled 未启用时 Push no-op（不外呼）。
func TestPush_Disabled(t *testing.T) {
	called := false
	actives := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { called = true }))
	defer actives.Close()

	n := New(Config{Enabled: false, ActivesURL: actives.URL, PushSecret: "s"})
	err := n.Push(context.Background(), "0xabc", EventTypeSystem, nil)
	assert.NoError(t, err)
	assert.False(t, called)
}

// TestPush_NotConfigured 未配置 actives 地址/密钥时 Push no-op（不外呼、不报错）。
func TestPush_NotConfigured(t *testing.T) {
	n := New(Config{Enabled: true})
	err := n.Push(context.Background(), "0xabc", EventTypeSystem, nil)
	assert.NoError(t, err)
}

// TestPush_ActivesRejects actives 返回非 2xx 时 Push 返回错误。
func TestPush_ActivesRejects(t *testing.T) {
	actives := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "invalid push secret", http.StatusUnauthorized)
	}))
	defer actives.Close()

	n := New(Config{Enabled: true, ActivesURL: actives.URL, PushSecret: "bad"})
	err := n.Push(context.Background(), "0xabc", EventTypeSystem, nil)
	require.Error(t, err)
	assert.True(t, strings.Contains(err.Error(), "401"))
}

// TestPush_SettlementDisabledByDefault 验收 1：结算通知子开关默认关闭，
// Usage settlement 消息不产生/不外呼；其余事件类型不受影响。
func TestPush_SettlementDisabledByDefault(t *testing.T) {
	var settlementCalls, otherCalls int
	actives := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		buf := make([]byte, r.ContentLength)
		_, _ = r.Body.Read(buf)
		if strings.Contains(string(buf), `"type":"settlement"`) {
			settlementCalls++
		} else {
			otherCalls++
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer actives.Close()

	// 默认 Config（SettlementEnabled 零值 = false）
	n := New(Config{Enabled: true, ActivesURL: actives.URL, PushSecret: "s"})
	assert.False(t, n.SettlementEnabled())

	// settlement 被短路（无外呼、无报错）
	err := n.Push(context.Background(), "0xabc", EventTypeSettlement, map[string]string{"calls": "10", "tokens": "1000"})
	assert.NoError(t, err)
	assert.Equal(t, 0, settlementCalls)

	// 其余事件类型正常推送
	err = n.Push(context.Background(), "0xabc", EventTypeModelOffline, map[string]string{"model": "gpt-4"})
	assert.NoError(t, err)
	assert.Equal(t, 1, otherCalls)
}

// TestPush_SettlementEnabled 验收 3：开关置 true 后结算通知恢复发送，无需改代码。
func TestPush_SettlementEnabled(t *testing.T) {
	var settlementCalls int
	actives := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		buf := make([]byte, r.ContentLength)
		_, _ = r.Body.Read(buf)
		if strings.Contains(string(buf), `"type":"settlement"`) {
			settlementCalls++
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer actives.Close()

	n := New(Config{Enabled: true, SettlementEnabled: true, ActivesURL: actives.URL, PushSecret: "s"})
	assert.True(t, n.SettlementEnabled())

	err := n.Push(context.Background(), "0xabc", EventTypeSettlement, map[string]string{"calls": "10", "tokens": "1000"})
	assert.NoError(t, err)
	assert.Equal(t, 1, settlementCalls)
}

// TestConfigFromEnv_SettlementSwitch 验收 1/3：环境变量 TEE_USAGE_SETTLEMENT_ENABLED
// 默认 false（暂停），显式 "true" 时恢复。
func TestConfigFromEnv_SettlementSwitch(t *testing.T) {
	t.Setenv("TEE_USAGE_SETTLEMENT_ENABLED", "")
	assert.False(t, configFromEnv().SettlementEnabled) // 未设置 → 默认关闭

	t.Setenv("TEE_USAGE_SETTLEMENT_ENABLED", "false")
	assert.False(t, configFromEnv().SettlementEnabled)

	t.Setenv("TEE_USAGE_SETTLEMENT_ENABLED", "true")
	assert.True(t, configFromEnv().SettlementEnabled)
}
