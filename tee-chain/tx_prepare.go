package teechain

import (
	"bytes"
	"fmt"
	"time"

	"github.com/computeflux/tee-provider/pkg/model"
	"github.com/computeflux/tee-provider/pkg/model/protoio"
	dbstore "github.com/computeflux/tee-provider/pkg/store"
	"github.com/computeflux/tee-provider/pkg/util"
)

func (s *TeeChain) PrepareTx(txs [][]byte, finaltx *[][]byte, height int64, blockTime time.Time, addMainChainTx bool) {
	txn := dbstore.SECRET_DB.NewTransaction()
	defer txn.Rollback()

	// RuntimeUpgrade 独占整个区块：有升级交易时忽略其他所有交易
	var runtimeUpgradeTx []byte
	hasRuntimeUpgrade := false
	for _, txbt := range txs {
		txbox := new(model.Tx)
		if protoio.ReadMessage(bytes.NewBuffer(txbt), txbox) != nil {
			continue
		}
		tx := new(model.SysCall)
		if protoio.ReadMessage(bytes.NewBuffer(txbox.Call), tx) != nil {
			continue
		}
		if _, ok := tx.Payload.(*model.SysCall_RuntimeUpgrade); ok {
			runtimeUpgradeTx = txbt
			hasRuntimeUpgrade = true
			break
		}
	}

	if hasRuntimeUpgrade {
		*finaltx = append(*finaltx, runtimeUpgradeTx)
		util.LogWithGreen("PrepareTx", "RuntimeUpgrade: exclusive block, only upgrade tx included")
		return
	}

	// 同一区块内每个 caller 只允许一笔交易
	seenCallers := make(map[string]bool)

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

		// 同 caller 只保留第一笔
		if txbox.GetCallerType() > 0 {
			callerKey := string(txbox.GetCaller())
			if seenCallers[callerKey] {
				util.LogWithGray("PrepareTx", "Skip duplicate tx, caller:", fmt.Sprintf("%x", txbox.GetCaller()))
				continue
			}
			seenCallers[callerKey] = true
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
		case *model.SysCall_Contract:
			contractNonce := tx.GetContract().GetNonce()
			currentNonce, err := TxnGetCurrentCallerNonce(txn, txbox.GetCaller(), txbox.GetCallerType())
			if err != nil {
				util.LogWithGray("PrepareTx", "Skip contract tx: failed to get nonce, err:", err)
				continue
			}

			expectedNonce := currentNonce + 1
			if contractNonce != expectedNonce && txbox.GetCallerType() > 0 {
				util.LogWithGray("PrepareTx", "Skip contract tx with invalid nonce, expected", expectedNonce, "got", contractNonce)
				continue
			}

			if err := TxnSetCallerNonce(txn, txbox.GetCaller(), txbox.GetCallerType(), contractNonce); err != nil {
				util.LogWithGray("PrepareTx", "Skip contract tx: failed to set nonce, err:", err)
				continue
			}

			*finaltx = append(*finaltx, txbt)
		default:
			break
		}
	}
}
