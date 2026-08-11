package util

import (
	"os"
	"strconv"
)

func GetEnv(key string, defaultValue string) string {
	val := os.Getenv(key)
	if len(val) == 0 {
		return defaultValue
	}
	return val
}

// GetEnvInt 读取整数环境变量，缺失或格式非法时返回 defaultValue。
// 注意：不再 panic —— 之前对非法值直接 panic 会导致配置错误时整个进程崩溃。
func GetEnvInt(key string, defaultValue int) int {
	val := os.Getenv(key)
	if len(val) == 0 {
		return defaultValue
	}
	i, err := strconv.Atoi(val)
	if err != nil {
		return defaultValue
	}
	return i
}
