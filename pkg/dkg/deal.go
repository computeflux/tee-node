package dkg

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/computeflux/tee-provider/pkg/model"
	"github.com/computeflux/tee-provider/pkg/util"
	pedersen "go.dedis.ch/kyber/v4/share/dkg/pedersen"
)

// sendDealMessage 发送交易信息到指定节点
func (dkg *DKG) sendDealMessage(node *model.To, message *model.ConsensusMsg) error {
	bt, err := json.Marshal(message)
	if err != nil {
		return err
	}

	return dkg.sendToNode(node, &model.DkgMessage{
		Type:    "deal",
		Payload: bt,
	})
}

// handleDeal 处理分发密钥生成协议中的交易消息
func (dkg *DKG) handleDeal(OrgId string, data []byte) error {
	pmessage := &model.ConsensusMsg{}
	err := json.Unmarshal(data, pmessage)
	if err != nil {
		return err
	}

	newMsg, err := model.DeepCopyIf(*pmessage)
	if err != nil {
		return err
	}
	dkg.SaveSponsor(newMsg)
	dkg.startConsensus(*newMsg)

	// 为每个 suite 存储 deal
	for _, st := range model.AllSuites() {
		dealBundle, ok := pmessage.DealBundles[st]
		if !ok || dealBundle == nil {
			continue
		}
		dkg.mu.Lock()
		dkg.deals[st][OrgId] = dealBundle
		dkg.mu.Unlock()
	}

	// 检查是否收集了所有 deals（以 ed25519 为例判断）
	dkg.mu.RLock()
	edDeals := dkg.deals[model.SuiteEd25519]
	mustDeals := len(dkg.Nodes)
	if pmessage.Epoch > StartEpoch {
		mustDeals = pmessage.ConsensusNodeNum
	}
	ready := len(edDeals) >= mustDeals
	dkg.mu.RUnlock()

	if !ready {
		return nil
	}

	// 为每个 suite 处理 deal
	for _, st := range model.AllSuites() {
		dkg.mu.RLock()
		suiteDeals := make([]*pedersen.DealBundle, 0, len(dkg.deals[st]))
		for _, d := range dkg.deals[st] {
			new, err := model.DeepCopyIf(*d)
			if err != nil {
				dkg.mu.RUnlock()
				return err
			}
			suiteDeals = append(suiteDeals, new.DealBundle)
		}
		gen := dkg.DistKeyGens[st]
		dkg.mu.RUnlock()

		if gen == nil {
			dkg.finishDkgConsensusStep(false, "DistKeyGen nil for:"+st.String())
			return fmt.Errorf("DistKeyGenerator is nil for %s", st)
		}

		resp, err := gen.ProcessDeals(suiteDeals)
		if err != nil {
			dkg.finishDkgConsensusStep(false, "ProcessDeals:"+st.String())
			return fmt.Errorf("ProcessDeals error for %s: %w", st, err)
		}
		if resp == nil {
			dkg.finishDkgConsensusStep(false, "ProcessDeals returned nil for:"+st.String())
			return errors.New("ProcessDeals error: resp is nil for " + st.String())
		}

		errNum := 0
		var logs = []any{"ProcessDeals [" + st.String() + "] ===> "}
		for _, r := range resp.Responses {
			logs = append(logs, fmt.Sprint(r.DealerIndex)+"="+fmt.Sprint(r.Status))
			if r.Status != pedersen.Success {
				errNum++
			}
		}
		dkg.log.Info(logs...)

		if errNum > 0 {
			dkg.finishDkgConsensusStep(false, "errNum > 0 for:"+st.String())
			return fmt.Errorf("ProcessDeals error for %s: errNum > 0", st)
		}
	}

	// 收集所有 suite 的 response 一起发送
	respMsg := &model.ConsensusRespMsg{
		Responses: make(map[model.SuiteType][]byte),
	}
	for _, st := range model.AllSuites() {
		dkg.mu.RLock()
		gen := dkg.DistKeyGens[st]
		dkg.mu.RUnlock()
		if gen == nil {
			continue
		}
		// 重新获取 response（因为 ProcessDeals 后需要再次获取）
		// 实际上 response 已经在上面处理过了，需要重新从 deals 生成
		dkg.mu.RLock()
		suiteDeals := make([]*pedersen.DealBundle, 0, len(dkg.deals[st]))
		for _, d := range dkg.deals[st] {
			suiteDeals = append(suiteDeals, d.DealBundle)
		}
		dkg.mu.RUnlock()

		resp, err := gen.ProcessDeals(suiteDeals)
		if err != nil || resp == nil {
			continue
		}
		bt, err := json.Marshal(resp)
		if err != nil {
			continue
		}
		respMsg.Responses[st] = bt
	}

	bt, err := json.Marshal(respMsg)
	if err != nil {
		return fmt.Errorf("marshal deal resp msg: %w", err)
	}

	err = dkg.sendToNode(model.SendToNodes(dkg.NewNetIds()), &model.DkgMessage{
		Type:    "deal_resp",
		Payload: bt,
	})
	if err != nil {
		util.LogError("DEAL", "Send deal_resp error", err)
	}

	return nil
}

// handleDealResp 处理交易响应消息
func (dkg *DKG) handleDealResp(OrgId string, data []byte) error {
	// 解析多 suite 响应
	respMsg := &model.ConsensusRespMsg{}
	err := json.Unmarshal(data, respMsg)
	if err != nil {
		// 兼容旧格式（单 response）
		message := &pedersen.ResponseBundle{}
		err = json.Unmarshal(data, message)
		if err != nil {
			util.LogError("DEAL", err)
			return err
		}
		// 旧格式默认当作 ed25519
		respMsg.Responses = map[model.SuiteType][]byte{
			model.SuiteEd25519: data,
		}
	}

	dkg.mu.Lock()
	for st, respData := range respMsg.Responses {
		resp := &pedersen.ResponseBundle{}
		if err := json.Unmarshal(respData, resp); err != nil {
			continue
		}
		dkg.responses[st][OrgId] = resp
	}

	// 检查是否收集了所有 responses（以 ed25519 为例判断）
	edResps := dkg.responses[model.SuiteEd25519]
	ready := len(edResps) >= len(dkg.NewNodes)
	dkg.mu.Unlock()

	if !ready {
		return nil
	}

	// 为每个 suite 处理 response
	allDone := true
	for _, st := range model.AllSuites() {
		dkg.mu.RLock()
		responses := make([]*pedersen.ResponseBundle, 0, len(dkg.responses[st]))
		for _, d := range dkg.responses[st] {
			responses = append(responses, d)
		}
		gen := dkg.DistKeyGens[st]
		dkg.mu.RUnlock()

		if gen == nil {
			continue
		}

		res, justification, err := gen.ProcessResponses(responses)
		if err != nil {
			dkg.finishDkgConsensusStep(false, "ProcessResponses:"+st.String())
			return fmt.Errorf("ProcessResponse for %s: %w", st, err)
		}

		if res != nil {
			dkg.mu.Lock()
			dkg.NewDkgKeyShares[st] = &model.DistKeyShare{
				CommitsWrap:  model.KyberPoints{Public: res.Key.Commits},
				PriShareWrap: model.PriShare{PriShare: res.Key.Share},
			}
			pub, err := model.PubKeyFromPointWithSuite(res.Key.Public(), st)
			if err != nil {
				dkg.mu.Unlock()
				dkg.finishDkgConsensusStep(false, "PubKeyFromPoint:"+st.String())
				return fmt.Errorf("PubKeyFromPoint for %s: %w", st, err)
			}
			dkg.NewDkgPubKeys[st] = pub
			dkg.mu.Unlock()
			continue
		}

		if justification == nil {
			res, err := gen.ProcessJustifications(nil)
			if err == nil && res != nil {
				dkg.mu.Lock()
				dkg.NewDkgKeyShares[st] = &model.DistKeyShare{
					CommitsWrap:  model.KyberPoints{Public: res.Key.Commits},
					PriShareWrap: model.PriShare{PriShare: res.Key.Share},
				}
				pub, err := model.PubKeyFromPointWithSuite(res.Key.Public(), st)
				if err != nil {
					dkg.mu.Unlock()
					dkg.finishDkgConsensusStep(false, "PubKeyFromPoint:"+st.String())
					return fmt.Errorf("PubKeyFromPoint for %s: %w", st, err)
				}
				dkg.NewDkgPubKeys[st] = pub
				dkg.mu.Unlock()
				continue
			}
		}

		allDone = false
	}

	if allDone {
		dkg.finishDkgConsensusStep(true, "")
	} else {
		dkg.finishDkgConsensusStep(false, "HandleDealResp not completed for all suites")
	}

	return nil
}
