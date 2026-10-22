package teechain

import (
	"time"

	"github.com/computeflux/tee-provider/pkg/chains"
	"github.com/computeflux/tee-provider/pkg/dkg"
	"github.com/computeflux/tee-provider/pkg/model"
	dbstore "github.com/computeflux/tee-provider/pkg/types"
	"github.com/computeflux/tee-provider/pkg/types/chaintypes"
	"github.com/computeflux/tee-provider/pkg/util"
	contracts "github.com/computeflux/tee-provider/tee-chain/runtime"
	"github.com/computeflux/tee-provider/tee-chain/runtime/module"
	"github.com/computeflux/tee-provider/tee-chain/runtime/module/batch"
	"github.com/pkg/errors"
)

// sendPartialSignByIds 根据 hub call IDs 加载数据并发送部分签名。
func (app *TeeChain) sendPartialSignByIds(chainId uint32, ids []uint64, proposer []byte, txn *dbstore.Txn) error {
	if len(ids) == 0 {
		return errors.New("sendPartialSignByIds: empty ids")
	}

	if app.dkg == nil {
		return errors.New("sendPartialSignByIds: dkg is nil")
	}

	chain, err := chains.GetChainByID(chainId)
	if err != nil {
		return errors.Wrap(err, "getChain error")
	}

	runtime := contracts.NewRuntimeWithVersion(0, time.Now(), txn, chaintypes.UniAddr{}, chaintypes.UniAddr{}, module.NativeVersion)
	bq := batch.BatchQuery{Batch: *batch.NewBatch(runtime)}

	hubCalls := make([]*batch.HubCallIndex, 0, len(ids))
	for _, id := range ids {
		hc, err := bq.HubcallsGet(id)
		if err != nil || hc == nil {
			util.LogWithRed("sendPartialSignByIds", "get hubcall", id, "error:", err)
			continue
		}
		hubCalls = append(hubCalls, hc)
	}

	if len(hubCalls) == 0 {
		return errors.New("sendPartialSignByIds: no valid hub calls found")
	}

	var dkgKey []byte
	cipher := chain.GetCipher()
	if pub, ok := app.dkg.DkgPubKeys[cipher]; ok && pub != nil {
		dkgKey = pub.Byte()
	}

	calls := make([]chaintypes.ChainCall, 0, len(hubCalls))
	for _, hc := range hubCalls {
		// 错误池：多次失败的 hub call 暂时跳过，避免反复执行（持久化，重启保留）
		if txErrPool.isPoisoned(chainId, hc.CallID) {
			util.LogWithRed("sendPartialSignByIds", "skip poisoned hub call", hc.CallID, "chainId=", chainId)
			continue
		}
		call, err := chain.TEECallToCall(hc, dkgKey)
		if err != nil {
			txErrPool.recordFailure(chainId, hc.CallID, err.Error())
			util.LogWithRed("sendPartialSignByIds", "TEECallToCall error:", err)
			continue
		}
		txErrPool.recordSuccess(chainId, hc.CallID)
		calls = append(calls, *call)
	}

	if len(calls) == 0 {
		// 所有 hub call 都 poisoned（或不可执行）：没有可发送的交易，
		// 若不处理，这些 hub call 永远停留在 HubCallPending，
		// targetSyncRelay 会每轮重新拾起 → 死循环。
		// 标记为 HubCallDone 放弃该链，让交易生命周期正常结束。
		// 状态更新纳入 txn，与区块提交原子化。
		bm := batch.BatchSecret{Batch: *batch.NewBatch(runtime)}
		for _, hc := range hubCalls {
			if hc.Status == batch.HubCallPending {
				util.LogWithRed("sendPartialSignByIds", "abandon all-poisoned hub call", hc.CallID, "chainId=", chainId)
				if err := bm.SetHubCallStatus(hc.CallID, batch.HubCallDone); err != nil {
					util.LogWithRed("sendPartialSignByIds", "SetHubCallStatus error for id", hc.CallID, ":", err)
				}
				// 已放弃，清除错误池记录，避免残留
				txErrPool.recordSuccess(chainId, hc.CallID)
			}
		}
		return errors.New("sendPartialSignByIds: no valid chain calls")
	}

	batchCall, err := chain.BatchCall("batch_all", calls)
	if err != nil {
		return errors.Wrap(err, "BatchCall error")
	}

	firstID := int64(ids[0])
	signer := dkg.NewDssSigner(app.dkg, 0, cipher)
	sig, err := chain.PartialSign(signer, *batchCall)
	if err != nil {
		return errors.Wrap(err, "PartialSign error")
	}

	if err := saveBatchCall(firstID, chainId, batchCall); err != nil {
		return err
	}

	if err := saveCallIds(firstID, chainId, ids); err != nil {
		return err
	}

	p2pId := app.dkg.P2PId()
	if p2pId == nil {
		return errors.New("P2PId not found in validator set")
	}

	psig := &model.BlockPartialSign{
		From:    p2pId.String(),
		HubSig:  sig,
		TxIndex: firstID,
		ChainId: chainId,
	}

	err = app.p2p.Send(&model.PeerMsg{
		To:      &model.To{Payload: &model.To_Node{Node: proposer}},
		Payload: &model.PeerMsg_BlockPartialSign{BlockPartialSign: psig},
	})
	if err != nil {
		util.LogWithRed("sendPartialSignByIds", "P2P Send error:", err)
	}

	return nil
}
