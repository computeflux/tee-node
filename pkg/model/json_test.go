package model

import (
	"encoding/json"
	"testing"

	"github.com/computeflux/tee-provider/pkg/types/crypto"
	"github.com/stretchr/testify/require"
)

func jsonTestPubKey(t *testing.T) *crypto.PubKey {
	t.Helper()
	secret, err := crypto.PrivateKeyFromHex("7512939e37970b04c2b9a6060b16654473cf0721b71f8e56126ee314cbd0a7e9fe9125a81688d932ea792e9722c777f7696117363f86a107ab9d3681a8c922c8")
	require.NoError(t, err)
	return secret.GetPublicForSuite(crypto.SuiteEd25519)
}

// 回归测试：encoding/json 无法对 oneof 接口字段反序列化，
// json.go 中的 protojson 实现必须保证包含 oneof 的消息可以完整 round-trip。
func TestPeerMsgJSONRoundTrip(t *testing.T) {
	original := &PeerMsg{
		To: &To{Payload: &To_Broadcast{}},
		Payload: &PeerMsg_DkgMessage{DkgMessage: &DkgMessage{
			From:    "alice",
			MsgId:   "m1",
			Payload: []byte{1, 2, 3},
			Type:    "deal",
		}},
	}

	data, err := json.Marshal(original)
	require.NoError(t, err)

	restored := &PeerMsg{}
	require.NoError(t, json.Unmarshal(data, restored))

	require.NotNil(t, restored.To)
	require.IsType(t, &To_Broadcast{}, restored.To.Payload)

	require.NotNil(t, restored.GetDkgMessage())
	msg := restored.GetDkgMessage()
	require.Equal(t, "alice", msg.From)
	require.Equal(t, "m1", msg.MsgId)
	require.Equal(t, []byte{1, 2, 3}, msg.Payload)
	require.Equal(t, "deal", msg.Type)
}

func TestPeerMsgJSONRoundTrip_AllPayloads(t *testing.T) {
	payloads := []isPeerMsg_Payload{
		&PeerMsg_DkgMessage{DkgMessage: &DkgMessage{MsgId: "d"}},
		&PeerMsg_BlockPartialSign{BlockPartialSign: &BlockPartialSign{TxIndex: 42}},
		&PeerMsg_SecretBox{SecretBox: &SecretBox{From: "s"}},
	}

	for _, p := range payloads {
		original := &PeerMsg{To: SendToNode(jsonTestPubKey(t)), Payload: p}
		data, err := json.Marshal(original)
		require.NoError(t, err)

		restored := &PeerMsg{}
		require.NoError(t, json.Unmarshal(data, restored))
		require.IsType(t, p, restored.Payload)
	}
}

func TestDkgMessageJSONRoundTrip(t *testing.T) {
	original := &DkgMessage{
		From:    "node-a",
		To:      SendToNode(jsonTestPubKey(t)),
		MsgId:   "deal-1",
		Payload: []byte("raw"),
		Type:    "deal",
	}
	data, err := json.Marshal(original)
	require.NoError(t, err)

	restored := &DkgMessage{}
	require.NoError(t, json.Unmarshal(data, restored))
	require.Equal(t, original.From, restored.From)
	require.Equal(t, original.MsgId, restored.MsgId)
	require.Equal(t, original.Payload, restored.Payload)
	require.Equal(t, original.Type, restored.Type)
	require.NotNil(t, restored.To)
	require.IsType(t, &To_Node{}, restored.To.Payload)
	require.Equal(t, original.To.Payload.(*To_Node).Node, restored.To.Payload.(*To_Node).Node)
}

func TestToJSONRoundTrip(t *testing.T) {
	// Broadcast
	data, err := json.Marshal(&To{Payload: &To_Broadcast{}})
	require.NoError(t, err)
	restored := &To{}
	require.NoError(t, json.Unmarshal(data, restored))
	require.IsType(t, &To_Broadcast{}, restored.Payload)

	// Node
	nodeKey := jsonTestPubKey(t)
	data, err = json.Marshal(SendToNode(nodeKey))
	require.NoError(t, err)
	restored = &To{}
	require.NoError(t, json.Unmarshal(data, restored))
	require.IsType(t, &To_Node{}, restored.Payload)
	require.Equal(t, nodeKey.Byte(), restored.Payload.(*To_Node).Node)

	// Nodes
	data, err = json.Marshal(SendToNodes([]*crypto.PubKey{nodeKey}))
	require.NoError(t, err)
	restored = &To{}
	require.NoError(t, json.Unmarshal(data, restored))
	require.IsType(t, &To_Nodes{}, restored.Payload)
	require.Equal(t, [][]byte{nodeKey.Byte()}, restored.Payload.(*To_Nodes).Nodes.L)
}

// null 与 nil 的 JSON 表示
func TestJSONNilHandling(t *testing.T) {
	var to *To
	data, err := json.Marshal(to)
	require.NoError(t, err)
	require.Equal(t, "null", string(data))

	restored := &PeerMsg{}
	require.NoError(t, json.Unmarshal([]byte(`{"to":null,"dkgMessage":{"msgId":"x"}}`), restored))
	require.Nil(t, restored.To)
	require.NotNil(t, restored.GetDkgMessage())
	require.Equal(t, "x", restored.GetDkgMessage().MsgId)
}

// 回归（OCR）：nil receiver 上调用 UnmarshalJSON 必须报错，而不是静默成功掩盖 bug。
func TestJSONUnmarshal_NilReceiver_Error(t *testing.T) {
	var to *To
	require.ErrorContains(t, to.UnmarshalJSON([]byte(`{}`)), "nil receiver")

	var msg *DkgMessage
	require.ErrorContains(t, msg.UnmarshalJSON([]byte(`{}`)), "nil receiver")

	var ps *BlockPartialSign
	require.ErrorContains(t, ps.UnmarshalJSON([]byte(`{}`)), "nil receiver")

	var sb *SecretBox
	require.ErrorContains(t, sb.UnmarshalJSON([]byte(`{}`)), "nil receiver")

	var pm *PeerMsg
	require.ErrorContains(t, pm.UnmarshalJSON([]byte(`{}`)), "nil receiver")
}
