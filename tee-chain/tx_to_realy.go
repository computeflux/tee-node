// tx_to_realy.go 侧链交易同步到主链。
//
// 流程：
//  1. targetSync() 检测 batch 中待同步的 hub call → 构建 SysCall_SyncTxStart 提交共识
//  2. FinalizeTx 处理 SyncTxStart → HubSyncStep2 标记开始 → sendPartialSign 收集部分签名
//  3. 签名足够 → SyncToHub 提交主链 → 成功/失败分别提交 SyncTxEnd(IsSeccseed=true/false)
//  4. FinalizeTx 处理 SyncTxEnd → HubSyncEnd 清理状态
package teechain

import (
	"fmt"
	"time"

	"github.com/centrifuge/go-substrate-rpc-client/v4/types/codec"
	"github.com/computeflux/tee-provider/pkg/chains"
	"github.com/computeflux/tee-provider/pkg/dkg"
	"github.com/computeflux/tee-provider/pkg/model"
	dbstore "github.com/computeflux/tee-provider/pkg/types"
	store "github.com/computeflux/tee-provider/pkg/types"
	"github.com/computeflux/tee-provider/pkg/types/chaintypes"
	"github.com/computeflux/tee-provider/pkg/util"
	contracts "github.com/computeflux/tee-provider/tee-chain/runtime"
	"github.com/computeflux/tee-provider/tee-chain/runtime/module"
	"github.com/computeflux/tee-provider/tee-chain/runtime/module/batch"
	"github.com/pkg/errors"
)

// ── SyncToHub: 提交聚合门限签名到主链 ──────────────────────────────

// submitSyncTxEnd 提交 SyncTxEnd 交易
func (s *TeeChain) submitSyncTxEnd(chainId uint32, ids []uint64, isSeccseed bool) error {
	tx := &model.SysCall{
		Payload: &model.SysCall_SyncTxEnd{
			SyncTxEnd: &model.SyncTxEnd{
				ChainId:    chainId,
				Ids:        ids,
				IsSeccseed: isSeccseed,
			},
		},
	}
	return s.SubmitCallFromNode(tx)
}

func saveCallIds(txIndex int64, chainId uint32, ids []uint64) error {
	baseKey := TxIndexPrefix + fmt.Sprint(txIndex) + "_" + fmt.Sprint(chainId)
	if err := store.DBSetJson(store.SECRET_DB, GLOABL_STATE, baseKey+TxIndexHubCallsSuffix, &hubCallsByIdsStore{Ids: ids}); err != nil {
		return errors.Wrap(err, "Set tx hubcalls error")
	}
	return nil
}

// loadHubCallIds 从 tx_index 存储中恢复 hub call IDs
func loadHubCallIds(txIndex int64, chainId uint32) ([]uint64, error) {
	baseKey := TxIndexPrefix + fmt.Sprint(txIndex) + "_" + fmt.Sprint(chainId)
	stored, err := store.DBGetJson[hubCallsByIdsStore](store.SECRET_DB, GLOABL_STATE, baseKey+TxIndexHubCallsSuffix)
	if err != nil || stored == nil {
		return nil, err
	}
	return stored.Ids, nil
}

// deleteBatchCallStore removes temporary batch call data for a txIndex within the given transaction.
// The deletion is only persisted when txn is committed by the caller.
func deleteBatchCallStore(txn *dbstore.Txn, txIndex int64, chainId uint32) error {
	baseKey := TxIndexPrefix + fmt.Sprint(txIndex) + "_" + fmt.Sprint(chainId)
	if err := txn.Delete([]byte(baseKey + TxIndexCallSuffix)); err != nil {
		return errors.Wrap(err, "delete batch call")
	}
	if err := txn.Delete([]byte(baseKey + TxIndexHubCallsSuffix)); err != nil {
		return errors.Wrap(err, "delete batch hub calls")
	}
	return nil
}

func saveBatchCall(txIndex int64, chainId uint32, call *chaintypes.ChainCall) error {
	baseKey := TxIndexPrefix + fmt.Sprint(txIndex) + "_" + fmt.Sprint(chainId)
	bt, err := codec.Encode(call)
	if err != nil {
		return errors.Wrap(err, "saveBatchCall codec error")
	}

	if err := store.SECRET_DB.SetKey(GLOABL_STATE, baseKey+TxIndexCallSuffix, bt); err != nil {
		return errors.Wrap(err, "saveBatchCall db error")
	}

	return nil
}

// loadStoredCall 从 tx_index 存储中加载已构造的 ChainCall（回退路径）
func loadBatchCall(txIndex int64, chainId uint32) (*chaintypes.ChainCall, error) {
	baseKey := TxIndexPrefix + fmt.Sprint(txIndex) + "_" + fmt.Sprint(chainId)
	bt, err := store.SECRET_DB.GetKey(GLOABL_STATE, baseKey+TxIndexCallSuffix)
	if err != nil {
		return nil, errors.New("stored call not found")
	}

	call := &chaintypes.ChainCall{}
	err = codec.Decode(bt, call)
	return call, err
}

// buildBatchCallFromIDs 从 batch store 加载 HubCallIndex 并打包为 ChainCall。
func buildBatchCallFromIDs(dkgInst *dkg.DKG, chainId uint32, ids []uint64) (*chaintypes.ChainCall, error) {
	chain, err := chains.GetChainByID(chainId)
	if err != nil {
		return nil, errors.Wrap(err, "getChain error")
	}

	txn := dbstore.SECRET_DB.NewTransaction()
	defer txn.Rollback()

	runtime := contracts.NewRuntimeWithVersion(0, time.Now(), txn, chaintypes.UniAddr{}, chaintypes.UniAddr{}, module.NativeVersion)
	bq := batch.BatchQuery{Batch: *batch.NewBatch(runtime)}

	var dkgKey []byte
	if dkgInst != nil {
		if pub, ok := dkgInst.DkgPubKeys[chain.GetCipher()]; ok && pub != nil {
			dkgKey = pub.Byte()
		}
	}

	calls := make([]chaintypes.ChainCall, 0, len(ids))
	for _, id := range ids {
		hc, err := bq.HubcallsGet(id)
		if err != nil || hc == nil {
			return nil, errors.Wrapf(err, "get hubcall %d", id)
		}
		call, err := chain.TEECallToCall(hc, dkgKey)
		if err != nil {
			return nil, errors.Wrapf(err, "TEECallToCall hubcall %d", id)
		}
		calls = append(calls, *call)
	}

	mainChain, ok := chain.(chains.MainChainApi)
	if !ok {
		return nil, errors.Errorf("chain %d does not support BatchCall", chainId)
	}
	return mainChain.BatchCall("batch_all", calls)
}

// ── Per-chain AsyncBatchState ──────────────────────────────

func hubSyncIndexKey(chainId uint32) string {
	return fmt.Sprintf("tx_sync_transaction_%d", chainId)
}

type AsyncBatchState struct {
	LastSync int64
}

// IsHubSyncRuning 检查是否有任意链正在同步中。
func IsHubSyncRuning() bool {
	for _, ch := range chains.Chains {
		if isHubSyncRuningForChain(ch.ChainId()) {
			return true
		}
	}
	return false
}

func isHubSyncRuningForChain(chainId uint32) bool {
	tx, err := store.DBGetJson[AsyncBatchState](store.SECRET_DB, GLOABL_STATE, hubSyncIndexKey(chainId))
	if err != nil {
		LogWithTime("IsHubSyncRuning get error: " + err.Error())
		return false
	}
	if tx == nil {
		tx = &AsyncBatchState{}
	}
	return time.Now().Unix()-tx.LastSync <= 360
}

// HubSyncStep2 标记同步批次已开始。
func HubSyncStep2(chainId uint32, ids []uint64, txn *store.Txn) error {
	if len(ids) == 0 {
		return errors.New("HubSyncStep2: empty ids")
	}

	tx, err := store.TxnGetJson[AsyncBatchState](txn, store.ComboNamespaceKey(GLOABL_STATE, hubSyncIndexKey(chainId)))
	if err != nil {
		return err
	}
	if tx == nil {
		tx = &AsyncBatchState{}
	}

	// 标记同步开始：isHubSyncRuningForChain 依赖 LastSync 判断“同步进行中”，
	// 若不设置，其他 proposer 会对同一批 batch 重复发起 SyncTxStart（重复聚合/提交）。
	tx.LastSync = time.Now().Unix()
	return store.TxnSetJson(txn, store.ComboNamespaceKey(GLOABL_STATE, hubSyncIndexKey(chainId)), tx)
}

// HubSyncEnd 标记同步批次已结束。
func HubSyncEnd(chainId uint32, ids []uint64, isSeccseed bool, txn *store.Txn) error {
	if len(ids) == 0 {
		return errors.New("HubSyncEnd: empty ids")
	}

	tx, err := store.TxnGetJson[AsyncBatchState](txn, store.ComboNamespaceKey(GLOABL_STATE, hubSyncIndexKey(chainId)))
	if err != nil {
		return err
	}
	if tx == nil {
		return errors.Errorf("HubSyncEnd: no active sync for chain %d", chainId)
	}

	if isSeccseed {
		tx.LastSync = 0
	} else {
		// 失败 60 秒后重新尝试
		tx.LastSync = time.Now().Unix() - 300
	}
	return store.TxnSetJson(txn, store.ComboNamespaceKey(GLOABL_STATE, hubSyncIndexKey(chainId)), tx)
}

// ── targetSync: 检测待同步交易并触发共识 ──────────────────────────

// targetSync 检查 batch 中是否有待同步的 hub call，如有则构建 SysCall_SyncTxStart 提交。
func (app *TeeChain) targetSyncRelay() bool {
	if app.dkg == nil {
		return false
	}

	if IsHubSyncRuning() {
		return false
	}

	height := app.state.Height
	txn := dbstore.SECRET_DB.NewTransaction()
	defer txn.Rollback()

	runtime := contracts.NewRuntimeWithVersion(height, time.Now(), txn, chaintypes.UniAddr{
		T: 0,
		V: []byte{},
	}, chaintypes.UniAddr{
		V: app.dkg.SystemPubKey().Byte(),
	}, module.NativeVersion)

	ins := batch.NewBatch(runtime)
	hubBatch, err := batch.BatchQuery{Batch: *ins}.PackCallToSunc()
	if err != nil || hubBatch == nil || len(hubBatch.IDs) == 0 {
		return false
	}

	util.LogWithYellow("RELAY-WORKER", "found pending sync tx, chainId:", hubBatch.ChainID, "ids:", hubBatch.IDs)

	// 构建 SysCall_SyncTxStart 触发共识
	tx := &model.SysCall{
		Payload: &model.SysCall_SyncTxStart{
			SyncTxStart: &model.SyncTxStart{
				ChainId: hubBatch.ChainID,
				Ids:     hubBatch.IDs,
				Time:    height,
			},
		},
	}
	if err := app.SubmitCallFromNode(tx); err != nil {
		util.LogWithRed("RELAY-WORKER", "submit SyncTxStart failed:", err)
		return false
	}

	util.LogWithYellow("RELAY-WORKER", "submitted SyncTxStart to trigger consensus, chainId:", hubBatch.ChainID)
	return true
}
