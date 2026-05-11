package local

import (
	"crypto/ed25519"
	"encoding/hex"
	"errors"
	"fmt"

	"github.com/wetee-dao/tee-dsecret/pkg/model"
	"go.dedis.ch/kyber/v4"
)

var (
	peers = make(map[string]*Peer)
)

func NewNetwork(priv *model.PrivKey, boots []string, nodes []*model.PubKey, tcp, udp uint32) (*Peer, error) {
	id := priv.GetPublic().String()

	// 创建 P2P 网络实例
	peer := &Peer{
		id:       id,
		privKey:  priv.PrivateKey,
		nodes:    nodes,
		handlers: make(map[string]func(any) error),
		callBack: func(ty string) error {
			fmt.Println("::::::::::::::::::::::::::::::::::::::::::::::::::::::::::::: netHook not found error")
			return nil
		},
		version: 1,
	}

	peers[id] = peer

	return peer, nil
}

type Peer struct {
	id         string
	privKey    ed25519.PrivateKey
	nodes      []*model.PubKey
	handlers   map[string]func(any) error
	callBack   func(string) error
	version    uint32
	PreCommits []kyber.Point
}

// Send implements pkg/network/peer.Peer.
// It routes messages to local in-memory peers based on PeerMsg.To and payload type.
func (p *Peer) Send(message *model.PeerMsg) error {
	if message == nil {
		return errors.New("nil peer message")
	}
	to := message.GetTo()
	if to == nil {
		return errors.New("nil peer message to")
	}

	switch payload := message.GetPayload().(type) {
	case *model.PeerMsg_DkgMessage:
		return p.sendTo(to, "dkg", payload.DkgMessage)
	case *model.PeerMsg_BlockPartialSign:
		return p.sendTo(to, "block-partial-sign", payload.BlockPartialSign)
	case *model.PeerMsg_SecretBox:
		return p.sendTo(to, "secret-box", payload.SecretBox)
	default:
		return errors.New("unknown peer payload type")
	}
}

func (p *Peer) sendTo(to *model.To, topic string, message any) error {
	switch to.Payload.(type) {
	case *model.To_Node:
		peer := peers[hex.EncodeToString(to.GetNode())]
		if peer == nil {
			return fmt.Errorf("peer not found: %x", to.GetNode())
		}
		if handler, ok := peer.handlers[topic]; ok {
			go handler(message)
		} else {
			fmt.Println("handler not found for topic: ", topic, "node", to)
		}
	case *model.To_Nodes:
		for _, node := range p.nodes {
			peer := peers[node.String()]
			if peer == nil {
				continue
			}
			if handler, ok := peer.handlers[topic]; ok {
				go handler(message)
			} else {
				fmt.Println("handler not found for topic: ", topic, "node", node)
			}
		}
	case *model.To_Broadcast:
		for _, node := range p.nodes {
			peer := peers[node.String()]
			if peer == nil {
				continue
			}
			if handler, ok := peer.handlers[topic]; ok {
				go handler(message)
			} else {
				fmt.Println("handler not found for topic: ", topic, "node", node)
			}
		}
	}

	return nil
}

func (p *Peer) PeerID() string {
	return p.id
}

func (p *Peer) Pub(topic string, data []byte) error {
	panic("Pub not implement")
}

func (p *Peer) Sub(topic string, handler func(any) error) error {
	p.handlers[topic] = handler
	return nil
}

func (p *Peer) AvailableNodes() []*model.PubKey {
	return p.nodes
}

func (p *Peer) AllNodes() []*model.PubKey {
	return p.nodes
}

func (p *Peer) SetNodes(nodes []*model.PubKey) {
	p.nodes = nodes
}
