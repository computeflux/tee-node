package bftbrigde

import (
	"errors"
	"sync"

	"github.com/cometbft/cometbft/libs/service"
	"github.com/cometbft/cometbft/p2p"
	"github.com/cometbft/cometbft/p2p/conn"
	"github.com/cometbft/cometbft/types"

	"github.com/computeflux/tee-provider/pkg/chains"
	"github.com/computeflux/tee-provider/pkg/model"
	"github.com/computeflux/tee-provider/pkg/types/crypto"
	"github.com/computeflux/tee-provider/pkg/util"
)

var MaxMsgSize = types.MaxBlockSizeBytes
var topics = map[string]p2p.ChannelDescriptor{
	"dkg": { // dkg msg
		ID:                  255,
		Priority:            10000,
		SendQueueCapacity:   1000,
		RecvBufferCapacity:  50 * 4096,
		RecvMessageCapacity: MaxMsgSize,
		MessageType:         &bftMsgWrapper{},
	},
	"block-partial-sign": { // block partial sign msg
		ID:                  254,
		Priority:            10000,
		SendQueueCapacity:   1000,
		RecvBufferCapacity:  50 * 4096,
		RecvMessageCapacity: MaxMsgSize,
		MessageType:         &bftMsgWrapper{},
	},
	"secret": {
		ID:                  253,
		Priority:            10000,
		SendQueueCapacity:   1000,
		RecvBufferCapacity:  50 * 4096,
		RecvMessageCapacity: MaxMsgSize,
		MessageType:         &bftMsgWrapper{},
	},
}

type BTFReactor struct {
	service.BaseService
	Switch p2p.Switcher

	id                      *crypto.PubKey
	validators              []*crypto.PubKey
	nodekeys                []*crypto.PubKey
	dkgHandler              func(any) error
	blockPartialSignHandler func(any) error
	secretHandler           func(any) error

	// P2P网络就绪回调（Switch设置后触发）
	onSwitchReady func()

	// 启动期间的消息暂存队列（P2P就绪前使用）
	pendingMsgs []*model.PeerMsg
	pendingMu   sync.Mutex
}

func NewBTFReactor(name string) *BTFReactor {
	r := &BTFReactor{}
	r.BaseService = *service.NewBaseService(nil, name, r)

	// 加载持久化的待发送消息
	r.loadPendingMsgs()

	return r
}

// SetID 设置本节点的公钥标识，应在 P2P 启动前调用。
// 设置后重新加载待发队列：pendingMsgKey 依赖 id，而 NewBTFReactor 构造时
// id 尚未设置，若不在此处加载，保存/加载使用的 key 不一致导致消息无法恢复。
func (r *BTFReactor) SetID(id *crypto.PubKey) {
	r.id = id
	r.loadPendingMsgs()
}

// SetOnSwitchReady 设置P2P网络就绪回调
func (r *BTFReactor) SetOnSwitchReady(callback func()) {
	r.onSwitchReady = callback
}

// 实现 Service 接口的 OnStart 生命周期钩子
func (r *BTFReactor) OnStart() error {
	nodeInfo := r.Switch.NodeInfo()
	address, err := nodeInfo.NetAddress()
	if err != nil {
		util.LogWithYellow("Local Address ", err)
	} else {
		util.LogWithYellow("Local Address ", address.String())
	}
	r.PrintPeers("P2P OnStart")

	return nil
}

// 实现 OnStop 生命周期钩子
func (r *BTFReactor) OnStop() {
	util.LogWithYellow("P2P OnStop")
}

func (r *BTFReactor) OnReset() error {
	util.LogWithYellow("P2P OnReset")
	return nil
}

func (dr *BTFReactor) SetSwitch(sw p2p.Switcher) {
	dr.Switch = sw

	// 发送启动期间暂存的所有消息
	dr.flushPendingMsgs()

	// 通知所有监听者P2P网络已就绪
	if dr.onSwitchReady != nil {
		dr.onSwitchReady()
	}
}

func (*BTFReactor) GetChannels() []*conn.ChannelDescriptor {
	channels := make([]*conn.ChannelDescriptor, 0, len(topics))
	for _, c := range topics {
		channels = append(channels, &c)
	}
	return channels
}

func (r *BTFReactor) AddPeer(p2p.Peer) {
	r.PrintPeers("P2P AddPeer")
}

func (r *BTFReactor) RemovePeer(p2p.Peer, any) {
	r.PrintPeers("P2P DelPeer")
}

func (*BTFReactor) InitPeer(peer p2p.Peer) p2p.Peer {
	return peer
}

func (r *BTFReactor) GetPubkeyFromPeerID(peer p2p.ID) (*crypto.PubKey, error) {
	for _, node := range r.nodekeys {
		if node.TeeChainNodeID() == peer {
			return node, nil
		}
	}

	return nil, errors.New("not found")
}

func (r *BTFReactor) PrintPeers(event string) {
	if chains.MainChain == nil {
		return
	}

	// get from main chain
	validatorWrap, pubkeys, err := chains.MainChain.GetNodes()
	if err != nil {
		util.LogWithRed(event, "GetNodes", err.Error())
	}

	// save self nodekey
	for _, v := range pubkeys {
		if v.TeeChainNodeID() == r.Switch.NodeInfo().ID() {
			r.id = v
		}
	}

	// save all nodekeys and validators
	r.nodekeys = pubkeys
	validators := make([]*crypto.PubKey, len(validatorWrap))
	for i, v := range validatorWrap {
		validators[i] = &v.P2pId
	}
	r.validators = validators

	outbound, inbound, dialing := r.Switch.NumPeers()
	util.LogWithYellow(event, "Peers outbound=>", outbound, "inbound=>", inbound, "dialing=>", dialing, " || nodekeys=>", len(r.nodekeys))
}
