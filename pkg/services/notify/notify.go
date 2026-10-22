// Package notify 实现 tee-node 链上事件 → 用户通知的生成与推送链路。
//
// 背景（模型下架降级方案）：LLM 模型下架（无法访问）曾导致 tee-node 通知无法生成。
// 本包把通知生成做成「LLM 优先 + 模板兜底」的降级链路：
//
//   - 模型可用时：走 LLM 生成个性化文案（Generate 内部调用 LLM）。
//   - 模型不可用 / 未配置 / 调用失败时：自动降级为模板化文案（TemplateFor，
//     按事件类型预置文案，纯模板、零 LLM 依赖）。
//
// 生成的 (title, content) 通过 Push 推送到 actives 消息接口（POST /messages/push，
// 共享密钥鉴权），落库后在 dapp 消息中心展示给用户。
//
// 设计约束：
//   - 全链路无 LLM 硬依赖——模板路径始终可用，模型完全不可访问时通知仍能产生并落库。
//   - 不引入新依赖，仅用 stdlib + 项目内 util。
package notify

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/computeflux/tee-provider/pkg/util"
	"github.com/computeflux/tee-provider/tee-chain/runtime/store"
)

// 预置事件类型（与 actives 侧 TemplateFor 保持一致）。
const (
	EventTypeModelOffline  = "model_offline"  // 模型下架/不可用（用户请求失败触发）
	EventTypeModelRestored = "model_restored" // 模型恢复（用户请求成功触发）
	EventTypeModelListed   = "model_listed"   // 模型上架（目录变更，推送给模型主人）
	EventTypeModelDelisted = "model_delisted" // 模型下架（目录变更，推送给模型主人）
	EventTypeSettlement    = "settlement"     // 用量结算
	EventTypeSystem        = "system"         // 系统公告
)

// Config 通知链路配置（全部来自环境变量，见 NewFromEnv）。
type Config struct {
	// Enabled 总开关；false 时 Push 直接 no-op（不产生任何外呼）。
	Enabled bool
	// ActivesURL actives 服务根地址（如 https://api.computeflux.ai），推送打到 <ActivesURL>/messages/push。
	ActivesURL string
	// PushSecret 与 actives 的 ACTIVES_MESSAGE_PUSH_SECRET 一致的共享密钥。
	PushSecret string
	// LLMBaseURL / LLMKey / LLMModel 可选的 LLM 生成端点（OpenAI 兼容 /chat/completions）。
	// 任一为空则视为「未配置 LLM」，Generate 直接走模板（等价于模型不可用）。
	LLMBaseURL string
	LLMKey     string
	LLMModel   string
	// LLMTimeout 单次 LLM 调用超时；模型不可用时快速失败并降级到模板。
	LLMTimeout time.Duration
	// PushTimeout 单次推送到 actives 的超时。
	PushTimeout time.Duration
	// HTTPClient 可选注入（测试用）；nil 时内部按 PushTimeout 构造。
	HTTPClient *http.Client
}

// configFromEnv 从环境变量构造 Config。
func configFromEnv() Config {
	enabled := util.GetEnv("NOTIFY_ENABLED", "true") != "false"
	return Config{
		Enabled:     enabled,
		ActivesURL:  strings.TrimRight(util.GetEnv("NOTIFY_ACTIVES_URL", ""), "/"),
		PushSecret:  util.GetEnv("NOTIFY_PUSH_SECRET", ""),
		LLMBaseURL:  strings.TrimRight(util.GetEnv("NOTIFY_LLM_BASE_URL", ""), "/"),
		LLMKey:      util.GetEnv("NOTIFY_LLM_KEY", ""),
		LLMModel:    util.GetEnv("NOTIFY_LLM_MODEL", ""),
		LLMTimeout:  time.Duration(util.GetEnvInt("NOTIFY_LLM_TIMEOUT_MS", 8000)) * time.Millisecond,
		PushTimeout: time.Duration(util.GetEnvInt("NOTIFY_PUSH_TIMEOUT_MS", 5000)) * time.Millisecond,
	}
}

// Notifier 通知生成 + 推送器。零值不可用，请用 New / NewFromEnv。
type Notifier struct {
	cfg Config
}

// New 用显式 Config 构造 Notifier。
func New(cfg Config) *Notifier {
	if cfg.LLMTimeout <= 0 {
		cfg.LLMTimeout = 8 * time.Second
	}
	if cfg.PushTimeout <= 0 {
		cfg.PushTimeout = 5 * time.Second
	}
	if cfg.HTTPClient == nil {
		cfg.HTTPClient = &http.Client{Timeout: cfg.PushTimeout}
	}
	return &Notifier{cfg: cfg}
}

// Enabled 返回通知链路总开关状态。
func (n *Notifier) Enabled() bool { return n.cfg.Enabled }

// ActivesURL 返回配置的 actives 服务地址（未配置时为空串）。
func (n *Notifier) ActivesURL() string { return n.cfg.ActivesURL }

// HasSecret 返回是否配置了推送密钥。
func (n *Notifier) HasSecret() bool { return n.cfg.PushSecret != "" }

// NewFromEnv 从环境变量构造 Notifier。
func NewFromEnv() *Notifier {
	return New(configFromEnv().withDefaults())
}

func (c Config) withDefaults() Config {
	if c.LLMTimeout <= 0 {
		c.LLMTimeout = 8 * time.Second
	}
	if c.PushTimeout <= 0 {
		c.PushTimeout = 5 * time.Second
	}
	return c
}

// llmConfigured 是否配置了可用的 LLM 端点。
func (n *Notifier) llmConfigured() bool {
	return n.cfg.LLMBaseURL != "" && n.cfg.LLMModel != ""
}

// Generate 生成 (title, content)。
//
// 降级链路：
//  1. 未配置 LLM → 直接模板（零 LLM 依赖）。
//  2. 配置了 LLM → 尝试 LLM 生成；任何失败（超时/网络/模型下架/非 2xx）→ 降级模板。
//
// 返回值保证非空：模板路径对未知类型也有 system 兜底文案。
func (n *Notifier) Generate(ctx context.Context, eventType string, vars map[string]string) (title, content string) {
	if n.llmConfigured() {
		if t, c, ok := n.generateByLLM(ctx, eventType, vars); ok {
			return t, c
		}
		// LLM 不可用 → 落到模板（下方统一处理）
	}
	return TemplateFor(eventType, vars)
}

// generateByLLM 调用 LLM 生成文案。ok=false 表示 LLM 不可用（应降级到模板）。
func (n *Notifier) generateByLLM(ctx context.Context, eventType string, vars map[string]string) (title, content string, ok bool) {
	cctx, cancel := context.WithTimeout(ctx, n.cfg.LLMTimeout)
	defer cancel()

	prompt := buildLLMPrompt(eventType, vars)
	body, err := json.Marshal(map[string]any{
		"model":       n.cfg.LLMModel,
		"messages":    []map[string]string{{"role": "user", "content": prompt}},
		"temperature": 0.3,
	})
	if err != nil {
		return "", "", false
	}

	req, err := http.NewRequestWithContext(cctx, http.MethodPost, n.cfg.LLMBaseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return "", "", false
	}
	req.Header.Set("Content-Type", "application/json")
	if n.cfg.LLMKey != "" {
		req.Header.Set("Authorization", "Bearer "+n.cfg.LLMKey)
	}

	resp, err := n.cfg.HTTPClient.Do(req)
	if err != nil {
		return "", "", false // 模型不可访问（网络/超时/下架）→ 降级
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		_, _ = io.Copy(io.Discard, resp.Body)
		return "", "", false // 模型下架/限流等 → 降级
	}

	var out struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return "", "", false
	}
	if err := json.Unmarshal(data, &out); err != nil || len(out.Choices) == 0 {
		return "", "", false
	}
	raw := strings.TrimSpace(out.Choices[0].Message.Content)
	if raw == "" {
		return "", "", false
	}
	// LLM 约定输出两行：第一行 title，第二行起 content。解析失败则整体作为 content。
	title, content = parseLLMOutput(raw)
	if title == "" && content == "" {
		return "", "", false
	}
	return title, content, true
}

// buildLLMPrompt 构造 LLM 提示词（要求两行输出：title / content）。
func buildLLMPrompt(eventType string, vars map[string]string) string {
	var b strings.Builder
	b.WriteString("你是 ComputeFlux 的用户通知文案助手。请为以下链上事件生成一条简短、友好的中文通知。\n")
	b.WriteString("事件类型: ")
	b.WriteString(eventType)
	b.WriteString("\n")
	if len(vars) > 0 {
		b.WriteString("变量: ")
		first := true
		for k, v := range vars {
			if !first {
				b.WriteString(", ")
			}
			b.WriteString(k)
			b.WriteString("=")
			b.WriteString(v)
			first = false
		}
		b.WriteString("\n")
	}
	b.WriteString("输出格式（严格两行，不要多余内容）:\n第一行: 标题（≤16字）\n第二行: 正文（≤60字）\n")
	return b.String()
}

// parseLLMOutput 解析 LLM 两行输出为 (title, content)。
func parseLLMOutput(raw string) (title, content string) {
	lines := strings.Split(strings.TrimSpace(raw), "\n")
	if len(lines) >= 2 {
		title = strings.TrimSpace(lines[0])
		content = strings.TrimSpace(strings.Join(lines[1:], "\n"))
	} else {
		content = strings.TrimSpace(raw)
	}
	return title, content
}

// TemplateFor 按事件类型返回预置 (title, content)。vars 用于 {占位符} 替换。
// 纯模板、零 LLM 依赖，是模型不可用时的兜底（与 actives 侧模板一致）。
func TemplateFor(eventType string, vars map[string]string) (title, content string) {
	switch eventType {
	case EventTypeModelOffline:
		title = "模型暂时不可用"
		content = "您使用的模型 {model} 暂时下架，相关调用已暂停。恢复后将自动通知您。"
	case EventTypeModelRestored:
		title = "模型已恢复"
		content = "您关注的模型 {model} 已恢复可用，欢迎继续使用。"
	case EventTypeModelListed:
		title = "新模型上架"
		content = "模型 {model} 已上架，现在可以调用了。"
	case EventTypeModelDelisted:
		title = "模型已下架"
		content = "模型 {model} 已下架，相关调用将不可用。"
	case EventTypeSettlement:
		title = "用量结算通知"
		content = "本期结算完成：调用 {calls} 次，消耗 {tokens} tokens。"
	default:
		title = "系统通知"
		content = "您有一条新的系统通知。"
	}
	for k, v := range vars {
		title = strings.ReplaceAll(title, "{"+k+"}", v)
		content = strings.ReplaceAll(content, "{"+k+"}", v)
	}
	return title, content
}

// WalletFromAddr 把链上 UniAddr 归一化为 actives 用户标识（小写）。
//
//   - T=0（EVM）：20 字节地址 → 0x 前缀小写 hex（与 dapp 钱包地址一致）。
//   - 其它（Substrate 等）：T:hex 形式（actives 侧按字符串存储，不要求 0x 地址）。
//
// 空地址返回空串（调用方据此跳过推送）。
func WalletFromAddr(a store.UniAddr) string {
	if len(a.V) == 0 {
		return ""
	}
	// T=2 是 EVM（见 graph/auth_helpers.go createUserAPIKey：以太坊地址 callerType=2）；
	// T=0 兼容旧数据。两者都按 20 字节 EVM 地址输出 0x 形式，否则下游（actives 按钱包寻址）
	// 会拿到 "2:hex" 这类非法地址而拒绝。
	if (a.T == 2 || a.T == 0) && len(a.V) == 20 {
		return "0x" + hex.EncodeToString(a.V)
	}
	return fmt.Sprintf("%d:%s", a.T, hex.EncodeToString(a.V))
}

// pushRequest 是推送到 actives /messages/push 的 body。
type pushRequest struct {
	Wallet  string            `json:"wallet"`
	Type    string            `json:"type"`
	Title   string            `json:"title"`
	Content string            `json:"content"`
	Vars    map[string]string `json:"vars,omitempty"`
}

// Push 生成通知并推送到 actives 消息接口。
//
// 返回 error 仅在「推送本身失败」（网络/actives 拒绝）时非 nil；LLM 不可用不算错误
// （已自动降级到模板）。未启用 / 未配置 actives 地址 / 空 wallet 时静默 no-op。
func (n *Notifier) Push(ctx context.Context, wallet, eventType string, vars map[string]string) error {
	if !n.cfg.Enabled {
		log.Printf("[notify] Push no-op: disabled (eventType=%s wallet=%s)", eventType, wallet)
		return nil
	}
	wallet = strings.ToLower(strings.TrimSpace(wallet))
	if wallet == "" {
		log.Printf("[notify] Push no-op: empty wallet (eventType=%s)", eventType)
		return nil
	}
	if n.cfg.ActivesURL == "" || n.cfg.PushSecret == "" {
		log.Printf("[notify] Push no-op: not configured (eventType=%s wallet=%s activesURL=%q secretSet=%v)",
			eventType, wallet, n.cfg.ActivesURL, n.cfg.PushSecret != "")
		return nil
	}

	title, content := n.Generate(ctx, eventType, vars)

	body, err := json.Marshal(pushRequest{
		Wallet:  wallet,
		Type:    eventType,
		Title:   title,
		Content: content,
		Vars:    vars,
	})
	if err != nil {
		return fmt.Errorf("notify: marshal push body: %w", err)
	}

	cctx, cancel := context.WithTimeout(ctx, n.cfg.PushTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(cctx, http.MethodPost, n.cfg.ActivesURL+"/messages/push", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("notify: build push request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Actives-Push-Secret", n.cfg.PushSecret)

	resp, err := n.cfg.HTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("notify: push to actives: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	data, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("notify: actives push returned %d: %s", resp.StatusCode, strings.TrimSpace(string(data)))
	}
	return nil
}
