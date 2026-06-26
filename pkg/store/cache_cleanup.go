package store

import (
	"context"
	"fmt"
	"time"
)

// StartCleanupTask 启动一个后台定时任务，周期性清理过期的 token 和临时会话数据。
// interval 为清理周期；如果 <= 0，则使用默认值 1 小时。
// 调用方应通过 ctx 控制任务生命周期（如 main 函数中传入可取消的 context）。
func StartCleanupTask(ctx context.Context, interval time.Duration) {
	if interval <= 0 {
		interval = time.Hour
	}

	ticker := time.NewTicker(interval)
	go func() {
		defer func() {
			ticker.Stop()
			if r := recover(); r != nil {
				fmt.Printf("[cleanup] panic recovered: %v\n", r)
			}
		}()

		// 启动时立即执行一次索引清理
		runCleanup()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				runCleanup()
			}
		}
	}()
}

func runCleanup() {
	// 清理所有 namespace 中已过期的缓存记录
	namespaces := []string{"GATEWAY", Web3AuthNamespace}
	for _, ns := range namespaces {
		if err := cacheCleanupExpiredByIndex(ns, func(subKey string) error {
			return SECRET_DB.DeleteKey(ns, subKey)
		}); err != nil {
			fmt.Printf("[cleanup] namespace=%s error: %v\n", ns, err)
		}
	}
}
