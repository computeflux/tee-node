package store

import (
	"fmt"
	"time"
)

// RefreshTokenInfo 存储刷新令牌与用户的映射关系，用于在 API Key 过期后换取新的 API Key。
type RefreshTokenInfo struct {
	Owner     string `json:"owner"`
	Address   string `json:"address"`
	IsActive  bool   `json:"isActive"`
	CreatedAt uint64 `json:"createdAt"`
	ExpiresAt uint64 `json:"expiresAt"` // 过期时间（Unix 秒），0=永不过期
}

// IsExpired 检查 refresh token 是否已过期。
func (t *RefreshTokenInfo) IsExpired() bool {
	if t.ExpiresAt == 0 {
		return false
	}
	return time.Now().Unix() > int64(t.ExpiresAt)
}

// SetRefreshToken 存储 refresh token 信息。
func SetRefreshToken(token string, info *RefreshTokenInfo) error {
	return SetJson("GATEWAY", fmt.Sprintf("refresh_token:%s", token), info)
}

// GetRefreshToken 通过 refresh token 查询对应的信息。
func GetRefreshToken(token string) (*RefreshTokenInfo, error) {
	return GetJson[RefreshTokenInfo]("GATEWAY", fmt.Sprintf("refresh_token:%s", token))
}

// FindRefreshToken 查询 refresh token，并检查是否有效（存在、活跃、未过期）。
// 若已过期则自动删除数据及对应过期索引，避免长期占用存储。
func FindRefreshToken(token string) *RefreshTokenInfo {
	tk, err := GetRefreshToken(token)
	if err != nil || tk == nil {
		return nil
	}
	if !tk.IsActive || tk.IsExpired() {
		_ = DeleteRefreshToken(token)
		return nil
	}
	return tk
}

// CleanupExpiredRefreshTokens 基于过期索引删除过期 refresh token。
func CleanupExpiredRefreshTokens() error {
	return CleanupExpiredByIndex("GATEWAY", func(subKey string) error {
		return DeleteKey("GATEWAY", subKey)
	})
}

// DeleteRefreshToken 删除 refresh token 记录，并同步清理过期索引。
// 先删除数据，成功后再清理索引；若数据删除失败则保留索引，确保后续清理仍能定位该记录。
func DeleteRefreshToken(token string) error {
	tk, _ := GetRefreshToken(token)
	if err := DeleteKey("GATEWAY", fmt.Sprintf("refresh_token:%s", token)); err != nil {
		return err
	}
	if tk != nil {
		_ = CancelExpiration("GATEWAY", "refresh_token:"+token, int64(tk.ExpiresAt))
	}
	return nil
}
