package relay

import (
	"crypto/tls"
	"net"
	"net/http"
	"sync"
	"sync/atomic"
	"time"
)

// ── Dynamic Connection Pool ──────────────────────────────
//
// MaxConnsPerHost 运行时动态调整：
//   - 通过 GraphQL mutation 或内部监控自动调整
//   - 原子替换 Transport，不影响正在进行的请求
//   - 默认 500，可按需扩大到 5000

type PoolConfig struct {
	MaxIdleConns        int `json:"maxIdleConns"`
	MaxIdleConnsPerHost int `json:"maxIdleConnsPerHost"`
	MaxConnsPerHost     int `json:"maxConnsPerHost"`
	DialTimeout         time.Duration
	KeepAlive           time.Duration
	IdleTimeout         time.Duration
	RequestTimeout      time.Duration
	// InsecureSkipVerify 关闭 TLS 证书校验。仅当上游使用自签证书且无法
	// 分发 CA 时才应开启；保持 false 是安全默认值。
	InsecureSkipVerify bool `json:"insecureSkipVerify,omitempty"`
}

var (
	defaultPoolConfig = PoolConfig{
		MaxIdleConns:        2000,
		MaxIdleConnsPerHost: 200,
		MaxConnsPerHost:     500,
		DialTimeout:         10 * time.Second,
		KeepAlive:           60 * time.Second,
		IdleTimeout:         90 * time.Second,
		RequestTimeout:      180 * time.Second,
	}

	// 原子指针，支持运行时替换
	currentTransport atomic.Value // *http.Transport
	currentClient    atomic.Value // *http.Client

	// defaultPoolConfigMu 保护 defaultPoolConfig 的并发读写。
	// 实际请求使用 currentClient/Transport，不受锁影响。
	defaultPoolConfigMu sync.RWMutex
)

func init() {
	ApplyPoolConfig(defaultPoolConfig)
}

// GetDefaultPoolConfig 线程安全地读取当前默认池配置（主要用于诊断/测试）。
func GetDefaultPoolConfig() PoolConfig {
	defaultPoolConfigMu.RLock()
	defer defaultPoolConfigMu.RUnlock()
	return defaultPoolConfig
}

// ApplyPoolConfig replaces the connection pool atomically.
// In-flight requests continue using old pool; new requests use new pool.
func ApplyPoolConfig(cfg PoolConfig) {
	defaultPoolConfigMu.Lock()
	defaultPoolConfig = cfg
	defaultPoolConfigMu.Unlock()

	tlsCfg := &tls.Config{
		MinVersion: tls.VersionTLS12,
	}
	if cfg.InsecureSkipVerify {
		// 仅在显式配置时跳过证书校验；调用方需自行承担风险。
		tlsCfg.InsecureSkipVerify = true
	}

	transport := &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			Timeout:   cfg.DialTimeout,
			KeepAlive: cfg.KeepAlive,
		}).DialContext,
		MaxIdleConns:          cfg.MaxIdleConns,
		MaxIdleConnsPerHost:   cfg.MaxIdleConnsPerHost,
		MaxConnsPerHost:       cfg.MaxConnsPerHost,
		IdleConnTimeout:       cfg.IdleTimeout,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
		TLSClientConfig:       tlsCfg,
		ForceAttemptHTTP2:     false,
	}

	client := &http.Client{
		Transport: transport,
		Timeout:   cfg.RequestTimeout,
	}

	currentTransport.Store(transport)
	currentClient.Store(client)
}

// DoHTTP executes request with current pool
func DoHTTP(req *http.Request) (*http.Response, error) {
	client, _ := currentClient.Load().(*http.Client)
	if client == nil {
		client = http.DefaultClient
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	return resp, nil
}
