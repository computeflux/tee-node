package contracts

import (
	"errors"

	"github.com/centrifuge/go-substrate-rpc-client/v4/types"
	"github.com/wetee-dao/tee-dsecret/pkg/model"
	"github.com/wetee-dao/tee-dsecret/pkg/util"
)

func (c *Contract) TEECallToCall(tcall *model.TeeCall, dkgKey types.AccountID) (*types.Call, error) {
	callWrap := tcall.Tx
	switch tx := callWrap.(type) {
	case *model.TeeCall_PodStart:
		pod := tx.PodStart

		pod_key, err := types.NewAccountID(tcall.Caller)
		if err != nil {
			return nil, errors.New("get pod key error")
		}

		call, err := c.TxCallOfStartPod(pod.Id, *pod_key, dkgKey)
		if err != nil {
			util.LogError("TxCallOfStartPod", err)
			return nil, err
		}

		return call, nil
	case *model.TeeCall_PodMint:
		pod := tx.PodMint
		call, err := c.TxCallOfMintPod(pod.Id, types.NewH256(pod.ReportHash), dkgKey)
		if err != nil {
			util.LogError("TxCallOfStartPod", err)
			return nil, err
		}

		return call, nil
	case *model.TeeCall_UploadSecret:
		upload := tx.UploadSecret
		call, err := c.TxCallOfUploadSecret(types.NewH160(upload.User), upload.Index, dkgKey)
		if err != nil {
			util.LogError("TxCallOfUploadSecret", err)
			return nil, err
		}

		return call, nil
	case *model.TeeCall_InitDisk:
		init := tx.InitDisk
		call, err := c.TxCallOfInitDisk(types.NewH160(init.User), init.Index, types.NewH256(init.Hash), dkgKey)
		if err != nil {
			util.LogError("TxCallOfInitDisk", err)
			return nil, err
		}

		return call, nil
		// case *model.TeeCall_BridgeCall:
	}
	return nil, errors.New("invalid tee call")
}
