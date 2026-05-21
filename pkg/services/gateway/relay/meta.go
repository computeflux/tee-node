package relay

import (
	"strings"
	"time"

	"github.com/computeflux/tee-provider/pkg/model"
)

// Meta 中继请求上下文（替代 token 的 meta.Meta）
type Meta struct {
	Mode               int
	ChannelType        int
	ChannelId          uint64
	TokenId            uint64
	TokenName          string
	UserId             string
	Group              string
	ModelMapping       map[string]string
	BaseURL            string
	APIKey             string
	APIType            int
	Config             model.GatewayProviderInfo
	IsStream           bool
	OriginModelName    string
	ActualModelName    string
	RequestURLPath     string
	PromptTokens       int
	ForcedSystemPrompt string
	StartTime          time.Time
}

// GetByContext 从 RelayContext 构建 Meta（替代 token 的 GetByContext）
func GetByContext(ctx *RelayContext) *Meta {
	meta := &Meta{
		OriginModelName: ctx.GetString("request_model"),
		BaseURL:         ctx.GetString("base_url"),
		APIKey:          strings.TrimPrefix(ctx.R.Header.Get("Authorization"), "Bearer "),
		RequestURLPath:  ctx.R.URL.String(),
		StartTime:       time.Now(),
	}

	if meta.BaseURL == "" {
		v, ok := ctx.Get("channel")
		if ok {
			if ch, ok2 := v.(*model.GatewayProviderInfo); ok2 {
				meta.BaseURL = ch.BaseURL
				meta.ChannelId = ch.ID
			}
		}
	}

	return meta
}
