package teechain

import (
	"bytes"
	"time"

	abci "github.com/cometbft/cometbft/abci/types"
	"github.com/computeflux/tee-provider/pkg/domain"
	"github.com/computeflux/tee-provider/pkg/model"
	"github.com/computeflux/tee-provider/pkg/model/protoio"
	dbstore "github.com/computeflux/tee-provider/pkg/types"
	"github.com/computeflux/tee-provider/pkg/types/chaintypes"
	"github.com/computeflux/tee-provider/pkg/util"
	"github.com/computeflux/tee-provider/tee-chain/relay"
	contracts "github.com/computeflux/tee-provider/tee-chain/runtime"
	"github.com/computeflux/tee-provider/tee-chain/runtime/module/batch"
	"github.com/pkg/errors"
)

func (app *TeeChain) FinalizeTx(txs [][]byte, txn *dbstore.Txn, height int64, ts time.Time, proposer []byte) ([]*abci.ExecTxResult, error) {
	res := []*abci.ExecTxResult{}

	for _, txbt := range txs {
		tx := new(model.Tx)
		err := protoio.ReadMessage(bytes.NewBuffer(txbt), tx)
		if err != nil {
			return nil, err
		}

		call, err := domain.VerifyTxSigner(tx)
		if err != nil {
			return nil, errors.Wrap(err, "verify tx signer")
		}

		switch p := call.Payload.(type) {
		case *model.SysCall_Empty:
			LogWithTime("📭 Empty TX:", p.Empty)
		case *model.SysCall_EpochStart:
			err := app.SetEpochStatus(p.EpochStart)
			if err != nil {
				return nil, err
			}
		case *model.SysCall_EpochEnd:
			app.calcValidatorUpdates(p.EpochEnd)
			err = app.SetEpoch(p.EpochEnd, txn)
			if err != nil {
				return nil, err
			}
			err = app.SetEpochStatus(0)
			if err != nil {
				return nil, err
			}
		case *model.SysCall_SyncTxStart:
			// 清理所有共识缓存（纳入 txn，与区块提交原子化）
			if err := app.DeleteSigOfTx(txn, int64(p.SyncTxStart.Ids[0])); err != nil {
				return nil, errors.Wrap(err, "delete sigs at sync tx start")
			}

			err = HubSyncStep2(p.SyncTxStart.ChainId, p.SyncTxStart.Ids, txn)
			if err != nil {
				return nil, err
			}

			if app.dkg != nil {
				propNode, propErr := app.ProposerAddressToNodeKey(proposer)
				if propErr != nil {
					util.LogWithRed("FinalizeTx", "ProposerAddressToNodeKey error:", propErr)
				} else {
					if err := app.sendPartialSignByIds(p.SyncTxStart.ChainId, p.SyncTxStart.Ids, propNode, txn); err != nil {
						util.LogWithRed("FinalizeTx", "sendPartialSignByIds error for chain", p.SyncTxStart.ChainId, ":", err)
					}
				}
			}
		case *model.SysCall_SyncTxEnd:
			err = HubSyncEnd(p.SyncTxEnd.ChainId, p.SyncTxEnd.Ids, p.SyncTxEnd.IsSeccseed, txn)
			if err != nil {
				return nil, err
			}

			if p.SyncTxEnd.IsSeccseed {
				util.LogWithPurple("FinalizeTx", "SyncTxEnd IsSeccseed", p.SyncTxEnd.IsSeccseed)
			} else {
				util.LogWithRed("FinalizeTx", "SyncTxEnd IsSeccseed", p.SyncTxEnd.IsSeccseed)
			}

			// 批量更新 batch 中 hub call 的状态
			{
				runtime, _ := app.buildContractRuntime(txn, height, ts)
				bm := batch.BatchSecret{Batch: *batch.NewBatch(runtime)}
				status := batch.HubCallDone

				// 只更新成功,失败交易后面重试,主链有防重放
				if p.SyncTxEnd.IsSeccseed {
					for _, id := range p.SyncTxEnd.Ids {
						if err := bm.SetHubCallStatus(id, status); err != nil {
							util.LogWithRed("FinalizeTx", "SetHubCallStatus error for id", id, ":", err)
						}
					}
				}
			}

			// 清理签名和临时存储的 batch call 数据（纳入 txn，与区块提交原子化）
			if len(p.SyncTxEnd.Ids) > 0 {
				if err := app.DeleteSigOfTx(txn, int64(p.SyncTxEnd.Ids[0])); err != nil {
					return nil, errors.Wrap(err, "delete sigs at sync tx end")
				}
				if err := deleteBatchCallStore(txn, int64(p.SyncTxEnd.Ids[0]), p.SyncTxEnd.ChainId); err != nil {
					return nil, errors.Wrap(err, "delete batch call store")
				}
			}
		case *model.SysCall_RelaySync:
			ev := relay.FromRelaySysCall(p.RelaySync)
			if app.relay != nil {
				runtime, _ := app.buildContractRuntime(txn, height, ts)
				if err := app.relay.ApplyRelayEvent(ev, runtime); err != nil {
					util.LogWithRed("Finalize TX", errors.Wrap(err, "apply relay event"))
					return nil, errors.Wrap(err, "contract mutation error")
				}
			}
		case *model.SysCall_Contract:
			caller := tx.GetCaller()
			callerType := tx.GetCallerType()
			txNonce := p.Contract.GetNonce()

			currentNonce, err := TxnGetCurrentCallerNonce(txn, caller, callerType)
			if err != nil {
				return nil, errors.Wrap(err, "get caller nonce")
			}

			expectedNonce := currentNonce + 1
			if txNonce != expectedNonce && callerType != 0 {
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

			if err := TxnSetCallerNonce(txn, caller, callerType, txNonce); err != nil {
				util.LogWithRed("Finalize TX", errors.Wrap(err, "Nonce update caller nonce"))
				return nil, errors.Wrap(err, "contract mutation error")
			}
		default:
			return nil, errors.New("invalid tx type")
		}

		res = append(res, &abci.ExecTxResult{Code: uint32(abci.CodeTypeOK)})
	}

	// 同步链上运行时版本状态（合约已通过 RuntimeUpgrade 完成版本管理）
	if v, _ := GetCurrentRuntimeVersion(txn); v != app.state.RuntimeVersion {
		app.state.RuntimeVersion = v
	}

	return res, nil
}

func (app *TeeChain) buildContractRuntime(txn *dbstore.Txn, height int64, ts time.Time) (*contracts.Runtime, error) {
	pub, err := app.GetDkgPubkey()
	if err != nil || pub == nil {
		return nil, errors.Wrap(err, "get dkg pubkey for relay sync")
	}

	version, err := GetRuntimeVersionAtHeight(txn, height)
	if err != nil {
		return nil, errors.Wrap(err, "get runtime version for relay sync")
	}

	runtime := contracts.NewRuntimeWithVersion(height, ts, txn, chaintypes.UniAddr{
		V: pub.ToBytes(),
	}, chaintypes.UniAddr{
		V: pub.ToBytes(),
	}, version)

	return runtime, nil
}
