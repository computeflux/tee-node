package relay

import (
	"crypto/tls"
	"net"
	"net/http"
	"sync/atomic"
	"time"

	"github.com/computeflux/tee-provider/pkg/util"
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
)

func init() {
	ApplyPoolConfig(defaultPoolConfig)
}

// GetPoolConfig returns current config
func GetPoolConfig() PoolConfig {
	return defaultPoolConfig
}

// ApplyPoolConfig replaces the connection pool atomically.
// In-flight requests continue using old pool; new requests use new pool.
func ApplyPoolConfig(cfg PoolConfig) {
	defaultPoolConfig = cfg

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
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: true,
		},
		ForceAttemptHTTP2: false,
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
	// util.DebugLog("[http] client.Do start url=%s method=%s", req.URL.String(), req.Method)
	resp, err := client.Do(req)
	if err != nil {
		util.DebugLog("[http] client.Do error: %v", err)
		return nil, err
	}
	// util.DebugLog("[http] client.Do done status=%d content-type=%s", resp.StatusCode, resp.Header.Get("Content-Type"))
	return resp, nil
}

// GetPoolStats returns approximate pool stats
func GetPoolStats() map[string]interface{} {
	cfg := GetPoolConfig()
	return map[string]interface{}{
		"maxConnsPerHost":     cfg.MaxConnsPerHost,
		"maxIdleConns":        cfg.MaxIdleConns,
		"maxIdleConnsPerHost": cfg.MaxIdleConnsPerHost,
		"requestTimeout":      cfg.RequestTimeout.String(),
	}
}
