package relay

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"strings"
)

// OpenCode(https://opencode.ai)要求客户端为每段对话在 x-opencode-session 中携带稳定的
// 会话 ID，以便其优化路由与提示词缓存。这里只注入该请求头，不修改 User-Agent。
const (
	openCodeSessionHeader = "x-opencode-session"
	openCodeURLPrefix     = "https://opencode.ai/"
)

// maxSessionHeaderLen 是会话 ID 原样透传的长度上限，超长时退化为哈希。
const maxSessionHeaderLen = 128

// upstreamSessionKey 承载「每请求的上游会话标识」。
// gateway 层在调用 scheduler 前写入，provider adaptor 在发请求前读取。
type upstreamSessionKey struct{}

// WithUpstreamSession 把会话标识写入 ctx，发往 OpenCode 的请求会将其作为
// x-opencode-session 发送。空串为 no-op（此时不加该头）。
func WithUpstreamSession(ctx context.Context, sessionID string) context.Context {
	if sessionID == "" {
		return ctx
	}
	return context.WithValue(ctx, upstreamSessionKey{}, sessionID)
}

// UpstreamSession 读取 ctx 中的会话标识，不存在时返回空串。
func UpstreamSession(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	s, _ := ctx.Value(upstreamSessionKey{}).(string)
	return s
}

// isOpenCodeURL 判断上游 URL 是否指向 OpenCode。
// 用前缀匹配，并要求域名后紧跟 "/" 或结束，避免把 opencode.ai.evil.com 一类
// 同前缀域名误判为 OpenCode。
func isOpenCodeURL(rawURL string) bool {
	return rawURL == strings.TrimSuffix(openCodeURLPrefix, "/") ||
		strings.HasPrefix(rawURL, openCodeURLPrefix)
}

// applyOpenCodeSession 对 OpenCode 上游注入 x-opencode-session：
//   - 非 OpenCode 上游、或 ctx 中没有会话标识：不加头
//   - 端点已通过 ExtraHeaders 显式配置该头：以显式配置为准，不覆盖
//   - 不设置 User-Agent，保持 net/http 默认行为
func applyOpenCodeSession(req *http.Request, rawURL, sessionID string) {
	if sessionID == "" || !isOpenCodeURL(rawURL) {
		return
	}
	if req.Header.Get(openCodeSessionHeader) != "" {
		return
	}
	req.Header.Set(openCodeSessionHeader, headerSafeSessionID(sessionID))
}

// headerSafeSessionID 把会话标识归一化为可安全放入 HTTP 头的值。
// 会话 ID 可能来自客户端请求体（不可信）：含控制字符/非 ASCII/超长会让 net/http
// 直接拒绝整个请求，这些情况退化为不可逆哈希（同一输入仍得到同一 ID，保持稳定）。
func headerSafeSessionID(s string) string {
	if s != "" && len(s) <= maxSessionHeaderLen {
		safe := true
		for i := 0; i < len(s); i++ {
			if c := s[i]; c < 0x21 || c > 0x7e {
				safe = false
				break
			}
		}
		if safe {
			return s
		}
	}
	h := sha256.Sum256([]byte(s))
	return "sid-" + hex.EncodeToString(h[:16])
}
