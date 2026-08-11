package model

import (
	"bytes"
	"crypto/ed25519"
	"encoding/json"
	"testing"

	"github.com/computeflux/tee-provider/pkg/types/crypto"
	"github.com/computeflux/tee-provider/tee-chain/runtime/store"
	"github.com/stretchr/testify/require"
)

func newTestPubKey(t *testing.T, seed byte) *crypto.PubKey {
	t.Helper()
	pk := make(ed25519.PublicKey, ed25519.PublicKeySize)
	for i := range pk {
		pk[i] = seed + byte(i)
	}
	return &crypto.PubKey{PublicKey: pk}
}

// ── SendToNodes / SendToNode / To.Check ────────────────────

func TestSendToNodes(t *testing.T) {
	id1 := newTestPubKey(t, 0x11)
	id2 := newTestPubKey(t, 0x22)

	to := SendToNodes([]*crypto.PubKey{id1, id2})
	nodes, ok := to.Payload.(*To_Nodes)
	require.True(t, ok)
	require.NotNil(t, nodes.Nodes)
	require.Len(t, nodes.Nodes.L, 2)
	require.True(t, bytes.Equal(nodes.Nodes.L[0], id1.Byte()))
	require.True(t, bytes.Equal(nodes.Nodes.L[1], id2.Byte()))
}

func TestSendToNodes_Empty(t *testing.T) {
	to := SendToNodes(nil)
	nodes, ok := to.Payload.(*To_Nodes)
	require.True(t, ok)
	require.Len(t, nodes.Nodes.L, 0)
}

func TestSendToNode(t *testing.T) {
	id := newTestPubKey(t, 0x33)
	to := SendToNode(id)
	node, ok := to.Payload.(*To_Node)
	require.True(t, ok)
	require.True(t, bytes.Equal(node.Node, id.Byte()))
}

func TestTo_Check(t *testing.T) {
	id1 := newTestPubKey(t, 0x11)
	id2 := newTestPubKey(t, 0x22)
	id3 := newTestPubKey(t, 0x33)

	t.Run("broadcast_always_true", func(t *testing.T) {
		to := &To{Payload: &To_Broadcast{}}
		require.True(t, to.Check(id1))
		require.True(t, to.Check(nil))
	})

	t.Run("nodes_contains_id", func(t *testing.T) {
		to := SendToNodes([]*crypto.PubKey{id1, id2})
		require.True(t, to.Check(id1))
		require.True(t, to.Check(id2))
		require.False(t, to.Check(id3))
	})

	t.Run("nodes_nil_id", func(t *testing.T) {
		to := SendToNodes([]*crypto.PubKey{id1})
		require.False(t, to.Check(nil))
	})

	t.Run("nodes_nil_nodes", func(t *testing.T) {
		to := &To{Payload: &To_Nodes{Nodes: nil}}
		require.False(t, to.Check(id1))
	})

	t.Run("node_match", func(t *testing.T) {
		to := SendToNode(id1)
		require.True(t, to.Check(id1))
		require.False(t, to.Check(id2))
	})

	t.Run("node_nil_id", func(t *testing.T) {
		to := SendToNode(id1)
		require.False(t, to.Check(nil))
	})

	t.Run("nil_payload", func(t *testing.T) {
		to := &To{}
		require.False(t, to.Check(id1))
	})
}

// ── JSON 编解码（oneof 字段必须能往返）──────────────────────

func TestTo_MarshalJSON_RoundTrip(t *testing.T) {
	id1 := newTestPubKey(t, 0x11)
	id2 := newTestPubKey(t, 0x22)

	// 注释声称 oneof 字段能被 protojson 正确处理；这里是回归验证
	to := SendToNodes([]*crypto.PubKey{id1, id2})
	raw, err := json.Marshal(to)
	require.NoError(t, err)

	var out To
	require.NoError(t, json.Unmarshal(raw, &out))
	nodes, ok := out.Payload.(*To_Nodes)
	require.True(t, ok, "oneof field must survive JSON round trip")
	require.Len(t, nodes.Nodes.L, 2)
	require.True(t, bytes.Equal(nodes.Nodes.L[0], id1.Byte()))
}

func TestTo_MarshalJSON_Nil(t *testing.T) {
	var to *To
	raw, err := json.Marshal(to)
	require.NoError(t, err)
	require.Equal(t, "null", string(raw))
}

func TestTo_MarshalJSON_NodeAndBroadcast(t *testing.T) {
	id := newTestPubKey(t, 0x44)
	to := SendToNode(id)
	raw, err := json.Marshal(to)
	require.NoError(t, err)
	var out To
	require.NoError(t, json.Unmarshal(raw, &out))
	_, ok := out.Payload.(*To_Node)
	require.True(t, ok)

	bc := &To{Payload: &To_Broadcast{}}
	raw, err = json.Marshal(bc)
	require.NoError(t, err)
	var out2 To
	require.NoError(t, json.Unmarshal(raw, &out2))
	_, ok = out2.Payload.(*To_Broadcast)
	require.True(t, ok)
}

func TestDkgMessage_MarshalJSON_RoundTrip(t *testing.T) {
	msg := &DkgMessage{
		From:    "node-1",
		MsgId:   "msg-1",
		Payload: []byte{0xde, 0xad},
		Type:    "dkg",
	}
	raw, err := json.Marshal(msg)
	require.NoError(t, err)

	var out DkgMessage
	require.NoError(t, json.Unmarshal(raw, &out))
	require.Equal(t, msg.From, out.From)
	require.Equal(t, msg.MsgId, out.MsgId)
	require.Equal(t, msg.Type, out.Type)
	require.True(t, bytes.Equal(msg.Payload, out.Payload))
}

func TestBlockPartialSign_MarshalJSON_RoundTrip(t *testing.T) {
	sig := &BlockPartialSign{
		From:    "node-1",
		TxIndex: 11,
		HubSig:  []byte{0x02},
		ChainId: 2,
	}
	raw, err := json.Marshal(sig)
	require.NoError(t, err)

	var out BlockPartialSign
	require.NoError(t, json.Unmarshal(raw, &out))
	require.Equal(t, sig.From, out.From)
	require.Equal(t, sig.TxIndex, out.TxIndex)
	require.Equal(t, sig.ChainId, out.ChainId)
	require.True(t, bytes.Equal(sig.HubSig, out.HubSig))
}

func TestSecretBox_MarshalJSON_RoundTrip(t *testing.T) {
	box := &SecretBox{
		From: "n1",
		Payload: &SecretBox_Req{
			Req: &PodStart{Id: 7, PubKey: []byte("pk")},
		},
	}
	raw, err := json.Marshal(box)
	require.NoError(t, err)

	var out SecretBox
	require.NoError(t, json.Unmarshal(raw, &out))
	req, ok := out.Payload.(*SecretBox_Req)
	require.True(t, ok, "oneof payload must survive JSON round trip")
	require.Equal(t, uint64(7), req.Req.Id)
	require.True(t, bytes.Equal(req.Req.PubKey, []byte("pk")))
}

func TestPeerMsg_MarshalJSON_RoundTrip(t *testing.T) {
	// PeerMsg 的 Payload 是 oneof——这是 json.go 注释中说的
	// “encoding/json 无法处理 oneof” 的核心场景，必须能往返
	pm := &PeerMsg{
		To: SendToNode(newTestPubKey(t, 0xaa)),
		Payload: &PeerMsg_DkgMessage{
			DkgMessage: &DkgMessage{From: "n1", Type: "dkg"},
		},
	}
	raw, err := json.Marshal(pm)
	require.NoError(t, err)

	var out PeerMsg
	require.NoError(t, json.Unmarshal(raw, &out))
	dm, ok := out.Payload.(*PeerMsg_DkgMessage)
	require.True(t, ok, "oneof payload must survive JSON round trip")
	require.Equal(t, "n1", dm.DkgMessage.From)
	require.NotNil(t, out.To)
}

func TestPeerMsg_MarshalJSON_BlockPartialSign(t *testing.T) {
	pm := &PeerMsg{
		Payload: &PeerMsg_BlockPartialSign{
			BlockPartialSign: &BlockPartialSign{From: "n2", ChainId: 3},
		},
	}
	raw, err := json.Marshal(pm)
	require.NoError(t, err)

	var out PeerMsg
	require.NoError(t, json.Unmarshal(raw, &out))
	b, ok := out.Payload.(*PeerMsg_BlockPartialSign)
	require.True(t, ok)
	require.Equal(t, uint32(3), b.BlockPartialSign.ChainId)
}

func TestPeerMsg_MarshalJSON_SecretBox(t *testing.T) {
	pm := &PeerMsg{
		Payload: &PeerMsg_SecretBox{
			SecretBox: &SecretBox{From: "n1"},
		},
	}
	raw, err := json.Marshal(pm)
	require.NoError(t, err)

	var out PeerMsg
	require.NoError(t, json.Unmarshal(raw, &out))
	sb, ok := out.Payload.(*PeerMsg_SecretBox)
	require.True(t, ok)
	require.Equal(t, "n1", sb.SecretBox.From)
}

func TestJSONMarshal_NilMessages(t *testing.T) {
	var nilMsg *DkgMessage
	raw, err := json.Marshal(nilMsg)
	require.NoError(t, err)
	require.Equal(t, "null", string(raw))

	var nilSig *BlockPartialSign
	raw, err = json.Marshal(nilSig)
	require.NoError(t, err)
	require.Equal(t, "null", string(raw))
}

// ── ContractCall.ToStore ───────────────────────────────────

func TestContractCall_ToStore(t *testing.T) {
	call := &ContractCall{
		Name:   []byte("gov"),
		Method: []byte{0xa4, 0x77, 0x1d, 0x9e},
		Args:   [][]byte{{0x01}, {0x02, 0x03}},
		Nonce:  42,
	}

	s := call.ToStore()
	sc, ok := s.(*store.InContractCall)
	require.True(t, ok)
	require.True(t, bytes.Equal(sc.Name, call.Name))
	require.True(t, bytes.Equal(sc.Method, call.Method))
	require.Len(t, sc.Args, 2)
	require.True(t, bytes.Equal(sc.Args[1], []byte{0x02, 0x03}))

	// 空调用不 panic
	empty := (&ContractCall{}).ToStore()
	require.NotNil(t, empty)
}
