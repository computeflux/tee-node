package util

import "fmt"

// DebugLog 用于临时调试日志。
// 不需要日志时，只需把下面的 fmt.Printf 一行注释掉即可全局关闭。
func DebugLog(format string, args ...any) {
	fmt.Printf("[DEBUG] "+format+"\n", args...)
}
