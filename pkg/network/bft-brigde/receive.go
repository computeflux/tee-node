package bftbrigde

import (
	"fmt"

	"github.com/cometbft/cometbft/p2p"
	"github.com/computeflux/tee-provider/pkg/model"
	"github.com/computeflux/tee-provider/pkg/model/protoio"
	"github.com/computeflux/tee-provider/pkg/util"
)

func (r *BTFReactor) Receive(e p2p.Envelope) {
	wrapper, ok := e.Message.(*bftMsgWrapper)
	if !ok {
		util.LogWithRed("P2P Receive", "unexpected message type", "type", fmt.Sprintf("%T", e.Message))
		return
	}
	data := wrapper.Bytes()

	switch e.ChannelID {
	case topics["dkg"].ID:
		msg := &model.DkgMessage{}
		if err := protoio.Unmarshal(data, msg); err != nil {
			util.LogWithRed("P2P Receive", "unmarshal dkg message", err)
			return
		}
		if msg.To == nil || !msg.To.Check(r.id) {
			return
		}
		if r.dkgHandler == nil {
			util.LogWithRed("P2P Receive", "dkgHandler not set")
			return
		}
		pub, err := r.GetPubkeyFromPeerID(e.Src.ID())
		if err != nil {
			util.LogWithYellow("P2P PubkeyFromPeerID", "Receive from unknown node", e.Src.ID(), "fallback to peer ID")
			msg.From = string(e.Src.ID())
		} else {
			msg.From = pub.String()
		}
		if err := r.dkgHandler(msg); err != nil {
			util.LogWithRed("P2P Receive error", "dkgHandler", err)
		}
	case topics["block-partial-sign"].ID:
		msg := &model.BlockPartialSign{}
		if err := protoio.Unmarshal(data, msg); err != nil {
			util.LogWithRed("P2P Receive", "unmarshal block partial sign message", err)
			return
		}
		if msg.To == nil || !msg.To.Check(r.id) {
			return
		}
		if r.blockPartialSignHandler == nil {
			util.LogWithRed("P2P Receive", "blockPartialSignHandler not set")
			return
		}
		pub, err := r.GetPubkeyFromPeerID(e.Src.ID())
		if err != nil {
			util.LogWithYellow("P2P PubkeyFromPeerID", "Receive from unknown node", e.Src.ID(), "fallback to peer ID")
			msg.From = string(e.Src.ID())
		} else {
			msg.From = pub.String()
		}
		if err := r.blockPartialSignHandler(msg); err != nil {
			util.LogWithRed("P2P Receive error", "blockPartialSignHandler", err)
		}
	case topics["secret"].ID:
		msg := &model.SecretBox{}
		if err := protoio.Unmarshal(data, msg); err != nil {
			util.LogWithRed("P2P Receive", "unmarshal secret box message", err)
			return
		}
		if msg.To == nil || !msg.To.Check(r.id) {
			return
		}
		if r.secretHandler == nil {
			util.LogWithRed("P2P Receive", "secretHandler not set")
			return
		}
		pub, err := r.GetPubkeyFromPeerID(e.Src.ID())
		if err != nil {
			util.LogWithYellow("P2P PubkeyFromPeerID", "Receive from unknown node", e.Src.ID(), "fallback to peer ID")
			msg.From = string(e.Src.ID())
		} else {
			msg.From = pub.String()
		}
		if err := r.secretHandler(msg); err != nil {
			util.LogWithRed("P2P Receive error", "secretHandler", err)
		}
	default:
		util.LogWithRed("P2P Receive", "unknown channel", e.ChannelID)
	}
}
