package relay

import (
	"time"
)

// Meta 中继请求上下文
type Meta struct {
	ProviderId      uint64
	BaseURL         string
	APIKey          string
	Config          interface{}
	IsStream        bool
	OriginModelName string
	RequestURLPath  string
	StartTime       time.Time
	ProviderType    uint16
}

// GetMetaByContext 从 RelayContext 构建 Meta
func GetMetaByContext(ctx *RelayContext) *Meta {
	return &Meta{
		RequestURLPath: ctx.R.URL.String(),
		StartTime:      time.Now(),
	}
}
