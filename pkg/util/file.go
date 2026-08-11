package util

import "os"

// IsFileExists 判断文件是否存在。
// 仅当 os.Stat 成功时返回 true；任何错误（不存在、无权限等）都返回 false，
// 避免在 stat 失败（如权限不足）时误判为文件存在。
func IsFileExists(filename string) bool {
	_, err := os.Stat(filename)
	return err == nil
}
