package teechain

import (
	"bytes"

	"github.com/computeflux/tee-provider/pkg/chains"
	"github.com/computeflux/tee-provider/pkg/model"
	"github.com/computeflux/tee-provider/pkg/model/protoio"
	"github.com/computeflux/tee-provider/pkg/util"
)

func (app *TeeChain) checkTx(txbt []byte) uint32 {
	txbox := new(model.Tx)
	err := protoio.Unmarshal(txbt, txbox)
	if err != nil {
		util.LogWarnf("CheckTx encoding error: %v", err)
		return CodeTypeEncodingError
	}

	_, err = model.VerifyTxSigner(txbox)
	// 验证交易签名
	if err != nil {
		util.LogWarnf("CheckTx VerifyTxSigner error: %v (caller %x, type %d)", err, txbox.GetCaller(), txbox.GetCallerType())
		return CodeTypeInvalidTxFormat
	}

	// 验证合约交易 nonce（预校验，减少无效交易进入 mempool）
	call := new(model.SysCall)
	if err := protoio.Unmarshal(txbox.Call, call); err == nil {
		switch p := call.Payload.(type) {
		case *model.SysCall_Contract:
			contract := p.Contract
			txNonce := contract.GetNonce()
			expectedNonce := GetNextCallerNonce(txbox.GetCaller(), txbox.GetCallerType())
			if txNonce != expectedNonce {
				util.LogWarnf("CheckTx invalid nonce: expected %d got %d (caller %x, type %d)", expectedNonce, txNonce, txbox.GetCaller(), txbox.GetCallerType())
				return CodeInvalidNode
			}

			err := app.ContractDryRun(txbox.Caller, txbox.CallerType, contract)
			if err != nil {
				util.LogWarnf("CheckTx DryRun error: %v (caller %x, type %d)", err, txbox.Caller, txbox.CallerType)
				return CodeInvalidNode
			}
		}
	}

	// 验证节点之间的数据交换需要验证彼此的节点ID
	if txbox.CallerType == 0 {
		if len(txbox.Caller) == 0 {
			util.LogWarnf("CheckTx invalid node caller is nil (type %d)", txbox.GetCallerType())
			return CodeInvalidNode
		}

		if chains.MainChain == nil {
			// Main chain not connected yet — cannot verify validator list.
			// Allow the tx to pass pre-check; final validation happens in FinalizeTx.
			return CodeTypeOK
		}

		keys, err := chains.MainChain.GetValidatorList()
		if err != nil {
			util.LogWarnf("CheckTx get validator list error: %v (caller %x)", err, txbox.Caller)
			return CodeInvalidNode
		}

		isIn := false
		for _, key := range keys {
			if bytes.Equal(txbox.Caller, key.ValidatorId.Byte()) {
				isIn = true
				break
			}
		}

		if !isIn {
			util.LogWarnf("CheckTx caller %x is not validator", txbox.Caller)
			return CodeInvalidNode
		}
	}

	return CodeTypeOK
}
