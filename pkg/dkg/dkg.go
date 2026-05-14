package dkg

import (
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/computeflux/tee-provider/pkg/model"
	p2peer "github.com/computeflux/tee-provider/pkg/network"
	"github.com/computeflux/tee-provider/pkg/util"
	"go.dedis.ch/kyber/v4"
	pedersen "go.dedis.ch/kyber/v4/share/dkg/pedersen"
	"go.dedis.ch/kyber/v4/suites"
)

// DKG 代表  DKG 协议的实例
type DKG struct {
	// Host 是 P2P 网络主机
	Peer p2peer.Peer
	// Suites 加密套件集合
	Suites map[model.SuiteType]suites.Suite
	// Signer 是用于签名的私钥
	Signer *model.PrivKey
	// DistKeyGenerators 各 suite 的分布式密钥生成器
	DistKeyGens map[model.SuiteType]*pedersen.DistKeyGenerator

	// Threshold 是密钥重建所需的最小份额数量
	Threshold int

	// epoch data
	Nodes        []*model.Validator
	Epoch        uint32
	DkgPubKeys   map[model.SuiteType]*model.PubKey
	DkgKeyShares map[model.SuiteType]*model.DistKeyShare

	// next epoch data
	NewNodes        []*model.Validator
	NewEpoch        uint32
	NewDkgPubKeys   map[model.SuiteType]*model.PubKey
	NewDkgKeyShares map[model.SuiteType]*model.DistKeyShare
	NewEpochSponsor *model.Validator
	NewEpochTime    int64

	// cache the deal, response, justification, result
	deals     map[model.SuiteType]map[string]*model.DealBundle
	responses map[model.SuiteType]map[string]*pedersen.ResponseBundle
	justifs   []*pedersen.JustificationBundle
	mu        sync.RWMutex

	// RandomPools holds pre-generated independent random shares for DSS signing.
	RandomPools map[model.SuiteType]*RandomSharePool

	// randomDkgStates tracks in-flight random DKG sessions (randomIndex -> state).
	randomDkgStates map[uint32]*randomDkgState
	randomDkgMu     sync.Mutex

	// mainChan is the channel to receive out message
	mainChain *model.PersistChan[*model.DkgMessage]

	// Consensus is running
	lastConsensusTime    int64
	failConsensusTimer   *time.Timer
	consensusSuccessBack func(*DssSigner, uint64)
	consensusFailBack    func(error)

	// cache
	NewEpochPartialSigTime int64
	NewEpochPartialSigs    map[string]*model.NewEpochMsg

	// 未初始化状态 => 0 | 初始化成功 => 1
	status uint8
	// dkg loger
	log pedersen.Logger
}

// NewDKG 创建一个新的  DKG 实例
func NewDKG(
	NodeSecret *model.PrivKey,
	peer p2peer.Peer,
	log pedersen.Logger,
) (*DKG, error) {
	if log == nil {
		log = NoLogger{}
	}

	suitesMap := make(map[model.SuiteType]suites.Suite)
	for _, st := range model.AllSuites() {
		suitesMap[st] = model.NewSuite(st)
	}

	// 创建 DKG 对象
	dkg := &DKG{
		Suites:          suitesMap,
		Signer:          NodeSecret,
		Peer:            peer,
		log:             log,
		deals:           make(map[model.SuiteType]map[string]*model.DealBundle),
		responses:       make(map[model.SuiteType]map[string]*pedersen.ResponseBundle),
		DkgPubKeys:      make(map[model.SuiteType]*model.PubKey),
		DkgKeyShares:    make(map[model.SuiteType]*model.DistKeyShare),
		NewDkgPubKeys:   make(map[model.SuiteType]*model.PubKey),
		NewDkgKeyShares: make(map[model.SuiteType]*model.DistKeyShare),
		DistKeyGens:     make(map[model.SuiteType]*pedersen.DistKeyGenerator),
		RandomPools:     make(map[model.SuiteType]*RandomSharePool),
	}

	for _, st := range model.AllSuites() {
		dkg.deals[st] = make(map[string]*model.DealBundle)
		dkg.responses[st] = make(map[string]*pedersen.ResponseBundle)
	}

	dkg.Peer.Sub("dkg", dkg.DkgOutHandler)

	// 复原 DKG 对象
	err := dkg.reState()
	if err != nil {
		return nil, fmt.Errorf("restore dkg: %w", err)
	}

	dkg.mainChain, err = model.NewPersistChan[*model.DkgMessage]("dkg", 1000)
	if err != nil {
		return nil, fmt.Errorf("create dkg persist chan: %w", err)
	}

	// Initialize random share pools for each suite.
	for _, st := range model.AllSuites() {
		dkg.RandomPools[st] = NewRandomSharePool(NodeSecret, DefaultRandomSharePoolCapacity)
	}

	return dkg, nil
}

func (dkg *DKG) SystemPubKey() *model.PubKey {
	return dkg.DkgPubKeys[model.SuiteEd25519]
}

// out dkg event Handler
func (dkg *DKG) DkgOutHandler(data any) error {
	msg, ok := data.(*model.DkgMessage)
	if !ok {
		return errors.New("invalid dkg message type")
	}
	dkg.mainChain.Push(msg)
	return nil
}

// Start DKG service
func (dkg *DKG) Start() error {
	util.LogOk("DKG", "Start")
	dkg.mainChain.Start(dkg.handleDkg)
	return nil
}

// Stop DKG
func (dkg *DKG) Stop() {
	dkg.mainChain.Stop()
}

// Get conected node number
func (dkg *DKG) AvailableNodeLen() int {
	dkg.mu.RLock()
	defer dkg.mu.RUnlock()

	// Count self only if we are in the validator set.
	var count int
	selfPub := dkg.Signer.GetPublicForSuite(model.SuiteEd25519)
	if selfPub != nil {
		for _, node := range dkg.Nodes {
			if node.ValidatorId.String() == selfPub.String() {
				count = 1
				break
			}
		}
	}

	peers := dkg.Peer.AvailableNodes()
	for _, p := range peers {
		for _, node := range dkg.Nodes {
			if p.String() == node.P2pId.String() {
				count++
			}
		}
	}
	return count
}

// Share returns the Ed25519 key share (for backward compatibility).
func (d *DKG) Share() (*model.DistKeyShare, error) {
	if d.DkgKeyShares == nil {
		return nil, errors.New("DKG key shares is nil")
	}
	share, ok := d.DkgKeyShares[model.SuiteEd25519]
	if !ok || share == nil {
		return nil, errors.New("DKG Ed25519 key share is nil")
	}
	return share, nil
}

// GetDkgKeyShare returns the key share for a specific suite.
func (d *DKG) GetDkgKeyShare(st model.SuiteType) *model.DistKeyShare {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.DkgKeyShares[st]
}

// GetDkgPubKey returns the public key for a specific suite.
func (d *DKG) GetDkgPubKey(st model.SuiteType) *model.PubKey {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.DkgPubKeys[st]
}

// P2PId returns the p2p id of self node.
func (dkg *DKG) P2PId() *model.PubKey {
	pub := dkg.Signer.GetPublicForSuite(model.SuiteEd25519)
	for _, p := range dkg.Nodes {
		if p.ValidatorId.String() == pub.String() {
			return &p.P2pId
		}
	}
	for _, p := range dkg.NewNodes {
		if p.ValidatorId.String() == pub.String() {
			return &p.P2pId
		}
	}

	util.LogError("P2P 404 ", dkg.Signer.GetPublicForSuite(model.SuiteEd25519).SS58())
	return nil
}

// Send message to node
func (dkg *DKG) sendToNode(to *model.To, message *model.DkgMessage) error {
	if to == nil {
		fmt.Println("sendToNode node is nil")
		return errors.New("node is nil")
	}

	p2pId := dkg.P2PId()
	if p2pId == nil {
		fmt.Println("sendToNode P2PID is nil")
		return errors.New("P2PID is nil")
	}

	message.From = p2pId.String()
	return dkg.Peer.Send(&model.PeerMsg{
		To:      to,
		Payload: &model.PeerMsg_DkgMessage{DkgMessage: message},
	})
}

func (dkg *DKG) longtermForSuite(st model.SuiteType) kyber.Scalar {
	return dkg.Signer.Scalar(st)
}

// Get node by string id
func (dkg *DKG) getNode(nodeId string) *model.PubKey {
	nodes := dkg.Peer.AvailableNodes()
	for _, node := range nodes {
		if node.String() == nodeId {
			return node
		}
	}

	return nil
}

// epoch to nonce
func epochToNonce(v uint32) []byte {
	var nonce [pedersen.NonceLength]byte
	var epoch = fmt.Append(nil, v)

	copy(nonce[:], epoch)

	return nonce[:]
}
