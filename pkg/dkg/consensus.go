package dkg

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/wetee-dao/tee-dsecret/pkg/model"
	"github.com/wetee-dao/tee-dsecret/pkg/util"
	"go.dedis.ch/kyber/v4"
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
	if available <= threshold {
		util.LogError("DKG Consensus", "validator node exapect >", threshold, ", got:", available)
		return fmt.Errorf("old validators count <= dkg.Threshold")
	}

	// check new nodes validators length
	if dkg.NewValidatorNodeLen(msg.Validators) <= len(msg.Validators)*3/4 {
		util.LogError("DKG Consensus", "exapect new validator count:", len(msg.Validators)*3/4, ", got:", dkg.NewValidatorNodeLen(msg.Validators))
		return fmt.Errorf("new validators count < len(Validators)*3/4")
	}

	// check local node is in validators, only validator node can start consensus
	dkg.mu.RLock()
	dkgKeyShare := dkg.DkgKeyShare
	nodeCount := len(dkg.Nodes)
	nodesCopy := make([]*model.Validator, 0, nodeCount)
	for _, n := range dkg.Nodes {
		nodesCopy = append(nodesCopy, n)
	}
	dkg.mu.RUnlock()

	if dkgKeyShare == nil && msg.Epoch > StartEpoch {
		util.LogError("DKG Consensus", "msg.Epoch", msg.Epoch, "| Node is not old validator, cannot start consensus")
		return errors.New("node is not old validator, cannot start consensus")
	}

	// init consensus msg
	if dkgKeyShare != nil {
		msg.ShareCommits = *util.DeepCopy(dkgKeyShare.CommitsWrap)
		msg.ConsensusNodeNum = nodeCount
		msg.OldValidators = nodesCopy
	} else {
		msg.ShareCommits = model.KyberPoints{Public: []kyber.Point{}}
		msg.ConsensusNodeNum = 0
	}

	// generate new validators sidekey
	dkgSigner := dkg.Signer.GetPublic().SS58()
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
	dkg.consensusSuccessBack = callback
	dkg.consensusFailBack = fail

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

	if dkg.Epoch >= msg.Epoch {
		dkg.setConsensusFree()
		return errors.New("DKG Epoch is not need to update")
	}

	dkg.addConsensusTimeout()

	if len(msg.ShareCommits.Public) == 0 {
		util.LogWithGray("InitConsensus Epoch ======> ", msg.Epoch)
		return dkg.initConsensus(msg)
	}

	util.LogWithGray("ReConsensus Epoch ======> ", msg.Epoch)
	return dkg.reConsensus(msg)
}

// Init Consensus
func (dkg *DKG) initConsensus(msg model.ConsensusMsg) error {
	// if flag.Lookup("test.v") == nil {
	// 	go dkg.HandleSecretSave()
	// }
	dkg.mu.Lock()
	dkg.Nodes = msg.Validators
	dkg.NewNodes = msg.Validators
	dkg.Threshold = len(msg.Validators) * 2 / 3
	dkg.mu.Unlock()

	// 如果已经初始化，则直接返回
	if dkg.status == 1 {
		return nil
	}

	// dkg 节点列表
	nodes := make([]pedersen.Node, 0, len(dkg.Nodes))
	for i, p := range dkg.Nodes {
		nodes = append(nodes, pedersen.Node{
			Index:  uint32(i),
			Public: p.ValidatorId.Point(),
		})
	}
	signer := schnorr.NewScheme(dkg.Suite)

	// 初始化协议配置
	dkg.mu.RLock()
	threshold := dkg.Threshold
	dkg.mu.RUnlock()
	conf := pedersen.Config{
		Suite:     dkg.Suite,
		NewNodes:  nodes,
		Threshold: threshold,
		Auth:      signer,
		FastSync:  true,
		Longterm:  dkg.Signer.Scalar(),
		Nonce:     epochToNonce(0),
		Log:       dkg.log,
	}

	// initialize dealer
	var err error
	dkg.DistKeyGenerator, err = pedersen.NewDistKeyHandler(&conf)
	if err != nil {
		dkg.finishDkgConsensusStep(false, "pedersen.NewDistKeyHandler")
		return fmt.Errorf("failed to initialize DKG protocol: %w", err)
	}

	// 获取当前节点的协议
	// get deal of current node
	deal, err := dkg.DistKeyGenerator.Deals()
	if err != nil {
		dkg.finishDkgConsensusStep(false, "dkg.DistKeyGenerator.Deals")
		return fmt.Errorf("failed to generate key shares: %w", err)
	}

	// 开启节点共识
	// send deal to all nodes
	newMsg := util.DeepCopy(msg)
	newMsg.DealBundle = &model.DealBundle{DealBundle: deal}
	newMsg.ShareCommits = model.KyberPoints{}
	err = dkg.sendDealMessage(model.SendToNodes(dkg.OldAndNetIds()), newMsg)
	if err != nil {
		fmt.Println("Send error:", err)
	}

	return nil
}

// Re-consensus DKG
func (dkg *DKG) reConsensus(msg model.ConsensusMsg) error {
	// old
	dkg.mu.Lock()
	dkg.Threshold = len(msg.OldValidators) * 2 / 3
	dkg.Nodes = msg.OldValidators
	// new
	dkg.NewNodes = msg.Validators
	dkg.NewEpoch = msg.Epoch
	dkg.mu.Unlock()

	// new DKG 节点列表
	newNodes := make([]pedersen.Node, 0, len(msg.Validators))
	for i, p := range msg.Validators {
		newNodes = append(newNodes, pedersen.Node{
			Index:  uint32(i),
			Public: p.ValidatorId.Point(),
		})
	}

	// 获取旧节点列表
	oldNodes := make([]pedersen.Node, 0, len(dkg.Nodes))
	for i, p := range dkg.Nodes {
		oldNodes = append(oldNodes, pedersen.Node{
			Index:  uint32(i),
			Public: p.ValidatorId.Point(),
		})
	}

	newThreshold := len(msg.Validators) * 2 / 3

	// 初始化协议配置
	conf := pedersen.Config{
		OldNodes:     oldNodes,
		OldThreshold: dkg.Threshold,
		Threshold:    newThreshold,
		NewNodes:     newNodes,
		Nonce:        epochToNonce(msg.Epoch),
		Suite:        dkg.Suite,
		Auth:         schnorr.NewScheme(dkg.Suite),
		FastSync:     true,
		Longterm:     dkg.Signer.Scalar(),
		Log:          dkg.log,
	}

	if dkg.DkgKeyShare != nil {
		priv := dkg.DkgKeyShare
		conf.Share = &pedersen.DistKeyShare{
			Commits: priv.Commitments(),
			Share:   priv.PriShare(),
		}
	} else {
		conf.PublicCoeffs = msg.ShareCommits.Public
	}

	var err error
	dkg.DistKeyGenerator, err = pedersen.NewDistKeyHandler(&conf)
	if err != nil {
		dkg.finishDkgConsensusStep(false, "pedersen.NewDistKeyHandler(&conf)")
		fmt.Println("unable to create DistKeyGenerator", err.Error())
		return err
	}

	priShare := dkg.DkgKeyShare

	// 重置 DKG Key
	dkg.mu.Lock()
	dkg.deals = map[string]*model.DealBundle{}
	dkg.responses = map[string]*pedersen.ResponseBundle{}
	dkg.justifs = []*pedersen.JustificationBundle{}
	dkg.mu.Unlock()

	// old node not issue deals
	if priShare == nil {
		dkg.log.Info("node is not old validator, not send deal")
		return nil
	}

	// 获取当前节点的协议
	deal, err := dkg.DistKeyGenerator.Deals()
	if err != nil {
		dkg.finishDkgConsensusStep(false, "dkg.DistKeyGenerator.Deals()")
		return fmt.Errorf("failed to generate key shares: %w", err)
	}

	// 开启节点共识
	newMsg := util.DeepCopy(msg)
	newMsg.DealBundle = &model.DealBundle{DealBundle: deal}
	err = dkg.sendDealMessage(model.SendToNodes(dkg.NewNetIds()), newMsg)
	if err != nil {
		fmt.Println("Send error:", err)
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
	if dkg.failConsensusTimer != nil {
		dkg.failConsensusTimer.Stop()
	}

	dkg.mu.Lock()
	dkg.deals = map[string]*model.DealBundle{}
	dkg.responses = map[string]*pedersen.ResponseBundle{}
	dkg.justifs = []*pedersen.JustificationBundle{}
	dkg.mu.Unlock()
	if !isok {
		util.LogWithRed("DKG dkg consensus", "failed <<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<< New Epoch", dkg.NewEpoch, "error", tag)
		dkg.setConsensusFree()
		if dkg.consensusFailBack != nil {
			dkg.consensusFailBack(errors.New("DKG dkg consensus failed"))
		}
		return
	}

	dkg.saveState()
	// if dkg.DkgPubKey == nil, set new data to init
	dkg.mu.Lock()
	if dkg.DkgPubKey == nil {
		dkg.Nodes = dkg.NewNodes
		dkg.Epoch = dkg.NewEpoch
		dkg.Threshold = len(dkg.NewNodes) * 2 / 3
		dkg.DkgPubKey = dkg.NewDkgPubKey
		dkg.DkgKeyShare = dkg.NewDkgKeyShare
	}
	dkg.mu.Unlock()
	dkg.SendNewEpochPartialSigToSponsor()
}

// to next epoch
func (dkg *DKG) ToNewEpoch() {
	if dkg.failConsensusTimer != nil {
		dkg.failConsensusTimer.Stop()
	}

	defer dkg.setConsensusFree()

	dkg.mu.Lock()
	if dkg.NewDkgKeyShare == nil {
		dkg.mu.Unlock()
		util.LogWithRed("DKG consensus ToNewEpoch", "failed <<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<< New Epoch", dkg.NewEpoch)
		if dkg.consensusFailBack != nil {
			dkg.consensusFailBack(errors.New("DKG consensus failed"))
		}
		return
	}

	dkg.Nodes = dkg.NewNodes
	dkg.Epoch = dkg.NewEpoch
	dkg.Threshold = len(dkg.NewNodes) * 2 / 3
	dkg.DkgPubKey = dkg.NewDkgPubKey
	dkg.DkgKeyShare = dkg.NewDkgKeyShare

	dkg.NewNodes = nil
	dkg.NewEpoch = 0
	dkg.NewDkgPubKey = nil
	dkg.NewDkgKeyShare = nil

	// reset cache
	dkg.NewEpochSponsor = nil
	dkg.mu.Unlock()

	util.LogWithGray("DKG consensus", "successfully <<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<< New Epoch", dkg.Epoch)
	dkg.saveState()

}

func (dkg *DKG) ConsensusIsbusy() bool {
	dkg.mu.Lock()
	defer dkg.mu.Unlock()
	return time.Now().Unix()-dkg.lastConsensusTime < 90
}

// TrySetConsensusBusy atomically checks if consensus is busy and sets it if not.
// Returns true if successfully set, false if already busy.
func (dkg *DKG) TrySetConsensusBusy() bool {
	dkg.mu.Lock()
	defer dkg.mu.Unlock()
	if time.Now().Unix()-dkg.lastConsensusTime < 90 {
		return false
	}
	dkg.lastConsensusTime = time.Now().Unix()
	return true
}

func (dkg *DKG) setConsensusBusy() {
	dkg.mu.Lock()
	dkg.lastConsensusTime = time.Now().Unix()
	dkg.mu.Unlock()
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
