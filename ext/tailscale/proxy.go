package tailscale

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"sync"
	"time"

	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"
)

// ProxyConfig configures the HTTP reverse proxy.
type ProxyConfig struct {
	// ListenAddr is the local address to listen on (e.g. ":8080").
	ListenAddr string `json:"listen_addr" yaml:"listen_addr"`

	// ReadTimeout for incoming requests.
	ReadTimeout time.Duration `json:"read_timeout" yaml:"read_timeout"`

	// WriteTimeout for proxied responses.
	WriteTimeout time.Duration `json:"write_timeout" yaml:"write_timeout"`

	// IdleTimeout for keep-alive connections.
	IdleTimeout time.Duration `json:"idle_timeout" yaml:"idle_timeout"`

	// MaxIdleConns controls the maximum number of idle connections per host.
	MaxIdleConns int `json:"max_idle_conns" yaml:"max_idle_conns"`

	// MaxIdleConnsPerHost limits idle connections per target node.
	MaxIdleConnsPerHost int `json:"max_idle_conns_per_host" yaml:"max_idle_conns_per_host"`

	// EnableH2C enables HTTP/2 cleartext (h2c) on both server and client side.
	// Allows multiplexing many concurrent requests over a single TCP connection.
	EnableH2C bool `json:"enable_h2c" yaml:"enable_h2c"`

	// PreConnect establishes TCP connections to known targets on startup.
	PreConnect bool `json:"pre_connect" yaml:"pre_connect"`

	// PreConnectConns is the number of connections to pre-establish per target.
	PreConnectConns int `json:"pre_connect_conns" yaml:"pre_connect_conns"`
}

// DefaultProxyConfig returns optimized defaults.
func DefaultProxyConfig() ProxyConfig {
	return ProxyConfig{
		ListenAddr:          ":8080",
		ReadTimeout:         30 * time.Second,
		WriteTimeout:        60 * time.Second,
		IdleTimeout:         120 * time.Second,
		MaxIdleConns:        512,
		MaxIdleConnsPerHost: 32,
		EnableH2C:           true,
		PreConnect:          true,
		PreConnectConns:     2,
	}
}

// Proxy is an HTTP reverse proxy that routes requests to Tailscale peers.
type Proxy struct {
	cfg    ProxyConfig
	node   *Node
	router Router
	srv    *http.Server
	mu     sync.RWMutex

	// Per-target reverse proxy cache
	proxies map[string]*httputil.ReverseProxy

	// Shared transport for HTTP/2 multiplexing
	transport *http.Transport

	// Pre-connected targets
	preConnected map[string]struct{}
}

// NewProxy creates a new HTTP proxy that routes to Tailscale peers.
func NewProxy(cfg ProxyConfig, node *Node, router Router) *Proxy {
	p := &Proxy{
		cfg:          cfg,
		node:         node,
		router:       router,
		proxies:      make(map[string]*httputil.ReverseProxy),
		preConnected: make(map[string]struct{}),
	}

	// Shared transport with HTTP/2 support
	p.transport = &http.Transport{
		DialContext:         node.server.Dial,
		MaxIdleConns:        cfg.MaxIdleConns,
		MaxIdleConnsPerHost: cfg.MaxIdleConnsPerHost,
		IdleConnTimeout:     cfg.IdleTimeout,
		MaxConnsPerHost:     64,
		DisableCompression:  false,

		// TCP keepalive for long-lived connections
		DialTLSContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			conn, err := node.server.Dial(ctx, network, addr)
			if err != nil {
				return nil, err
			}
			if tcp, ok := conn.(*net.TCPConn); ok {
				tcp.SetKeepAlive(true)
				tcp.SetKeepAlivePeriod(30 * time.Second)
			}
			return conn, nil
		},

		// HTTP/2: RFC 7540 prior knowledge (no TLS upgrade needed on Tailscale)
		ForceAttemptHTTP2: cfg.EnableH2C,
	}

	// Configure HTTP/2 transport settings
	if cfg.EnableH2C {
		if t2, err := http2.ConfigureTransports(p.transport); err == nil {
			t2.ReadIdleTimeout = cfg.IdleTimeout
			t2.PingTimeout = 15 * time.Second
			t2.WriteByteTimeout = 30 * time.Second
			t2.MaxHeaderListSize = 16 << 10 // 16KB
		}
	}

	return p
}

// Start begins listening and serving.
func (p *Proxy) Start() error {
	mux := http.NewServeMux()
	mux.HandleFunc("/", p.handleRequest)
	mux.HandleFunc("/health", p.handleHealth)

	handler := http.Handler(mux)

	// HTTP/2 cleartext server
	if p.cfg.EnableH2C {
		handler = h2c.NewHandler(mux, &http2.Server{
			IdleTimeout:          p.cfg.IdleTimeout,
			MaxConcurrentStreams: 256,
			MaxReadFrameSize:     1 << 20, // 1MB
		})
	}

	p.srv = &http.Server{
		Addr:         p.cfg.ListenAddr,
		Handler:      handler,
		ReadTimeout:  p.cfg.ReadTimeout,
		WriteTimeout: p.cfg.WriteTimeout,
		IdleTimeout:  p.cfg.IdleTimeout,
	}

	go p.preConnectTargets()

	log.Printf("[tailscale-proxy] listening on %s (h2c=%v)", p.cfg.ListenAddr, p.cfg.EnableH2C)
	return p.srv.ListenAndServe()
}

// Shutdown gracefully stops the proxy.
func (p *Proxy) Shutdown(ctx context.Context) error {
	p.transport.CloseIdleConnections()
	return p.srv.Shutdown(ctx)
}

// preConnectTargets pre-establishes connections to known targets.
func (p *Proxy) preConnectTargets() {
	if !p.cfg.PreConnect {
		return
	}

	// Discover all known targets from static routes + peers
	targets := p.router.AllTargets()
	if len(targets) == 0 {
		return
	}

	for _, target := range targets {
		p.preConnect(target)
	}
}

// preConnect establishes idle connections to a target.
func (p *Proxy) preConnect(target string) {
	p.mu.Lock()
	if _, ok := p.preConnected[target]; ok {
		p.mu.Unlock()
		return
	}
	p.preConnected[target] = struct{}{}
	p.mu.Unlock()

	for i := 0; i < p.cfg.PreConnectConns; i++ {
		go func(t string) {
			conn, err := p.transport.DialContext(context.Background(), "tcp", t)
			if err != nil {
				log.Printf("[tailscale-proxy] preconnect failed %s: %v", t, err)
				return
			}
			conn.Close()
		}(target)
	}
	log.Printf("[tailscale-proxy] preconnecting %d conns to %s", p.cfg.PreConnectConns, target)
}

// handleHealth returns node status and discovered peers.
func (p *Proxy) handleHealth(w http.ResponseWriter, r *http.Request) {
	ip, _ := p.node.LocalIP()
	w.Header().Set("Content-Type", "application/json")
	fmt.Fprintf(w, `{"status":"ok","hostname":"%s","tailscale_ip":"%s"}`,
		p.node.LocalHostname(), ip)
}

// handleRequest routes an incoming request to the appropriate Tailscale peer.
func (p *Proxy) handleRequest(w http.ResponseWriter, r *http.Request) {
	chainID := firstNonEmpty(
		r.Header.Get("X-Chain-Id"),
		r.URL.Query().Get("chain_id"),
	)
	service := firstNonEmpty(
		r.Header.Get("X-Service"),
		r.URL.Query().Get("service"),
		"rpc",
	)
	region := r.Header.Get("X-Region") // optional: request's origin region

	if chainID == "" {
		http.Error(w, "missing X-Chain-Id header or chain_id query param", http.StatusBadRequest)
		return
	}

	// Resolve target with region preference
	target, err := p.router.Route(chainID, service, region)
	if err != nil {
		log.Printf("[tailscale-proxy] route failed: chain=%s svc=%s region=%s err=%v",
			chainID, service, region, err)
		http.Error(w, fmt.Sprintf("no route: %v", err), http.StatusBadGateway)
		return
	}

	rp := p.getOrCreateProxy(target)

	// Strip routing headers before forwarding
	r.Header.Del("X-Chain-Id")
	r.Header.Del("X-Service")
	r.Header.Del("X-Region")

	log.Printf("[tailscale-proxy] %s %s → %s (chain=%s svc=%s region=%s)",
		r.Method, r.URL.Path, target, chainID, service, region)

	rp.ServeHTTP(w, r)
}

// getOrCreateProxy returns a cached reverse proxy for the target.
func (p *Proxy) getOrCreateProxy(target string) *httputil.ReverseProxy {
	p.mu.RLock()
	rp, ok := p.proxies[target]
	p.mu.RUnlock()
	if ok {
		return rp
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	if rp, ok = p.proxies[target]; ok {
		return rp
	}

	targetURL, _ := url.Parse("http://" + target)

	// If h2c enabled, use prior-knowledge HTTP/2 dialer
	var dial func(ctx context.Context, network, addr string) (net.Conn, error)
	if p.cfg.EnableH2C {
		dial = func(ctx context.Context, network, addr string) (net.Conn, error) {
			conn, err := p.node.server.Dial(ctx, network, addr)
			if err != nil {
				return nil, err
			}
			if tcp, ok := conn.(*net.TCPConn); ok {
				tcp.SetKeepAlive(true)
				tcp.SetKeepAlivePeriod(30 * time.Second)
			}
			return conn, nil
		}
	} else {
		dial = p.node.server.Dial
	}

	rp = &httputil.ReverseProxy{
		Rewrite: func(pr *httputil.ProxyRequest) {
			pr.SetURL(targetURL)
			pr.Out.Host = targetURL.Host
		},
		Transport: &http.Transport{
			DialContext:         dial,
			MaxIdleConns:        p.cfg.MaxIdleConns,
			MaxIdleConnsPerHost: p.cfg.MaxIdleConnsPerHost,
			IdleConnTimeout:     p.cfg.IdleTimeout,
			MaxConnsPerHost:     64,
			ForceAttemptHTTP2:   p.cfg.EnableH2C,
			DisableCompression:  false,
		},
		ErrorHandler: func(w http.ResponseWriter, r *http.Request, err error) {
			log.Printf("[tailscale-proxy] proxy error to %s: %v", target, err)
			http.Error(w, fmt.Sprintf("proxy error: %v", err), http.StatusBadGateway)
		},
		// Reduce buffer copying for large responses (blockchain RPC)
		BufferPool: &bufferPool{size: 32 << 10}, // 32KB
	}

	p.proxies[target] = rp
	return rp
}

// bufferPool implements httputil.BufferPool for zero-allocation buffer reuse.
type bufferPool struct {
	pool sync.Pool
	size int
}

func (bp *bufferPool) Get() []byte {
	if v := bp.pool.Get(); v != nil {
		return v.([]byte)
	}
	return make([]byte, bp.size)
}

func (bp *bufferPool) Put(buf []byte) {
	if cap(buf) == bp.size {
		bp.pool.Put(buf[:bp.size])
	}
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}
