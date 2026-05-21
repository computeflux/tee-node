package dkg

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/computeflux/tee-provider/pkg/model"
	"github.com/computeflux/tee-provider/pkg/util"
	pedersen "go.dedis.ch/kyber/v4/share/dkg/pedersen"
	"go.dedis.ch/kyber/v4/sign/schnorr"
)

const StartEpoch = 1

func (dkg *DKG) TryEpochConsensus(
	msg model.ConsensusMsg,
	callback func(*DssSigner, uint64),
	fail func(error),
) error {
	// check consensus is busy
	if dkg.ConsensusIsbusy() {
		util.LogError("DKG Consensus", "in consensus")
		return errors.New("in consensus")
	}

	// check old validators length
	dkg.mu.RLock()
	available := dkg.AvailableNodeLen()
	threshold := dkg.Threshold
	dkg.mu.RUnlock()
	if available < threshold {
		util.LogError("DKG Consensus", "old validator node exapect >", threshold, ", got:", available)
		return fmt.Errorf("old validators count < dkg.Threshold")
	}

	// check new nodes validators length
	if dkg.NewValidatorNodeLen(msg.Validators) <= len(msg.Validators)*3/4 {
		util.LogError("DKG Consensus", "exapect new validator count:", len(msg.Validators)*3/4, ", got:", dkg.NewValidatorNodeLen(msg.Validators))
		return fmt.Errorf("new validators count < len(Validators)*3/4")
	}

	// get ed25519 share for backward compat check
	dkg.mu.RLock()
	edShare := dkg.GetDkgKeyShare(model.SuiteEd25519)
	nodeCount := len(dkg.Nodes)
	nodesCopy := make([]*model.Validator, 0, nodeCount)
	for _, n := range dkg.Nodes {
		nodesCopy = append(nodesCopy, n)
	}
	dkg.mu.RUnlock()

	if edShare == nil && msg.Epoch > StartEpoch {
		util.LogError("DKG Consensus", "msg.Epoch", msg.Epoch, "| Node is not old validator, cannot start consensus")
		return errors.New("node is not old validator, cannot start consensus")
	}

	// init consensus msg
	// 注：ShareCommits 按 suite 携带，每个 suite 的 reConsensus 都需要 PublicCoeffs
	msg.ShareCommits = make(map[model.SuiteType]model.KyberPoints)
	if edShare != nil {
		for _, st := range model.AllSuites() {
			share := dkg.GetDkgKeyShare(st)
			if share != nil {
				commitsCopy, err := model.DeepCopyIf(share.CommitsWrap)
				if err != nil {
					return err
				}
				msg.ShareCommits[st] = *commitsCopy
			}
		}
		msg.ConsensusNodeNum = nodeCount
		msg.OldValidators = nodesCopy
	} else {
		msg.ConsensusNodeNum = 0
	}

	// generate new validators sidekey
	dkgSigner := dkg.Signer.GetPublicForSuite(model.SuiteEd25519).SS58()
	validator := new(model.Validator)
	for _, v := range msg.Validators {
		if v.ValidatorId.SS58() == dkgSigner {
			validator = v
		}
	}

	// check local node is in new validators
	if validator == nil {
		util.LogError("DKG Consensus", "Currunt Node is not in new validators")
		return errors.New("DKG Consensus Currunt Node is not in new validators")
	}

	// set sponsor
	msg.Sponsor = validator
	msg.EpochTime = time.Now().Unix()

	// set callback
	dkg.mu.Lock()
	dkg.consensusSuccessBack = callback
	dkg.consensusFailBack = fail
	dkg.mu.Unlock()

	bt, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("marshal consensus msg: %w", err)
	}
	dkg.DkgOutHandler(&model.DkgMessage{
		Type:    "consensus",
		Payload: bt,
	})

	return nil
}

// start dkg consensus
func (dkg *DKG) startConsensus(msg model.ConsensusMsg) error {
	if !dkg.TrySetConsensusBusy() {
		return errors.New("DKG Consensus going")
	}

	dkg.mu.RLock()
	epoch := dkg.Epoch
	status := dkg.status
	dkg.mu.RUnlock()

	if epoch >= msg.Epoch {
		dkg.setConsensusFree()
		return errors.New("DKG Epoch is not need to update")
	}

	dkg.addConsensusTimeout()

	hasShareCommits := false
	for _, st := range model.AllSuites() {
		if len(msg.ShareCommits[st].Public) > 0 {
			hasShareCommits = true
			break
		}
	}
	if !hasShareCommits {
		util.LogWithGray("InitConsensus Epoch ======> ", msg.Epoch)
		// 只允许首次初始化，避免异步 deal 重复触发 initConsensus 覆盖 DistKeyGens
		if status == 1 && epoch == msg.Epoch {
			dkg.setConsensusFree()
			return nil
		}
		return dkg.initConsensus(msg)
	}

	util.LogWithGray("ReConsensus Epoch ======> ", msg.Epoch)
	return dkg.reConsensus(msg)
}

// Init Consensus — runs DKG for ALL suites in parallel
func (dkg *DKG) initConsensus(msg model.ConsensusMsg) error {
	dkg.mu.Lock()
	dkg.Nodes = msg.Validators
	dkg.NewNodes = msg.Validators
	dkg.NewEpoch = msg.Epoch
	dkg.Threshold = len(msg.Validators) - (len(msg.Validators)-1)/3
	// Snapshot before unlock (avoids race)
	nodesSnapshot := make([]*model.Validator, len(dkg.Nodes))
	copy(nodesSnapshot, dkg.Nodes)
	threshold := dkg.Threshold
	dkg.mu.Unlock()

	// 为所有 suite 并行生成 deal
	newMsg, err := model.DeepCopyIf(msg)
	if err != nil {
		return err
	}
	newMsg.DealBundles = make(map[model.SuiteType]*model.DealBundle)
	newMsg.ShareCommits = make(map[model.SuiteType]model.KyberPoints)

	for _, st := range model.AllSuites() {
		suite := dkg.Suites[st]

		// Build node list from snapshot.
		nodes := make([]pedersen.Node, 0, len(nodesSnapshot))
		for i, p := range nodesSnapshot {
			pub := validatorPubForSuite(p, st)
			if pub == nil {
				dkg.finishDkgConsensusStep(false, "failed to get public key for:"+st.String())
				return fmt.Errorf("failed to get public key for validator %d suite %s", i, st)
			}
			nodes = append(nodes, pedersen.Node{
				Index:  uint32(i),
				Public: pub,
			})
		}
		signer := schnorr.NewScheme(suite)

		// 初始化协议配置
		conf := pedersen.Config{
			Suite:     suite,
			NewNodes:  nodes,
			Threshold: uint32(threshold),
			Auth:      signer,
			FastSync:  true,
			Longterm:  dkg.longtermForSuite(st),
			Nonce:     epochToNonce(0),
			Log:       dkg.log,
		}

		// initialize dealer
		gen, err := pedersen.NewDistKeyHandler(&conf)
		if err != nil {
			dkg.finishDkgConsensusStep(false, "pedersen.NewDistKeyHandler:"+st.String())
			return fmt.Errorf("failed to initialize DKG protocol for %s: %w", st, err)
		}

		// 获取当前节点的 deal（必须在设置 DistKeyGens 之前，确保成功后才对外可见）
		deal, err := gen.Deals()
		if err != nil {
			dkg.finishDkgConsensusStep(false, "Deals:"+st.String())
			return fmt.Errorf("failed to generate key shares for %s: %w", st, err)
		}

		dkg.mu.Lock()
		dkg.DistKeyGens[st] = gen
		dkg.mu.Unlock()
		newMsg.DealBundles[st] = &model.DealBundle{DealBundle: deal, Suite: st.String()}
	}

	// 发送包含所有 suite deal 的消息
	err = dkg.sendDealMessage(model.SendToNodes(dkg.OldAndNetIds()), newMsg)
	if err != nil {
		fmt.Println("Send error:", err)
	}

	return nil
}

// Re-consensus DKG — runs DKG for ALL suites
func (dkg *DKG) reConsensus(msg model.ConsensusMsg) error {
	// old
	dkg.mu.Lock()
	dkg.Threshold = len(msg.OldValidators) - (len(msg.OldValidators)-1)/3
	dkg.Nodes = msg.OldValidators
	// new
	dkg.NewNodes = msg.Validators
	dkg.NewEpoch = msg.Epoch
	// Snapshot before unlock
	nodesSnapshot := make([]*model.Validator, len(dkg.Nodes))
	copy(nodesSnapshot, dkg.Nodes)
	threshold := dkg.Threshold
	dkg.mu.Unlock()

	newThreshold := len(msg.Validators) - (len(msg.Validators)-1)/3

	// 为所有 suite 并行生成 deal
	newMsg, err := model.DeepCopyIf(msg)
	if err != nil {
		return err
	}
	newMsg.DealBundles = make(map[model.SuiteType]*model.DealBundle)

	for _, st := range model.AllSuites() {
		suite := dkg.Suites[st]

		// new DKG 节点列表
		newNodes := make([]pedersen.Node, 0, len(msg.Validators))
		for i, p := range msg.Validators {
			pub := validatorPubForSuite(p, st)
			if pub == nil {
				dkg.finishDkgConsensusStep(false, "re: failed to get public key for:"+st.String())
				return fmt.Errorf("re: failed to get public key for validator %d suite %s", i, st)
			}
			newNodes = append(newNodes, pedersen.Node{
				Index:  uint32(i),
				Public: pub,
			})
		}

		// 获取旧节点列表
		oldNodes := make([]pedersen.Node, 0, len(nodesSnapshot))
		for i, p := range nodesSnapshot {
			pub := validatorPubForSuite(p, st)
			if pub == nil {
				dkg.finishDkgConsensusStep(false, "re: failed to get old public key for:"+st.String())
				return fmt.Errorf("re: failed to get old public key for validator %d suite %s", i, st)
			}
			oldNodes = append(oldNodes, pedersen.Node{
				Index:  uint32(i),
				Public: pub,
			})
		}

		// 初始化协议配置
		conf := pedersen.Config{
			OldNodes:     oldNodes,
			OldThreshold: uint32(threshold),
			Threshold:    uint32(newThreshold),
			NewNodes:     newNodes,
			Nonce:        epochToNonce(msg.Epoch),
			Suite:        suite,
			Auth:         schnorr.NewScheme(suite),
			FastSync:     true,
			Longterm:     dkg.longtermForSuite(st),
			Log:          dkg.log,
		}

		oldShare := dkg.GetDkgKeyShare(st)
		if oldShare != nil {
			conf.Share = &pedersen.DistKeyShare{
				Commits: oldShare.Commitments(),
				Share:   oldShare.PriShare(),
			}
		} else if commits, ok := msg.ShareCommits[st]; ok && len(commits.Public) > 0 {
			conf.PublicCoeffs = commits.Public
		}

		gen, err := pedersen.NewDistKeyHandler(&conf)
		if err != nil {
			dkg.finishDkgConsensusStep(false, "re-NewDistKeyHandler:"+st.String())
			fmt.Println("unable to create DistKeyGenerator for", st, err.Error())
			return err
		}

		dkg.mu.Lock()
		dkg.DistKeyGens[st] = gen
		dkg.mu.Unlock()
		if oldShare != nil {
			// only old validators can generate deals during resharing
			deal, err := gen.Deals()
			if err != nil {
				dkg.finishDkgConsensusStep(false, "re-Deals:"+st.String())
				return fmt.Errorf("failed to generate key shares for %s: %w", st, err)
			}
			newMsg.DealBundles[st] = &model.DealBundle{DealBundle: deal, Suite: st.String()}
		}
	}

	// 重置缓存
	dkg.mu.Lock()
	for _, st := range model.AllSuites() {
		dkg.deals[st] = make(map[string]*model.DealBundle)
		dkg.responses[st] = make(map[string]*pedersen.ResponseBundle)
	}
	dkg.justifs = []*pedersen.JustificationBundle{}
	dkg.mu.Unlock()

	// 发送包含所有 suite deal 的消息
	if len(newMsg.DealBundles) > 0 {
		err := dkg.sendDealMessage(model.SendToNodes(dkg.NewNetIds()), newMsg)
		if err != nil {
			fmt.Println("Send error:", err)
		}
	} else {
		dkg.log.Info("node is not old validator, not send deal")
	}

	return nil
}

// set consensus time out
func (dkg *DKG) addConsensusTimeout() {
	if dkg.failConsensusTimer != nil {
		dkg.failConsensusTimer.Stop()
	}
	dkg.failConsensusTimer = time.AfterFunc(time.Second*30, func() {
		dkg.finishDkgConsensusStep(false, "timeout")
	})
}

// stop consensus
func (dkg *DKG) finishDkgConsensusStep(isok bool, tag string) {
	dkg.mu.Lock()
	if dkg.failConsensusTimer != nil {
		dkg.failConsensusTimer.Stop()
	}
	for _, st := range model.AllSuites() {
		dkg.deals[st] = make(map[string]*model.DealBundle)
		dkg.responses[st] = make(map[string]*pedersen.ResponseBundle)
	}
	dkg.justifs = []*pedersen.JustificationBundle{}
	newEpoch := dkg.NewEpoch
	consensusFailBack := dkg.consensusFailBack
	dkg.mu.Unlock()

	if !isok {
		util.LogWithRed("DKG dkg consensus", "failed <<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<< New Epoch", newEpoch, "error", tag)
		dkg.setConsensusFree()
		if consensusFailBack != nil {
			consensusFailBack(errors.New("DKG dkg consensus failed"))
		}
		return
	}

	dkg.saveState()

	// 首次完成：迁移 new → current
	dkg.mu.Lock()
	edPub := dkg.SystemPubKey()
	if edPub == nil {
		dkg.Nodes = dkg.NewNodes
		// dkg.Epoch = dkg.NewEpoch
		dkg.Threshold = len(dkg.NewNodes) - (len(dkg.NewNodes)-1)/3
		for _, st := range model.AllSuites() {
			dkg.DkgPubKeys[st] = dkg.NewDkgPubKeys[st]
			dkg.DkgKeyShares[st] = dkg.NewDkgKeyShares[st]
		}
	}
	epoch := dkg.Epoch
	threshold := dkg.Threshold
	dkg.mu.Unlock()

	util.LogOk("DKG consensus", "state migrated", "epoch", epoch, "threshold", threshold)
	dkg.SendNewEpochPartialSigToSponsor()
}

// to next epoch
func (dkg *DKG) ToNewEpoch() {
	dkg.mu.Lock()
	if dkg.failConsensusTimer != nil {
		dkg.failConsensusTimer.Stop()
	}
	dkg.mu.Unlock()

	defer dkg.setConsensusFree()

	dkg.mu.Lock()
	edShare := dkg.NewDkgKeyShares[model.SuiteEd25519]
	if edShare == nil {
		dkg.mu.Unlock()
		util.LogWithRed("DKG consensus ToNewEpoch", "failed <<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<< New Epoch", dkg.NewEpoch)
		failBack := dkg.consensusFailBack
		dkg.mu.Unlock()
		if failBack != nil {
			failBack(errors.New("DKG consensus failed"))
		}
		return
	}

	dkg.Nodes = dkg.NewNodes
	dkg.Epoch = dkg.NewEpoch
	dkg.Threshold = len(dkg.NewNodes) - (len(dkg.NewNodes)-1)/3
	for _, st := range model.AllSuites() {
		dkg.DkgPubKeys[st] = dkg.NewDkgPubKeys[st]
		dkg.DkgKeyShares[st] = dkg.NewDkgKeyShares[st]
		dkg.NewDkgPubKeys[st] = nil
		dkg.NewDkgKeyShares[st] = nil
	}

	dkg.NewNodes = nil
	dkg.NewEpoch = 0

	// reset cache
	dkg.NewEpochSponsor = nil
	dkg.mu.Unlock()

	dkg.mu.RLock()
	epoch := dkg.Epoch
	dkg.mu.RUnlock()

	util.LogWithGray("DKG consensus", "successfully <<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<< New Epoch", epoch)
	dkg.saveState()
}

func (dkg *DKG) ConsensusIsbusy() bool {
	dkg.mu.RLock()
	defer dkg.mu.RUnlock()
	return time.Now().Unix()-dkg.lastConsensusTime < 90
}

func (dkg *DKG) TrySetConsensusBusy() bool {
	dkg.mu.Lock()
	defer dkg.mu.Unlock()
	if time.Now().Unix()-dkg.lastConsensusTime < 90 {
		return false
	}
	dkg.lastConsensusTime = time.Now().Unix()
	return true
}

func (dkg *DKG) setConsensusFree() {
	dkg.mu.Lock()
	dkg.lastConsensusTime = 0
	dkg.mu.Unlock()
}

func (dkg *DKG) NewValidatorNodeLen(nodes []*model.Validator) int {
	var count int = 1
	peers := dkg.Peer.AvailableNodes()
	for _, p := range peers {
		for _, node := range nodes {
			if p.String() == node.P2pId.String() {
				count = count + 1
			}
		}
	}
	return count
}

func (dkg *DKG) OldAndNetIds() []*model.PubKey {
	news := make([]*model.PubKey, 0)
	olds := make([]*model.PubKey, 0)
	for _, node := range dkg.NewNodes {
		news = append(news, &node.P2pId)
	}
	for _, node := range dkg.Nodes {
		olds = append(olds, &node.P2pId)
	}

	return MergePubKey(news, olds)
}

func (dkg *DKG) NewNetIds() []*model.PubKey {
	news := make([]*model.PubKey, 0)
	for _, node := range dkg.NewNodes {
		news = append(news, &node.P2pId)
	}
	return news
}

func (dkg *DKG) NetIds() []*model.PubKey {
	olds := make([]*model.PubKey, 0)
	for _, node := range dkg.Nodes {
		olds = append(olds, &node.P2pId)
	}
	return olds
}

func MergePubKey(slices ...[]*model.PubKey) []*model.PubKey {
	seen := make(map[string]bool)
	var result []*model.PubKey

	for _, slice := range slices {
		for _, val := range slice {
			key := val.String()
			if !seen[key] {
				seen[key] = true
				result = append(result, val)
			}
		}
	}

	return result
}
