package bftbrigde

import (
	"sync"
	"testing"
	"time"

	"github.com/cometbft/cometbft/libs/log"
	"github.com/cometbft/cometbft/libs/service"
	"github.com/cometbft/cometbft/p2p"
	"github.com/computeflux/tee-provider/pkg/model"
	"github.com/computeflux/tee-provider/pkg/model/protoio"
	dbstore "github.com/computeflux/tee-provider/pkg/types"
	"github.com/computeflux/tee-provider/pkg/types/crypto"
	"github.com/stretchr/testify/require"
)

// ---------- p2p.Switcher 测试替身 ----------

type mockPeerSet struct{}

func (mockPeerSet) Has(key p2p.ID) bool     { return false }
func (mockPeerSet) Get(key p2p.ID) p2p.Peer { return nil }
func (mockPeerSet) Copy() []p2p.Peer        { return nil }
func (mockPeerSet) Size() int               { return 0 }
func (mockPeerSet) ForEach(func(p2p.Peer))  {}
func (mockPeerSet) Random() p2p.Peer        { return nil }

type mockSwitcher struct {
	service.BaseService
	id   p2p.ID
	mu   sync.Mutex
	envs []p2p.Envelope
}

func newMockSwitcher(id p2p.ID) *mockSwitcher {
	s := &mockSwitcher{id: id}
	s.BaseService = *service.NewBaseService(nil, "mock-switch", s)
	return s
}

func (s *mockSwitcher) NodeInfo() p2p.NodeInfo {
	return p2p.DefaultNodeInfo{DefaultNodeID: s.id}
}

func (s *mockSwitcher) Log() log.Logger { return log.NewNopLogger() }

// ReactorManager
func (s *mockSwitcher) Reactor(name string) (p2p.Reactor, bool)           { return nil, false }
func (s *mockSwitcher) AddReactor(name string, r p2p.Reactor) p2p.Reactor { return r }
func (s *mockSwitcher) RemoveReactor(name string, r p2p.Reactor)          {}

// PeerManager
func (s *mockSwitcher) Peers() p2p.IPeerSet                                  { return mockPeerSet{} }
func (s *mockSwitcher) NumPeers() (int, int, int)                            { return 0, 0, 0 }
func (s *mockSwitcher) MaxNumOutboundPeers() int                             { return 0 }
func (s *mockSwitcher) AddPersistentPeers(addrs []string) error              { return nil }
func (s *mockSwitcher) AddPrivatePeerIDs(ids []string) error                 { return nil }
func (s *mockSwitcher) AddUnconditionalPeerIDs(ids []string) error           { return nil }
func (s *mockSwitcher) DialPeerWithAddress(addr *p2p.NetAddress) error       { return nil }
func (s *mockSwitcher) DialPeersAsync(peers []string) error                  { return nil }
func (s *mockSwitcher) StopPeerForError(peer p2p.Peer, reason any)           {}
func (s *mockSwitcher) StopPeerGracefully(peer p2p.Peer)                     {}
func (s *mockSwitcher) IsDialingOrExistingAddress(addr *p2p.NetAddress) bool { return false }
func (s *mockSwitcher) IsPeerPersistent(addr *p2p.NetAddress) bool           { return false }
func (s *mockSwitcher) IsPeerUnconditional(id p2p.ID) bool                   { return false }
func (s *mockSwitcher) MarkPeerAsGood(peer p2p.Peer)                         {}

// Broadcaster
func (s *mockSwitcher) BroadcastAsync(e p2p.Envelope) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.envs = append(s.envs, e)
}
func (s *mockSwitcher) TryBroadcast(e p2p.Envelope) { s.BroadcastAsync(e) }

func (s *mockSwitcher) envCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.envs)
}

var _ p2p.Switcher = (*mockSwitcher)(nil)

// ---------- 测试工具 ----------

const (
	testSecretA = "7512939e37970b04c2b9a6060b16654473cf0721b71f8e56126ee314cbd0a7e9fe9125a81688d932ea792e9722c777f7696117363f86a107ab9d3681a8c922c8"
	testSecretB = "dc72ceaf44e1e382a2ee4bbf47d6eabcca460740d5c28dd7bc097db90700594636922ff5c13eded54d1ba710bf043eed221112eb726da532149e1e3619fb9336"
)

func testPubkey(t *testing.T, secretHex string) *crypto.PubKey {
	t.Helper()
	secret, err := crypto.PrivateKeyFromHex(secretHex)
	require.NoError(t, err)
	return secret.GetPublicForSuite(crypto.SuiteEd25519)
}

// newTestReactor 创建带临时 DB 的 reactor（SECRET_DB 全局指向临时库）。
func newTestReactor(t *testing.T) (*BTFReactor, *crypto.PubKey) {
	t.Helper()
	db, err := dbstore.NewDBWithPath(t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() { db.Close() })

	pub := testPubkey(t, testSecretA)
	r := NewBTFReactor("test-reactor")
	r.SetID(pub)
	return r, pub
}

func marshalVT(t *testing.T, msg any) []byte {
	t.Helper()
	data, err := protoio.MarshalVT(msg)
	require.NoError(t, err)
	return data
}

func envelope(t *testing.T, channel byte, data []byte, src p2p.ID) p2p.Envelope {
	t.Helper()
	return p2p.Envelope{
		Src:       LocalPeer{id: src},
		Message:   &bftMsgWrapper{data: data},
		ChannelID: channel,
	}
}

// ---------- Sub ----------

func TestSub(t *testing.T) {
	r, _ := newTestReactor(t)

	for _, topic := range []string{"dkg", "block-partial-sign", "secret"} {
		require.NoError(t, r.Sub(topic, func(any) error { return nil }))
	}
	require.NotNil(t, r.dkgHandler)
	require.NotNil(t, r.blockPartialSignHandler)
	require.NotNil(t, r.secretHandler)

	require.Error(t, r.Sub("unknown", func(any) error { return nil }))
}

// ---------- Receive ----------

func TestReceive_DkgMessage(t *testing.T) {
	r, pub := newTestReactor(t)

	got := make(chan *model.DkgMessage, 1)
	require.NoError(t, r.Sub("dkg", func(v any) error {
		got <- v.(*model.DkgMessage)
		return nil
	}))

	// 目标正确 → handler 被调用；nodekeys 为空时 From 回退为 peer ID
	r.Receive(envelope(t, topics["dkg"].ID, marshalVT(t, &model.DkgMessage{
		To: model.SendToNode(pub), MsgId: "m1", Type: "deal",
	}), "peer-1"))
	select {
	case m := <-got:
		require.Equal(t, "m1", m.MsgId)
		require.Equal(t, "peer-1", m.From)
	case <-time.After(2 * time.Second):
		t.Fatal("dkgHandler not called")
	}

	// 目标不是本节点 → 丢弃
	r.Receive(envelope(t, topics["dkg"].ID, marshalVT(t, &model.DkgMessage{
		To: model.SendToNode(testPubkey(t, testSecretB)), MsgId: "m2",
	}), "peer-2"))
	select {
	case m := <-got:
		t.Fatalf("unexpected handler call: %+v", m)
	case <-time.After(200 * time.Millisecond):
	}

	// To 为 nil → 不 panic、不调用 handler
	r.Receive(envelope(t, topics["dkg"].ID, marshalVT(t, &model.DkgMessage{
		MsgId: "m3",
	}), "peer-3"))
	select {
	case m := <-got:
		t.Fatalf("unexpected handler call for nil To: %+v", m)
	case <-time.After(200 * time.Millisecond):
	}
}

func TestReceive_DkgMessage_FromKnownPeer(t *testing.T) {
	r, pub := newTestReactor(t)
	other := testPubkey(t, testSecretB)
	r.nodekeys = []*crypto.PubKey{other}

	got := make(chan *model.DkgMessage, 1)
	require.NoError(t, r.Sub("dkg", func(v any) error {
		got <- v.(*model.DkgMessage)
		return nil
	}))

	r.Receive(envelope(t, topics["dkg"].ID, marshalVT(t, &model.DkgMessage{
		To: model.SendToNode(pub), MsgId: "m1",
	}), other.TeeChainNodeID()))
	select {
	case m := <-got:
		require.Equal(t, other.String(), m.From)
	case <-time.After(2 * time.Second):
		t.Fatal("dkgHandler not called")
	}
}

func TestReceive_BlockPartialSign(t *testing.T) {
	r, pub := newTestReactor(t)

	got := make(chan *model.BlockPartialSign, 1)
	require.NoError(t, r.Sub("block-partial-sign", func(v any) error {
		got <- v.(*model.BlockPartialSign)
		return nil
	}))

	r.Receive(envelope(t, topics["block-partial-sign"].ID, marshalVT(t, &model.BlockPartialSign{
		To: model.SendToNode(pub), TxIndex: 7,
	}), "peer-1"))
	select {
	case m := <-got:
		require.Equal(t, int64(7), m.TxIndex)
	case <-time.After(2 * time.Second):
		t.Fatal("blockPartialSignHandler not called")
	}

	// 目标不是本节点 → 丢弃
	r.Receive(envelope(t, topics["block-partial-sign"].ID, marshalVT(t, &model.BlockPartialSign{
		To: model.SendToNode(testPubkey(t, testSecretB)), TxIndex: 8,
	}), "peer-2"))
	select {
	case m := <-got:
		t.Fatalf("unexpected handler call: %+v", m)
	case <-time.After(200 * time.Millisecond):
	}
}

func TestReceive_SecretBox(t *testing.T) {
	r, pub := newTestReactor(t)

	got := make(chan *model.SecretBox, 1)
	require.NoError(t, r.Sub("secret", func(v any) error {
		got <- v.(*model.SecretBox)
		return nil
	}))

	r.Receive(envelope(t, topics["secret"].ID, marshalVT(t, &model.SecretBox{
		To: model.SendToNode(pub),
	}), "peer-1"))
	select {
	case m := <-got:
		require.NotNil(t, m)
	case <-time.After(2 * time.Second):
		t.Fatal("secretHandler not called")
	}
}

// secretHandler 未订阅时收到 secret 消息：不 panic（回归测试）
func TestReceive_SecretBox_NoHandlerNoPanic(t *testing.T) {
	r, pub := newTestReactor(t)

	r.Receive(envelope(t, topics["secret"].ID, marshalVT(t, &model.SecretBox{
		To: model.SendToNode(pub),
	}), "peer-1"))
}

// 未知 channel / 无法解析的数据：不 panic
func TestReceive_GarbageNoPanic(t *testing.T) {
	r, _ := newTestReactor(t)

	r.Receive(envelope(t, 200, []byte("not a proto"), "peer-1"))
	r.Receive(envelope(t, topics["dkg"].ID, []byte("not a proto"), "peer-1"))
	r.Receive(p2p.Envelope{Src: LocalPeer{id: "peer-1"}, ChannelID: topics["dkg"].ID})
}

// ---------- Send / doSend ----------

func TestSend_PersistsWhenSwitchNil(t *testing.T) {
	r, _ := newTestReactor(t)

	msg := &model.PeerMsg{
		To: &model.To{Payload: &model.To_Broadcast{}},
		Payload: &model.PeerMsg_DkgMessage{DkgMessage: &model.DkgMessage{
			MsgId: "p1",
		}},
	}
	require.NoError(t, r.Send(msg))

	// DB 中已持久化（JSON round-trip 需能正确恢复 oneof 字段）
	store, err := r.loadPendingMsgsStore()
	require.NoError(t, err)
	require.Len(t, store.Msgs, 1)
	require.Equal(t, "p1", store.Msgs[0].GetDkgMessage().MsgId)
	require.NotNil(t, store.Msgs[0].To)

	// 内存队列同步（防止 flush 时 DB 消息被清掉而丢失）
	r.pendingMu.Lock()
	require.Len(t, r.pendingMsgs, 1)
	r.pendingMu.Unlock()
}

// 模拟重启：新 reactor 应从 DB 恢复待发消息（回归测试：原实现 oneof 无法 JSON 反序列化导致消息丢失）
func TestLoadPendingMsgs_RecoverFromDB(t *testing.T) {
	db, err := dbstore.NewDBWithPath(t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() { db.Close() })

	pub := testPubkey(t, testSecretA)

	r1 := NewBTFReactor("first")
	r1.SetID(pub)
	require.NoError(t, r1.Send(&model.PeerMsg{
		To: &model.To{Payload: &model.To_Broadcast{}},
		Payload: &model.PeerMsg_BlockPartialSign{BlockPartialSign: &model.BlockPartialSign{
			TxIndex: 42,
		}},
	}))

	r2 := NewBTFReactor("second")
	r2.SetID(pub)
	r2.pendingMu.Lock()
	defer r2.pendingMu.Unlock()
	require.Len(t, r2.pendingMsgs, 1)
	require.Equal(t, int64(42), r2.pendingMsgs[0].GetBlockPartialSign().TxIndex)
}

func TestSetSwitch_FlushesPending(t *testing.T) {
	r, pub := newTestReactor(t)

	require.NoError(t, r.Send(&model.PeerMsg{
		To: &model.To{Payload: &model.To_Broadcast{}},
		Payload: &model.PeerMsg_DkgMessage{DkgMessage: &model.DkgMessage{
			MsgId: "f1",
		}},
	}))

	sw := newMockSwitcher(pub.TeeChainNodeID())
	r.SetSwitch(sw)

	require.Equal(t, 1, sw.envCount())

	// DB 与内存队列均已清空
	store, err := r.loadPendingMsgsStore()
	require.NoError(t, err)
	require.Len(t, store.Msgs, 0)
	r.pendingMu.Lock()
	require.Len(t, r.pendingMsgs, 0)
	r.pendingMu.Unlock()
}

func TestDoSend(t *testing.T) {
	r, pub := newTestReactor(t)
	sw := newMockSwitcher(pub.TeeChainNodeID())
	r.SetSwitch(sw)
	require.Equal(t, 0, sw.envCount())

	// 发给其他节点：广播发生（接收端按 To 过滤），本地不回调
	require.NoError(t, r.doSend(&model.PeerMsg{
		To:      model.SendToNode(testPubkey(t, testSecretB)),
		Payload: &model.PeerMsg_DkgMessage{DkgMessage: &model.DkgMessage{MsgId: "d1"}},
	}))
	require.Equal(t, 1, sw.envCount())

	// To 为 nil：不 panic（回归测试）
	require.NoError(t, r.doSend(&model.PeerMsg{
		Payload: &model.PeerMsg_SecretBox{SecretBox: &model.SecretBox{}},
	}))
	require.Equal(t, 2, sw.envCount())

	// 未知 payload 类型 → 报错且不广播
	require.Error(t, r.doSend(&model.PeerMsg{To: &model.To{Payload: &model.To_Broadcast{}}}))
	require.Equal(t, 2, sw.envCount())
}

// 发给自己的消息：doSend 直接走本地 Receive 回调
func TestDoSend_LocalDelivery(t *testing.T) {
	r, pub := newTestReactor(t)
	sw := newMockSwitcher(pub.TeeChainNodeID())
	r.SetSwitch(sw)

	got := make(chan string, 1)
	require.NoError(t, r.Sub("dkg", func(v any) error {
		got <- v.(*model.DkgMessage).MsgId
		return nil
	}))

	require.NoError(t, r.doSend(&model.PeerMsg{
		To:      model.SendToNode(pub),
		Payload: &model.PeerMsg_DkgMessage{DkgMessage: &model.DkgMessage{MsgId: "self"}},
	}))

	select {
	case id := <-got:
		require.Equal(t, "self", id)
	case <-time.After(2 * time.Second):
		t.Fatal("local handler not called")
	}
}

// ---------- 节点查询 ----------

func TestGetPubkeyFromPeerID(t *testing.T) {
	r, _ := newTestReactor(t)
	other := testPubkey(t, testSecretB)
	r.nodekeys = []*crypto.PubKey{other}

	got, err := r.GetPubkeyFromPeerID(other.TeeChainNodeID())
	require.NoError(t, err)
	require.Equal(t, other.String(), got.String())

	_, err = r.GetPubkeyFromPeerID("unknown-id")
	require.Error(t, err)
}

func TestAvailableNodes_EmptyPeers(t *testing.T) {
	r, pub := newTestReactor(t)
	r.SetSwitch(newMockSwitcher(pub.TeeChainNodeID()))
	r.nodekeys = []*crypto.PubKey{testPubkey(t, testSecretB)}

	require.Len(t, r.AvailableNodes(), 0)
	require.Len(t, r.AllNodes(), 1)
}

// ---------- 生命周期与通道 ----------

func TestOnStart_NoPanic(t *testing.T) {
	r, pub := newTestReactor(t)
	r.SetSwitch(newMockSwitcher(pub.TeeChainNodeID()))
	require.NoError(t, r.OnStart())
}

func TestGetChannels(t *testing.T) {
	r, _ := newTestReactor(t)
	chs := r.GetChannels()
	require.Len(t, chs, len(topics))
	ids := make(map[byte]bool, len(chs))
	for _, c := range chs {
		ids[c.ID] = true
	}
	for _, topic := range topics {
		require.True(t, ids[topic.ID], "channel %d missing", topic.ID)
	}
}

func TestPendingMsgKey(t *testing.T) {
	r, pub := newTestReactor(t)
	require.Equal(t, "bft_reactor_pending_msgs_"+pub.String(), r.pendingMsgKey())

	other := &BTFReactor{}
	require.Equal(t, "bft_reactor_pending_msgs", other.pendingMsgKey())
}
