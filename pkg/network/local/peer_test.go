package local

import (
	"crypto/rand"
	"encoding/hex"
	"testing"
	"time"

	"github.com/computeflux/tee-provider/pkg/model"
	"github.com/computeflux/tee-provider/pkg/types/crypto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newTestPeer 创建一个注册到全局 peer 表的本地节点。
func newTestPeer(t *testing.T) (*Peer, *crypto.PubKey) {
	t.Helper()
	priv, pub, err := crypto.GenerateEd25519KeyPair(rand.Reader)
	require.NoError(t, err)
	peer, err := NewNetwork(priv, nil, nil, 0, 0)
	require.NoError(t, err)
	t.Cleanup(peer.Wait)
	return peer, pub
}

func TestSend_ToNode(t *testing.T) {
	peerA, _ := newTestPeer(t)
	peerB, pubB := newTestPeer(t)

	received := make(chan *model.DkgMessage, 1)
	require.NoError(t, peerB.Sub("dkg", func(m any) error {
		received <- m.(*model.DkgMessage)
		return nil
	}))

	msg := &model.DkgMessage{From: peerA.PeerID(), Payload: []byte("hello")}
	err := peerA.Send(&model.PeerMsg{
		To:      &model.To{Payload: &model.To_Node{Node: pubB.PublicKey}},
		Payload: &model.PeerMsg_DkgMessage{DkgMessage: msg},
	})
	require.NoError(t, err)
	peerB.Wait()

	select {
	case got := <-received:
		assert.Equal(t, []byte("hello"), got.Payload)
	case <-time.After(2 * time.Second):
		t.Fatal("handler was not invoked")
	}
}

func TestSend_ToNodes_Broadcast(t *testing.T) {
	peerA, _ := newTestPeer(t)
	peerB, pubB := newTestPeer(t)
	peerC, pubC := newTestPeer(t)

	// A 的节点列表包含 B、C
	peerA.SetNodes([]*crypto.PubKey{pubB, pubC})

	var gotB, gotC atomicBool
	require.NoError(t, peerB.Sub("secret-box", func(any) error { gotB.set(); return nil }))
	require.NoError(t, peerC.Sub("secret-box", func(any) error { gotC.set(); return nil }))

	err := peerA.Send(&model.PeerMsg{
		To:      &model.To{Payload: &model.To_Nodes{}},
		Payload: &model.PeerMsg_SecretBox{SecretBox: &model.SecretBox{}},
	})
	require.NoError(t, err)
	peerB.Wait()
	peerC.Wait()

	assert.True(t, gotB.get(), "peer B should receive broadcast")
	assert.True(t, gotC.get(), "peer C should receive broadcast")
}

func TestSend_Errors(t *testing.T) {
	peerA, _ := newTestPeer(t)

	t.Run("nil_message", func(t *testing.T) {
		require.Error(t, peerA.Send(nil))
	})

	t.Run("nil_to", func(t *testing.T) {
		err := peerA.Send(&model.PeerMsg{Payload: &model.PeerMsg_DkgMessage{}})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "to")
	})

	t.Run("unknown_payload", func(t *testing.T) {
		err := peerA.Send(&model.PeerMsg{To: &model.To{Payload: &model.To_Node{Node: []byte{1}}}})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "unknown peer payload")
	})

	t.Run("unknown_peer", func(t *testing.T) {
		err := peerA.Send(&model.PeerMsg{
			To:      &model.To{Payload: &model.To_Node{Node: []byte{0xde, 0xad}}},
			Payload: &model.PeerMsg_DkgMessage{},
		})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "peer not found")
	})
}

func TestNodes(t *testing.T) {
	peerA, pubA := newTestPeer(t)
	_, pubB := newTestPeer(t)

	peerA.SetNodes([]*crypto.PubKey{pubB})

	assert.Equal(t, []*crypto.PubKey{pubB}, peerA.AvailableNodes())
	assert.Equal(t, []*crypto.PubKey{pubB}, peerA.AllNodes())
	assert.Equal(t, hex.EncodeToString(pubA.PublicKey), peerA.PeerID())

	// SetNodes 必须拷贝，避免外部修改影响内部状态
	peerA.SetNodes([]*crypto.PubKey{pubA})
	got := peerA.AvailableNodes()
	got[0] = nil
	assert.NotNil(t, peerA.AvailableNodes()[0], "SetNodes must copy the slice")
}

// ── OCR 修复回归测试 ──────────────────────────────────────

func TestNewNetwork_CopiesNodes(t *testing.T) {
	// 回归：NewNetwork 必须拷贝 nodes 切片，否则调用方修改原切片
	// 会绕过 nodesMu，造成数据竞争。
	priv, pub, err := crypto.GenerateEd25519KeyPair(rand.Reader)
	require.NoError(t, err)

	nodes := []*crypto.PubKey{pub}
	peer, err := NewNetwork(priv, nil, nodes, 0, 0)
	require.NoError(t, err)
	t.Cleanup(peer.Wait)

	nodes[0] = nil // 模拟调用方修改原切片
	require.NotNil(t, peer.AllNodes()[0], "NewNetwork must copy the nodes slice")
}

func TestDispatch_AfterWait_DropsMessage(t *testing.T) {
	// 回归：Wait 返回后 dispatch 不能再 wg.Add(1)（否则 WaitGroup 归零后
	// Add 会 panic），且 Wait 后的消息应被丢弃。
	peerA, _ := newTestPeer(t)
	peerB, pubB := newTestPeer(t)

	delivered := make(chan struct{}, 1)
	require.NoError(t, peerB.Sub("dkg", func(any) error {
		delivered <- struct{}{}
		return nil
	}))

	peerB.Wait()

	require.NotPanics(t, func() {
		_ = peerA.Send(&model.PeerMsg{
			To:      &model.To{Payload: &model.To_Node{Node: pubB.PublicKey}},
			Payload: &model.PeerMsg_DkgMessage{DkgMessage: &model.DkgMessage{Payload: []byte("late")}},
		})
	})

	select {
	case <-delivered:
		t.Fatal("message must not be delivered after Wait")
	case <-time.After(100 * time.Millisecond):
	}
}

func TestDispatch_BeforeWait_Delivers(t *testing.T) {
	// 对照：Wait 之前的消息仍要正常送达。
	peerA, _ := newTestPeer(t)
	peerB, pubB := newTestPeer(t)

	delivered := make(chan struct{}, 1)
	require.NoError(t, peerB.Sub("dkg", func(any) error {
		delivered <- struct{}{}
		return nil
	}))

	require.NoError(t, peerA.Send(&model.PeerMsg{
		To:      &model.To{Payload: &model.To_Node{Node: pubB.PublicKey}},
		Payload: &model.PeerMsg_DkgMessage{DkgMessage: &model.DkgMessage{Payload: []byte("ok")}},
	}))

	peerB.Wait()

	select {
	case <-delivered:
	default:
		t.Fatal("message should be delivered before Wait")
	}
}

// ── 小工具 ────────────────────────────────────────────────

type atomicBool struct {
	c chan struct{}
}

func (a *atomicBool) init() {
	if a.c == nil {
		a.c = make(chan struct{}, 1)
	}
}

func (a *atomicBool) set() {
	a.init()
	select {
	case a.c <- struct{}{}:
	default:
	}
}

func (a *atomicBool) get() bool {
	a.init()
	select {
	case <-a.c:
		return true
	default:
		return false
	}
}

func hexString(b []byte) string {
	return hex.EncodeToString(b)
}
