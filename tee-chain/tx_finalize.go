package teechain

import (
	"encoding/hex"
	"fmt"
	"sort"
	"time"

	abci "github.com/cometbft/cometbft/abci/types"
	"github.com/computeflux/tee-provider/pkg/model"
	"github.com/computeflux/tee-provider/pkg/model/protoio"
	dbstore "github.com/computeflux/tee-provider/pkg/types"
	"github.com/computeflux/tee-provider/pkg/types/chaintypes"
	"github.com/computeflux/tee-provider/pkg/util"
	"github.com/computeflux/tee-provider/tee-chain/relay"
	contracts "github.com/computeflux/tee-provider/tee-chain/runtime"
	"github.com/computeflux/tee-provider/tee-chain/runtime/module/batch"
	"github.com/computeflux/tee-provider/tee-chain/runtime/wasm"
	"github.com/pkg/errors"
)

// newPipelineEvent 构造一个交易级管道事件（属性按 key 排序后追加，保证顺序确定；
// 所有属性标记 Index=true 供 CometBFT TxIndexer 索引；事件 type 统一小写下划线命名）。
func newPipelineEvent(eventType string, attrs map[string]string) abci.Event {
	event := abci.Event{Type: eventType}
	if len(attrs) == 0 {
		return event
	}
	keys := make([]string, 0, len(attrs))
	for k := range attrs {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		event.Attributes = append(event.Attributes, abci.EventAttribute{
			Key:   k,
			Value: attrs[k],
			Index: true,
		})
	}
	return event
}

func (app *TeeChain) FinalizeTx(txs [][]byte, txn *dbstore.Txn, height int64, ts time.Time, proposer []byte) ([]*abci.ExecTxResult, error) {
	res := []*abci.ExecTxResult{}

	for _, txbt := range txs {
		tx := new(model.Tx)
		err := protoio.Unmarshal(txbt, tx)
		if err != nil {
			return nil, err
		}

		call, err := model.VerifyTxSigner(tx)
		if err != nil {
			return nil, errors.Wrap(err, "verify tx signer")
		}

		// 每笔交易的事件管道：合约执行期间累积，最终挂到 ExecTxResult.Events
		var txEvents []abci.Event

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
			// 空 ids 会让 DeleteSigOfTx(Ids[0]) 越界 panic；
			// 且 HubSyncStep2 也会拒绝，若不拦截 FinalizeBlock 将报错导致链停摆
			if len(p.SyncTxStart.Ids) == 0 {
				return nil, errors.New("sync tx start: empty ids")
			}
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
			if p.SyncTxEnd.IsSeccseed {
				runtime, err := app.buildContractRuntime(txn, height, ts)
				if err != nil {
					util.LogWithRed("FinalizeTx", "buildContractRuntime error:", err)
				} else {
					bm := batch.BatchSecret{Batch: *batch.NewBatch(runtime)}
					status := batch.HubCallDone
					for _, id := range p.SyncTxEnd.Ids {
						if err := bm.SetHubCallStatus(id, status); err != nil {
							util.LogWithRed("FinalizeTx", "SetHubCallStatus error for id", id, ":", err)
						}
						// hub call 已确认完成，清除错误池记录（若存在）
						txErrPool.recordSuccess(p.SyncTxEnd.ChainId, id)
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
				runtime, err := app.buildContractRuntime(txn, height, ts)
				if err != nil {
					util.LogWithRed("Finalize TX", "buildContractRuntime error:", err)
				} else if err := app.relay.ApplyRelayEvent(ev, runtime); err != nil {
					util.LogWithRed("Finalize TX", errors.Wrap(err, "apply relay event"))
					return nil, errors.Wrap(err, "contract mutation error")
				} else {
					// 管道事件：relay 同步完成（交易详情页展示跨链同步的数据基础）
					txEvents = append(txEvents, newPipelineEvent("relay_synced", map[string]string{
						"chain_id":    fmt.Sprint(ev.ChainID),
						"nonce":       fmt.Sprint(ev.Nonce),
						"block_num":   fmt.Sprint(ev.BlockNum),
						"contract":    string(ev.TargetContract),
						"event_type":  string(ev.EventType),
					}))
					// 合约执行期间通过 EmitEvent 发射的事件一并挂上
					txEvents = append(txEvents, runtime.TakeEvents()...)
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

			var mutationEvents []abci.Event
			mutationEvents, err = app.ContractMutation(caller, callerType, txn, ts, height, p.Contract)
			if err != nil {
				util.LogWithRed("Finalize TX", errors.Wrap(err, "contract mutation error"))
				return nil, errors.Wrap(err, "contract mutation error")
			}
			// 管道事件：合约调用完成（交易详情页展示合约调用的数据基础）
			txEvents = append(txEvents, newPipelineEvent("contract_call", map[string]string{
				"contract": string(p.Contract.GetName()),
				"method":   hex.EncodeToString(p.Contract.GetMethod()),
				"caller":   hex.EncodeToString(caller),
				"nonce":    fmt.Sprint(txNonce),
			}))
			// 合约执行期间通过 EmitEvent 发射的事件一并挂上
			txEvents = append(txEvents, mutationEvents...)

			if err := TxnSetCallerNonce(txn, caller, callerType, txNonce); err != nil {
				util.LogWithRed("Finalize TX", errors.Wrap(err, "Nonce update caller nonce"))
				return nil, errors.Wrap(err, "contract mutation error")
			}
		default:
			return nil, errors.New("invalid tx type")
		}

		res = append(res, &abci.ExecTxResult{Code: uint32(abci.CodeTypeOK), Events: txEvents})
	}

	// 同步链上运行时版本状态（合约已通过 RuntimeUpgrade 完成版本管理）
	if v, _ := GetCurrentRuntimeVersion(txn); v != app.state.RuntimeVersion {
		app.state.RuntimeVersion = v

		// 刷新 wasm 包级字节缓存：避免后续每次合约调用都重新查 gov + 解压
		if err := wasm.RefreshCurrentVersion(v, getGovQuery(txn)); err != nil {
			util.LogError("FinalizeTx", " refresh wasm cache error", err)
		}

		// 执行合约版本迁移：通过 ContractMutation 路由，WASM 升级后可正确调用对应版本的 Migration
		if pub, err := app.GetDkgPubkey(); err != nil || pub == nil {
			util.LogError("FinalizeTx migration", errors.New("get dkg pubkey"))
		} else {
			versionBytes := []byte{byte(v), byte(v >> 8), byte(v >> 16), byte(v >> 24)}
			migrationCall := &model.ContractCall{
				Name:   []byte("gov"),
				Method: []byte{0xa6, 0x2c, 0x84, 0xf8}, // migration 方法选择器
				Args:   [][]byte{versionBytes},
			}
			if _, err := app.ContractMutation(pub.ToBytes(), 0, txn, ts, height, migrationCall); err != nil {
				util.LogError("FinalizeTx migration", err)
			} else {
				util.LogOk("FinalizeTx migration", "success")
			}
		}
	}

	return res, nil
}

func (app *TeeChain) buildContractRuntime(txn *dbstore.Txn, height int64, ts time.Time) (*contracts.Runtime, error) {
	pub, err := app.GetDkgPubkey()
	if err != nil {
		return nil, errors.Wrap(err, "get dkg pubkey for relay sync")
	}
	if pub == nil {
		return nil, errors.New("get dkg pubkey for relay sync: nil pubkey")
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
