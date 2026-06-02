package teechain

import (
	"bytes"
	"fmt"

	"github.com/computeflux/tee-provider/pkg/chains"
	"github.com/computeflux/tee-provider/pkg/model"
	"github.com/computeflux/tee-provider/pkg/model/protoio"
	"github.com/computeflux/tee-provider/pkg/util"
)

func (app *TeeChain) checkTx(txbt []byte) uint32 {
	txbox := new(model.Tx)
	err := protoio.ReadMessage(bytes.NewBuffer(txbt), txbox)
	if err != nil {
		fmt.Println("TeeChain CheckTx protoio.ReadMessage TxBox error")
		return CodeTypeEncodingError
	}

	_, err = model.VerifyTxSigner(txbox)
	// 验证交易签名
	if err != nil {
		fmt.Println("TeeChain CheckTx VerifyTxSigner error", err)
		return CodeTypeInvalidTxFormat
	}

	// 验证合约交易 nonce（预校验，减少无效交易进入 mempool）
	call := new(model.SysCall)
	if err := protoio.ReadMessage(bytes.NewBuffer(txbox.Call), call); err == nil {
		switch p := call.Payload.(type) {
		case *model.SysCall_Contract:
			contract := p.Contract
			txNonce := contract.GetNonce()
			expectedNonce := GetNextCallerNonce(txbox.GetCaller(), txbox.GetCallerType())
			if txNonce != expectedNonce {
				util.LogWithYellow("CHECK TX", "invalid nonce: expected %d, got %d (caller %x, type %d)\n",
					expectedNonce, txNonce, txbox.GetCaller(), txbox.GetCallerType())
				return CodeInvalidNode
			}

			err := app.ContractDryRun(txbox.Caller, txbox.CallerType, contract)
			if err != nil {
				util.LogWithYellow("CHECK TX", "DryRun error", err.Error())
				return CodeInvalidNode
			}
		case *model.SysCall_RuntimeUpgrade:
			// RuntimeUpgrade 不需要 nonce 和 DryRun 预校验
		}
	}

	// 验证节点之间的数据交换需要验证彼此的节点ID
	if txbox.CallerType == 0 {
		if len(txbox.Caller) == 0 {
			fmt.Println("invalid node caller is nil")
			return CodeInvalidNode
		}

		keys, err := chains.MainChain.GetValidatorList()
		if err != nil {
			fmt.Println("invalid node: get validator list error", err)
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
			fmt.Println("invalid node is not validator")
			return CodeInvalidNode
		}
	}

	return CodeTypeOK
}
