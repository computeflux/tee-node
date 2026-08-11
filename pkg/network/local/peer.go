package local

import (
	"crypto/ed25519"
	"encoding/hex"
	"errors"
	"fmt"
	"sync"

	"github.com/computeflux/tee-provider/pkg/model"
	"github.com/computeflux/tee-provider/pkg/types/crypto"
	"github.com/computeflux/tee-provider/pkg/util"
	"go.dedis.ch/kyber/v4"
)

var (
	peersMu sync.RWMutex
	peers   = make(map[string]*Peer)
)

func NewNetwork(priv *crypto.PrivKey, boots []string, nodes []*crypto.PubKey, tcp, udp uint32) (*Peer, error) {
	id := priv.GetPublicForSuite(crypto.SuiteEd25519).String()

	// 创建 P2P 网络实例
	peer := &Peer{
		id:       id,
		privKey:  priv.PrivateKey,
		nodes:    append([]*crypto.PubKey(nil), nodes...), // 防御性拷贝，避免外部切片修改绕过 nodesMu
		handlers: make(map[string]func(any) error),
		callBack: func(ty string) error {
			fmt.Println("::::::::::::::::::::::::::::::::::::::::::::::::::::::::::::: netHook not found error")
			return nil
		},
		version: 1,
	}

	peersMu.Lock()
	peers[id] = peer
	peersMu.Unlock()

	return peer, nil
}

type Peer struct {
	id         string
	privKey    ed25519.PrivateKey
	handlersMu sync.RWMutex
	handlers   map[string]func(any) error
	nodesMu    sync.RWMutex
	nodes      []*crypto.PubKey
	callBack   func(string) error
	version    uint32
	PreCommits []kyber.Point

	// wg 跟踪本节点上正在执行的消息处理 goroutine，
	// Wait 等待全部处理完成后再关闭底层资源（如 DB）。
	wg sync.WaitGroup

	// closed 置位后 dispatch 不再接受新消息，保证 Wait 返回后不会再有
	// wg.Add 调用（否则 WaitGroup 归零后 Add 会 panic）。
	closedMu sync.Mutex
	closed   bool
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
		peer := lookupPeer(hex.EncodeToString(to.GetNode()))
		if peer == nil {
			return fmt.Errorf("peer not found: %x", to.GetNode())
		}
		dispatch(peer, topic, message)
	case *model.To_Nodes, *model.To_Broadcast:
		for _, node := range p.nodesSnapshot() {
			peer := lookupPeer(node.String())
			if peer == nil {
				continue
			}
			dispatch(peer, topic, message)
		}
	}

	return nil
}

// lookupPeer 从全局 peer 注册表中查找节点。
func lookupPeer(id string) *Peer {
	peersMu.RLock()
	defer peersMu.RUnlock()
	return peers[id]
}

// nodesSnapshot 返回当前节点列表的副本，避免调用方并发修改。
func (p *Peer) nodesSnapshot() []*crypto.PubKey {
	p.nodesMu.RLock()
	defer p.nodesMu.RUnlock()
	return append([]*crypto.PubKey(nil), p.nodes...)
}

// dispatch 将消息异步投递给目标节点上注册的 handler，
// 并通过 wg 跟踪在途 goroutine，供 Wait 等待。
func dispatch(peer *Peer, topic string, message any) {
	peer.handlersMu.RLock()
	handler, ok := peer.handlers[topic]
	peer.handlersMu.RUnlock()
	if !ok {
		util.LogWithYellow("handler not found for topic", topic, "node", peer.id)
		return
	}

	// 与 Wait 互斥：closed 已置位则丢弃消息；否则先 Add 再解锁，
	// 保证 Wait 返回后不会再有 Add 导致 WaitGroup misuse panic。
	peer.closedMu.Lock()
	if peer.closed {
		peer.closedMu.Unlock()
		return
	}
	peer.wg.Add(1)
	peer.closedMu.Unlock()

	go func() {
		defer peer.wg.Done()
		_ = handler(message)
	}()
}

// Wait 阻塞直到本节点所有在途消息处理 goroutine 执行完毕。
// 应在停止服务后、关闭底层资源（如 DB）前调用。
// 调用后本节点不再接受新的消息投递。
func (p *Peer) Wait() {
	p.closedMu.Lock()
	p.closed = true
	p.closedMu.Unlock()
	p.wg.Wait()
}

func (p *Peer) PeerID() string {
	return p.id
}

func (p *Peer) Pub(topic string, data []byte) error {
	panic("Pub not implement")
}

func (p *Peer) Sub(topic string, handler func(any) error) error {
	p.handlersMu.Lock()
	p.handlers[topic] = handler
	p.handlersMu.Unlock()
	return nil
}

func (p *Peer) AvailableNodes() []*crypto.PubKey {
	return p.nodesSnapshot()
}

func (p *Peer) AllNodes() []*crypto.PubKey {
	return p.nodesSnapshot()
}

func (p *Peer) SetNodes(nodes []*crypto.PubKey) {
	p.nodesMu.Lock()
	p.nodes = append([]*crypto.PubKey(nil), nodes...)
	p.nodesMu.Unlock()
}
