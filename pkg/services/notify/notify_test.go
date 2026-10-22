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
	assert.Equal(t, "模型暂时不可用", tt)
	assert.Contains(t, cc, "gpt-4")
	assert.NotContains(t, cc, "{model}")

	tt2, cc2 := TemplateFor(EventTypeSettlement, map[string]string{"calls": "10", "tokens": "1000"})
	assert.Equal(t, "用量结算通知", tt2)
	assert.Contains(t, cc2, "10")
	assert.Contains(t, cc2, "1000")

	// 模型上架/下架（目录变更，推送给模型主人）
	ttL, ccL := TemplateFor(EventTypeModelListed, map[string]string{"model": "claude-3"})
	assert.Equal(t, "新模型上架", ttL)
	assert.Contains(t, ccL, "claude-3")
	assert.NotContains(t, ccL, "{model}")

	ttD, ccD := TemplateFor(EventTypeModelDelisted, map[string]string{"model": "gpt-4o"})
	assert.Equal(t, "模型已下架", ttD)
	assert.Contains(t, ccD, "gpt-4o")
	assert.NotContains(t, ccD, "{model}")

	// 未知类型 → system 兜底
	tt3, cc3 := TemplateFor("unknown", nil)
	assert.Equal(t, "系统通知", tt3)
	assert.NotEmpty(t, cc3)
}

// TestGenerate_NoLLM 未配置 LLM 时 Generate 直接走模板（全链路无 LLM 依赖）。
func TestGenerate_NoLLM(t *testing.T) {
	n := New(Config{Enabled: true})
	title, content := n.Generate(context.Background(), EventTypeModelOffline, map[string]string{"model": "gpt-4"})
	assert.Equal(t, "模型暂时不可用", title)
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
	assert.Equal(t, "模型暂时不可用", title)
	assert.Contains(t, content, "gpt-4")
}

// TestGenerate_LLMOK 配置了 LLM 且可用 → 使用 LLM 输出。
func TestGenerate_LLMOK(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/chat/completions", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"自定义标题\n自定义正文内容"}}]}`))
	}))
	defer srv.Close()

	n := New(Config{
		Enabled:    true,
		LLMBaseURL: srv.URL,
		LLMModel:   "gpt-4",
		LLMTimeout: 2 * time.Second,
	})
	title, content := n.Generate(context.Background(), EventTypeSystem, nil)
	assert.Equal(t, "自定义标题", title)
	assert.Equal(t, "自定义正文内容", content)
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
	assert.Contains(t, gotBody, "模型暂时不可用") // 模板兜底文案
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
