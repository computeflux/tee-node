package dkg

import (
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/computeflux/tee-provider/pkg/chaintypes"
	"github.com/computeflux/tee-provider/pkg/crypto"
	dkgtypes "github.com/computeflux/tee-provider/pkg/dkg/types"
	"github.com/computeflux/tee-provider/pkg/model"
	p2peer "github.com/computeflux/tee-provider/pkg/network"
	"github.com/computeflux/tee-provider/pkg/store"
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
	Suites map[crypto.SuiteType]suites.Suite
	// Signer 是用于签名的私钥
	Signer *crypto.PrivKey
	// DistKeyGenerators 各 suite 的分布式密钥生成器
	DistKeyGens map[crypto.SuiteType]*pedersen.DistKeyGenerator

	// Threshold 是密钥重建所需的最小份额数量
	Threshold int

	// epoch data
	Nodes        []*chaintypes.Validator
	Epoch        uint32
	DkgPubKeys   map[crypto.SuiteType]*crypto.PubKey
	DkgKeyShares map[crypto.SuiteType]*dkgtypes.DistKeyShare

	// next epoch data
	NewNodes        []*chaintypes.Validator
	NewEpoch        uint32
	NewDkgPubKeys   map[crypto.SuiteType]*crypto.PubKey
	NewDkgKeyShares map[crypto.SuiteType]*dkgtypes.DistKeyShare
	NewEpochSponsor *chaintypes.Validator
	NewEpochTime    int64

	// cache the deal, response, justification, result
	deals     map[crypto.SuiteType]map[string]*dkgtypes.DealBundle
	responses map[crypto.SuiteType]map[string]*pedersen.ResponseBundle
	justifs   []*pedersen.JustificationBundle
	mu        sync.RWMutex

	// RandomPools holds pre-generated independent random shares for DSS signing.
	RandomPools map[crypto.SuiteType]*RandomSharePool

	// randomDkgStates tracks in-flight random DKG sessions (randomIndex -> state).
	randomDkgStates map[uint32]*randomDkgState
	randomDkgMu     sync.Mutex

	// mainChan is the channel to receive out message
	mainChain *store.PersistChan[*model.DkgMessage]

	// Consensus is running
	lastConsensusTime    int64
	failConsensusTimer   *time.Timer
	consensusSuccessBack func(*DssSigner, uint64)
	consensusFailBack    func(error)

	// cache
	NewEpochPartialSigTime int64
	NewEpochPartialSigs    map[string]*dkgtypes.NewEpochMsg

	// 未初始化状态 => 0 | 初始化成功 => 1
	status uint8
	// dkg loger
	log pedersen.Logger
}

// NewDKG 创建一个新的  DKG 实例
func NewDKG(
	NodeSecret *crypto.PrivKey,
	peer p2peer.Peer,
	log pedersen.Logger,
) (*DKG, error) {
	if log == nil {
		log = NoLogger{}
	}

	suitesMap := make(map[crypto.SuiteType]suites.Suite)
	for _, st := range crypto.AllSuites() {
		suitesMap[st] = crypto.NewSuite(st)
	}

	// 创建 DKG 对象
	dkg := &DKG{
		Suites:          suitesMap,
		Signer:          NodeSecret,
		Peer:            peer,
		log:             log,
		deals:           make(map[crypto.SuiteType]map[string]*dkgtypes.DealBundle),
		responses:       make(map[crypto.SuiteType]map[string]*pedersen.ResponseBundle),
		DkgPubKeys:      make(map[crypto.SuiteType]*crypto.PubKey),
		DkgKeyShares:    make(map[crypto.SuiteType]*dkgtypes.DistKeyShare),
		NewDkgPubKeys:   make(map[crypto.SuiteType]*crypto.PubKey),
		NewDkgKeyShares: make(map[crypto.SuiteType]*dkgtypes.DistKeyShare),
		DistKeyGens:     make(map[crypto.SuiteType]*pedersen.DistKeyGenerator),
		RandomPools:     make(map[crypto.SuiteType]*RandomSharePool),
	}

	for _, st := range crypto.AllSuites() {
		dkg.deals[st] = make(map[string]*dkgtypes.DealBundle)
		dkg.responses[st] = make(map[string]*pedersen.ResponseBundle)
	}

	dkg.Peer.Sub("dkg", dkg.DkgOutHandler)

	// 复原 DKG 对象
	err := dkg.reState()
	if err != nil {
		return nil, fmt.Errorf("restore dkg: %w", err)
	}

	dkg.mainChain, err = store.NewPersistChan[*model.DkgMessage]("dkg", 1000)
	if err != nil {
		return nil, fmt.Errorf("create dkg persist chan: %w", err)
	}

	// Initialize random share pools for each suite.
	for _, st := range crypto.AllSuites() {
		dkg.RandomPools[st] = NewRandomSharePool(NodeSecret, DefaultRandomSharePoolCapacity)
	}

	return dkg, nil
}

func (dkg *DKG) SystemPubKey() *crypto.PubKey {
	return dkg.DkgPubKeys[crypto.SuiteEd25519]
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
	selfPub := dkg.Signer.GetPublicForSuite(crypto.SuiteEd25519)
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
func (d *DKG) Share() (*dkgtypes.DistKeyShare, error) {
	if d.DkgKeyShares == nil {
		return nil, errors.New("DKG key shares is nil")
	}
	share, ok := d.DkgKeyShares[crypto.SuiteEd25519]
	if !ok || share == nil {
		return nil, errors.New("DKG Ed25519 key share is nil")
	}
	return share, nil
}

// GetDkgKeyShare returns the key share for a specific suite.
func (d *DKG) GetDkgKeyShare(st crypto.SuiteType) *dkgtypes.DistKeyShare {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.DkgKeyShares[st]
}

// GetDkgPubKey returns the public key for a specific suite.
func (d *DKG) GetDkgPubKey(st crypto.SuiteType) *crypto.PubKey {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.DkgPubKeys[st]
}

// P2PId returns the p2p id of self node.
func (dkg *DKG) P2PId() *crypto.PubKey {
	dkg.mu.RLock()
	defer dkg.mu.RUnlock()
	pub := dkg.Signer.GetPublicForSuite(crypto.SuiteEd25519)
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

	util.LogError("P2P 404 ", dkg.Signer.GetPublicForSuite(crypto.SuiteEd25519).SS58())
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

func (dkg *DKG) longtermForSuite(st crypto.SuiteType) kyber.Scalar {
	return dkg.Signer.Scalar(st)
}

// epoch to nonce
func epochToNonce(v uint32) []byte {
	var nonce [pedersen.NonceLength]byte
	var epoch = fmt.Append(nil, v)

	copy(nonce[:], epoch)

	return nonce[:]
}
