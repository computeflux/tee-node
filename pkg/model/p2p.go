package model

import (
	"bytes"
)

// p2p send msg to all node
func SendBroadcast() *To {
	return &To{
		Payload: &To_Broadcast{
			Broadcast: true,
		},
	}
}

// p2p send msg to modes
func SendToNodes(ids []*PubKey) *To {
	keys := make([][]byte, len(ids))
	for i, id := range ids {
		keys[i] = id.Byte()
	}
	return &To{
		Payload: &To_Nodes{
			Nodes: &Nodes{
				L: keys,
			},
		},
	}
}

// p2p send msg to node
func SendToNode(id *PubKey) *To {
	return &To{
		Payload: &To_Node{
			Node: id.Byte(),
		},
	}
}

func (s *To) Check(id *PubKey) bool {
	switch to := s.Payload.(type) {
	case *To_Broadcast:
		return true
	case *To_Nodes:
		if id == nil {
			return false
		}
		for _, v := range to.Nodes.L {
			if bytes.Equal(v, id.Byte()) {
				return true
			}
		}
		return false
	case *To_Node:
		if id == nil {
			return false
		}
		if bytes.Equal(to.Node, id.Byte()) {
			return true
		}
		return false
	}
	return false
}
