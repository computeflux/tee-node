package teechain

import (
	"bytes"
	"fmt"
	"time"

	"github.com/computeflux/tee-provider/pkg/model"
	"github.com/computeflux/tee-provider/pkg/model/protoio"
	"github.com/computeflux/tee-provider/pkg/util"
)

func (s *TeeChain) PrepareTx(txs [][]byte, finaltx *[][]byte, height int64, blockTime time.Time, addMainChainTx bool) {
	hubtx := make([][]byte, 0, 50)
	hubCalls := make([]*model.HubCall, 0, 50)

	// 使用临时 txn 累积 nonce 状态，确保同区块内多笔同 caller 合约交易的 nonce 连续
	txn := model.DBINS.NewTransaction()
	defer txn.Rollback()

	// 第一步：收集所有HubCall并检查正在提交到主链的块中是否有相同caller
	for _, txbt := range txs {
		txbox := new(model.Tx)
		err := protoio.ReadMessage(bytes.NewBuffer(txbt), txbox)
		if err != nil {
			continue
		}

		tx := new(model.SysCall)
		err = protoio.ReadMessage(bytes.NewBuffer(txbox.Call), tx)
		if err != nil {
			continue
		}

		switch tx.Payload.(type) {
		case *model.SysCall_Empty:
			*finaltx = append(*finaltx, txbt)
		case *model.SysCall_EpochStart:
			*finaltx = append(*finaltx, txbt)
		case *model.SysCall_EpochEnd:
			*finaltx = append(*finaltx, txbt)
		case *model.SysCall_SyncTxStart:
			*finaltx = append(*finaltx, txbt)
		case *model.SysCall_SyncTxEnd:
			*finaltx = append(*finaltx, txbt)
		case *model.SysCall_SyncTxRetry:
			*finaltx = append(*finaltx, txbt)
		case *model.SysCall_HubCall:
			if addMainChainTx {
				hubCall := tx.GetHubCall()
				hubCalls = append(hubCalls, hubCall)
				hubtx = append(hubtx, txbt)
			}
		case *model.SysCall_Contract:
			// PrepareProposal 阶段预校验合约交易 nonce，过滤过期交易
			// 使用 txn 读取 nonce，感知同区块内前面交易对 nonce 的修改
			contractNonce := tx.GetContract().GetNonce()
			currentNonce, err := TxnGetCurrentCallerNonce(txn, txbox.GetCaller(), txbox.GetCallerType())
			if err != nil {
				util.LogWithGray("PrepareTx", "Skip contract tx: failed to get nonce, err:", err, "caller", fmt.Sprintf("%x", txbox.GetCaller()))
				continue
			}

			expectedNonce := currentNonce + 1
			if contractNonce != expectedNonce {
				util.LogWithGray("PrepareTx", "Skip contract tx with invalid nonce, expected", expectedNonce, "got", contractNonce, "caller", fmt.Sprintf("%x", txbox.GetCaller()))
				continue
			}

			// 在 txn 中更新 nonce，使后续同 caller 交易的 nonce 校验基于最新状态
			if err := TxnSetCallerNonce(txn, txbox.GetCaller(), txbox.GetCallerType(), contractNonce); err != nil {
				util.LogWithGray("PrepareTx", "Skip contract tx: failed to set nonce, err:", err, "caller", fmt.Sprintf("%x", txbox.GetCaller()))
				continue
			}
			*finaltx = append(*finaltx, txbt)
		case *model.SysCall_RuntimeUpgrade:
			// RuntimeUpgrade 不需要 nonce 校验，直接包含在区块中
			*finaltx = append(*finaltx, txbt)
		default:
			break
		}
	}

	// 第二步：在同一块内去重相同caller的HubCall，只保留第一个
	hubtx = s.deduplicateCallersInBlock(hubtx, hubCalls)

	if len(hubtx) > 0 {
		tx, err := HubSyncStep1(s)
		if err != nil {
			util.LogWithRed("PrepareTx", "TryTxStart err:", err)
			time.Sleep(time.Second * 2)
			return
		}

		if s.dkg.AvailableNodeLen() < s.dkg.Threshold+1 {
			util.LogWithRed("PrepareTx", "exapect validator node:", s.dkg.Threshold+1, ", got:", s.dkg.AvailableNodeLen())
			time.Sleep(time.Second * 2)
			return
		}

		*finaltx = append(*finaltx, tx)
		*finaltx = append(*finaltx, hubtx...)
		// TODO: DeleteSigOfTx should be moved to FinalizeTx or Commit to ensure
		// it only runs after the block is actually committed.
		s.DeleteSigOfTx(height)
	}
}

// extractCallersFromHubCall 从HubCall中提取所有caller
func extractCallersFromHubCall(hub *model.HubCall) map[string]bool {
	callers := make(map[string]bool)
	if hub == nil {
		return callers
	}
	for _, call := range hub.Call {
		if call != nil && len(call.Caller) > 0 {
			callerKey := string(call.Caller)
			callers[callerKey] = true
		}
	}
	return callers
}

// deduplicateCallersInBlock 在同一块内去重相同caller的HubCall，只保留第一个
func (s *TeeChain) deduplicateCallersInBlock(hubtx [][]byte, hubCalls []*model.HubCall) [][]byte {
	if len(hubtx) == 0 {
		return hubtx
	}

	seenCallers := make(map[string]bool)
	result := make([][]byte, 0, len(hubtx))

	for i, hub := range hubCalls {
		if hub == nil {
			continue
		}

		callers := extractCallersFromHubCall(hub)
		hasDuplicate := false

		// 检查是否有已见过的caller
		for caller := range callers {
			if seenCallers[caller] {
				hasDuplicate = true
				util.LogWithGray("PrepareTx", "Duplicate caller in block, skipping:", caller)
				break
			}
		}

		// 如果没有重复，添加到结果中并记录caller
		if !hasDuplicate {
			result = append(result, hubtx[i])
			for caller := range callers {
				seenCallers[caller] = true
			}
		}
	}

	return result
}
