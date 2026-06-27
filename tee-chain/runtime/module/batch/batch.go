package batch

import (
	"fmt"

	"github.com/computeflux/tee-provider/tee-chain/runtime/store"
)

// Batch 批量合约执行器。
// 提供 execute_batch 方法：接收一组 ContractCall，在当前 caller 上下文中
// 按顺序依次调用，任一失败则整体回滚。
type Batch struct {
	api store.ContractApi

	hubcalls *store.IndexedList[uint64, HubCallIndex]
	status   *store.StoreMapping[uint64, uint16]
}

type BatchQuery struct{ Batch }
type BatchMutation struct{ Batch }
type BatchSecret struct{ Batch }

// Init 初始化占位。
func (b BatchMutation) Init() error { return nil }

// ExecuteBatch 按顺序执行一组合约调用。
func (b BatchMutation) ExecuteBatch(calls []BatchCall) error {
	for i, call := range calls {
		if len(call.Name) == 0 {
			return fmt.Errorf("batch call %d: empty contract name", i)
		}
		if len(call.Method) == 0 {
			return fmt.Errorf("batch call %d: empty method selector", i)
		}
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

// PackCall 内部 secret 方法：将一笔 hub call 插入待同步队列。
func (b BatchSecret) PackCall(hubtype uint8, chainID uint32, id uint64) error {
	_, err := b.hubcalls.Insert(b.api.GetTxn(), HubCallIndex{
		CallType: hubtype,
		ChainID:  chainID,
		CallID:   id,
		Status:   HubCallPending,
	})
	return err
}

// SetHubCallStatus 更新 hub call 状态。
func (b BatchSecret) SetHubCallStatus(key uint64, status uint8) error {
	return b.hubcalls.UpdateFn(b.api.GetTxn(), key, func(hc *HubCallIndex) {
		hc.Status = status
	})
}

// ── Query operations ──

// HubcallsGet 根据 IndexedList 主键获取单个 HubCallIndex。
func (b BatchQuery) HubcallsGet(id uint64) (*HubCallIndex, error) {
	return b.hubcalls.Get(b.api.GetTxn(), id)
}

// PackCallToSunc 查询待同步的 hub call，按 chainId 分组返回第一批。
// 返回值: chainId, IndexedList 主键列表, error
func (b BatchQuery) PackCallToSunc() (*HubCallBatch, error) {
	txn := b.api.GetTxn()
	ids, err := b.hubcalls.Query(txn, store.QueryCond{Field: "Status", Val: uint64(HubCallPending)})
	if err != nil {
		return nil, err
	}

	if len(ids) == 0 {
		return nil, nil
	}

	last, err := b.hubcalls.Get(txn, ids[0])
	if err != nil {
		return nil, err
	}

	ids, err = b.hubcalls.Query(txn, store.QueryAND{
		store.QueryCond{Field: "Status", Val: uint64(HubCallPending)},
		store.QueryCond{Field: "ChainID", Val: uint64(last.ChainID)},
	})
	if err != nil {
		return nil, err
	}

	return &HubCallBatch{
		ChainID: last.ChainID,
		IDs:     ids,
	}, nil
}
