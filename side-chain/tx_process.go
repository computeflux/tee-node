package sidechain

import (
	"bytes"
	"fmt"

	abci "github.com/cometbft/cometbft/abci/types"
	"github.com/wetee-dao/tee-dsecret/pkg/model"
	"github.com/wetee-dao/tee-dsecret/pkg/model/protoio"
)

// Process tx
func (app *SideChain) ProcessTx(txs [][]byte) abci.ResponseProcessProposal_ProposalStatus {
	for _, txbt := range txs {
		txbox := new(model.Tx)
		err := protoio.ReadMessage(bytes.NewBuffer(txbt), txbox)
		if err != nil {
			return abci.ResponseProcessProposal_REJECT
		}

		tx := new(model.SysCall)
		err = protoio.ReadMessage(bytes.NewBuffer(txbox.Call), tx)
		if err != nil {
			return abci.ResponseProcessProposal_REJECT
		}

		switch tx.Payload.(type) {
		case *model.SysCall_EpochStart:
		case *model.SysCall_EpochEnd:
		case *model.SysCall_SyncTxStart:
		case *model.SysCall_SyncTxEnd:
		case *model.SysCall_SyncTxRetry:
		case *model.SysCall_Empty:
		case *model.SysCall_HubCall:
		case *model.SysCall_Contract:
		default:
			fmt.Println("Payload is not set or unknown")
			return abci.ResponseProcessProposal_REJECT
		}
	}

	return abci.ResponseProcessProposal_ACCEPT
}
