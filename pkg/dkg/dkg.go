package dkg

import (
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/wetee-dao/tee-dsecret/pkg/model"
	p2peer "github.com/wetee-dao/tee-dsecret/pkg/network"
	"github.com/wetee-dao/tee-dsecret/pkg/util"
	pedersen "go.dedis.ch/kyber/v4/share/dkg/pedersen"
	"go.dedis.ch/kyber/v4/suites"
)

// DKG 代表  DKG 协议的实例
type DKG struct {
	// Host 是 P2P 网络主机
	Peer p2peer.Peer
	// Suite 是加密套件
	Suite suites.Suite
	// Signer 是用于签名的私钥
	Signer *model.PrivKey
	// DistKeyGenerator
	DistKeyGenerator *pedersen.DistKeyGenerator

	// Threshold 是密钥重建所需的最小份额数量
	Threshold int

	// epoch data
	Nodes       []*model.Validator
	Epoch       uint32
	DkgPubKey   *model.PubKey
	DkgKeyShare *model.DistKeyShare

	// next epoch data
	NewNodes        []*model.Validator
	NewEpoch        uint32
	NewDkgPubKey    *model.PubKey // dkg key
	NewDkgKeyShare  *model.DistKeyShare
	NewEpochSponsor *model.Validator
	NewEpochTime    int64

	// cache the deal, response, justification, result
	deals     map[string]*model.DealBundle
	responses map[string]*pedersen.ResponseBundle
	justifs   []*pedersen.JustificationBundle
	mu        sync.RWMutex

	// RandomPool holds pre-generated independent random shares for DSS signing.
	RandomPool *RandomSharePool

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

	// 创建 DKG 对象
	dkg := &DKG{
		Suite:     suites.MustFind("Ed25519"),
		Signer:    NodeSecret,
		Peer:      peer,
		log:       log,
		deals:     make(map[string]*model.DealBundle),
		responses: make(map[string]*pedersen.ResponseBundle),
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

	// Initialize random share pool.
	dkg.RandomPool = NewRandomSharePool(NodeSecret, DefaultRandomSharePoolCapacity)

	return dkg, nil
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
	var count int = 1
	peers := dkg.Peer.AvailableNodes()
	for _, p := range peers {
		for _, node := range dkg.Nodes {
			if p.String() == node.P2pId.String() {
				count = count + 1
			}
		}
	}
	return count
}

func (d *DKG) Share() (*model.DistKeyShare, error) {
	if d.DkgKeyShare == nil {
		return nil, errors.New("DKG key share is nil")
	}
	return d.DkgKeyShare, nil
}

func (d *DKG) GetDkgKeyShare() *model.DistKeyShare {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.DkgKeyShare
}

func (d *DKG) GetDkgPubKey() *model.PubKey {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.DkgPubKey
}

// // Get validator id
// func (dkg *DKG) validatorID() *model.PubKey {
// 	pub := dkg.Signer.GetPublic()
// 	for _, p := range dkg.Nodes {
// 		if p.ValidatorId.String() == pub.String() {
// 			return &p.ValidatorId
// 		}
// 	}
// 	for _, p := range dkg.NewNodes {
// 		if p.ValidatorId.String() == pub.String() {
// 			return &p.ValidatorId
// 		}
// 	}
// 	return nil
// }

// Get p2p id of self node
func (dkg *DKG) P2PId() *model.PubKey {
	pub := dkg.Signer.GetPublic()
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

	util.LogError("P2P 404 ", dkg.Signer.GetPublic().SS58())
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
	// if message.From == to.String() {
	// 	dkg.mainChain.Push(message)
	// 	return nil
	// }

	return dkg.Peer.Send(&model.PeerMsg{
		To:      to,
		Payload: &model.PeerMsg_DkgMessage{DkgMessage: message},
	})
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
