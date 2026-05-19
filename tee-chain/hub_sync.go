// tx_sync.go 用于记录提交到主链的交易：SyncToHub、tx_index_ 存储（call/hubCalls）、
// SyncTxStart/End 状态（HubSyncIndexKey、HubSyncStep1/2/End、IsHubSyncRuning）及清理逻辑。
package teechain

import (
	"fmt"
	"time"

	"github.com/computeflux/tee-provider/pkg/chains"
	"github.com/computeflux/tee-provider/pkg/dkg"
	"github.com/computeflux/tee-provider/pkg/model"
	"github.com/computeflux/tee-provider/pkg/util"
	"github.com/pkg/errors"
)

// Submit sync tx to polkadot hub
func (s *TeeChain) SyncToHub(txIndex int64, sigs [][]byte) error {
	baseKey := TxIndexPrefix + fmt.Sprint(txIndex)
	call, err := model.GetCodec[model.ChainCall](GLOABL_STATE, baseKey+TxIndexCallSuffix)
	if err != nil || call == nil {
		util.LogWithRed("Sync to polkadot hub", "error: call not found call data")
		return errors.New("sync to polkadot hub error: call not found call data")
	}

	stored, err := model.GetJson[hubCallsStore](GLOABL_STATE, baseKey+TxIndexHubCallsSuffix)
	if err != nil || stored == nil || len(stored.HubCalls) == 0 {
		LogWithTime("SyncToHub", "hubCalls not found for txIndex:", txIndex)
		return errors.New("sync to polkadot hub error: hubCalls not found")
	}

	chainId := stored.HubCalls[0].ChainId
	chain, err := s.getChain(chainId)
	if err != nil {
		return errors.Wrap(err, "getChain error")
	}

	// Aggregate signature
	util.LogWithGray("Sync to polkadot hub", "sync sigs = ", len(sigs))
	signer := dkg.NewDssSigner(s.dkg, txIndex, chain.GetCipher())
	signer.SetSigs(sigs)

	// submit sync tx to polkadot hub
	err = chains.MainChain.SignAndSubmit(signer, *call, false, 0)
	if err != nil {
		util.LogWithRed("Sync to polkadot hub", "error => ", err.Error())
		fmt.Println("                    ", " SS58 => ", s.dkg.DkgPubKeys[chain.GetCipher()])
		fmt.Println("                    ", " SYNC at batch tx id", txIndex)
		// 提交失败时，将 SyncTxRetry 提交到 mempool，由下一轮 proposer 打包进块
		// （只有本节点会执行 SyncToHub，故只提交一次；mempool 会广播给其他节点）
		util.LogWithYellow("Sync to polkadot hub", "submitting SyncTxRetry to mempool", "txIndex:", txIndex)

		tx := &model.SysCall{
			Payload: &model.SysCall_SyncTxRetry{
				SyncTxRetry: txIndex,
			},
		}

		return s.SubmitCallFromNode(tx)
	}

	util.LogWithGreen("Sync to polkadot hub", "success at batch tx id", fmt.Sprint(txIndex))
	// 仅成功后提交 SyncHubEnd，使所有节点在 FinalizeTx 处理时统一清理 tx_index_ 储存

	tx := &model.SysCall{
		Payload: &model.SysCall_SyncTxEnd{
			SyncTxEnd: txIndex,
		},
	}

	return s.SubmitCallFromNode(tx)
}

// deleteTxIndexStore 删除 tx_index_<id> 相关储存（call + hubCalls）。应在所有节点共识到提交成功（即处理 SyncTxEnd）时调用。
func deleteTxIndexStore(txIndex int64) {
	_ = model.DeletekeysByPrefix(GLOABL_STATE, TxIndexPrefix+fmt.Sprint(txIndex)+"_")
}

// sync transaction index
var HubSyncIndexKey = "tx_sync_transaction"

type AsyncBatchState struct {
	Going    int64
	Done     int64
	LastSync int64
}

// check sync is running
func IsHubSyncRuning() bool {
	tx, err := model.GetJson[AsyncBatchState](GLOABL_STATE, HubSyncIndexKey)
	if err != nil {
		// Fail open: on DB error, allow hub calls to be processed.
		LogWithTime("IsHubSyncRuning get error: " + err.Error())
		return false
	}
	if tx == nil {
		tx = &AsyncBatchState{
			Going: 0,
			Done:  0,
		}
	}

	return tx.Going > tx.Done && time.Now().Unix()-tx.LastSync <= 360
}

// sync transaction step1
func HubSyncStep1(s *TeeChain) ([]byte, error) {
	tx, err := model.GetJson[AsyncBatchState](GLOABL_STATE, HubSyncIndexKey)
	if err != nil {
		return nil, err
	}

	if tx == nil {
		tx = &AsyncBatchState{
			Going: 0,
			Done:  0,
		}
	}

	if tx.Going > tx.Done && time.Now().Unix()-tx.LastSync <= 360 {
		return nil, errors.New("sync step1 one transaction is runing")
	}

	ntx := &model.SysCall{
		Payload: &model.SysCall_SyncTxStart{
			SyncTxStart: tx.Going + 1,
		},
	}

	return s.GetCallBytesFromNode(ntx)
}

// sync transaction step2
func HubSyncStep2(i int64, txn *model.Txn) error {
	tx, err := model.TxnGetJson[AsyncBatchState](txn, model.ComboNamespaceKey(GLOABL_STATE, HubSyncIndexKey))
	if err != nil {
		return err
	}

	if tx == nil {
		tx = &AsyncBatchState{
			Going: 0,
			Done:  0,
		}
	}

	if tx.Going > tx.Done && time.Now().Unix()-tx.LastSync <= 360 {
		// return errors.New("sync step2 one transaction is runing")
	}

	tx.Going = i
	tx.Done = i - 1
	return model.TxnSetJson(txn, model.ComboNamespaceKey(GLOABL_STATE, HubSyncIndexKey), tx)
}

// sync transaction step3
func HubSyncEnd(i int64, txn *model.Txn) error {
	tx, err := model.TxnGetJson[AsyncBatchState](txn, model.ComboNamespaceKey(GLOABL_STATE, HubSyncIndexKey))
	if err != nil {
		return err
	}

	if i != tx.Going && time.Now().Unix()-tx.LastSync <= 360 {
		util.LogWithRed("SyncEnd", "i is not equal to tx.Going")
	}

	tx.Done = tx.Going
	tx.LastSync = time.Now().Unix()
	return model.TxnSetJson(txn, model.ComboNamespaceKey(GLOABL_STATE, HubSyncIndexKey), tx)
}
