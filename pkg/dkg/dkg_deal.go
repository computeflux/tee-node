package dkg

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/computeflux/tee-provider/pkg/crypto"
	"github.com/computeflux/tee-provider/pkg/dkgtypes"
	"github.com/computeflux/tee-provider/pkg/model"
	"github.com/computeflux/tee-provider/pkg/util"
	pedersen "go.dedis.ch/kyber/v4/share/dkg/pedersen"
)

// sendDealMessage 发送交易信息到指定节点
func (dkg *DKG) sendDealMessage(node *model.To, message *dkgtypes.ConsensusMsg) error {
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
	pmessage := &dkgtypes.ConsensusMsg{}
	err := json.Unmarshal(data, pmessage)
	if err != nil {
		return err
	}

	newMsg, err := dkgtypes.DeepCopyIf(*pmessage)
	if err != nil {
		return err
	}
	dkg.SaveSponsor(newMsg)
	// 仅当 epoch 匹配时才处理，跳过过期 deal
	if dkg.Epoch >= newMsg.Epoch {
		return nil
	}
	dkg.startConsensus(*newMsg)

	// 再次检查：startConsensus 可能更新了 Epoch
	if dkg.Epoch >= newMsg.Epoch {
		return nil
	}

	// 为每个 suite 存储 deal
	for _, st := range crypto.AllSuites() {
		dealBundle, ok := pmessage.DealBundles[st]
		if !ok || dealBundle == nil {
			continue
		}
		dkg.mu.Lock()
		dkg.deals[st][OrgId] = dealBundle
		dkg.mu.Unlock()
	}

	// 检查是否收集了所有 deals
	dkg.mu.RLock()
	edCount := len(dkg.deals[crypto.SuiteEd25519])
	mustDeals := len(dkg.Nodes)
	if pmessage.Epoch > StartEpoch {
		mustDeals = pmessage.ConsensusNodeNum
	}
	dkg.mu.RUnlock()
	ready := edCount >= mustDeals

	if !ready {
		return nil
	}

	// 为每个 suite 处理 deal，并缓存响应用于后续发送
	cachedResps := make(map[crypto.SuiteType]*pedersen.ResponseBundle)
	for _, st := range crypto.AllSuites() {
		dkg.mu.RLock()
		suiteDeals := make([]*pedersen.DealBundle, 0, len(dkg.deals[st]))
		for _, d := range dkg.deals[st] {
			suiteDeals = append(suiteDeals, d.DealBundle)
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

		cachedResps[st] = resp
	}

	// 收集所有 suite 的 response 一起发送（使用缓存的结果，避免二次调用 ProcessDeals）
	respMsg := &dkgtypes.ConsensusRespMsg{
		Responses: make(map[crypto.SuiteType][]byte),
	}
	for st, resp := range cachedResps {
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

	util.LogOk("DEAL", "Sent deal responses for epoch", dkg.Epoch)
	return nil
}

// handleDealResp 处理交易响应消息
func (dkg *DKG) handleDealResp(OrgId string, data []byte) error {
	// 解析多 suite 响应
	respMsg := &dkgtypes.ConsensusRespMsg{}
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
		respMsg.Responses = map[crypto.SuiteType][]byte{
			crypto.SuiteEd25519: data,
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

	// 检查是否收集了所有 responses
	edCount := len(dkg.responses[crypto.SuiteEd25519])
	dkg.mu.Unlock()
	ready := edCount >= len(dkg.NewNodes)

	if !ready {
		return nil
	}

	// 为每个 suite 处理 response
	allDone := 0
	for _, st := range crypto.AllSuites() {
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
			dkg.NewDkgKeyShares[st] = &dkgtypes.DistKeyShare{
				CommitsWrap:  dkgtypes.KyberPoints{Public: res.Key.Commits, Suite: st.String()},
				PriShareWrap: dkgtypes.PriShareWithSuite{PriShare: res.Key.Share, Suite: st.String()},
			}
			pub, err := crypto.PubKeyFromPointWithSuite(res.Key.Public(), st)
			if err != nil {
				dkg.mu.Unlock()
				dkg.finishDkgConsensusStep(false, "PubKeyFromPoint:"+st.String())
				return fmt.Errorf("PubKeyFromPoint for %s: %w", st, err)
			}

			dkg.NewDkgPubKeys[st] = pub
			dkg.mu.Unlock()

			allDone++
			continue
		}

		if justification == nil {
			res, err := gen.ProcessJustifications(nil)
			if err == nil && res != nil {
				dkg.mu.Lock()
				dkg.NewDkgKeyShares[st] = &dkgtypes.DistKeyShare{
					CommitsWrap:  dkgtypes.KyberPoints{Public: res.Key.Commits, Suite: st.String()},
					PriShareWrap: dkgtypes.PriShareWithSuite{PriShare: res.Key.Share, Suite: st.String()},
				}
				pub, err := crypto.PubKeyFromPointWithSuite(res.Key.Public(), st)
				if err != nil {
					dkg.mu.Unlock()
					dkg.finishDkgConsensusStep(false, "PubKeyFromPoint:"+st.String())
					return fmt.Errorf("PubKeyFromPoint for %s: %w", st, err)
				}
				dkg.NewDkgPubKeys[st] = pub
				dkg.mu.Unlock()

				allDone++
				continue
			}
		}
	}

	if allDone == len(crypto.AllSuites()) {
		dkg.finishDkgConsensusStep(true, "")
	}
	// 注意：未完成时不调用 finishDkgConsensusStep(false)，
	// 否则会清空已收集的 deals/responses，导致永远无法完成。

	return nil
}
