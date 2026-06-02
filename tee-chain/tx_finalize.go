package teechain

import (
	"bytes"
	"fmt"
	"time"

	"github.com/centrifuge/go-substrate-rpc-client/v4/types"
	abci "github.com/cometbft/cometbft/abci/types"
	"github.com/computeflux/tee-provider/pkg/model"
	"github.com/computeflux/tee-provider/pkg/model/protoio"
	"github.com/computeflux/tee-provider/pkg/util"
	"github.com/computeflux/tee-provider/tee-chain/contracts"
	"github.com/computeflux/tee-provider/tee-chain/relay"
	"github.com/pkg/errors"
)

func (app *TeeChain) FinalizeTx(txs [][]byte, txn *model.Txn, height int64, ts time.Time, proposer []byte) ([]*abci.ExecTxResult, error) {
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
			LogWithTime("📭 Empty TX:", p.Empty)
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
			propNode, err := app.ProposerAddressToNodeKey(proposer)
			if err != nil {
				return nil, errors.Wrap(err, "SyncTxRetry: ProposerAddressToNodeKey")
			}
			err = app.sendPartialSign(stored.HubCalls[0].ChainId, p.SyncTxRetry, stored.HubCalls, propNode)
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
			caller := tx.GetCaller()
			callerType := tx.GetCallerType()
			txNonce := p.Contract.GetNonce()

			currentNonce, err := TxnGetCurrentCallerNonce(txn, caller, callerType)
			if err != nil {
				return nil, errors.Wrap(err, "get caller nonce")
			}

			expectedNonce := currentNonce + 1
			if txNonce != expectedNonce {
				return nil, errors.Errorf(
					"invalid nonce: expected %d, got %d (caller %x, type %d)",
					expectedNonce, txNonce, caller, callerType,
				)
			}

			err = app.ContractMutation(caller, callerType, txn, ts, height, p.Contract)
			if err != nil {
				util.LogWithRed("Finalize TX", errors.Wrap(err, "contract mutation error"))
				res = append(res, &abci.ExecTxResult{Code: uint32(abci.CheckTxType_Recheck)})
				continue
			}

			// 合约执行成功后，原子更新 nonce
			if err := TxnSetCallerNonce(txn, caller, callerType, txNonce); err != nil {
				util.LogWithRed("Finalize TX", errors.Wrap(err, "Nonce update caller nonce"))
				res = append(res, &abci.ExecTxResult{Code: uint32(abci.CheckTxType_Recheck)})
				continue
			}
		case *model.SysCall_RelaySync:
			ev := relay.FromRelaySysCall(p.RelaySync)
			if app.relay != nil {
				pub, _ := app.GetDkgPubkey()
				version, _ := GetRuntimeVersionAtHeight(height)
				runtime := contracts.NewRuntimeWithVersion(height, ts, txn, model.UniAddr{
					V: pub.ToBytes(),
				}, model.UniAddr{
					V: pub.ToBytes(),
				}, version)

				if err := app.relay.ApplyRelayEvent(ev, runtime); err != nil {
					util.LogWithRed("Finalize TX", errors.Wrap(err, "apply relay event"))
					res = append(res, &abci.ExecTxResult{Code: uint32(abci.CheckTxType_Recheck)})
					continue
				}
			}
		case *model.SysCall_RuntimeUpgrade:
			upgrade := p.RuntimeUpgrade
			if upgrade == nil {
				return nil, errors.New("runtime upgrade is nil")
			}

			// 权限校验：仅 system account 可提交 runtime upgrade
			caller := tx.GetCaller()
			callerType := tx.GetCallerType()
			address := model.UniAddr{T: callerType, V: caller}

			pub, err := app.GetDkgPubkey()
			if err != nil {
				return nil, errors.Wrap(err, "get dkg pubkey for runtime upgrade")
			}
			sudo := model.UniAddr{V: pub.ToBytes()}
			if !bytes.Equal(address.V, sudo.V) || address.T != sudo.T {
				return nil, errors.New("runtime upgrade: caller is not system account")
			}

			currentVersion, err := GetCurrentRuntimeVersion()
			if err != nil {
				return nil, errors.Wrap(err, "get current runtime version")
			}

			// 所有版本升级必须严格递增（包括 v0 也不能通过 RuntimeUpgrade 补充）
			expectedVersion := currentVersion + 1
			if upgrade.Version != expectedVersion {
				return nil, errors.Errorf(
					"runtime upgrade version invalid: expected %d, got %d",
					expectedVersion, upgrade.Version,
				)
			}
			if len(upgrade.WasmBytes) == 0 {
				return nil, errors.New("runtime upgrade: wasm_bytes is empty")
			}

			info := &model.RuntimeVersionInfo{
				Version:          upgrade.Version,
				ActivationHeight: height,
				CodeHash:         upgrade.CodeHash,
				WasmBytes:        upgrade.WasmBytes,
				Description:      upgrade.Description,
			}
			if err := ApplyRuntimeUpgrade(txn, info, height); err != nil {
				return nil, errors.Wrap(err, "apply runtime upgrade")
			}

			// 更新内存中的当前版本
			app.state.RuntimeVersion = upgrade.Version

		default:
			return nil, errors.New("invalid tx type")
		}

		res = append(res, &abci.ExecTxResult{Code: uint32(abci.CodeTypeOK)})
	}

	// if hub tx, send partial sign
	if txIndex > 0 && len(hubCalls) > 0 && hubCalls[0] != nil && app.dkg != nil {
		propNode, err := app.ProposerAddressToNodeKey(proposer)
		if err != nil {
			return nil, err
		}
		err = app.sendPartialSign(hubCalls[0].ChainId, txIndex, hubCalls, propNode)
		if err != nil {
			return nil, err
		}
	}
	return res, nil
}

func (app *TeeChain) finalizeHubCall(hub *model.HubCall, txn *model.Txn) error {
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
