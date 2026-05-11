package bftbrigde

import (
	"net"

	"github.com/cometbft/cometbft/libs/log"
	"github.com/cometbft/cometbft/p2p"
	cmtconn "github.com/cometbft/cometbft/p2p/conn"
)

type LocalPeer struct {
	id p2p.ID
}

func (LocalPeer) FlushStop() {}
func (l LocalPeer) ID() p2p.ID {
	return l.id
}
func (LocalPeer) RemoteIP() net.IP                 { panic("local peer not implemented") }
func (LocalPeer) RemoteAddr() net.Addr             { panic("local peer not implemented") }
func (LocalPeer) IsOutbound() bool                 { panic("local peer not implemented") }
func (LocalPeer) IsPersistent() bool               { panic("local peer not implemented") }
func (LocalPeer) CloseConn() error                 { panic("local peer not implemented") }
func (LocalPeer) NodeInfo() p2p.NodeInfo           { panic("local peer not implemented") }
func (LocalPeer) Status() cmtconn.ConnectionStatus { panic("local peer not implemented") }
func (LocalPeer) SocketAddr() *p2p.NetAddress      { panic("local peer not implemented") }
func (LocalPeer) Send(e p2p.Envelope) bool         { panic("local peer not implemented") }
func (LocalPeer) TrySend(e p2p.Envelope) bool      { panic("local peer not implemented") }
func (LocalPeer) Set(key string, value any)        {}
func (LocalPeer) Get(key string) any               { panic("local peer not implemented") }
func (LocalPeer) SetRemovalFailed()                {}
func (LocalPeer) GetRemovalFailed() bool           { panic("local peer not implemented") }
func (LocalPeer) Start() error                     { panic("local peer not implemented") }
func (LocalPeer) OnStart() error                   { panic("local peer not implemented") }
func (LocalPeer) Stop() error                      { panic("local peer not implemented") }
func (LocalPeer) OnStop()                          {}
func (LocalPeer) Reset() error                     { panic("local peer not implemented") }
func (LocalPeer) OnReset() error                   { panic("local peer not implemented") }
func (LocalPeer) IsRunning() bool                  { panic("local peer not implemented") }
func (LocalPeer) Quit() <-chan struct{}            { panic("local peer not implemented") }
func (LocalPeer) String() string                   { panic("local peer not implemented") }
func (LocalPeer) SetLogger(l log.Logger)           {}
