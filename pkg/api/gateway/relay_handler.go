package gateway

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/computeflux/tee-provider/pkg/api/gateway/protocol"
	"github.com/computeflux/tee-provider/pkg/indexer"
	relay "github.com/computeflux/tee-provider/pkg/services/gateway_relay"
	"github.com/computeflux/tee-provider/pkg/services/scheduler"
	"github.com/computeflux/tee-provider/pkg/types"
	cgateway "github.com/computeflux/tee-provider/tee-chain/runtime/module/gateway"
	"github.com/maximhq/bifrost/core/schemas"
)

func RegisterGatewayRoutes(r chi.Router) {
	common := func(r chi.Router) {
		r.Use(RecoveryMiddleware)
		r.Use(RequestIDMiddleware)
		r.Use(LoggerMiddleware)
		r.Use(GzipDecodeMiddleware)
		r.Use(RateLimitMiddleware(60, time.Minute))
		r.Use(CacheMiddleware)
	}

	// openai
	r.Route("/v1", func(r chi.Router) {
		common(r)
		r.Get("/models", HandleListModels)
		r.Group(func(r chi.Router) {
			r.Use(TokenAuthMiddleware)
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

		// OpenAI Responses API 入口
		r.Group(func(r chi.Router) {
			r.Use(protocol.OpenAIResponsesMiddleware)
			r.Use(TokenAuthMiddleware)
			r.Group(func(r chi.Router) {
				r.Use(DistributeMiddleware)
				r.Post("/responses", HandleRelay)
			})
		})
	})

	// anthropic
	r.Route("/anthropic", func(r chi.Router) {
		common(r)
		r.Group(func(r chi.Router) {
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
	gw := scheduler.GetGlobal()
	if gw == nil {
		writeProtocolError(w, r, http.StatusServiceUnavailable, "scheduler not ready", "server_error")
		return
	}

	// 逻辑模型/路由名：route 以 Edge/ MiningPool/ TEE/ 开头，其余必须显式 provider/model。
	modelNameArg := GetModelNameFromContext(r)
	var provider, modelKey string
	if isRouteName(modelNameArg) {
		provider = ""
		modelKey = modelNameArg
	} else {
		if !strings.Contains(modelNameArg, "/") {
			writeProtocolError(w, r, http.StatusBadRequest, "model must be provider/model or route", "invalid_request")
			return
		}
		provider, modelKey = parseProviderModel(modelNameArg)
	}
	bodyBytes := GetBodyFromContext(r)
	token := GetTokenFromContext(r)

	var reqBody struct {
		Stream bool `json:"stream"`
	}
	json.Unmarshal(bodyBytes, &reqBody)

	inbound := protocol.FromContext(r)
	requestPath := r.URL.Path
	if inbound == protocol.ProtocolAnthropic {
		requestPath = protocol.CanonicalAnthropicPath(requestPath)
	}

	// session 亲和:同一会话+模型优先复用上次端点(命中上游 KV cache)。
	// 优先级:请求体里的 session_id > 按入口协议解析首条消息哈希 > token.Owner。
	sid := extractSessionID(bodyBytes, inbound)
	sessionID := token.Owner
	if sid != "" {
		sessionID = token.Owner + ":" + sid
	}

	// 把入口协议 + 请求路径塞进 ctx,scheduler 选完端点后由 provider client 据此转换请求体。
	// 同时带上会话标识:发往 OpenCode 的请求会以 x-opencode-session 透出。
	ctx := scheduler.WithRequest(r.Context(), string(inbound), requestPath)
	ctx = relay.WithUpstreamSession(ctx, upstreamSessionID(token.Owner, sid))

	if reqBody.Stream {
		res, err := gw.ScheduleStream(ctx, provider, modelKey, bodyBytes, sessionID)
		if err != nil {
			st, msg, typ := mapScheduleError(err)
			writeProtocolError(w, r, st, msg, typ)
			GATEWAYLOG().Printf("[relay] stream schedule error model=%s: %v", modelKey, err)
			recordScheduleFailure(token, modelKey, err)
			return
		}
		ep := res.Meta.Endpoint

		// 判断是否还需转换：endpoint 原生支持入口协议则直接调用，否则转换到 endpoint 的原生协议
		inboundType := protocol.ToType(inbound)
		prov := inbound
		if !ep.SupportsProtocol(inboundType) {
			prov = protocol.ProviderNativeProtocol(ep.NativeProtocol())
		}

		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")

		// 流式响应可能跑数分钟（长思考 + 长 output）。http.Server.WriteTimeout 在所有 endpoint 上
		// 全局生效，到期会强制关连接并取消 r.Context()，表现就是 "stream aborted by client"。
		// 在写首个字节前用 ResponseController 把 per-request 写超时清零，
		// 让 http.Server.WriteTimeout 对本连接失效；生命周期完全交给 ctx 控制。
		// 注：ResponseController.SetWriteDeadline 必须在 WriteHeader 隐式触发之前调用；
		// 在它之前设 Header/WriteHeader 都仍受默认 deadline 保护。
		if err := http.NewResponseController(w).SetWriteDeadline(time.Time{}); err != nil {
			// 非致命:不支持就退回到服务级 WriteTimeout(本身已被我调到 600s)。
			// http.ErrNotSupported 表示底层连接不支持 per-request 写超时（如被中间件包装），
			// 这是预期内的退化路径，600s WriteTimeout 仍可兜底，不再刷噪音日志。
			if !errors.Is(err, http.ErrNotSupported) {
				GATEWAYLOG().Printf("[relay] stream: clear write deadline failed model=%s: %v", modelKey, err)
			}
		}

		usage, serr := writeRelayStream(ctx, w, res.Stream, inbound, prov)
		if serr != nil {
			// 客户端主动断连（context.Canceled）≠ provider 故障：上游已经发回了部分/全部响应字节，
			// 计费按已发出 usage 走；指标也不计入失败窗口，避免污染熔断统计。
			// 其他错误（含 context.DeadlineExceeded、transport 失败等）仍按失败处理。
			if errors.Is(serr, context.Canceled) {
				GATEWAYLOG().Printf("[relay] stream aborted by client model=%s provider=%d", modelKey, ep.ContractProviderID)
			} else {
				EmitProviderMetric(ep.ContractProviderID, false)
				GATEWAYLOG().Printf("[relay] stream error model=%s provider=%d: %v", modelKey, ep.ContractProviderID, serr)
				recordRelayFailure(token, ep)
			}
			recordRelayUsage(token, ep, usage)
			return
		}
		EmitProviderMetric(ep.ContractProviderID, true)
		recordRelayUsage(token, ep, usage)
		return
	}

	res, err := gw.Schedule(ctx, provider, modelKey, bodyBytes, sessionID)
	if err != nil {
		st, msg, typ := mapScheduleError(err)
		writeProtocolError(w, r, st, msg, typ)
		GATEWAYLOG().Printf("[relay] schedule error model=%s: %v", modelKey, err)
		recordScheduleFailure(token, modelKey, err)
		return
	}
	ep := res.Meta.Endpoint

	// 判断是否还需转换：endpoint 原生支持入口协议则直接调用，否则转换到 endpoint 的原生协议
	inboundType := protocol.ToType(inbound)
	prov := inbound
	if !ep.SupportsProtocol(inboundType) {
		prov = protocol.ProviderNativeProtocol(ep.NativeProtocol())
	}

	var (
		outBody []byte
		usage   *relay.Usage
	)
	if inbound == prov {
		outBody = res.Body
		usage = parseNativeUsage(res.Body, prov)
	} else {
		var conv error
		outBody, usage, conv = convertResponse(res.Body, inbound, prov, bodyBytes, ep.NativeProtocol())
		if conv != nil {
			EmitProviderMetric(ep.ContractProviderID, false)
			writeProtocolError(w, r, http.StatusInternalServerError, conv.Error(), "server_error")
			GATEWAYLOG().Printf("[relay] convert error model=%s provider=%d: %v", modelKey, ep.ContractProviderID, conv)
			recordRelayFailure(token, ep)
			return
		}
	}

	w.Header().Set("Content-Type", "application/json")
	if _, err := w.Write(outBody); err != nil {
		GATEWAYLOG().Printf("[relay] write response body error model=%s provider=%d: %v", modelKey, ep.ContractProviderID, err)
	}
	EmitProviderMetric(ep.ContractProviderID, true)
	recordRelayUsage(token, ep, usage)
}

// writeRelayStream 把 scheduler 返回的 provider-native 流写成 inbound 协议的 SSE。
//   - 同协议:逐行透传 + 尽力 sniff usage
//   - 跨协议:把 chunk 重组成 reader 喂给 streamConvert 转换
func writeRelayStream(ctx context.Context, w http.ResponseWriter, s scheduler.Stream, inbound, prov protocol.Protocol) (*relay.Usage, error) {
	flusher, _ := w.(http.Flusher)

	if inbound != prov {
		return streamConvert(newChunkReader(ctx, s), flushWriter{w: w, fl: flusher}, inbound, prov)
	}

	var usage *relay.Usage
	for {
		chunk, err := s.Next(ctx)
		if err != nil {
			return usage, err
		}
		if chunk != nil && len(chunk.Data) > 0 {
			if _, err := w.Write(chunk.Data); err != nil {
				return usage, err
			}
			if flusher != nil {
				flusher.Flush()
			}
			if u := sniffStreamLine(chunk.Data, prov); u != nil {
				usage = u
			}
		}
		if chunk != nil && chunk.IsFinal {
			return usage, nil
		}
	}
}

// sniffStreamLine 从一条 raw SSE 行(可能形如 "data: {...}\n")里提取 usage。
func sniffStreamLine(line []byte, prov protocol.Protocol) *relay.Usage {
	s := trimSSELine(line)
	if s == nil {
		return nil
	}
	return sniffStreamUsage(s, prov)
}

// trimSSELine 去掉行尾空白和可选的 "data:" 前缀,返回 payload(非 data 行返回 nil)。
func trimSSELine(line []byte) []byte {
	// 去尾部 \r\n 和空白
	end := len(line)
	for end > 0 && (line[end-1] == '\n' || line[end-1] == '\r' || line[end-1] == ' ') {
		end--
	}
	s := line[:end]
	if len(s) >= 5 && string(s[:5]) == "data:" {
		s = s[5:]
		for len(s) > 0 && s[0] == ' ' {
			s = s[1:]
		}
		return s
	}
	return nil
}

// recordRelayUsage 异步记录用量计费。Endpoint 已携带计费身份(ModelID/ContractProviderID)。
func recordRelayUsage(token *types.GatewayTokenInfo, ep *scheduler.Endpoint, usage *relay.Usage) {
	if usage == nil || token == nil || ep == nil {
		return
	}
	model := &cgateway.ModelInfoWithID{
		ID:   ep.ModelID,
		Data: cgateway.ModelInfoV2{ProviderID: ep.ContractProviderID, Name: ep.ModelName},
	}
	recordUsageAsyncFn(token.Addr, token.ID, model, ep.RouteID,
		uint64(usage.PromptTokens), uint64(usage.CompletionTokens), uint64(usage.PromptTokensDetails.CachedTokens))
}

// mapScheduleError 把 scheduler 错误映射为 HTTP 状态码 + 协议错误类型。
func mapScheduleError(err error) (status int, msg, errType string) {
	// 先处理带具体 HTTP status 的上游错误。
	// 注意:SchedError.Is 仅按 Class 匹配,ErrModelNotFound 也是 ClassClient,
	// 若先走 sentinel 分支会把所有 4xx 上游错误误判成 404,因此 Status!=0 优先。
	var se *scheduler.SchedError
	if errors.As(err, &se) && se.Status != 0 {
		typ := "upstream_error"
		switch {
		case se.Status == http.StatusTooManyRequests:
			typ = "rate_limit"
		case se.Status >= 400 && se.Status < 500:
			typ = "invalid_request"
		}
		return se.Status, se.Message, typ
	}
	switch {
	case errors.Is(err, scheduler.ErrModelNotFound):
		return http.StatusNotFound, "model not found", "not_found"
	case errors.Is(err, scheduler.ErrNoHealthyEndpoint), errors.Is(err, scheduler.ErrAllEndpointsDead):
		return http.StatusServiceUnavailable, "no available endpoint", "server_error"
	case errors.Is(err, scheduler.ErrCanceled):
		return http.StatusBadRequest, "request cancelled", "invalid_request"
	}
	return http.StatusBadGateway, err.Error(), "server_error"
}

func convertRequest(bodyBytes []byte, inbound, provider protocol.Protocol, modelName string, providerType uint16) ([]byte, error) {
	// 方向约束：Responses API → 其他格式不支持（不支持转出去）
	if inbound == protocol.ProtocolOpenAIResponses {
		return nil, fmt.Errorf("unsupported conversion: responses api -> %v", provider)
	}
	switch {
	case provider == protocol.ProtocolOpenAIResponses && inbound == protocol.ProtocolOpenAI:
		return protocol.ConvertOpenAIRequestToResponses(bodyBytes, providerType)
	case provider == protocol.ProtocolOpenAIResponses && inbound == protocol.ProtocolAnthropic:
		return protocol.ConvertAnthropicRequestToResponses(bodyBytes, providerType)
	case inbound == protocol.ProtocolAnthropic && provider == protocol.ProtocolOpenAI:
		return protocol.ConvertAnthropicRequestToOpenAI(bodyBytes, providerType)
	case inbound == protocol.ProtocolOpenAI && provider == protocol.ProtocolAnthropic:
		return protocol.ConvertOpenAIRequestToAnthropic(bodyBytes, providerType)
	case inbound == protocol.ProtocolOpenAI && provider == protocol.ProtocolGemini:
		return protocol.ConvertOpenAIRequestToGemini(bodyBytes, providerType)
	case inbound == protocol.ProtocolAnthropic && provider == protocol.ProtocolGemini:
		return protocol.ConvertAnthropicRequestToGemini(bodyBytes, providerType)
	}
	return bodyBytes, nil
}

func convertResponse(body []byte, inbound, provider protocol.Protocol, origBody []byte, providerType uint16) ([]byte, *relay.Usage, error) {
	// 方向约束：Responses API → 其他格式不支持（不支持转出去）
	if inbound == protocol.ProtocolOpenAIResponses {
		return nil, nil, fmt.Errorf("unsupported conversion: responses api -> %v", provider)
	}
	switch {
	case provider == protocol.ProtocolOpenAIResponses && inbound == protocol.ProtocolOpenAI:
		out, usage, err := protocol.ConvertResponsesResponseToOpenAI(body)
		if err != nil {
			return nil, nil, err
		}
		return out, bifrostUsageToRelay(usage), nil
	case provider == protocol.ProtocolOpenAIResponses && inbound == protocol.ProtocolAnthropic:
		out, usage, err := protocol.ConvertResponsesResponseToAnthropic(body)
		if err != nil {
			return nil, nil, err
		}
		return out, bifrostUsageToRelay(usage), nil
	case inbound == protocol.ProtocolAnthropic && provider == protocol.ProtocolOpenAI:
		out, usage, err := protocol.ConvertOpenAIResponseToAnthropic(body)
		if err != nil {
			return nil, nil, err
		}
		return out, bifrostUsageToRelay(usage), nil
	case inbound == protocol.ProtocolOpenAI && provider == protocol.ProtocolAnthropic:
		out, usage, err := protocol.ConvertAnthropicResponseToOpenAI(body)
		if err != nil {
			return nil, nil, err
		}
		return out, bifrostUsageToRelay(usage), nil
	case inbound == protocol.ProtocolOpenAI && provider == protocol.ProtocolGemini:
		out, usage, err := protocol.ConvertGeminiResponseToOpenAI(body)
		if err != nil {
			return nil, nil, err
		}
		return out, bifrostUsageToRelay(usage), nil
	case inbound == protocol.ProtocolAnthropic && provider == protocol.ProtocolGemini:
		out, usage, err := protocol.ConvertGeminiResponseToAnthropic(body)
		if err != nil {
			return nil, nil, err
		}
		return out, bifrostUsageToRelay(usage), nil
	default:
		return body, nil, fmt.Errorf("unsupported protocol pair: inbound=%v provider=%v", inbound, provider)
	}
}

// bifrostUsageToRelay 将 Bifrost 的 usage 转换为 relay 层的 usage。
// 注意：Bifrost 端的 CompletionTokensDetails.ReasoningTokens 等字段被故意忽略，
// 因为 relay.Usage 当前不携带这些信息。若后续计费/限流需要，应在 relay.Usage 中新增字段并在此补充映射。
func bifrostUsageToRelay(u *schemas.BifrostLLMUsage) *relay.Usage {
	if u == nil {
		return nil
	}
	cachedTokens := 0
	if u.PromptTokensDetails != nil {
		cachedTokens = u.PromptTokensDetails.CachedReadTokens
	}
	return &relay.Usage{
		PromptTokens:     u.PromptTokens,
		CompletionTokens: u.CompletionTokens,
		TotalTokens:      u.TotalTokens,
		PromptTokensDetails: relay.PromptTokensDetails{
			CachedTokens: cachedTokens,
		},
	}
}

func streamConvert(r io.Reader, w io.Writer, inbound, provider protocol.Protocol) (*relay.Usage, error) {
	// 方向约束：Responses API → 其他格式不支持；Responses API 方向的流式转换暂未实现
	if inbound == protocol.ProtocolOpenAIResponses || provider == protocol.ProtocolOpenAIResponses {
		return nil, fmt.Errorf("stream conversion with responses api not supported (inbound=%v provider=%v)", inbound, provider)
	}
	switch {
	case inbound == protocol.ProtocolAnthropic && provider == protocol.ProtocolOpenAI:
		usage, err := protocol.StreamOpenAIResponseToAnthropic(r, w)
		if err != nil {
			return nil, err
		}
		return bifrostUsageToRelay(usage), nil
	case inbound == protocol.ProtocolOpenAI && provider == protocol.ProtocolAnthropic:
		usage, err := protocol.StreamAnthropicResponseToOpenAI(r, w)
		if err != nil {
			return nil, err
		}
		return bifrostUsageToRelay(usage), nil
	case inbound == protocol.ProtocolOpenAI && provider == protocol.ProtocolGemini:
		usage, err := protocol.StreamGeminiResponseToOpenAI(r, w)
		if err != nil {
			return nil, err
		}
		return bifrostUsageToRelay(usage), nil
	case inbound == protocol.ProtocolAnthropic && provider == protocol.ProtocolGemini:
		usage, err := protocol.StreamGeminiResponseToAnthropic(r, w)
		if err != nil {
			return nil, err
		}
		return bifrostUsageToRelay(usage), nil
	default:
		return nil, fmt.Errorf("unsupported stream protocol pair: inbound=%v provider=%v", inbound, provider)
	}
}

// upstreamSessionID 计算发往 OpenCode(x-opencode-session)的会话标识。
// 优先用请求体解析出的真实会话 ID;解析不到时退化为 owner 的不可逆哈希——
// 保持稳定且不把用户身份(邮箱/地址)直接发给上游。始终非空。
func upstreamSessionID(owner, sid string) string {
	if sid != "" {
		return sid
	}
	h := sha256.Sum256([]byte("opencode-owner:" + owner))
	return "acct-" + hex.EncodeToString(h[:16])
}

// extractSessionID 按入口协议解析请求体,提取会话 ID。
// 约定:客户端统一传 session_id(唯一标准字段,不猜别名);
// 没传就用首条消息哈希(同一对话稳定)。
//   - openai:    messages[0].content
//   - anthropic: messages[0].content(string 或 blocks[].text)
//   - gemini:    contents[0].parts[].text
func extractSessionID(body []byte, p protocol.Protocol) string {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(body, &raw); err != nil {
		return ""
	}

	// 1. 客户端显式传的会话 ID(兼容常见 agent 的不同字段名)
	for _, k := range []string{"session_id", "sessionId", "session", "conversation_id", "conversationId", "conversation", "thread_id", "threadId"} {
		if v, ok := raw[k]; ok {
			var s string
			if json.Unmarshal(v, &s) == nil && s != "" {
				return s
			}
		}
	}

	// 1b. 也可能放在 metadata 里(部分 SDK 约定,如 Anthropic metadata.user_id)
	if meta, ok := raw["metadata"]; ok {
		var m map[string]string
		if json.Unmarshal(meta, &m) == nil {
			for _, k := range []string{"session_id", "sessionId", "conversation_id", "conversationId", "thread_id", "threadId", "user_id"} {
				if s := m[k]; s != "" {
					return s
				}
			}
		}
	}

	// 2. 按入口协议解析首条消息
	var firstMsg string
	switch p {
	case protocol.ProtocolAnthropic:
		if msgs, ok := raw["messages"]; ok {
			var arr []struct {
				Content any `json:"content"`
			}
			if json.Unmarshal(msgs, &arr) == nil && len(arr) > 0 {
				firstMsg = messageContentText(arr[0].Content)
			}
		}
	case protocol.ProtocolGemini:
		if contents, ok := raw["contents"]; ok {
			var arr []struct {
				Parts []struct {
					Text string `json:"text"`
				} `json:"parts"`
			}
			if json.Unmarshal(contents, &arr) == nil && len(arr) > 0 {
				for _, part := range arr[0].Parts {
					firstMsg += part.Text
				}
			}
		}
	default: // openai
		if msgs, ok := raw["messages"]; ok {
			var arr []struct {
				Content any `json:"content"`
			}
			if json.Unmarshal(msgs, &arr) == nil && len(arr) > 0 {
				firstMsg = messageContentText(arr[0].Content)
			}
		}
	}
	if firstMsg != "" {
		h := sha256.Sum256([]byte(firstMsg))
		return "msg:" + hex.EncodeToString(h[:16])
	}
	return ""
}

// messageContentText 把不同形态的 message content 提取为文本。
// string → 原样;数组(多模态 blocks) → 拼接 text 字段。
func messageContentText(content any) string {
	switch c := content.(type) {
	case string:
		return c
	case []any:
		var sb strings.Builder
		for _, part := range c {
			if m, ok := part.(map[string]any); ok {
				if t, ok := m["text"].(string); ok {
					sb.WriteString(t)
				}
			}
		}
		return sb.String()
	}
	return ""
}

// HandleListModels 直接读索引器返回可调用模型列表（与 GraphQL Models 同一数据路径），
// 模型 id 为 providerName/modelName，可直接用于后续请求。
func HandleListModels(w http.ResponseWriter, r *http.Request) {
	idx := indexer.GetGlobal()
	if idx == nil {
		writeProtocolError(w, r, http.StatusServiceUnavailable, "models index not ready", "server_error")
		return
	}

	entries, err := idx.ListModels(r.Context(), indexer.GatewayModelSearchRequest{Offset: 0, Limit: 100000})
	if err != nil {
		writeProtocolError(w, r, http.StatusInternalServerError, "list models failed", "server_error")
		return
	}

	models := make([]map[string]any, 0, len(entries))
	for _, e := range entries {
		providerName := e.Provider.Name
		if providerName == "" {
			providerName = fmt.Sprintf("provider-%x", e.Provider.Owner)
		}
		models = append(models, map[string]any{
			"id":       providerName + "/" + e.Model.Name,
			"object":   "model",
			"created":  e.Model.CreatedAt,
			"owned_by": fmt.Sprintf("%x", e.Model.Owner.V),
		})
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{"object": "list", "data": models})
}
