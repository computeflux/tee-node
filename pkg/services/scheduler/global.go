package scheduler

import "sync"

// 全局单例,给 DistributeMiddleware/HandleRelay 拿。
var (
	global     *GatewayScheduler
	globalOnce sync.RWMutex
)

// SetGlobal 注册全局单例(在 main.go 启动时调用)。
func SetGlobal(g *GatewayScheduler) {
	globalOnce.Lock()
	defer globalOnce.Unlock()
	global = g
}

// GetGlobal 取全局单例。返回 nil 表示 scheduler 未初始化。
// 在 DistributeMiddleware/HandleRelay 里用它。
func GetGlobal() *GatewayScheduler {
	globalOnce.RLock()
	defer globalOnce.RUnlock()
	return global
}

// ResetGlobal 测试用。
func ResetGlobal() {
	globalOnce.Lock()
	defer globalOnce.Unlock()
	global = nil
}
