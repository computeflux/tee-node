// local-client 是本地模型反向隧道客户端（Yamux 版）。
//
// 用法：
//   go run main.go -config config.json
//   或直接传参：go run main.go -gateway tee-node:19000 -model llama3 -key xxx -local http://localhost:11434
//
// 多模型配置示例（config.json）：
//   {
//     "gateway": "tee-node:19000",
//     "key": "xxx",
//     "local": "http://localhost:11434",
//     "models": {
//       "qwen2.5": "http://localhost:11434",
//       "llama3": "http://localhost:8000"
//     }
//   }
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
	"sync"
	"syscall"
	"time"

	localtunnel "github.com/computeflux/tee-provider/pkg/services/local_tunnel"
	"github.com/libp2p/go-yamux/v5"
)

// Config 客户端配置
type Config struct {
	GatewayAddr         string            `json:"gateway"`          // TEE-Node 隧道地址
	ModelID             string            `json:"model"`            // 主模型标识（兼容旧版）
	APIKey              string            `json:"key"`              // API Key
	LocalURL            string            `json:"local"`            // 本地服务默认地址
	ModelURLs           map[string]string `json:"models"`           // 模型 ID -> 本地服务地址
	AdminAddr           string            `json:"admin"`            // 管理 HTTP 监听地址（空则禁用）
	MaxConcurrency      int               `json:"maxConcurrency"`   // 最大并发处理数（防止压垮本地 GPU）
	HealthCheckInterval time.Duration     `json:"healthCheckInterval"`
	ReconnectBackoff    time.Duration     `json:"reconnectBackoff"`
	MaxBackoff          time.Duration     `json:"maxBackoff"`
}

// 运行时动态增删的模型地址表（启动时从 cfg.ModelURLs 初始化）
var (
	appCfg Config

	extraModelURLs   = make(map[string]string)
	extraModelURLsMu sync.RWMutex

	currentSession   *yamux.Session
	currentSessionMu sync.RWMutex
)

func setCurrentSession(s *yamux.Session) {
	currentSessionMu.Lock()
	currentSession = s
	currentSessionMu.Unlock()
}

func getCurrentSession() *yamux.Session {
	currentSessionMu.RLock()
	defer currentSessionMu.RUnlock()
	return currentSession
}

// registeredModels 返回需要向 TEE-Node 注册的所有模型 ID
func registeredModels(cfg Config) []string {
	seen := make(map[string]struct{})
	if cfg.ModelID != "" {
		seen[cfg.ModelID] = struct{}{}
	}
	extraModelURLsMu.RLock()
	for modelID := range extraModelURLs {
		if modelID != "" {
			seen[modelID] = struct{}{}
		}
	}
	extraModelURLsMu.RUnlock()
	models := make([]string, 0, len(seen))
	for modelID := range seen {
		models = append(models, modelID)
	}
	return models
}

// modelLocalURL 返回指定模型对应的本地服务地址
func modelLocalURL(cfg Config, modelID string) string {
	extraModelURLsMu.RLock()
	if u, ok := extraModelURLs[modelID]; ok && u != "" {
		extraModelURLsMu.RUnlock()
		return u
	}
	extraModelURLsMu.RUnlock()
	return cfg.LocalURL
}

// allLocalURLs 返回所有配置的本地服务地址（去重）
func allLocalURLs(cfg Config) []string {
	seen := make(map[string]struct{})
	if cfg.LocalURL != "" {
		seen[cfg.LocalURL] = struct{}{}
	}
	extraModelURLsMu.RLock()
	for _, u := range extraModelURLs {
		if u != "" {
			seen[u] = struct{}{}
		}
	}
	extraModelURLsMu.RUnlock()
	urls := make([]string, 0, len(seen))
	for u := range seen {
		urls = append(urls, u)
	}
	return urls
}

func cloneExtraModelURLs() map[string]string {
	extraModelURLsMu.RLock()
	defer extraModelURLsMu.RUnlock()
	m := make(map[string]string, len(extraModelURLs))
	for k, v := range extraModelURLs {
		m[k] = v
	}
	return m
}

var defaultConfig = Config{
	GatewayAddr:         "localhost:19000",
	AdminAddr:           ":19001",
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
	adminAddr   = flag.String("admin", "", "管理 HTTP 监听地址 (默认 :19001，空则禁用)")
	configFile  = flag.String("config", "", "配置文件路径 (JSON/YAML)")
)

func main() {
	flag.Parse()

	appCfg = loadConfig()
	if appCfg.APIKey == "" || (appCfg.ModelID == "" && len(appCfg.ModelURLs) == 0) {
		fmt.Fprintln(os.Stderr, "Usage: local-client -model <model> -key <key> [-gateway <addr>] [-local <url>] [-config <file>]")
		flag.PrintDefaults()
		os.Exit(1)
	}

	// 初始化运行时模型表
	extraModelURLsMu.Lock()
	for k, v := range appCfg.ModelURLs {
		extraModelURLs[k] = v
	}
	extraModelURLsMu.Unlock()

	fmt.Printf("[LOCAL-CLIENT] models=%v gateway=%s local=%s modelUrls=%v maxConcurrency=%d\n",
		registeredModels(appCfg), appCfg.GatewayAddr, appCfg.LocalURL, appCfg.ModelURLs, appCfg.MaxConcurrency)

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	// 启动管理 HTTP 接口
	if appCfg.AdminAddr != "" {
		adminSrv := startAdminServer(ctx, appCfg.AdminAddr)
		defer adminSrv.Shutdown(context.Background())
	}

	backoff := appCfg.ReconnectBackoff
	for {
		if err := runClient(ctx, appCfg); err != nil {
			fmt.Printf("[LOCAL-CLIENT] connection lost: %v, reconnecting in %v...\n", err, backoff)
			select {
			case <-time.After(backoff):
				backoff *= 2
				if backoff > appCfg.MaxBackoff {
					backoff = appCfg.MaxBackoff
				}
				continue
			case <-ctx.Done():
				fmt.Println("[LOCAL-CLIENT] shutdown")
				return
			}
		}
		backoff = appCfg.ReconnectBackoff
	}
}

func startAdminServer(ctx context.Context, addr string) *http.Server {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /admin/models", handleRegisterModel)
	mux.HandleFunc("DELETE /admin/models/{modelId}", handleUnregisterModel)

	srv := &http.Server{Addr: addr, Handler: mux}
	go func() {
		fmt.Printf("[LOCAL-CLIENT] admin server listening on %s\n", addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			fmt.Printf("[LOCAL-CLIENT] admin server error: %v\n", err)
		}
	}()
	go func() {
		<-ctx.Done()
		_ = srv.Shutdown(context.Background())
	}()
	return srv
}

func handleRegisterModel(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ModelID string `json:"modelId"`
		URL     string `json:"url"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if req.ModelID == "" || req.URL == "" {
		http.Error(w, "modelId and url are required", http.StatusBadRequest)
		return
	}

	if err := sendModelControl(localtunnel.FrameTypeModelRegister, req.ModelID, req.URL); err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}

	extraModelURLsMu.Lock()
	extraModelURLs[req.ModelID] = req.URL
	extraModelURLsMu.Unlock()

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok", "modelId": req.ModelID})
}

func handleUnregisterModel(w http.ResponseWriter, r *http.Request) {
	modelID := r.PathValue("modelId")
	if modelID == "" {
		http.Error(w, "modelId is required", http.StatusBadRequest)
		return
	}

	if err := sendModelControl(localtunnel.FrameTypeModelUnregister, modelID, ""); err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}

	extraModelURLsMu.Lock()
	delete(extraModelURLs, modelID)
	extraModelURLsMu.Unlock()

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok", "modelId": modelID})
}

func sendModelControl(ft localtunnel.FrameType, modelID, url string) error {
	session := getCurrentSession()
	if session == nil {
		return fmt.Errorf("not connected to TEE-Node")
	}

	stream, err := session.OpenStream(context.Background())
	if err != nil {
		return fmt.Errorf("open control stream: %w", err)
	}
	defer stream.Close()

	if err := localtunnel.WriteJSONFrame(stream, ft, localtunnel.ModelControlFrame{ModelID: modelID, BaseURL: url}); err != nil {
		return fmt.Errorf("write control frame: %w", err)
	}
	return readControlResponse(stream)
}

func readControlResponse(stream net.Conn) error {
	ft, payload, err := localtunnel.ReadTypedFrame(stream)
	if err != nil {
		return fmt.Errorf("read control response: %w", err)
	}

	switch ft {
	case localtunnel.FrameTypeHeader:
		var h localtunnel.HeaderFrame
		if err := json.Unmarshal(payload, &h); err != nil {
			return fmt.Errorf("decode header frame: %w", err)
		}
		if h.Status != 200 {
			return fmt.Errorf("control failed status=%d", h.Status)
		}
		ft2, _, err := localtunnel.ReadTypedFrame(stream)
		if err != nil {
			return fmt.Errorf("read done frame: %w", err)
		}
		if ft2 != localtunnel.FrameTypeDone {
			return fmt.Errorf("unexpected frame type %d", ft2)
		}
		return nil
	case localtunnel.FrameTypeError:
		var e localtunnel.ErrorFrame
		if err := json.Unmarshal(payload, &e); err != nil {
			return fmt.Errorf("decode error frame: %w", err)
		}
		return fmt.Errorf(e.Message)
	default:
		return fmt.Errorf("unexpected frame type %d", ft)
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
	if *adminAddr != "" {
		cfg.AdminAddr = *adminAddr
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
		Version:   localtunnel.ProtocolVersion,
		ModelID:   cfg.ModelID,
		APIKey:    cfg.APIKey,
		BaseURL:   cfg.LocalURL,
		Models:    registeredModels(cfg),
		ModelURLs: cloneExtraModelURLs(),
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
	setCurrentSession(session)
	defer func() {
		setCurrentSession(nil)
		session.Close()
	}()

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
			fmt.Printf("[LOCAL-CLIENT] max concurrency reached, rejecting stream (inflight=%d/%d)\n", len(sema), cfg.MaxConcurrency)
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

	// 根据请求中的模型 ID 路由到对应的本地服务地址
	modelID := req.ModelID
	if modelID == "" {
		modelID = cfg.ModelID
	}
	localURL := modelLocalURL(cfg, modelID)
	if localURL == "" {
		sendErrorFrame(stream, "no local URL configured for model "+modelID)
		return
	}
	localEndpoint := localURL + req.Path
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
	start := time.Now()
	resp, err := client.Do(upstreamReq)
	if err != nil {
		sendErrorFrame(stream, "upstream error: "+err.Error())
		fmt.Printf("[LOCAL-CLIENT] upstream error path=%s err=%v\n", req.Path, err)
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
	dataBytes := 0
	for {
		n, err := resp.Body.Read(buf)
		if n > 0 {
			dataBytes += n
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
	fmt.Printf("[LOCAL-CLIENT] request done path=%s status=%d bytes=%d latency=%v\n", req.Path, resp.StatusCode, dataBytes, time.Since(start))
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
	urls := allLocalURLs(cfg)
	if len(urls) == 0 {
		return
	}
	ticker := time.NewTicker(cfg.HealthCheckInterval)
	defer ticker.Stop()

	client := &http.Client{Timeout: 5 * time.Second}

	for {
		select {
		case <-ticker.C:
			for _, localURL := range urls {
				resp, err := client.Get(localURL + "/v1/models")
				if err != nil {
					fmt.Printf("[LOCAL-CLIENT] health check FAIL for %s: %v\n", localURL, err)
				} else {
					resp.Body.Close()
					if resp.StatusCode != 200 {
						fmt.Printf("[LOCAL-CLIENT] health check WARN for %s: status=%d\n", localURL, resp.StatusCode)
					}
				}
			}
		case <-ctx.Done():
			return
		}
	}
}

// localHopByHopHeaders 是 local-client 不应透传给 TEE-Node 的 headers。
// 包括 HTTP/1.1 hop-by-hop headers，以及 http.Client 自动解压后失效的 Content-Encoding/Content-Length。
var localHopByHopHeaders = map[string]bool{
	"Connection":          true,
	"Keep-Alive":          true,
	"Proxy-Authenticate":  true,
	"Proxy-Authorization": true,
	"Te":                  true,
	"Trailers":            true,
	"Transfer-Encoding":   true,
	"Upgrade":             true,
	"Content-Encoding":    true, // http.Client 可能已自动解压，该 header 失效
	"Content-Length":      true, // body 已被解压，需要重新计算，不能透传旧值
}

func headersToMap(h http.Header) map[string]string {
	m := make(map[string]string)
	for k, vs := range h {
		if localHopByHopHeaders[k] {
			continue
		}
		if len(vs) > 0 {
			m[k] = vs[0]
		}
	}
	return m
}
