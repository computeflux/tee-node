package graph

import (
	"encoding/hex"
	"fmt"
	"strings"
)

// normalizeHexHash 把前端传入的哈希规范化为裸 hex 字节（兼容 0x 前缀）。
// 供 block_detail 等按哈希查询的 resolver 使用。
func normalizeHexHash(s string) ([]byte, error) {
	s = strings.TrimSpace(s)
	if len(s) >= 2 && strings.EqualFold(s[:2], "0x") {
		s = s[2:]
	}
	if s == "" {
		return nil, fmt.Errorf("empty hash")
	}
	return hex.DecodeString(s)
}
