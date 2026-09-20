package teechain

import (
	"fmt"
	"time"

	abci "github.com/cometbft/cometbft/abci/types"
	"github.com/computeflux/tee-provider/pkg/model"
	"github.com/computeflux/tee-provider/pkg/model/protoio"
	dbstore "github.com/computeflux/tee-provider/pkg/types"
)

// ProcessTx 验证节点在 ProcessProposal 阶段验证区块中的交易。
// 对 Contract 类型交易进行 nonce 预校验，确保区块中不包含 nonce 无效的交易，
// 避免浪费区块空间（FinalizeBlock 中 nonce 无效会标记失败但仍占用空间）。
func (app *TeeChain) ProcessTx(txs [][]byte, height int64, time time.Time) abci.ResponseProcessProposal_ProposalStatus {
	txn := dbstore.SECRET_DB.NewTransaction()
	defer txn.Rollback()

	for _, txbt := range txs {
		txbox := new(model.Tx)
		err := protoio.Unmarshal(txbt, txbox)
		if err != nil {
			return abci.ResponseProcessProposal_REJECT
		}

		tx := new(model.SysCall)
		err = protoio.Unmarshal(txbox.Call, tx)
		if err != nil {
			return abci.ResponseProcessProposal_REJECT
		}

		switch p := tx.Payload.(type) {
		case *model.SysCall_EpochStart:
		case *model.SysCall_EpochEnd:
		case *model.SysCall_SyncTxStart:
			// 空 ids 会在 FinalizeBlock 时触发 DeleteSigOfTx(Ids[0]) 越界/链停摆，
			// 必须在提案阶段拦截
			if len(p.SyncTxStart.Ids) == 0 {
				fmt.Println("ProcessTx REJECT: SyncTxStart with empty ids")
				return abci.ResponseProcessProposal_REJECT
			}
		case *model.SysCall_SyncTxEnd:
			// 空 ids 会使 HubSyncEnd 报错，FinalizeBlock 失败导致链停摆
			if len(p.SyncTxEnd.Ids) == 0 {
				fmt.Println("ProcessTx REJECT: SyncTxEnd with empty ids")
				return abci.ResponseProcessProposal_REJECT
			}
		case *model.SysCall_Empty:
		case *model.SysCall_Contract:
			caller := txbox.GetCaller()
			callerType := txbox.GetCallerType()
			txNonce := p.Contract.GetNonce()

			// 使用 txn 读取 nonce，感知同区块内前面交易对 nonce 的修改
			currentNonce, err := TxnGetCurrentCallerNonce(txn, caller, callerType)
			if err != nil {
				fmt.Printf("ProcessTx REJECT: failed to get nonce: %v (caller %x, type %d)\n", err, caller, callerType)
				return abci.ResponseProcessProposal_REJECT
			}

			expectedNonce := currentNonce + 1
			if txNonce != expectedNonce {
				fmt.Printf("ProcessTx REJECT: invalid nonce, expected %d, got %d (caller %x, type %d)\n", expectedNonce, txNonce, caller, callerType)
				return abci.ResponseProcessProposal_REJECT
			}

			// 额外：合约 DryRun 预执行，确保合约逻辑不会失败，避免浪费区块空间
			err = app.ContractMutation(caller, callerType, txn, time, height, p.Contract)
			if err != nil {
				fmt.Println("ProcessTx ContractMutation err:", err.Error())
				return abci.ResponseProcessProposal_REJECT
			}

			// 在 txn 中更新 nonce，使后续同 caller 交易的 nonce 校验基于最新状态
			if err := TxnSetCallerNonce(txn, caller, callerType, txNonce); err != nil {
				fmt.Printf("ProcessTx REJECT: failed to set nonce: %v (caller %x, type %d)\n", err, caller, callerType)
				return abci.ResponseProcessProposal_REJECT
			}
		case *model.SysCall_RelaySync:
		default:
			fmt.Println("Payload is not set or unknown")
			return abci.ResponseProcessProposal_REJECT
		}
	}

	return abci.ResponseProcessProposal_ACCEPT
}
