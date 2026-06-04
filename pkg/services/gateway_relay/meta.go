package relay

import (
	"time"

	"github.com/computeflux/tee-provider/pkg/store"
)

// Meta 中继请求上下文（替代 token 的 meta.Meta）
type Meta struct {
	ProviderId      uint64
	BaseURL         string
	APIKey          string
	Config          store.GatewayProviderInfo
	IsStream        bool
	OriginModelName string
	RequestURLPath  string
	StartTime       time.Time
}

// GetByContext 从 RelayContext 构建 Meta（替代 token 的 GetByContext）
func GetMetaByContext(ctx *RelayContext) *Meta {
	meta := &Meta{
		RequestURLPath: ctx.R.URL.String(),
		StartTime:      time.Now(),
	}

	if meta.BaseURL == "" {
		v, ok := ctx.Get("provider")
		if ok {
			if ch, ok2 := v.(*store.GatewayProviderInfo); ok2 {
				meta.ProviderId = ch.ID
			}
		}
	}

	return meta
}
