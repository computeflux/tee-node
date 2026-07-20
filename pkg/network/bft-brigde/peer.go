package bftbrigde

import (
	"fmt"

	"github.com/computeflux/tee-provider/pkg/types/crypto"
)

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

// Get all available nodes
func (p *BTFReactor) AvailableNodes() []*crypto.PubKey {
	peers := p.Switch.Peers()
	nodes := make([]*crypto.PubKey, 0, peers.Size())
	for _, n := range p.nodekeys {
		for _, peer := range peers.Copy() {
			if peer.ID() == n.TeeChainNodeID() {
				nodes = append(nodes, n)
			}
		}
	}

	return nodes
}

// Get all nodes
func (p *BTFReactor) AllNodes() []*crypto.PubKey {
	return p.nodekeys
}
