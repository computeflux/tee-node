package localtunnel

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/libp2p/go-yamux/v5"
)

// ServerConfig 本地隧道服务器配置
type ServerConfig struct {
	Addr           string             // 监听地址，如 ":19000"
	WarmupDuration time.Duration      // 预热时间
	ValidateKey    ValidateAPIKeyFunc // API Key 验证回调
	TLSConfig      *tls.Config        // 可选 TLS 配置（nil 则为明文 TCP）
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

	// ProviderID 必须由服务端根据 apiKey 推导，忽略客户端传入的任何值
	handshake.ProviderID = 0

	// 2. 验证握手（同时推导 ProviderID）
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

	models := handshake.RegisteredModels()
	if len(models) == 0 {
		conn.Close()
		fmt.Printf("[LOCAL-TUNNEL] no models registered from %s\n", remoteAddr)
		return
	}

	entries := make([]*PoolEntry, 0, len(models))
	var entriesMu sync.Mutex
	for _, modelID := range models {
		entry := &PoolEntry{
			Session:    session,
			ProviderID: handshake.ProviderID,
			ModelID:    modelID,
			APIKey:     handshake.APIKey,
			BaseURL:    handshake.ModelBaseURL(modelID),
			RemoteAddr: remoteAddr,
			ConnectedAt: time.Now(),
			Available:  false,
		}
		s.pool.Add(entry)
		entries = append(entries, entry)
	}
	fmt.Printf("[LOCAL-TUNNEL] client connected from=%s provider=%d models=%v baseUrl=%s, warmup=%s\n",
		remoteAddr, handshake.ProviderID, models, handshake.BaseURL, s.config.WarmupDuration)

	// 4. 启动控制流 goroutine，处理客户端动态增删模型
	var ctrlWg sync.WaitGroup
	ctrlWg.Add(1)
	go func() {
		defer ctrlWg.Done()
		s.handleControlStreams(session, handshake, remoteAddr, &entries, &entriesMu)
	}()

	// 5. 监听 session 关闭事件
	select {
	case <-session.CloseChan():
	case <-s.ctx.Done():
		session.Close()
	}

	// 等待控制流 goroutine 退出后再清理
	ctrlWg.Wait()

	entriesMu.Lock()
	for _, e := range entries {
		s.pool.Remove(e)
	}
	entriesMu.Unlock()
	fmt.Printf("[LOCAL-TUNNEL] client disconnected from=%s provider=%d models=%v\n", remoteAddr, handshake.ProviderID, models)
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

// handleControlStreams 接受并处理本地客户端发起的控制流
func (s *Server) handleControlStreams(session *yamux.Session, handshake *Handshake, remoteAddr string, entries *[]*PoolEntry, entriesMu *sync.Mutex) {
	for {
		stream, err := session.AcceptStream()
		if err != nil {
			return
		}
		go s.handleControlStream(stream, session, handshake, remoteAddr, entries, entriesMu)
	}
}

func (s *Server) handleControlStream(stream net.Conn, session *yamux.Session, handshake *Handshake, remoteAddr string, entries *[]*PoolEntry, entriesMu *sync.Mutex) {
	defer stream.Close()

	ft, payload, err := ReadTypedFrame(stream)
	if err != nil {
		return
	}

	switch ft {
	case FrameTypeModelRegister:
		var ctrl ModelControlFrame
		if err := json.Unmarshal(payload, &ctrl); err != nil || ctrl.ModelID == "" {
			sendControlError(stream, "invalid register frame")
			return
		}
		if err := s.registerModel(session, handshake, remoteAddr, ctrl.ModelID, ctrl.BaseURL, entries, entriesMu); err != nil {
			sendControlError(stream, err.Error())
			return
		}
		sendControlSuccess(stream)

	case FrameTypeModelUnregister:
		var ctrl ModelControlFrame
		if err := json.Unmarshal(payload, &ctrl); err != nil || ctrl.ModelID == "" {
			sendControlError(stream, "invalid unregister frame")
			return
		}
		if err := s.unregisterModel(handshake, ctrl.ModelID, entries, entriesMu); err != nil {
			sendControlError(stream, err.Error())
			return
		}
		sendControlSuccess(stream)

	default:
		sendControlError(stream, "unknown control frame")
	}
}

func (s *Server) registerModel(session *yamux.Session, handshake *Handshake, remoteAddr, modelID, baseURL string, entries *[]*PoolEntry, entriesMu *sync.Mutex) error {
	if modelID == "" {
		return fmt.Errorf("modelId is required")
	}
	if baseURL == "" {
		baseURL = handshake.ModelBaseURL(modelID)
	}

	// API Key 校验（必须解析到与当前 session 相同的 ProviderID）
	if s.pool.validateKey != nil {
		pid, ok := s.pool.validateKey(modelID, handshake.APIKey)
		if !ok || pid != handshake.ProviderID {
			return fmt.Errorf("invalid api key for model %s", modelID)
		}
	}

	// 连接数上限校验
	rk := routeKey(handshake.ProviderID, modelID)
	if err := s.pool.CheckModelQuota(rk); err != nil {
		return err
	}

	entriesMu.Lock()
	defer entriesMu.Unlock()

	// 去重（同一供应商同一模型只能注册一次）
	for _, e := range *entries {
		if e.ProviderID == handshake.ProviderID && e.ModelID == modelID {
			return fmt.Errorf("model %s already registered", modelID)
		}
	}

	entry := &PoolEntry{
		Session:    session,
		ProviderID: handshake.ProviderID,
		ModelID:    modelID,
		APIKey:     handshake.APIKey,
		BaseURL:    baseURL,
		RemoteAddr: remoteAddr,
		ConnectedAt: time.Now(),
		Available:  false,
	}
	s.pool.Add(entry)
	*entries = append(*entries, entry)
	fmt.Printf("[LOCAL-TUNNEL] model registered from=%s provider=%d model=%s baseUrl=%s\n", remoteAddr, handshake.ProviderID, modelID, baseURL)
	return nil
}

func (s *Server) unregisterModel(handshake *Handshake, modelID string, entries *[]*PoolEntry, entriesMu *sync.Mutex) error {
	entriesMu.Lock()
	defer entriesMu.Unlock()

	list := *entries
	for i, e := range list {
		if e.ProviderID == handshake.ProviderID && e.ModelID == modelID {
			s.pool.Remove(e)
			*entries = append(list[:i], list[i+1:]...)
			fmt.Printf("[LOCAL-TUNNEL] model unregistered provider=%d model=%s\n", handshake.ProviderID, modelID)
			return nil
		}
	}
	return fmt.Errorf("model %s not found", modelID)
}

func sendControlSuccess(stream net.Conn) {
	_ = WriteJSONFrame(stream, FrameTypeHeader, HeaderFrame{Status: 200})
	_ = WriteTypedFrame(stream, FrameTypeDone, nil)
}

func sendControlError(stream net.Conn, message string) {
	_ = WriteJSONFrame(stream, FrameTypeError, ErrorFrame{Message: message})
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
		Addr:           addr,
		WarmupDuration: 10 * time.Second,
		ValidateKey:    validateKey,
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
