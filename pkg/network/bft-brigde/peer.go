package bftbrigde

import (
	"fmt"

	"github.com/cometbft/cometbft/p2p"
	"github.com/wetee-dao/tee-dsecret/pkg/model"
	"github.com/wetee-dao/tee-dsecret/pkg/util"
)

const pendingMsgKey = "bft_reactor_pending_msgs"

func (p *BTFReactor) Send(msg *model.PeerMsg) error {
	// 检查P2P网络是否就绪
	if p.Switch == nil {
		// P2P未就绪，将消息持久化存储
		if err := p.savePendingMsg(msg); err != nil {
			util.LogWithRed("BTFReactor.Send", "Failed to save pending message:", err)
			return err
		}
		util.LogWithYellow("BTFReactor.Send", "P2P not ready, message saved to db")
		return nil
	}

	return p.doSend(msg)
}

// doSend 实际执行发送逻辑
func (p *BTFReactor) doSend(msg *model.PeerMsg) error {
	sendData := p2p.Envelope{}

	// set sender id
	sendData.Src = LocalPeer{id: p.Switch.NodeInfo().ID()}

	// 根据 payload 类型设置 channel 和 message
	switch payload := msg.Payload.(type) {
	case *model.PeerMsg_DkgMessage:
		sendData.ChannelID = topics["dkg"].ID
		payload.DkgMessage.To = msg.To
		sendData.Message = payload.DkgMessage
	case *model.PeerMsg_BlockPartialSign:
		sendData.ChannelID = topics["block-partial-sign"].ID
		payload.BlockPartialSign.To = msg.To
		sendData.Message = payload.BlockPartialSign
	case *model.PeerMsg_SecretBox:
		sendData.ChannelID = topics["secret"].ID
		payload.SecretBox.To = msg.To
		sendData.Message = payload.SecretBox
	default:
		return fmt.Errorf("unknown message type: %T", payload)
	}

	if msg.To.Check(p.id) {
		p.Receive(sendData)
	}

	p.Switch.Broadcast(sendData)
	return nil
}

// savePendingMsg 将消息持久化到数据库
func (p *BTFReactor) savePendingMsg(msg *model.PeerMsg) error {
	// 加载现有队列
	store, err := p.loadPendingMsgsStore()
	if err != nil {
		return fmt.Errorf("load pending msgs failed: %w", err)
	}

	// 添加新消息
	store.Msgs = append(store.Msgs, msg)

	// 保存到数据库
	if err := model.SetJson("", pendingMsgKey, store); err != nil {
		return fmt.Errorf("save pending msgs failed: %w", err)
	}

	return nil
}

// pendingMsgsStore 持久化存储包装器
type pendingMsgsStore struct {
	Msgs []*model.PeerMsg `json:"msgs"`
}

// loadPendingMsgsStore 从数据库加载待发送消息
func (p *BTFReactor) loadPendingMsgsStore() (*pendingMsgsStore, error) {
	store, err := model.GetJson[pendingMsgsStore]("", pendingMsgKey)
	if err != nil {
		return nil, err
	}
	if store == nil {
		return &pendingMsgsStore{Msgs: make([]*model.PeerMsg, 0)}, nil
	}
	return store, nil
}

// loadPendingMsgs 从数据库加载消息到内存队列（在NewBTFReactor中调用）
func (p *BTFReactor) loadPendingMsgs() {
	store, err := p.loadPendingMsgsStore()
	if err != nil {
		util.LogWithRed("BTFReactor.loadPendingMsgs", "Failed to load pending msgs:", err)
		return
	}

	if len(store.Msgs) == 0 {
		return
	}

	// 直接存储 PeerMsg 到内存队列
	p.pendingMu.Lock()
	p.pendingMsgs = append(p.pendingMsgs, store.Msgs...)
	p.pendingMu.Unlock()

	util.LogWithYellow("BTFReactor.loadPendingMsgs", "Loaded", len(p.pendingMsgs), "pending messages from db")
}

// clearPendingMsgs 清空数据库中的待发送消息
func (p *BTFReactor) clearPendingMsgs() error {
	return model.DeleteKey("", pendingMsgKey)
}

// flushPendingMsgs 发送所有暂存的消息（P2P就绪后调用）
func (p *BTFReactor) flushPendingMsgs() {
	p.pendingMu.Lock()
	if len(p.pendingMsgs) == 0 {
		p.pendingMu.Unlock()
		return
	}

	msgs := p.pendingMsgs
	p.pendingMsgs = nil
	p.pendingMu.Unlock()

	util.LogWithYellow("BTFReactor.flushPendingMsgs", "Flushing", len(msgs), "pending messages")

	for _, msg := range msgs {
		if err := p.doSend(msg); err != nil {
			util.LogWithRed("BTFReactor.flushPendingMsgs", "Failed to send pending message:", err)
		}
	}

	// 清空数据库
	if err := p.clearPendingMsgs(); err != nil {
		util.LogWithRed("BTFReactor.flushPendingMsgs", "Failed to clear pending msgs from db:", err)
	}

	util.LogWithYellow("BTFReactor.flushPendingMsgs", "All pending messages flushed")
}

// func (p *BTFReactor) Pub(topic string, data []byte) error {
// 	channel, ok := topics[topic]
// 	if !ok {
// 		return errors.New("topic not found")
// 	}

// 	p.Switch.Broadcast(p2p.Envelope{
// 		ChannelID: channel.ID,
// 	})

// 	return nil
// }

func (p *BTFReactor) Sub(topic string, handler func(any) error) error {
	switch topic {
	case "dkg":
		p.dkgHandler = handler
	case "block-partial-sign":
		p.blockPartialSignHandler = handler
	case "secret":
		p.secretHandler = handler
	default:
		return fmt.Errorf("topic not found: %s", topic)
	}
	return nil
}

// Get all available nodes
func (p *BTFReactor) AvailableNodes() []*model.PubKey {
	peers := p.Switch.Peers()
	nodes := make([]*model.PubKey, 0, peers.Size())
	for _, n := range p.nodekeys {
		for _, peer := range peers.List() {
			if peer.ID() == n.SideChainNodeID() {
				nodes = append(nodes, n)
			}
		}
	}

	return nodes
}

// Get all nodes
func (p *BTFReactor) AllNodes() []*model.PubKey {
	return p.nodekeys
}
