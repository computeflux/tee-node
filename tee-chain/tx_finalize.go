package teechain

import (
	"bytes"
	"fmt"
	"time"

	"github.com/centrifuge/go-substrate-rpc-client/v4/types"
	abci "github.com/cometbft/cometbft/abci/types"
	"github.com/computeflux/tee-provider/pkg/chaintypes"
	"github.com/computeflux/tee-provider/pkg/domain"
	"github.com/computeflux/tee-provider/pkg/model"
	"github.com/computeflux/tee-provider/pkg/model/protoio"
	dbstore "github.com/computeflux/tee-provider/pkg/store"
	"github.com/computeflux/tee-provider/pkg/util"
	"github.com/computeflux/tee-provider/tee-chain/contracts"
	"github.com/computeflux/tee-provider/tee-chain/relay"
	"github.com/pkg/errors"
)

func (app *TeeChain) FinalizeTx(txs [][]byte, txn *dbstore.Txn, height int64, ts time.Time, proposer []byte) ([]*abci.ExecTxResult, error) {
	res := []*abci.ExecTxResult{}

	// Per-chain grouping for multi-main-chain async sync.
	chainHubCalls := make(map[uint32][]*model.HubCall)
	chainTxIndex := make(map[uint32]int64)

	for _, txbt := range txs {
		tx := new(model.Tx)
		err := protoio.ReadMessage(bytes.NewBuffer(txbt), tx)
		if err != nil {
			return nil, err
		}

		// 所有交易必须验证签名
		call, err := domain.VerifyTxSigner(tx)
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
			chainTxIndex[p.SyncTxStart.ChainId] = p.SyncTxStart.TxIndex
			err = HubSyncStep2(p.SyncTxStart.TxIndex, p.SyncTxStart.ChainId, txn)
			if err != nil {
				return nil, err
			}
		case *model.SysCall_SyncTxEnd: // end hub sync tx
			err = HubSyncEnd(p.SyncTxEnd.TxIndex, p.SyncTxEnd.ChainId, txn)
			if err != nil {
				return nil, err
			}
			err = markChainDone(p.SyncTxEnd.TxIndex, p.SyncTxEnd.ChainId, txn)
			if err != nil {
				return nil, err
			}
			// Only delete tx_index storage when ALL chains for this txIndex are done.
			allDone, err := checkAllChainsDone(p.SyncTxEnd.TxIndex, txn)
			if err != nil {
				return nil, err
			}
			if allDone {
				if err := deleteTxIndexStore(p.SyncTxEnd.TxIndex, txn); err != nil {
					return nil, err
				}
				// 同步成功后清理部分签名（不是共识状态，直接删全局 DB 即可）
				_ = app.DeleteSigOfTx(p.SyncTxEnd.TxIndex)
			}
		case *model.SysCall_SyncTxRetry: // retry hub sync tx，重新收集签名
			if app.dkg == nil {
				LogWithTime("SyncTxRetry", "dkg is nil, skipping retry for txIndex:", p.SyncTxRetry.TxIndex, "chainId:", p.SyncTxRetry.ChainId)
				break
			}

			// 从存储加载该交易的 hubCalls（按 chain 隔离）
			baseKey := TxIndexPrefix + fmt.Sprint(p.SyncTxRetry.TxIndex) + "_" + fmt.Sprint(p.SyncTxRetry.ChainId)
			stored, err := dbstore.GetJson[hubCallsStore](GLOABL_STATE, baseKey+TxIndexHubCallsSuffix)
			if err != nil || stored == nil || len(stored.HubCalls) == 0 {
				LogWithTime("SyncTxRetry", "hubCalls not found for txIndex:", p.SyncTxRetry.TxIndex, "chainId:", p.SyncTxRetry.ChainId)
				break
			}

			// 清除旧的部分签名，以便重新收集
			_ = app.DeleteSigOfTxChain(p.SyncTxRetry.TxIndex, p.SyncTxRetry.ChainId)

			// 重新发起部分签名收集：本节点向当前 proposer 发送部分签名，其他节点同样会在 FinalizeTx 中发送
			propNode, err := app.ProposerAddressToNodeKey(proposer)
			if err != nil {
				util.LogWithRed("SyncTxRetry", "ProposerAddressToNodeKey error:", err)
				break
			}
			err = app.sendPartialSign(p.SyncTxRetry.ChainId, p.SyncTxRetry.TxIndex, stored.HubCalls, propNode, txn)
			if err != nil {
				util.LogWithRed("SyncTxRetry", "sendPartialSign error:", err)
				break
			}
		// case *model.SysCall_HubCall: // add hub call
		// 	if p.HubCall == nil {
		// 		return nil, errors.New("HubCall payload is nil")
		// 	}

		// 	err := app.finalizeHubCall(p.HubCall, txn)
		// 	if err != nil {
		// 		return nil, err
		// 	}

		// 	// Group hubCalls by chainId (extracted from the first TeeCall).
		// 	if len(p.HubCall.Call) > 0 {
		// 		cid := p.HubCall.Call[0].ChainId
		// 		chainHubCalls[cid] = append(chainHubCalls[cid], p.HubCall)
		// 	}
		case *model.SysCall_RelaySync:
			ev := relay.FromRelaySysCall(p.RelaySync)
			if app.relay != nil {
				pub, err := app.GetDkgPubkey()
				if err != nil || pub == nil {
					return nil, errors.Wrap(err, "get dkg pubkey for relay sync")
				}
				version, err := GetRuntimeVersionAtHeight(height)
				if err != nil {
					return nil, errors.Wrap(err, "get runtime version for relay sync")
				}
				runtime := contracts.NewRuntimeWithVersion(height, ts, txn, chaintypes.UniAddr{
					V: pub.ToBytes(),
				}, chaintypes.UniAddr{
					V: pub.ToBytes(),
				}, version)

				if err := app.relay.ApplyRelayEvent(ev, runtime); err != nil {
					util.LogWithRed("Finalize TX", errors.Wrap(err, "apply relay event"))
					return nil, errors.Wrap(err, "contract mutation error")
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
			address := chaintypes.UniAddr{T: callerType, V: caller}

			pub, err := app.GetDkgPubkey()
			if err != nil {
				return nil, errors.Wrap(err, "get dkg pubkey for runtime upgrade")
			}
			sudo := chaintypes.UniAddr{V: pub.ToBytes()}
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

			info := &domain.RuntimeVersionInfo{
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
				return nil, errors.Wrap(err, "contract mutation error")
			}

			// 合约执行成功后，原子更新 nonce
			if err := TxnSetCallerNonce(txn, caller, callerType, txNonce); err != nil {
				util.LogWithRed("Finalize TX", errors.Wrap(err, "Nonce update caller nonce"))
				return nil, errors.Wrap(err, "contract mutation error")
			}
		default:
			return nil, errors.New("invalid tx type")
		}

		res = append(res, &abci.ExecTxResult{Code: uint32(abci.CodeTypeOK)})
	}

	// Send partial signs per chain.
	// Build txIndex -> []chainId mapping and persist it atomically via txn
	// so that SyncTxEnd knows when all chains are done.
	txIndexChains := make(map[int64][]uint32)
	for cid := range chainHubCalls {
		txIdx, ok := chainTxIndex[cid]
		if !ok || txIdx <= 0 {
			// Fallback: if no SyncTxStart for this chain, skip (should not happen in normal flow).
			continue
		}
		txIndexChains[txIdx] = append(txIndexChains[txIdx], cid)
	}

	for txIdx, cids := range txIndexChains {
		if err := saveTxIndexChains(txIdx, cids, txn); err != nil {
			return nil, errors.Wrap(err, "saveTxIndexChains")
		}
	}

	if app.dkg != nil {
		propNode, err := app.ProposerAddressToNodeKey(proposer)
		if err != nil {
			util.LogWithRed("FinalizeTx", "ProposerAddressToNodeKey error:", err)
		} else {
			for cid, calls := range chainHubCalls {
				txIdx, ok := chainTxIndex[cid]
				if !ok || txIdx <= 0 {
					continue
				}
				if len(calls) == 0 || calls[0] == nil {
					continue
				}
				if err := app.sendPartialSign(cid, txIdx, calls, propNode, txn); err != nil {
					util.LogWithRed("FinalizeTx", "sendPartialSign error for chain", cid, ":", err)
				}
			}
		}
	}

	return res, nil
}

func (app *TeeChain) finalizeHubCall(hub *model.HubCall, txn *dbstore.Txn) error {
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
