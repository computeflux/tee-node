// Package notify 提供 tee-node 链上事件 → 用户通知 的生成与推送能力。
//
// 核心设计（模型下架降级方案）：
//   - 通知文案优先走 LLM 生成；当 LLM 未配置或调用失败（模型下架/不可访问）时，
//     自动降级为「按事件类型预置的模板化文案」，保证全链路无 LLM 依赖也能产出通知。
//   - 生成后的通知推送到 actives 消息中心（POST /messages/push），由 dapp 展示给用户。
//
// 本包仅依赖标准库 + 项目内 util，不引入新的第三方依赖。
package notify

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/computeflux/tee-provider/pkg/util"
)

// Event 类型常量（与 actives user_messages.type 对齐）。
const (
	EventModelDeactivated = "model_deactivated" // 模型下架 / 停用
	EventModelActivated   = "model_activated"   // 模型恢复 / 上架
	EventModelRegistered  = "model_registered"  // 新模型注册
	EventSystem           = "system"            // 系统公告
)

// Event 一条待生成/推送的链上事件通知。
type Event struct {
	Type         string            `json:"type"`
	UserID       string            `json:"userId"` // 目标用户（EVM 地址，小写）
	ModelName    string            `json:"modelName,omitempty"`
	ProviderName string            `json:"providerName,omitempty"`
	Extra        map[string]string `json:"extra,omitempty"` // 模板变量（如 blockHeight、reason）
}

// Config 通知服务配置（全部来自环境变量，缺失时对应能力降级）。
type Config struct {
	// Actives 推送
	ActivesURL     string // actives 服务根地址（如 http://actives:9100）
	ActivesPushTok string // X-Actives-Token（与 actives 侧 ACTIVES_PUSH_TOKEN 一致）

	// LLM（可选；未配置或调用失败时走模板降级）
	LLMBaseURL string // OpenAI 兼容 base（如 https://api.openai.com/v1）
	LLMKey     string
	LLMModel   string
	LLMTimeout time.Duration

	HTTPTimeout time.Duration // 推送 actives 的超时
}

// LoadConfigFromEnv 从环境变量装配 Config。
func LoadConfigFromEnv() Config {
	return Config{
		ActivesURL:     util.GetEnv("ACTIVES_URL", ""),
		ActivesPushTok: util.GetEnv("ACTIVES_PUSH_TOKEN", ""),
		LLMBaseURL:     util.GetEnv("NOTIFY_LLM_BASE_URL", ""),
		LLMKey:         util.GetEnv("NOTIFY_LLM_KEY", ""),
		LLMModel:       util.GetEnv("NOTIFY_LLM_MODEL", "gpt-4o-mini"),
		LLMTimeout:     time.Duration(util.GetEnvInt("NOTIFY_LLM_TIMEOUT_MS", 8000)) * time.Millisecond,
		HTTPTimeout:    time.Duration(util.GetEnvInt("NOTIFY_PUSH_TIMEOUT_MS", 5000)) * time.Millisecond,
	}
}

// Notifier 通知生成 + 推送器。
type Notifier struct {
	cfg  Config
	http *http.Client
}

// NewNotifier 创建通知器。cfg.ActivesURL 为空时 Push 会返回错误（未配置推送目标）。
func NewNotifier(cfg Config) *Notifier {
	if cfg.HTTPTimeout <= 0 {
		cfg.HTTPTimeout = 5 * time.Second
	}
	if cfg.LLMTimeout <= 0 {
		cfg.LLMTimeout = 8 * time.Second
	}
	return &Notifier{
		cfg:  cfg,
		http: &http.Client{Timeout: cfg.HTTPTimeout},
	}
}

// Enabled 返回推送能力是否就绪（配置了 actives 地址与 token）。
func (n *Notifier) Enabled() bool {
	return n.cfg.ActivesURL != "" && n.cfg.ActivesPushTok != ""
}

// LLMEnabled 返回是否配置了 LLM（未配置则直接走模板）。
func (n *Notifier) LLMEnabled() bool {
	return n.cfg.LLMBaseURL != "" && n.cfg.LLMKey != ""
}

// Generate 生成通知文案（title + content）。
// 优先 LLM；LLM 未配置或调用失败时降级为模板。返回 viaLLM 标识实际来源。
func (n *Notifier) Generate(ctx context.Context, ev Event) (title, content string, viaLLM bool) {
	if n.LLMEnabled() {
		if t, c, err := n.generateByLLM(ctx, ev); err == nil && strings.TrimSpace(c) != "" {
			return t, c, true
		}
		// LLM 失败 → 降级模板（不阻断通知产出）
	}
	return TemplateFor(ev)
}

// Push 生成并推送一条通知到 actives 消息中心。
func (n *Notifier) Push(ctx context.Context, ev Event) (viaLLM bool, err error) {
	if !n.Enabled() {
		return false, fmt.Errorf("notify: actives push not configured (ACTIVES_URL/ACTIVES_PUSH_TOKEN)")
	}
	title, content, via := n.Generate(ctx, ev)
	if strings.TrimSpace(ev.UserID) == "" {
		return via, fmt.Errorf("notify: event %s has empty userId", ev.Type)
	}

	body, _ := json.Marshal(map[string]string{
		"userId":  ev.UserID,
		"type":    ev.Type,
		"title":   title,
		"content": content,
	})

	url := strings.TrimRight(n.cfg.ActivesURL, "/") + "/messages/push"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return via, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Actives-Token", n.cfg.ActivesPushTok)

	resp, err := n.http.Do(req)
	if err != nil {
		return via, fmt.Errorf("notify: push to actives: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return via, fmt.Errorf("notify: actives returned %d: %s", resp.StatusCode, strings.TrimSpace(string(b)))
	}
	return via, nil
}

// generateByLLM 调用 OpenAI 兼容 /chat/completions 生成文案。
// 任何错误（网络、模型下架 404/403、超时）都返回 error，由 Generate 降级为模板。
func (n *Notifier) generateByLLM(ctx context.Context, ev Event) (title, content string, err error) {
	prompt := buildLLMPrompt(ev)

	payload, _ := json.Marshal(map[string]any{
		"model":       n.cfg.LLMModel,
		"messages":    []map[string]string{{"role": "user", "content": prompt}},
		"temperature": 0.4,
		"max_tokens":  200,
	})

	url := strings.TrimRight(n.cfg.LLMBaseURL, "/") + "/chat/completions"
	reqCtx, cancel := context.WithTimeout(ctx, n.cfg.LLMTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(reqCtx, http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return "", "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+n.cfg.LLMKey)

	resp, err := n.http.Do(req)
	if err != nil {
		return "", "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return "", "", fmt.Errorf("llm status %d: %s", resp.StatusCode, strings.TrimSpace(string(b)))
	}

	var out struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return "", "", err
	}
	if len(out.Choices) == 0 {
		return "", "", fmt.Errorf("llm: empty choices")
	}
	text := strings.TrimSpace(out.Choices[0].Message.Content)
	if text == "" {
		return "", "", fmt.Errorf("llm: empty content")
	}
	// LLM 输出约定：第一行为标题，其余为正文；无换行则整体作正文。
	if idx := strings.IndexByte(text, '\n'); idx >= 0 {
		return strings.TrimSpace(text[:idx]), strings.TrimSpace(text[idx+1:]), nil
	}
	return defaultTitle(ev), text, nil
}

// buildLLMPrompt 构造 LLM 提示词（要求输出「标题\n正文」，简洁、面向用户）。
func buildLLMPrompt(ev Event) string {
	var b strings.Builder
	b.WriteString("你是 ComputeFlux 链上事件通知助手。请根据以下事件，用简洁的中文写一条面向用户的通知。\n")
	b.WriteString("输出格式：第一行是标题（≤20字），第二行起是正文（≤80字）。不要输出多余内容。\n\n")
	b.WriteString("事件类型: " + ev.Type + "\n")
	if ev.ModelName != "" {
		b.WriteString("模型: " + ev.ModelName + "\n")
	}
	if ev.ProviderName != "" {
		b.WriteString("提供方: " + ev.ProviderName + "\n")
	}
	for k, v := range ev.Extra {
		b.WriteString(fmt.Sprintf("%s: %s\n", k, v))
	}
	return b.String()
}
