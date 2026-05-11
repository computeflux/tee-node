package dkg

import (
	"encoding/json"
	"errors"

	"github.com/wetee-dao/ink.go/pallet/revive"
	"github.com/wetee-dao/tee-dsecret/pkg/chains"
	"github.com/wetee-dao/tee-dsecret/pkg/model"
	"github.com/wetee-dao/tee-dsecret/pkg/util"
)

var skipPartialSign = false

func (dkg *DKG) SendNewEpochPartialSigToSponsor() {
	if dkg.NewEpochSponsor == nil {
		return
	}

	// skip partial sign return []byte for util test
	if skipPartialSign {
		msg := model.NewEpochMsg{}
		bt, _ := json.Marshal(msg)
		dkg.sendToNode(model.SendToNode(&dkg.NewEpochSponsor.P2pId), &model.DkgMessage{
			Type:    "consensus_to_newpoch",
			Payload: bt,
		})
		return
	}

	if chains.MainChain == nil {
		util.LogWithRed("DKG SendNewEpochPartialSigToSponsor", "chains.MainChain is nil, not send new epoch partial sig to sponsor")
		return
	}

	client := chains.MainChain.GetClient()

	if dkg.NewDkgPubKey == nil {
		util.LogWithRed("DKG SendNewEpochPartialSigToSponsor", "NewDkgPubKey is nil")
		return
	}
	h160 := dkg.NewDkgPubKey.H160()
	_, isSome, err := revive.GetOriginalAccountLatest(client.Api().RPC.State, h160)
	if err != nil {
		util.LogWithRed("DKG SendNewEpochPartialSigToSponsor", "GetOriginalAccountLatest error:"+err.Error())
		return
	}

	var sig []byte
	// at first epoch account must call MapAccount
	if !isSome {
		runtimeCall := revive.MakeMapAccountCall()
		call, err := (runtimeCall).AsCall()
		if err != nil {
			util.LogWithRed("DKG SendNewEpochPartialSigToSponsor", "AsCall error:"+err.Error())
			return
		}
		signer := NewDssSigner(dkg, 0)

		sig, err = client.PartialSign(signer, call)
		if err != nil {
			util.LogWithRed("DKG SendNewEpochPartialSigToSponsor", "MapAccount PartialSign error:"+err.Error())
			return
		}
	} else {
		signer := NewDssSigner(dkg, 0)
		call, err := chains.MainChain.TxCallOfSetNextEpoch(dkg.NewEpochSponsor.NodeID, signer.AccountID())
		util.LogWithPurple("DKG SendNewEpochPartialSigToSponsor", "side chain key", dkg.NewDkgPubKey.SS58(), dkg.NewDkgPubKey.H160().Hex())
		if err != nil {
			util.LogWithRed("DKG SendNewEpochPartialSigToSponsor", "TxCallOfSetNextEpoch error:"+err.Error())
			return
		}
		if err != nil {
			util.LogWithRed("DKG SendNewEpochPartialSigToSponsor", "MainChain.TxCallOfSetNextEpoch error:"+err.Error())
			return
		}

		sig, err = client.PartialSign(signer, *call)
		if err != nil && isSome {
			util.LogWithRed("DKG SendNewEpochPartialSigToSponsor", "PartialSign error:"+err.Error())
			return
		}
	}

	msg := model.NewEpochMsg{
		Time:       dkg.NewEpochTime,
		PartialSig: sig,
	}
	bt, _ := json.Marshal(msg)
	dkg.sendToNode(model.SendToNode(&dkg.NewEpochSponsor.P2pId), &model.DkgMessage{
		Type:    "consensus_to_newpoch",
		Payload: bt,
	})
}

func (dkg *DKG) RevPartialSig(OrgId string, data []byte) error {
	if !dkg.ConsensusIsbusy() {
		return nil
	}

	msg := new(model.NewEpochMsg)
	err := json.Unmarshal(data, msg)
	if err != nil {
		dkg.consensusFailBack(errors.New("SendNewEpochPartialSigToSponsor client.PartialSign error:" + err.Error()))
		return err
	}

	if dkg.NewEpochPartialSigs == nil || dkg.NewEpochPartialSigTime != dkg.NewEpochTime {
		dkg.NewEpochPartialSigs = make(map[string]*model.NewEpochMsg)
		dkg.NewEpochPartialSigTime = dkg.NewEpochTime
	}

	dkg.NewEpochPartialSigs[OrgId] = msg
	if len(dkg.NewEpochPartialSigs) <= dkg.Threshold {
		return nil
	}

	// add for test
	if skipPartialSign {
		dkg.consensusSuccessBack(&DssSigner{}, dkg.NewEpochSponsor.NodeID)
		return nil
	}

	shares := make([][]byte, 0, len(dkg.NewEpochPartialSigs))
	// mapShares := make([][]byte, 0, len(dkg.NewEpochPartialSigs))
	for v := range dkg.NewEpochPartialSigs {
		shares = append(shares, dkg.NewEpochPartialSigs[v].PartialSig)
		// mapShares = append(mapShares, dkg.NewEpochPartialSigs[v].MapAccountPartialSig)
	}

	// check key is has been mapped
	client := chains.MainChain.GetClient()
	h160 := dkg.NewDkgPubKey.H160()
	_, isSome, err := revive.GetOriginalAccountLatest(client.Api().RPC.State, h160)
	if err != nil {
		dkg.consensusFailBack(errors.New("SendNewEpochPartialSigToSponsor GetOriginalAccountLatest error:" + err.Error()))
		return errors.New("SendNewEpochPartialSigToSponsor error:" + err.Error())
	}

	// mapAccount
	if !isSome {
		signer := DssSigner{dkg: dkg}
		signer.SetSigs(shares)

		util.LogWithGreen("SendNewEpochPartialSigToSponsor mapAccount", dkg.DkgPubKey.SS58())

		runtimeCall := revive.MakeMapAccountCall()
		call, err := (runtimeCall).AsCall()
		if err != nil {
			dkg.consensusFailBack(errors.New("SendNewEpochPartialSigToSponsor AsCall error:" + err.Error()))
			return errors.New("AsCall error:" + err.Error())
		}
		err = client.SignAndSubmit(&signer, call, false, 0)
		if err != nil {
			dkg.consensusFailBack(errors.New("SendNewEpochPartialSigToSponsor side chain key MapAccount error:" + err.Error()))
			return errors.New("side chain key MapAccount error:" + err.Error())
		}

		dkg.consensusFailBack(errors.New("SendNewEpochPartialSigToSponsor run makeaccount, next time submit tx"))
		return nil
	}

	// util.LogWithPurple("DKG", "SubmitToNepoch side chain key", dkg.NewDkgPubKey.SS58(), dkg.NewDkgPubKey.H160().Hex())
	signer := DssSigner{
		dkg: dkg,
	}
	signer.SetSigs(shares)

	dkg.consensusSuccessBack(&signer, dkg.NewEpochSponsor.NodeID)
	return nil
}

func (dkg *DKG) SaveSponsor(newMsg *model.ConsensusMsg) {
	if newMsg.Sponsor != nil {
		dkg.NewEpochSponsor = newMsg.Sponsor
		dkg.NewEpochTime = newMsg.EpochTime
	}
}
