// local-client 是本地模型反向隧道客户端（Yamux 版）。
//
// 用法：
//   go run main.go -config config.yaml
//   或直接传参：go run main.go -gateway tee-node:19000 -model llama3 -key xxx -local http://localhost:11434
//
// 工作流程：
//   1. TCP 连接到 TEE-Node :19000
//   2. 发送 Handshake（协议版本 + 模型信息 + API Key）
//   3. 创建 Yamux client session
//   4. 循环 AcceptStream，处理 TEE-Node 转发请求
//   5. 将请求转发到本地 Ollama/vLLM 等服务
//   6. 将响应通过 Yamux stream 返回
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	localtunnel "github.com/computeflux/tee-provider/pkg/services/local_tunnel"
	"github.com/libp2p/go-yamux/v5"
)

// Config 客户端配置
type Config struct {
	GatewayAddr      string        `json:"gateway"`       // TEE-Node 隧道地址
	ModelID          string        `json:"model"`         // 主模型标识
	APIKey           string        `json:"key"`           // API Key
	LocalURL         string        `json:"local"`         // 本地服务地址
	MaxConcurrency   int           `json:"maxConcurrency"` // 最大并发处理数（防止压垮本地 GPU）
	HealthCheckInterval time.Duration `json:"healthCheckInterval"`
	ReconnectBackoff time.Duration `json:"reconnectBackoff"`
	MaxBackoff       time.Duration `json:"maxBackoff"`
}

var defaultConfig = Config{
	GatewayAddr:         "localhost:19000",
	MaxConcurrency:      10,
	HealthCheckInterval: 30 * time.Second,
	ReconnectBackoff:    1 * time.Second,
	MaxBackoff:          30 * time.Second,
}

var (
	gatewayAddr = flag.String("gateway", "", "TEE-Node TCP tunnel address (default: localhost:19000)")
	modelID     = flag.String("model", "", "模型标识")
	apiKey      = flag.String("key", "", "API Key")
	localURL    = flag.String("local", "", "本地模型服务地址 (default: http://localhost:11434)")
	configFile  = flag.String("config", "", "配置文件路径 (JSON/YAML)")
)

func main() {
	flag.Parse()

	cfg := loadConfig()
	if cfg.ModelID == "" || cfg.APIKey == "" {
		fmt.Fprintln(os.Stderr, "Usage: local-client -model <model> -key <key> [-gateway <addr>] [-local <url>]")
		flag.PrintDefaults()
		os.Exit(1)
	}

	fmt.Printf("[LOCAL-CLIENT] model=%s gateway=%s local=%s maxConcurrency=%d\n",
		cfg.ModelID, cfg.GatewayAddr, cfg.LocalURL, cfg.MaxConcurrency)

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	backoff := cfg.ReconnectBackoff
	for {
		if err := runClient(ctx, cfg); err != nil {
			fmt.Printf("[LOCAL-CLIENT] connection lost: %v, reconnecting in %v...\n", err, backoff)
			select {
			case <-time.After(backoff):
				backoff *= 2
				if backoff > cfg.MaxBackoff {
					backoff = cfg.MaxBackoff
				}
				continue
			case <-ctx.Done():
				fmt.Println("[LOCAL-CLIENT] shutdown")
				return
			}
		}
		backoff = cfg.ReconnectBackoff
	}
}

func loadConfig() Config {
	cfg := defaultConfig

	// 配置文件优先
	if *configFile != "" {
		data, err := os.ReadFile(*configFile)
		if err == nil {
			json.Unmarshal(data, &cfg)
		}
	}

	// 命令行参数覆盖
	if *gatewayAddr != "" {
		cfg.GatewayAddr = *gatewayAddr
	}
	if *modelID != "" {
		cfg.ModelID = *modelID
	}
	if *apiKey != "" {
		cfg.APIKey = *apiKey
	}
	if *localURL != "" {
		cfg.LocalURL = *localURL
	}

	return cfg
}

func runClient(ctx context.Context, cfg Config) error {
	dialer := net.Dialer{Timeout: 10 * time.Second}
	conn, err := dialer.DialContext(ctx, "tcp", cfg.GatewayAddr)
	if err != nil {
		return fmt.Errorf("tcp dial: %w", err)
	}
	defer conn.Close()

	fmt.Println("[LOCAL-CLIENT] TCP connected, sending handshake...")

	// 1. 发送注册握手
	handshake := localtunnel.Handshake{
		Version: localtunnel.ProtocolVersion,
		ModelID: cfg.ModelID,
		APIKey:  cfg.APIKey,
		BaseURL: cfg.LocalURL,
		Models:  []string{cfg.ModelID},
	}
	if err := localtunnel.WriteHandshake(conn, handshake); err != nil {
		return fmt.Errorf("send handshake: %w", err)
	}

	// 2. 创建 yamux client session
	config := yamux.DefaultConfig()
	config.KeepAliveInterval = 10 * time.Second
	config.ConnectionWriteTimeout = 5 * time.Second
	config.MaxStreamWindowSize = 1024 * 1024
	config.AcceptBacklog = 256

	session, err := yamux.Client(conn, config, nil)
	if err != nil {
		return fmt.Errorf("yamux client: %w", err)
	}
	defer session.Close()

	fmt.Println("[LOCAL-CLIENT] registered, waiting for requests...")

	// 3. 启动本地服务健康检查
	healthCtx, healthCancel := context.WithCancel(ctx)
	go healthCheckLoop(healthCtx, cfg)
	defer healthCancel()

	// 4. 并发控制（防止压垮本地 Ollama）
	sema := make(chan struct{}, cfg.MaxConcurrency)

	// 5. 循环 AcceptStream 处理请求
	for {
		stream, err := session.AcceptStream()
		if err != nil {
			return fmt.Errorf("accept stream: %w", err)
		}

		select {
		case sema <- struct{}{}:
			go func(s net.Conn) {
				defer func() { <-sema }()
				handleStream(s, cfg)
			}(stream)
		case <-ctx.Done():
			stream.Close()
			return ctx.Err()
		default:
			// 并发满，发送 Error 帧后关闭 stream
			sendErrorFrame(stream, "max concurrency reached")
			stream.Close()
			fmt.Println("[LOCAL-CLIENT] max concurrency reached, rejecting stream")
		}
	}
}

func handleStream(stream net.Conn, cfg Config) {
	defer stream.Close()

	// 读取请求帧（JSON）
	var req localtunnel.RequestFrame
	if err := localtunnel.ReadJSONFrame(stream, localtunnel.FrameTypeRequest, &req); err != nil {
		sendErrorFrame(stream, "invalid request: "+err.Error())
		return
	}

	// 转发到本地 Ollama/vLLM
	localEndpoint := cfg.LocalURL + req.Path
	upstreamReq, err := http.NewRequest(req.Method, localEndpoint, bytes.NewReader(req.Body))
	if err != nil {
		sendErrorFrame(stream, "create request: "+err.Error())
		return
	}
	for k, v := range req.Headers {
		upstreamReq.Header.Set(k, v)
	}

	// 复用 HTTP client（支持连接池）
	client := &http.Client{
		Timeout:   120 * time.Second,
		Transport: sharedTransport,
	}
	resp, err := client.Do(upstreamReq)
	if err != nil {
		sendErrorFrame(stream, "upstream error: "+err.Error())
		return
	}
	defer resp.Body.Close()

	// 发送响应头帧（JSON）
	if err := localtunnel.WriteJSONFrame(stream, localtunnel.FrameTypeHeader, localtunnel.HeaderFrame{
		Status:  resp.StatusCode,
		Headers: headersToMap(resp.Header),
	}); err != nil {
		fmt.Printf("[LOCAL-CLIENT] write header error: %v\n", err)
		return
	}

	// 流式转发响应体（二进制 Data 帧，无需 JSON wrap）
	buf := make([]byte, 32*1024)
	for {
		n, err := resp.Body.Read(buf)
		if n > 0 {
			if err := localtunnel.WriteTypedFrame(stream, localtunnel.FrameTypeData, buf[:n]); err != nil {
				fmt.Printf("[LOCAL-CLIENT] write data error: %v\n", err)
				return
			}
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			fmt.Printf("[LOCAL-CLIENT] read upstream error: %v\n", err)
			break
		}
	}

	// 发送结束帧
	if err := localtunnel.WriteTypedFrame(stream, localtunnel.FrameTypeDone, nil); err != nil {
		fmt.Printf("[LOCAL-CLIENT] write done error: %v\n", err)
	}
}

func sendErrorFrame(stream net.Conn, message string) {
	errPayload, _ := json.Marshal(localtunnel.ErrorFrame{Message: message})
	_ = localtunnel.WriteTypedFrame(stream, localtunnel.FrameTypeError, errPayload)
}

// 共享 HTTP Transport，复用连接池到本地 Ollama
var sharedTransport = &http.Transport{
	MaxIdleConns:        100,
	MaxIdleConnsPerHost: 100,
	IdleConnTimeout:     90 * time.Second,
}

func healthCheckLoop(ctx context.Context, cfg Config) {
	if cfg.HealthCheckInterval <= 0 {
		return
	}
	ticker := time.NewTicker(cfg.HealthCheckInterval)
	defer ticker.Stop()

	client := &http.Client{Timeout: 5 * time.Second}
	healthURL := cfg.LocalURL + "/v1/models"

	for {
		select {
		case <-ticker.C:
			resp, err := client.Get(healthURL)
			if err != nil {
				fmt.Printf("[LOCAL-CLIENT] health check FAIL: %v\n", err)
			} else {
				resp.Body.Close()
				if resp.StatusCode != 200 {
					fmt.Printf("[LOCAL-CLIENT] health check WARN: status=%d\n", resp.StatusCode)
				}
			}
		case <-ctx.Done():
			return
		}
	}
}

func headersToMap(h http.Header) map[string]string {
	m := make(map[string]string)
	for k, vs := range h {
		if len(vs) > 0 {
			m[k] = vs[0]
		}
	}
	return m
}
