package dkg

import (
	"encoding/json"
	"errors"

	"github.com/computeflux/tee-provider/pkg/chains"
	"github.com/computeflux/tee-provider/pkg/model"
	"github.com/computeflux/tee-provider/pkg/util"
)

func (dkg *DKG) SendNewEpochPartialSigToSponsor() {
	dkg.mu.RLock()
	sponsor := dkg.NewEpochSponsor
	successBack := dkg.consensusSuccessBack
	dkg.mu.RUnlock()

	if sponsor == nil {
		// 无 sponsor（如测试），直接触发回调
		if chains.MainChain == nil && successBack != nil {
			successBack(NewDssSigner(dkg, 0, model.SuiteEd25519), 0)
		}
		return
	}

	// 无链环境（如测试），直接触发回调
	if chains.MainChain == nil {
		if successBack != nil {
			successBack(NewDssSigner(dkg, 0, model.SuiteEd25519), sponsor.NodeID)
		}
		return
	}

	// skip partial sign return []byte for util test
	if skipPartialSign {
		msg := model.NewEpochMsg{}
		bt, err := json.Marshal(msg)
		if err != nil {
			util.LogError("DKG", "marshal NewEpochMsg error:", err)
			return
		}
		dkg.sendToNode(model.SendToNode(&dkg.NewEpochSponsor.P2pId), &model.DkgMessage{
			Type:    "consensus_to_newpoch",
			Payload: bt,
		})
		return
	}

	cipher := chains.MainChain.GetCipher()
	if dkg.NewDkgPubKeys[cipher] == nil {
		util.LogWithRed("DKG SendNewEpochPartialSigToSponsor", "NewDkgPubKey is nil")
		return
	}

	var sig []byte
	// if cipher == model.SuiteEd25519 {
	// 	h160 := dkg.NewDkgPubKeys[cipher].ChainH160()
	// 	isSome, err := chains.MainChain.CheckMapAccount(h160)
	// 	if err != nil {
	// 		util.LogWithRed("DKG SendNewEpochPartialSigToSponsor", "GetOriginalAccountLatest error:"+err.Error())
	// 		return
	// 	}
	// 	if !isSome {
	// 		runtimeCall := revive.MakeMapAccountCall()
	// 		call, err := (runtimeCall).AsCall()
	// 		if err != nil {
	// 			util.LogWithRed("DKG SendNewEpochPartialSigToSponsor", "AsCall error:"+err.Error())
	// 			return
	// 		}
	// 		signer := NewDssSigner(dkg, 0, model.SuiteEd25519)

	// 		sig, err = chains.MainChain.PartialSign(signer, model.ChainCall{Data: call.Args})
	// 		if err != nil {
	// 			util.LogWithRed("DKG SendNewEpochPartialSigToSponsor", "MapAccount PartialSign error:"+err.Error())
	// 			return
	// 		}
	// 	} else {
	// 		signer := NewDssSigner(dkg, 0, model.SuiteEd25519)
	// 		call, err := chains.MainChain.TxCallOfSetNextEpoch(dkg.NewEpochSponsor.NodeID, signer.ChainAccountID())
	// 		util.LogWithPurple("DKG SendNewEpochPartialSigToSponsor", "side chain key", dkg.NewDkgPubKeys[model.SuiteEd25519].SS58(), dkg.NewDkgPubKeys[model.SuiteEd25519].ChainH160().Hex())
	// 		if err != nil {
	// 			util.LogWithRed("DKG SendNewEpochPartialSigToSponsor", "TxCallOfSetNextEpoch error:"+err.Error())
	// 			return
	// 		}

	// 		sig, err = chains.MainChain.PartialSign(signer, *call)
	// 		if err != nil && isSome {
	// 			util.LogWithRed("DKG SendNewEpochPartialSigToSponsor", "PartialSign error:"+err.Error())
	// 			return
	// 		}
	// 	}
	// }
	signer := NewDssSigner(dkg, 0, cipher)
	call, err := chains.MainChain.TxCallOfSetNextEpoch(dkg.NewEpochSponsor.NodeID, signer.ChainAccountID())
	// util.LogWithPurple("DKG SendNewEpochPartialSigToSponsor", "side chain key", dkg.NewDkgPubKeys[cipher].AccountString())
	if err != nil {
		util.LogWithRed("DKG SendNewEpochPartialSigToSponsor", "TxCallOfSetNextEpoch error:"+err.Error())
		return
	}

	sig, err = chains.MainChain.PartialSign(signer, *call)
	if err != nil {
		util.LogWithRed("DKG SendNewEpochPartialSigToSponsor", "PartialSign error:"+err.Error())
		return
	}

	msg := model.NewEpochMsg{
		Time:       dkg.NewEpochTime,
		PartialSig: sig,
	}
	bt, err := json.Marshal(msg)
	if err != nil {
		util.LogError("DKG", "marshal NewEpochMsg error:", err)
		return
	}

	dkg.sendToNode(model.SendToNode(&sponsor.P2pId), &model.DkgMessage{
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
	enough := len(dkg.NewEpochPartialSigs) >= dkg.Threshold

	if !enough {
		return nil
	}
	util.LogWithBlue("DKG RevPartialSig", "enough")

	// add for test
	if skipPartialSign {
		dkg.consensusSuccessBack(&DssSigner{suite: model.SuiteEd25519}, dkg.NewEpochSponsor.NodeID)
		return nil
	}

	cipher := chains.MainChain.GetCipher()

	shares := make([][]byte, 0, len(dkg.NewEpochPartialSigs))
	// mapShares := make([][]byte, 0, len(dkg.NewEpochPartialSigs))
	for v := range dkg.NewEpochPartialSigs {
		shares = append(shares, dkg.NewEpochPartialSigs[v].PartialSig)
		// mapShares = append(mapShares, dkg.NewEpochPartialSigs[v].MapAccountPartialSig)
	}

	// check key is has been mapped
	// h160 := dkg.NewDkgPubKeys[model.SuiteEd25519].ChainH160()
	// isSome, err := chains.MainChain.CheckMapAccount(h160)
	// if err != nil {
	// 	dkg.consensusFailBack(errors.New("SendNewEpochPartialSigToSponsor GetOriginalAccountLatest error:" + err.Error()))
	// 	return errors.New("SendNewEpochPartialSigToSponsor error:" + err.Error())
	// }

	// mapAccount
	// if !isSome {
	// 	signer := DssSigner{dkg: dkg, suite: model.SuiteEd25519}
	// 	signer.SetSigs(shares)

	// 	util.LogWithGreen("SendNewEpochPartialSigToSponsor mapAccount", dkg.DkgPubKeys[model.SuiteEd25519].SS58())

	// 	runtimeCall := revive.MakeMapAccountCall()
	// 	call, err := (runtimeCall).AsCall()
	// 	if err != nil {
	// 		dkg.consensusFailBack(errors.New("SendNewEpochPartialSigToSponsor AsCall error:" + err.Error()))
	// 		return errors.New("AsCall error:" + err.Error())
	// 	}
	// 	err = chains.MainChain.SignAndSubmit(&signer, model.ChainCall{Data: call.Args}, false, 0)
	// 	if err != nil {
	// 		dkg.consensusFailBack(errors.New("SendNewEpochPartialSigToSponsor side chain key MapAccount error:" + err.Error()))
	// 		return errors.New("side chain key MapAccount error:" + err.Error())
	// 	}

	// 	// MapAccount tx submitted but needs on-chain confirmation.
	// 	// Signal "failure" to trigger retry on next block epoch.
	// 	dkg.consensusFailBack(errors.New("SendNewEpochPartialSigToSponsor run makeaccount, next time submit tx"))
	// 	return nil
	// }

	// util.LogWithPurple("DKG", "SubmitToNepoch side chain key", dkg.NewDkgPubKeys[model.SuiteEd25519].SS58(), dkg.NewDkgPubKeys[model.SuiteEd25519].ChainH160().Hex())
	signer := DssSigner{
		dkg:   dkg,
		suite: cipher,
	}
	signer.SetSigs(shares)

	util.LogWithPurple("DKG SubmitToNepoch", "signer", signer.ChainAccountID(), "newEpochPartialSigs", len(dkg.NewEpochPartialSigs))

	dkg.consensusSuccessBack(&signer, dkg.NewEpochSponsor.NodeID)
	return nil
}
