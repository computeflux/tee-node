package sidechain

import (
	"bytes"
	"fmt"

	"github.com/centrifuge/go-substrate-rpc-client/v4/types"
	abci "github.com/cometbft/cometbft/abci/types"
	"github.com/pkg/errors"
	"github.com/wetee-dao/tee-dsecret/pkg/model"
	"github.com/wetee-dao/tee-dsecret/pkg/model/protoio"
)

func (app *SideChain) FinalizeTx(txs [][]byte, txn *model.Txn, height int64, proposer []byte) ([]*abci.ExecTxResult, error) {
	res := []*abci.ExecTxResult{}
	hubCalls := make([]*model.HubCall, 0, len(txs))
	var txIndex int64 = 0

	for _, txbt := range txs {
		tx := new(model.Tx)
		err := protoio.ReadMessage(bytes.NewBuffer(txbt), tx)
		if err != nil {
			return nil, err
		}

		// 所有交易必须验证签名
		call, err := model.VerifyTxSigner(tx)
		if err != nil {
			return nil, errors.Wrap(err, "verify tx signer")
		}

		switch p := call.Payload.(type) {
		case *model.SysCall_Empty:
			LogWithTime("Empty TX:", p.Empty)
		case *model.SysCall_EpochStart: // set epoch last time
			err := app.SetEpochStatus(p.EpochStart)
			if err != nil {
				return nil, err
			}
		case *model.SysCall_EpochEnd:
			app.calcValidatorUpdates(p.EpochEnd) // calc validator updates
			err = app.SetEpoch(p.EpochEnd, txn)  // set epoch and validators
			if err != nil {
				return nil, err
			}
			err = app.SetEpochStatus(0)
			if err != nil {
				return nil, err
			}
		case *model.SysCall_SyncTxStart: // start hub sync tx
			txIndex = p.SyncTxStart
			err = HubSyncStep2(p.SyncTxStart, txn)
			if err != nil {
				return nil, err
			}
		case *model.SysCall_SyncTxEnd: // end hub sync tx
			err = HubSyncEnd(p.SyncTxEnd, txn)
			if err != nil {
				return nil, err
			}
			// TODO: deleteTxIndexStore currently runs outside the block transaction.
			// If FinalizeBlock rolls back, the tx_index store is already deleted,
			// leaving state inconsistent. It should be performed through `txn`
			// so it is atomic with the block.
			deleteTxIndexStore(p.SyncTxEnd)
		case *model.SysCall_SyncTxRetry: // retry hub sync tx，重新收集签名
			if app.dkg == nil {
				LogWithTime("SyncTxRetry", "dkg is nil, skipping retry for txIndex:", p.SyncTxRetry)
				break
			}

			// 从存储加载该交易的 hubCalls
			baseKey := TxIndexPrefix + fmt.Sprint(p.SyncTxRetry)
			stored, err := model.GetJson[hubCallsStore](GLOABL_STATE, baseKey+TxIndexHubCallsSuffix)
			if err != nil || stored == nil || len(stored.HubCalls) == 0 {
				LogWithTime("SyncTxRetry", "hubCalls not found for txIndex:", p.SyncTxRetry)
				break
			}

			// 清除旧的部分签名，以便重新收集
			_ = app.DeleteSigOfTx(p.SyncTxRetry)

			// 重新发起部分签名收集：本节点向当前 proposer 发送部分签名，其他节点同样会在 FinalizeTx 中发送
			err = app.sendPartialSign(stored.HubCalls[0].ChainId, p.SyncTxRetry, stored.HubCalls, app.ProposerAddressToNodeKey(proposer))
			if err != nil {
				return nil, errors.Wrap(err, "SyncTxRetry: sendPartialSign")
			}
		case *model.SysCall_HubCall: // add hub call
			err := app.finalizeHubCall(p.HubCall, txn)
			if err != nil {
				return nil, err
			}

			hubCalls = append(hubCalls, p.HubCall)
		case *model.SysCall_Contract:
			err = app.ContractMutation(tx.GetCaller(), tx.GetCallerType(), txn, height, p.Contract)
			if err != nil {
				return nil, errors.Wrap(err, "contract mutation error")
			}
		default:
			return nil, errors.New("invalid tx type")
		}

		res = append(res, &abci.ExecTxResult{Code: uint32(abci.CodeTypeOK)})
	}

	// if hub tx, send partial sign
	if txIndex > 0 && len(hubCalls) > 0 && hubCalls[0] != nil && app.dkg != nil {
		err := app.sendPartialSign(hubCalls[0].ChainId, txIndex, hubCalls, app.ProposerAddressToNodeKey(proposer))
		if err != nil {
			return nil, err
		}
	}
	return res, nil
}

func (app *SideChain) finalizeHubCall(hub *model.HubCall, txn *model.Txn) error {
	for _, callWrap := range hub.Call {
		switch tx := callWrap.Tx.(type) {
		case *model.TeeCall_PodStart:
			continue
		case *model.TeeCall_PodMint:
			continue
		case *model.TeeCall_UploadSecret:
			upload := tx.UploadSecret
			user := types.H160(upload.User)
			err := app.SaveSecret(user, upload.Index, upload.Data, txn)
			if err != nil {
				return errors.Wrap(err, "finalizeHubCall SaveSecret")
			}
		case *model.TeeCall_InitDisk:
			initDisk := tx.InitDisk
			user := types.H160(initDisk.User)
			err := app.SaveDiskKey(user, initDisk.Index, initDisk.Data, txn)
			if err != nil {
				return errors.Wrap(err, "finalizeHubCall InitDisk")
			}
		default:
			return errors.New("finalizeHubCall invalid tx type")
		}
	}
	return nil
}
