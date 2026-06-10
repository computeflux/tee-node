package localtunnel

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/libp2p/go-yamux/v5"
)

// ServerConfig 本地隧道服务器配置
type ServerConfig struct {
	Addr             string            // 监听地址，如 ":19000"
	MaxConnsPerModel int               // 单模型最大连接数
	WarmupDuration   time.Duration     // 预热时间
	ValidateKey      ValidateAPIKeyFunc // API Key 验证回调
	TLSConfig        *tls.Config       // 可选 TLS 配置（nil 则为明文 TCP）
}

// Server 本地隧道服务端
type Server struct {
	config   ServerConfig
	listener net.Listener
	pool     *Pool
	wg       sync.WaitGroup
	ctx      context.Context
	cancel   context.CancelFunc
	mu       sync.Mutex
}

// NewServer 创建服务器
func NewServer(cfg ServerConfig) *Server {
	ctx, cancel := context.WithCancel(context.Background())
	return &Server{
		config: cfg,
		pool: NewPool(
			WithMaxConnsPerModel(cfg.MaxConnsPerModel),
			WithWarmupDuration(cfg.WarmupDuration),
			WithValidateKey(cfg.ValidateKey),
		),
		ctx:    ctx,
		cancel: cancel,
	}
}

// Start 启动监听
func (s *Server) Start() error {
	var listener net.Listener
	var err error

	if s.config.TLSConfig != nil {
		listener, err = tls.Listen("tcp", s.config.Addr, s.config.TLSConfig)
	} else {
		listener, err = net.Listen("tcp", s.config.Addr)
	}
	if err != nil {
		return fmt.Errorf("local tunnel listen %s: %w", s.config.Addr, err)
	}

	s.mu.Lock()
	s.listener = listener
	s.mu.Unlock()

	fmt.Printf("[LOCAL-TUNNEL] listening on %s (tls=%v)\n", s.config.Addr, s.config.TLSConfig != nil)

	s.wg.Add(2)
	go s.acceptLoop()
	go s.heartbeatLoop()
	return nil
}

func (s *Server) acceptLoop() {
	defer s.wg.Done()
	for {
		conn, err := s.listener.Accept()
		if err != nil {
			select {
			case <-s.ctx.Done():
				return // 正常关闭
			default:
				fmt.Printf("[LOCAL-TUNNEL] accept error: %v\n", err)
				continue
			}
		}
		s.wg.Add(1)
		go func(c net.Conn) {
			defer s.wg.Done()
			s.handleClientConn(c)
		}(conn)
	}
}

func (s *Server) handleClientConn(conn net.Conn) {
	remoteAddr := conn.RemoteAddr().String()

	// 1. 读取握手注册信息（10s 超时）
	_ = conn.SetReadDeadline(time.Now().Add(10 * time.Second))
	handshake, err := ReadHandshake(conn)
	if err != nil {
		conn.Close()
		fmt.Printf("[LOCAL-TUNNEL] handshake failed from %s: %v\n", remoteAddr, err)
		return
	}
	_ = conn.SetReadDeadline(time.Time{})

	// 2. 验证握手
	if err := s.pool.Validate(handshake, remoteAddr); err != nil {
		conn.Close()
		fmt.Printf("[LOCAL-TUNNEL] handshake rejected from %s model=%s: %v\n", remoteAddr, handshake.ModelID, err)
		return
	}

	// 3. 创建 yamux server session（成功后 yamux 接管 conn 生命周期）
	config := yamux.DefaultConfig()
	config.KeepAliveInterval = 10 * time.Second
	config.ConnectionWriteTimeout = 5 * time.Second
	config.MaxStreamWindowSize = 1024 * 1024 // 1MB window for high throughput
	config.AcceptBacklog = 256               // 允许 256 个 pending stream

	session, err := yamux.Server(conn, config, nil)
	if err != nil {
		conn.Close()
		fmt.Printf("[LOCAL-TUNNEL] yamux server failed from %s: %v\n", remoteAddr, err)
		return
	}
	defer session.Close()

	entry := &PoolEntry{
		Session:     session,
		ModelID:     handshake.ModelID,
		APIKey:      handshake.APIKey,
		BaseURL:     handshake.BaseURL,
		RemoteAddr:  remoteAddr,
		ConnectedAt: time.Now(),
		Available:   false,
	}
	s.pool.Add(entry)
	fmt.Printf("[LOCAL-TUNNEL] client connected from=%s model=%s baseUrl=%s, warmup=%s\n",
		remoteAddr, handshake.ModelID, handshake.BaseURL, s.config.WarmupDuration)

	// 4. 监听 session 关闭事件
	select {
	case <-session.CloseChan():
	case <-s.ctx.Done():
		session.Close()
	}

	s.pool.Remove(entry)
	fmt.Printf("[LOCAL-TUNNEL] client disconnected from=%s model=%s\n", remoteAddr, handshake.ModelID)
}

func (s *Server) heartbeatLoop() {
	defer s.wg.Done()
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			all := s.pool.AllEntries()
			for _, e := range all {
				if e.Session.IsClosed() {
					s.pool.Remove(e)
					fmt.Printf("[LOCAL-TUNNEL] removed dead session model=%s addr=%s\n", e.ModelID, e.RemoteAddr)
				}
			}
		case <-s.ctx.Done():
			return
		}
	}
}

// Pool 返回内部连接池（供 ForwardRequest 使用）
func (s *Server) Pool() *Pool {
	return s.pool
}

// Metrics 返回服务器指标
func (s *Server) Metrics() PoolMetrics {
	return s.pool.Metrics()
}

// Shutdown 优雅关闭服务器
func (s *Server) Shutdown(timeout time.Duration) error {
	s.cancel()

	s.mu.Lock()
	if s.listener != nil {
		s.listener.Close()
	}
	s.mu.Unlock()

	// 关闭所有连接池 session
	s.pool.Close()

	// 等待所有 goroutine 退出
	done := make(chan struct{})
	go func() {
		s.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		fmt.Println("[LOCAL-TUNNEL] shutdown complete")
		return nil
	case <-time.After(timeout):
		return fmt.Errorf("shutdown timeout")
	}
}

// ── Global server instance ───────────────────────────────

var globalServer *Server

// StartGlobalServer 启动全局服务器
func StartGlobalServer(addr string, validateKey ValidateAPIKeyFunc) error {
	cfg := ServerConfig{
		Addr:             addr,
		MaxConnsPerModel: 10,
		WarmupDuration:   10 * time.Second,
		ValidateKey:      validateKey,
	}
	globalServer = NewServer(cfg)
	return globalServer.Start()
}

// ShutdownGlobalServer 关闭全局服务器
func ShutdownGlobalServer(timeout time.Duration) error {
	if globalServer == nil {
		return nil
	}
	return globalServer.Shutdown(timeout)
}

// GetGlobalPool 返回全局连接池
func GetGlobalPool() *Pool {
	if globalServer == nil {
		return GlobalPool
	}
	return globalServer.Pool()
}
