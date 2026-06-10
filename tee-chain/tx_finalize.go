package teechain

import (
	"bytes"
	"time"

	abci "github.com/cometbft/cometbft/abci/types"
	"github.com/computeflux/tee-provider/pkg/chaintypes"
	"github.com/computeflux/tee-provider/pkg/domain"
	"github.com/computeflux/tee-provider/pkg/model"
	"github.com/computeflux/tee-provider/pkg/model/protoio"
	dbstore "github.com/computeflux/tee-provider/pkg/store"
	"github.com/computeflux/tee-provider/pkg/util"
	"github.com/computeflux/tee-provider/tee-chain/contracts"
	"github.com/computeflux/tee-provider/tee-chain/contracts/module/batch"
	"github.com/computeflux/tee-provider/tee-chain/relay"
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
		case *model.SysCall_RuntimeUpgrade:
			upgrade := p.RuntimeUpgrade
			if upgrade == nil {
				return nil, errors.New("runtime upgrade is nil")
			}

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
			app.state.RuntimeVersion = upgrade.Version
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

			if err := TxnSetCallerNonce(txn, caller, callerType, txNonce); err != nil {
				util.LogWithRed("Finalize TX", errors.Wrap(err, "Nonce update caller nonce"))
				return nil, errors.Wrap(err, "contract mutation error")
			}
		default:
			return nil, errors.New("invalid tx type")
		}

		res = append(res, &abci.ExecTxResult{Code: uint32(abci.CodeTypeOK)})
	}

	return res, nil
}

func (app *TeeChain) buildContractRuntime(txn *dbstore.Txn, height int64, ts time.Time) (*contracts.Runtime, error) {
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

	return runtime, nil
}
