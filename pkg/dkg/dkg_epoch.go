package dkg

import (
	"encoding/json"
	"errors"

	"github.com/computeflux/tee-provider/pkg/chains"
	"github.com/computeflux/tee-provider/pkg/model"
	"github.com/computeflux/tee-provider/pkg/util"
	"github.com/wetee-dao/ink.go/pallet/revive"
)

// SendNewEpochPartialSigToSponsor sends a partial signature for the new epoch
// to the consensus sponsor, enabling on-chain epoch finalization.
func (dkg *DKG) SendNewEpochPartialSigToSponsor() {
	dkg.mu.RLock()
	sponsor := dkg.NewEpochSponsor
	successBack := dkg.consensusSuccessBack
	dkg.mu.RUnlock()

	// No sponsor (e.g., test mode): trigger callback directly.
	if sponsor == nil {
		if chains.MainChain == nil && successBack != nil {
			successBack(NewDssSigner(dkg, 0, model.SuiteEd25519), 0)
		}
		return
	}

	// No chain environment (e.g., test mode): trigger callback directly.
	if chains.MainChain == nil {
		if successBack != nil {
			successBack(NewDssSigner(dkg, 0, model.SuiteEd25519), sponsor.NodeID)
		}
		return
	}

	// Test-only: skip actual chain partial signing.
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

	// TODO
	if cipher == model.SuiteEd25519 {
		isSome, sig, err := dkg.CheckMapAccount(dkg.NewDkgPubKeys[model.SuiteEd25519])
		if err != nil {
			util.LogWithRed("DKG SendNewEpochPartialSigToSponsor", "CheckMapAccount error:"+err.Error())
			return
		}
		if !isSome {
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
			return
		}
	}

	// 获取当前签名对象
	signer := NewDssSigner(dkg, 0, cipher)
	call, err := chains.MainChain.TxCallOfSetNextEpoch(dkg.NewEpochSponsor.NodeID, *signer.Pubkey())
	if err != nil {
		util.LogWithRed("DKG SendNewEpochPartialSigToSponsor", "TxCallOfSetNextEpoch error:"+err.Error())
		return
	}

	// 计算当前签名
	sig, err := chains.MainChain.PartialSign(signer, *call)
	if err != nil {
		util.LogWithRed("DKG SendNewEpochPartialSigToSponsor", "PartialSign error:"+err.Error())
		return
	}
	msg := model.NewEpochMsg{
		Time:       dkg.NewEpochTime,
		PartialSig: sig,
	}

	// 格式化消息并发送给共识发起者
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

// RevPartialSig collects partial signatures from peers for the new epoch.
// Once enough signatures (>= threshold) are gathered, it aggregates them
// and triggers the consensus success callback.
func (dkg *DKG) RevPartialSig(OrgId string, data []byte) error {
	if !dkg.ConsensusIsbusy() {
		return nil
	}

	msg := new(model.NewEpochMsg)
	err := json.Unmarshal(data, msg)
	if err != nil {
		dkg.consensusFailBack(errors.New("RevPartialSig client.PartialSign error:" + err.Error()))
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

	// Test-only: skip actual chain signing.
	if skipPartialSign {
		dkg.consensusSuccessBack(&DssSigner{suite: model.SuiteEd25519}, dkg.NewEpochSponsor.NodeID)
		return nil
	}

	cipher := chains.MainChain.GetCipher()

	shares := make([][]byte, 0, len(dkg.NewEpochPartialSigs))
	for v := range dkg.NewEpochPartialSigs {
		shares = append(shares, dkg.NewEpochPartialSigs[v].PartialSig)
	}

	// TODO
	if cipher == model.SuiteEd25519 {
		pub := dkg.NewDkgPubKeys[model.SuiteEd25519]
		isSome, err := chains.MainChain.CheckMapAccount(*pub)
		if err != nil {
			dkg.consensusFailBack(errors.New("RevPartialSig CheckMapAccount error:" + err.Error()))
			return errors.New("RevPartialSig CheckMapAccount error:" + err.Error())
		}

		// mapAccount
		if !isSome {
			signer := DssSigner{dkg: dkg}
			signer.SetSigs(shares)

			util.LogWithGreen("RevPartialSig mapAccount", pub.SS58())

			runtimeCall := revive.MakeMapAccountCall()
			call, err := (runtimeCall).AsCall()
			if err != nil {
				dkg.consensusFailBack(errors.New("RevPartialSig AsCall error:" + err.Error()))
				return errors.New("AsCall error:" + err.Error())
			}

			err = chains.MainChain.SignAndSubmit(&signer, model.ChainCall{Substrate: call}, false, 0)
			if err != nil {
				dkg.consensusFailBack(errors.New("RevPartialSig side chain key MapAccount error:" + err.Error()))
				return errors.New("side chain key MapAccount error:" + err.Error())
			}

			dkg.consensusFailBack(errors.New("RevPartialSig run makeaccount, next time submit tx"))
			return nil
		}
	}

	signer := DssSigner{
		dkg:   dkg,
		suite: cipher,
	}
	signer.SetSigs(shares)

	util.LogWithPurple("DKG SubmitToNepoch", "signer", signer.Pubkey().String(), "newEpochPartialSigs", len(dkg.NewEpochPartialSigs))

	dkg.consensusSuccessBack(&signer, dkg.NewEpochSponsor.NodeID)
	return nil
}

func (dkg *DKG) CheckMapAccount(addr *model.PubKey) (bool, []byte, error) {
	isSome, err := chains.MainChain.CheckMapAccount(*addr)
	if err != nil {
		return false, nil, err
	}

	var sig []byte
	// at first epoch account must call MapAccount
	if !isSome {
		runtimeCall := revive.MakeMapAccountCall()
		call, err := (runtimeCall).AsCall()
		if err != nil {
			util.LogWithRed("DKG SendNewEpochPartialSigToSponsor", "AsCall error:"+err.Error())
			return false, nil, err
		}

		signer := NewDssSigner(dkg, 0, model.SuiteEd25519)
		sig, err = chains.MainChain.PartialSign(signer, model.ChainCall{Substrate: call})
		if err != nil {
			util.LogWithRed("DKG SendNewEpochPartialSigToSponsor", "MapAccount PartialSign error:"+err.Error())
			return false, nil, err
		}

		return false, sig, nil
	}

	return true, nil, nil
}
