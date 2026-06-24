package gateway

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime/debug"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/computeflux/tee-provider/pkg/api/gateway/protocol"
	relay "github.com/computeflux/tee-provider/pkg/services/gateway_relay"
	"github.com/computeflux/tee-provider/pkg/store"
	cgateway "github.com/computeflux/tee-provider/tee-chain/contracts/module/gateway"
	"github.com/maximhq/bifrost/core/schemas"
)

// dumpOpenAIRequestBodyIfNeeded 把带有 tools 的 Anthropic->OpenAI 转换后请求体写入
// 带时间戳的文件，避免被后续无 tool 请求覆盖。优先写入 /token_chain_data，
// 不可写时回退到当前目录。
func dumpOpenAIRequestBodyIfNeeded(body []byte) string {
	var req struct {
		Tools []json.RawMessage `json:"tools"`
	}
	if err := json.Unmarshal(body, &req); err != nil || len(req.Tools) == 0 {
		return ""
	}
	candidates := []string{"/token_chain_data", "."}
	for _, dir := range candidates {
		if err := os.MkdirAll(dir, 0755); err != nil {
			continue
		}
		path := filepath.Join(dir, fmt.Sprintf("gateway_openai_request_%d.json", time.Now().UnixMilli()))
		if err := os.WriteFile(path, body, 0644); err == nil {
			return path
		}
	}
	return ""
}

func RegisterGatewayRoutes(r chi.Router) {
	common := func(r chi.Router) {
		r.Use(RecoveryMiddleware)
		r.Use(RequestIDMiddleware)
		r.Use(LoggerMiddleware)
		r.Use(GzipDecodeMiddleware)
		r.Use(RateLimitMiddleware(60, time.Minute))
		r.Use(CacheMiddleware)
	}

	// OpenAI 兼容入口：/v1/*
	r.Route("/v1", func(r chi.Router) {
		common(r)

		// 公开端点（模型列表）
		r.Get("/models", HandleListModels)

		// 需要 Token 认证的端点
		r.Group(func(r chi.Router) {
			r.Use(TokenAuthMiddleware)

			// AI 调用（需要 DistributeMiddleware 做模型路由）
			r.Group(func(r chi.Router) {
				r.Use(DistributeMiddleware)
				r.Post("/chat/completions", HandleRelay)
				r.Post("/completions", HandleRelay)
				r.Post("/embeddings", HandleRelay)
				r.Post("/images/generations", HandleRelay)
				r.Post("/audio/speech", HandleRelay)
				r.Post("/audio/transcriptions", HandleRelay)
			})
		})
	})

	// Anthropic 原生入口：/anthropic/*
	r.Route("/anthropic", func(r chi.Router) {
		common(r)

		r.Group(func(r chi.Router) {
			// AnthropicMiddleware 放在 TokenAuthMiddleware 前面，
			// 这样鉴权失败时也能按 Anthropic 格式返回错误。
			r.Use(protocol.AnthropicMiddleware)
			r.Use(TokenAuthMiddleware)
			r.Group(func(r chi.Router) {
				r.Use(DistributeMiddleware)
				r.Post("/v1/messages", HandleRelay)
				r.Post("/v1/messages/count_tokens", HandleRelay)
				r.Post("/v1/complete", HandleRelay)
			})
		})
	})
}

func HandleRelay(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	endpoint := GetEndpointFromContext(r)
	modelData := GetModelFromContext(r)
	if endpoint == nil || modelData == nil {
		writeProtocolError(w, r, http.StatusServiceUnavailable, "no route", "server_error")
		return
	}

	inboundProto := protocol.FromContext(r)
	// util.DebugLog("[relay] %s %s inbound=%s providerType=%d model=%s endpoint=%s", r.Method, r.URL.Path, inboundProto, modelData.Data.ProviderType, modelData.Data.Name, endpoint.URL)

	// Endpoint 级别限流 — 这才是真正的瓶颈
	// 优先使用合约中的 ProviderType 做限流分类，本地模型不再被误判为 openai。
	providerTypeStr := providerTypeToLimiterKey(modelData.Data.ProviderType)
	rpmLimit, maxConc := GetProviderLimits(providerTypeStr)
	lim := getEndpointLimiter(endpoint.ID, rpmLimit)
	lim.SetLimits(rpmLimit, maxConc)
	if !lim.Allow() {
		rpm, inf := lim.Stats()
		writeProtocolError(w, r, http.StatusTooManyRequests,
			fmt.Sprintf("rate limited: %d/%d RPM, %d/%d inflight", rpm, rpmLimit, inf, maxConc),
			"rate_limit")
		return
	}
	defer lim.Done()

	// Use cached body from DistributeMiddleware if available, otherwise read.
	bodyBytes := GetBodyFromContext(r)

	var reqBody struct {
		Model  string `json:"model"`
		Stream bool   `json:"stream"`
	}
	json.Unmarshal(bodyBytes, &reqBody)

	providerProto := providerNativeProtocol(modelData.Data.ProviderType)

	// 把 /anthropic/* 归一化为 /v1/*，便于后续做 provider 协议判断和 upstream 路径映射。
	requestPath := r.URL.Path
	if inboundProto == protocol.ProtocolAnthropic {
		requestPath = canonicalAnthropicPath(requestPath)
	}

	upstreamPath, pathErr := resolveUpstreamPath(inboundProto, providerProto, requestPath)
	if pathErr != nil {
		writeProtocolError(w, r, http.StatusNotImplemented, pathErr.Error(), "not_implemented")
		return
	}

	relayR := r.Clone(r.Context())
	relayR.URL.Path = upstreamPath

	var (
		requestConverter  func([]byte, uint16) ([]byte, error)
		responseConverter func([]byte, uint16) ([]byte, error)
		streamRelay       func(http.ResponseWriter, *http.Request, *store.GatewayEndpoint, string, []byte, uint16, bool) (*relay.Usage, *relay.ErrorWithStatusCode, error)
	)

	// Anthropic -> OpenAI 转换时，提前提取原始请求里的 tool schemas，
	// 用于在响应转换阶段过滤上游模型生成的非法 tool 参数。
	var toolSchemas []schemas.ChatTool
	if inboundProto == protocol.ProtocolAnthropic {
		toolSchemas = protocol.ExtractBifrostChatTools(bodyBytes, modelData.Data.ProviderType)
		toolSchemas = protocol.HardenToolParameters(toolSchemas, modelData.Data.Name)
	}

	// Anthropic -> OpenAI 路径的上游不是 Anthropic 原生模型，返回的 reasoning/thinking
	// 没有 Anthropic 要求的 signature，不能直接以 thinking 块转发给客户端，所以统一丢弃。
	dropThinking := inboundProto == protocol.ProtocolAnthropic && providerProto == protocol.ProtocolOpenAI

	switch {
	case inboundProto == providerProto:
		// 协议一致，不需要转换。
	case inboundProto == protocol.ProtocolAnthropic && providerProto == protocol.ProtocolOpenAI:
		requestConverter = func(body []byte, pt uint16) ([]byte, error) {
			return protocol.ConvertAnthropicRequestToOpenAI(body, pt, toolSchemas...)
		}
		responseConverter = func(body []byte, _ uint16) ([]byte, error) {
			return protocol.ConvertOpenAIResponseToAnthropic(body, dropThinking, toolSchemas...)
		}
		streamRelay = func(w http.ResponseWriter, r *http.Request, endpoint *store.GatewayEndpoint, modelName string, bodyBytes []byte, providerType uint16, dropThinking bool) (*relay.Usage, *relay.ErrorWithStatusCode, error) {
			return relayAnthropicStream(w, r, endpoint, modelName, bodyBytes, providerType, dropThinking, toolSchemas)
		}
	case inboundProto == protocol.ProtocolOpenAI && providerProto == protocol.ProtocolAnthropic:
		requestConverter = protocol.ConvertOpenAIRequestToAnthropic
		responseConverter = func(body []byte, _ uint16) ([]byte, error) { return protocol.ConvertAnthropicResponseToOpenAI(body) }
		streamRelay = relayOpenAIStreamFromAnthropic
	case inboundProto == protocol.ProtocolOpenAI && providerProto == protocol.ProtocolGemini:
		requestConverter = protocol.ConvertOpenAIRequestToGemini
		responseConverter = func(body []byte, _ uint16) ([]byte, error) { return protocol.ConvertGeminiResponseToOpenAI(body) }
		streamRelay = relayGeminiStream
	}

	var usage *relay.Usage
	var bizErr *relay.ErrorWithStatusCode
	var err error

	if requestConverter != nil {
		converted, convErr := func() (converted []byte, err error) {
			defer func() {
				if rec := recover(); rec != nil {
					err = fmt.Errorf("panic during request conversion: %v\n%s", rec, debug.Stack())
				}
			}()
			return requestConverter(bodyBytes, modelData.Data.ProviderType)
		}()
		if convErr != nil {
			EmitProviderMetric(modelData.Data.ProviderID, false)
			writeProtocolError(w, r, http.StatusBadRequest, convErr.Error(), "invalid_request")
			return
		}
		bodyBytes = converted

		// Anthropic -> OpenAI 路径仅在请求带 tools 时落盘/记录，防止被后续无 tool 请求覆盖。
		if inboundProto == protocol.ProtocolAnthropic && providerProto == protocol.ProtocolOpenAI {
			if dumpPath := dumpOpenAIRequestBodyIfNeeded(bodyBytes); dumpPath != "" {
				// 同时把转换后的 OpenAI 请求体完整写入 gateway.log，便于直接分析。
				GATEWAYLOG().Printf("[OPENAI-REQUEST-BODY] %s", string(bodyBytes))
			}
		}
	}

	// 协议转换后可能改变 stream 标志，优先从实际转发给上游的请求体重新解析；
	// 若转换后的请求体没有显式声明 stream，则保留原始请求的 stream 标志（Gemini 等请求体不含 stream 字段）。
	isStream := reqBody.Stream
	if requestConverter != nil {
		var convertedReq struct {
			Stream *bool `json:"stream,omitempty"`
		}
		json.Unmarshal(bodyBytes, &convertedReq)
		if convertedReq.Stream != nil {
			isStream = *convertedReq.Stream
		}
	}

	if isStream && streamRelay != nil {
		// util.DebugLog("[relay] streaming via %s", providerTypeToLimiterKey(modelData.Data.ProviderType))
		usage, bizErr, err = streamRelay(w, relayR, endpoint, modelData.Data.Name, bodyBytes, modelData.Data.ProviderType, dropThinking)
	} else {
		// util.DebugLog("[relay] non-stream via %s", providerTypeToLimiterKey(modelData.Data.ProviderType))
		var origW http.ResponseWriter
		var recorder *protocol.ResponseRecorder
		if responseConverter != nil {
			origW = w
			recorder = protocol.NewResponseRecorder(w)
			w = recorder
		}

		usage, bizErr, err = relayToUpstreamFn(w, relayR, endpoint, modelData.Data.Name, bodyBytes, isStream, modelData.Data.ProviderType)

		// 出错时恢复原始 ResponseWriter，避免错误写到 recorder 里发不出去。
		if (err != nil || bizErr != nil) && origW != nil {
			w = origW
		}

		if err == nil && bizErr == nil && responseConverter != nil && recorder != nil {
			convertedResp, convErr := func() (converted []byte, err error) {
				defer func() {
					if rec := recover(); rec != nil {
						err = fmt.Errorf("panic during response conversion: %v\n%s", rec, debug.Stack())
					}
				}()
				return responseConverter(recorder.Body(), 0)
			}()
			if convErr != nil {
				EmitProviderMetric(modelData.Data.ProviderID, false)
				writeProtocolError(origW, r, http.StatusInternalServerError, convErr.Error(), "server_error")
				return
			}
			recorder.Header().Set("Content-Type", "application/json")
			recorder.Header().Del("Content-Length")
			recorder.FlushStatus()
			if _, writeErr := recorder.ResponseWriter().Write(convertedResp); writeErr != nil {
				GATEWAYLOG().Printf("[relay] failed to write converted response: %v", writeErr)
			}
		}
	}

	if bizErr != nil {
		EmitProviderMetric(modelData.Data.ProviderID, false)
		writeProtocolError(w, r, bizErr.StatusCode, bizErr.Error.Message, "upstream_error")
		return
	}
	if err != nil {
		EmitProviderMetric(modelData.Data.ProviderID, false)
		writeProtocolError(w, r, http.StatusBadGateway, err.Error(), "server_error")
		return
	}

	// 统计调用成功率和使用量
	EmitProviderMetric(modelData.Data.ProviderID, true)

	// 统计用户 Token 使用
	token := GetTokenFromContext(r)
	if usage != nil {
		recordUsageAsyncFn(
			token.Addr,
			token.ID,
			modelData,
			uint64(usage.PromptTokens),     // 总输入 token
			uint64(usage.CompletionTokens), // 输出 token
			uint64(usage.PromptTokensDetails.CachedTokens), // 命中缓存的输入 token
		)
	}

	// 记录调用时间
	elapsed := time.Since(start)
	tkns := 0
	if usage != nil {
		tkns = usage.TotalTokens
	}
	GATEWAYLOG().Printf("%s %s -> %s (%d tokens, %v)", r.Method, modelData.Data.Name, endpoint.URL, tkns, elapsed)
}

// relayToUpstreamFn 可被测试替换为 mock。
var relayToUpstreamFn = func(w http.ResponseWriter, r *http.Request, endpoint *store.GatewayEndpoint, modelName string, bodyBytes []byte, isStream bool, providerType uint16) (*relay.Usage, *relay.ErrorWithStatusCode, error) {
	return relayToUpstream(w, r, endpoint, modelName, bodyBytes, isStream, providerType)
}

func relayToUpstream(w http.ResponseWriter, r *http.Request, endpoint *store.GatewayEndpoint, modelName string, bodyBytes []byte, isStream bool, providerType uint16) (*relay.Usage, *relay.ErrorWithStatusCode, error) {
	ctx := relay.NewRelayContext(w, r)

	meta := relay.GetMetaByContext(ctx)
	meta.BaseURL = endpoint.URL
	meta.APIKey = endpoint.APIKey
	meta.IsStream = isStream
	meta.OriginModelName = modelName
	meta.ProviderId = endpoint.ProviderID
	meta.ProviderType = providerType

	// 优先根据合约中的 ProviderType 选择 Adaptor，避免通过 URL 字符串猜测。
	// 本地模型地址（如 http://192.168.x.x）会被 DetectProvider 误判为 openai。
	adaptor := relay.GetAdaptorForProviderTypeCode(providerType)
	adaptor.Init(meta)

	// util.DebugLog("[relay] upstream request provider=%s url=%s body=%d stream=%v", adaptor.GetProviderName(), r.URL.String(), len(bodyBytes), isStream)
	resp, err := adaptor.DoRequest(ctx, meta, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, nil, fmt.Errorf("upstream: %w", err)
	}
	// util.DebugLog("[relay] upstream response status=%d content-type=%s", resp.StatusCode, resp.Header.Get("Content-Type"))

	usage, bizErr := adaptor.DoResponse(ctx, resp, meta)
	// 立即关闭 body，避免 defer 在函数返回时才执行可能引入的并发时序问题
	if resp != nil && resp.Body != nil {
		resp.Body.Close()
	}
	if bizErr != nil {
		return nil, bizErr, nil
	}
	// util.DebugLog("[relay] upstream response handled usage=%+v", usage)
	return usage, nil, nil
}

type streamConverter func(io.Reader, io.Writer, bool, []schemas.ChatTool) (*schemas.BifrostLLMUsage, error)

// relayStream 把已转好的请求体发给上游，并把返回的 SSE 流通过 converter 实时转换后写给客户端。
func relayStream(w http.ResponseWriter, r *http.Request, endpoint *store.GatewayEndpoint, modelName string, bodyBytes []byte, providerType uint16, convert streamConverter, logLabel string, dropThinking bool, toolSchemas []schemas.ChatTool) (*relay.Usage, *relay.ErrorWithStatusCode, error) {
	ctx := relay.NewRelayContext(w, r)

	meta := relay.GetMetaByContext(ctx)
	meta.BaseURL = endpoint.URL
	meta.APIKey = endpoint.APIKey
	meta.IsStream = true
	meta.OriginModelName = modelName
	meta.ProviderId = endpoint.ProviderID
	meta.ProviderType = providerType

	adaptor := relay.GetAdaptorForProviderTypeCode(providerType)
	adaptor.Init(meta)

	resp, err := adaptor.DoRequest(ctx, meta, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, nil, fmt.Errorf("upstream: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, readErr := io.ReadAll(resp.Body)
		if readErr != nil {
			GATEWAYLOG().Printf("[MONITOR] %s failed to read upstream error body: %v", logLabel, readErr)
		}
		return nil, &relay.ErrorWithStatusCode{
			StatusCode: resp.StatusCode,
			Error: relay.ErrorDetail{
				Message: string(body),
				Type:    "upstream_error",
			},
		}, nil
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	// 注意：第一个 SSE 事件写入时会隐式提交 HTTP 200。如果 convert 在写出数据后失败，
	// 响应已提交，外层不应再尝试写入错误响应。

	// 落盘原始 SSE，便于排查模型实际返回。
	dumpFile, _ := dumpGatewayStreamResponse(logLabel)
	if dumpFile != nil {
		defer dumpFile.Close()
		resp.Body = &teeReadCloser{Reader: io.TeeReader(resp.Body, dumpFile), Closer: resp.Body}
	}

	// 落盘转换后的 Anthropic SSE，便于调试对比 OpenAI 输出和最终 Anthropic 返回。
	anthropicDumpFile, _ := dumpGatewayAnthropicOutput(logLabel)
	var outWriter io.Writer = w
	if anthropicDumpFile != nil {
		defer anthropicDumpFile.Close()
		outWriter = &anthropicOutputDumper{w: w, dump: anthropicDumpFile}
	}

	bifrostUsage, err := convert(resp.Body, outWriter, dropThinking, toolSchemas)
	if err != nil {
		GATEWAYLOG().Printf("[MONITOR] %s stream convert error: %v", logLabel, err)
		// 流已启动时无法安全地返回 HTTP 错误，直接结束。
		return nil, nil, nil
	}

	if bifrostUsage != nil {
		return &relay.Usage{
			PromptTokens:     bifrostUsage.PromptTokens,
			CompletionTokens: bifrostUsage.CompletionTokens,
			TotalTokens:      bifrostUsage.TotalTokens,
		}, nil, nil
	}
	return nil, nil, nil
}

// anthropicOutputDumper 同时把 Anthropic SSE 输出写入 HTTP ResponseWriter 和落盘文件，
// 同时实现 http.Flusher 以支持 SSE 的实时刷新。
type anthropicOutputDumper struct {
	w    http.ResponseWriter
	dump *os.File
}

func (d *anthropicOutputDumper) Write(p []byte) (int, error) {
	if d.dump != nil {
		d.dump.Write(p)
	}
	return d.w.Write(p)
}

func (d *anthropicOutputDumper) Flush() {
	if f, ok := d.w.(http.Flusher); ok {
		f.Flush()
	}
}

// dumpGatewayAnthropicOutput 为转换后的 Anthropic SSE 输出创建落盘文件。
func dumpGatewayAnthropicOutput(logLabel string) (*os.File, error) {
	candidates := []string{"/token_chain_data", "."}
	for _, dir := range candidates {
		if err := os.MkdirAll(dir, 0755); err != nil {
			continue
		}
		path := filepath.Join(dir, fmt.Sprintf("gateway_anthropic_output_%s_%d.sse", logLabel, time.Now().UnixMilli()))
		f, err := os.Create(path)
		if err == nil {
			return f, nil
		}
	}
	return nil, nil
}

// teeReadCloser 把 io.Reader 和 io.Closer 组合成 io.ReadCloser，用于在读取时
// 通过 io.TeeReader 把原始 SSE 同步写入调试文件。
type teeReadCloser struct {
	io.Reader
	io.Closer
}

// dumpGatewayStreamResponse 为原始 SSE 流创建落盘文件。
func dumpGatewayStreamResponse(logLabel string) (*os.File, string) {
	candidates := []string{"/token_chain_data", "."}
	for _, dir := range candidates {
		if err := os.MkdirAll(dir, 0755); err != nil {
			continue
		}
		path := filepath.Join(dir, fmt.Sprintf("gateway_stream_response_%s_%d.sse", logLabel, time.Now().UnixMilli()))
		f, err := os.Create(path)
		if err == nil {
			return f, path
		}
	}
	return nil, ""
}

// relayAnthropicStream 发送 OpenAI 流式请求并实时把 SSE 转换为 Anthropic 格式。
func relayAnthropicStream(w http.ResponseWriter, r *http.Request, endpoint *store.GatewayEndpoint, modelName string, bodyBytes []byte, providerType uint16, dropThinking bool, toolSchemas []schemas.ChatTool) (*relay.Usage, *relay.ErrorWithStatusCode, error) {
	convert := func(reader io.Reader, writer io.Writer, dropThinking bool, schemas []schemas.ChatTool) (*schemas.BifrostLLMUsage, error) {
		return protocol.StreamOpenAIResponseToAnthropic(reader, writer, dropThinking, schemas...)
	}
	return relayStream(w, r, endpoint, modelName, bodyBytes, providerType, convert, "anthropic", dropThinking, toolSchemas)
}

// relayOpenAIStreamFromAnthropic 发送 Anthropic 流式请求并实时把 SSE 转换为 OpenAI 格式。
func relayOpenAIStreamFromAnthropic(w http.ResponseWriter, r *http.Request, endpoint *store.GatewayEndpoint, modelName string, bodyBytes []byte, providerType uint16, _ bool) (*relay.Usage, *relay.ErrorWithStatusCode, error) {
	convert := func(reader io.Reader, writer io.Writer, _ bool, _ []schemas.ChatTool) (*schemas.BifrostLLMUsage, error) {
		return protocol.StreamAnthropicResponseToOpenAI(reader, writer)
	}
	return relayStream(w, r, endpoint, modelName, bodyBytes, providerType, convert, "openai", false, nil)
}

// relayGeminiStream 发送 Gemini 流式请求并实时把 SSE 转换为 OpenAI 格式。
func relayGeminiStream(w http.ResponseWriter, r *http.Request, endpoint *store.GatewayEndpoint, modelName string, bodyBytes []byte, providerType uint16, _ bool) (*relay.Usage, *relay.ErrorWithStatusCode, error) {
	convert := func(reader io.Reader, writer io.Writer, _ bool, _ []schemas.ChatTool) (*schemas.BifrostLLMUsage, error) {
		return protocol.StreamGeminiResponseToOpenAI(reader, writer)
	}
	return relayStream(w, r, endpoint, modelName, bodyBytes, providerType, convert, "gemini", false, nil)
}

// providerTypeToLimiterKey 将合约 ProviderType 数值映射为限流配置中的字符串 key。
func providerTypeToLimiterKey(providerType uint16) string {
	switch providerType {
	case cgateway.ProviderClaude:
		return "anthropic"
	case cgateway.ProviderGemini:
		return "gemini"
	case cgateway.ProviderPersonal:
		return "local"
	default:
		return "openai"
	}
}

func HandleListModels(w http.ResponseWriter, r *http.Request) {
	models := make([]map[string]any, 0)

	// 从合约读取模型列表
	for _, m := range getCachedGatewayModelsFn() {
		models = append(models, map[string]any{
			"id": m.Name, "object": "model", "owned_by": fmt.Sprintf("%x", m.Owner.V),
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{"object": "list", "data": models})
}
