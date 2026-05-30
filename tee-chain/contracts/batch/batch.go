package batch

import (
	"fmt"

	"github.com/computeflux/tee-provider/pkg/model"
)

// Batch 批量合约执行器。
// 提供 execute_batch 方法：接收一组 ContractCall，在当前 caller 上下文中
// 按顺序依次调用，任一失败则整体回滚。
type Batch struct {
	api model.ContractApi

	status *model.StoreMapping[uint64, uint16]
}

type BatchQuery struct{ Batch }
type BatchMutation struct{ Batch }

// Init 初始化占位。
func (b BatchMutation) Init() error { return nil }

// ExecuteBatch 按顺序执行一组合约调用。
// 每个子调用复用当前 Batch 的 runtime（同一 txn / caller / height）。
// 任一子调用失败立即返回错误，之前的状态变更随 txn 回滚。
func (b BatchMutation) ExecuteBatch(calls []BatchCall) error {
	for i, call := range calls {
		if len(call.Name) == 0 {
			return fmt.Errorf("batch call %d: empty contract name", i)
		}
		if len(call.Method) == 0 {
			return fmt.Errorf("batch call %d: empty method selector", i)
		}

		// 使用 runtime.Call 复用同一 txn，保持原子性
		_, err := b.api.Call(b.api.GetCaller(), call.ToContractCall())
		if err != nil {
			return fmt.Errorf("batch call %d (%s): %w", i, string(call.Name), err)
		}
	}

	return nil
}

func (b BatchQuery) Status(index uint64) (string, error) {
	return "ok", nil
}
