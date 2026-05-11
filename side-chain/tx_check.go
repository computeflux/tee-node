package sidechain

import (
	"bytes"
	"fmt"

	"github.com/wetee-dao/tee-dsecret/pkg/chains"
	"github.com/wetee-dao/tee-dsecret/pkg/model"
	"github.com/wetee-dao/tee-dsecret/pkg/model/protoio"
)

func (app *SideChain) checkTx(txbt []byte) uint32 {
	txbox := new(model.Tx)
	err := protoio.ReadMessage(bytes.NewBuffer(txbt), txbox)
	if err != nil {
		fmt.Println("SideChain CheckTx protoio.ReadMessage TxBox error")
		return CodeTypeEncodingError
	}

	_, err = model.VerifyTxSigner(txbox)
	// 验证交易签名
	if err != nil {
		fmt.Println("SideChain CheckTx VerifyTxSigner error", err)
		return CodeTypeInvalidTxFormat
	}

	// 验证节点之间的数据交换需要验证彼此的节点ID
	if txbox.CallerType == 0 {
		if len(txbox.Caller) == 0 {
			fmt.Println("invalid node1")
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
			fmt.Println("invalid node2")
			return CodeInvalidNode
		}
	}

	return CodeTypeOK
}
