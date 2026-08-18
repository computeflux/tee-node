package scheduler

import (
	"context"
	"fmt"
	"sync"
)

// 协议转换在 scheduler 内部、选完端点之后发生——因为目标协议取决于被选中端点的
// Protocol（同一逻辑模型的不同端点可能是 openai/anthropic/gemini 不同协议）。
//
// 但具体的转换逻辑（Anthropic↔OpenAI↔Gemini）位于 pkg/api/gateway 层，
// scheduler 作为更底层的服务不能反向 import gateway（会形成循环）。
// 因此这里用「函数注入 + ctx 携带每请求信息」的方式解耦：
//   - gateway 启动时调用 SetRequestTransform 注入转换实现
//   - HandleRelay 进入 Schedule 前用 WithRequest 把入口协议 + 请求路径写进 ctx
//   - provider client(gateway_relay)在真正发请求前调用 ResolveRequest，按
//     ep.Protocol 转换请求体并解析上游路径

// RequestTransformFunc 把「入口协议的请求体 + 请求路径」转换为「目标 provider 协议的
// 请求体 + 上游路径」。inbound == providerProto 时应原样返回（仅解析路径）。
// 返回 error 表示该 (inbound, providerProto) 组合不支持转换。
type RequestTransformFunc func(
	inbound, providerProto string,
	reqPath string,
	providerType uint16,
	body []byte,
) (outBody []byte, upstreamPath string, err error)

var (
	requestTransform   RequestTransformFunc
	requestTransformMu sync.RWMutex
)

// SetRequestTransform 注入请求转换实现（gateway 层在启动时调用一次）。
func SetRequestTransform(f RequestTransformFunc) {
	requestTransformMu.Lock()
	requestTransform = f
	requestTransformMu.Unlock()
}

func loadRequestTransform() RequestTransformFunc {
	requestTransformMu.RLock()
	defer requestTransformMu.RUnlock()
	return requestTransform
}

// reqInfo 是每请求的转换上下文（入口协议 + 原始请求路径）。
type reqInfo struct {
	Inbound string // 入口协议："openai" / "anthropic" / "gemini"
	Path    string // 入口请求路径（已归一化），如 "/v1/chat/completions"
}

type reqInfoKey struct{}

// WithRequest 把入口协议 + 请求路径写入 ctx，供 provider client 调用 ResolveRequest 时读取。
func WithRequest(ctx context.Context, inbound, path string) context.Context {
	if inbound == "" {
		panic("scheduler.WithRequest: inbound protocol must not be empty")
	}
	return context.WithValue(ctx, reqInfoKey{}, reqInfo{Inbound: inbound, Path: path})
}

func reqInfoFrom(ctx context.Context) (reqInfo, bool) {
	v := ctx.Value(reqInfoKey{})
	if v == nil {
		return reqInfo{}, false
	}
	ri, ok := v.(reqInfo)
	return ri, ok
}

// ResolveRequest 根据 ctx 中的入口信息和端点协议，决定发往上游的 body 与路径。
// 由 provider client 实现(gateway_relay)在真正发请求前调用。
func ResolveRequest(ctx context.Context, ep *Endpoint, body []byte) (outBody []byte, upstreamPath string, err error) {
	if ep == nil {
		return nil, "", fmt.Errorf("scheduler: nil endpoint passed to ResolveRequest")
	}
	if loadRequestTransform() == nil {
		return nil, "", fmt.Errorf("scheduler: request transform not injected (gateway not initialized?)")
	}
	ri, ok := reqInfoFrom(ctx)
	if !ok || ri.Inbound == "" {
		return nil, "", fmt.Errorf("scheduler: inbound protocol missing in ctx")
	}
	providerProto := ClientTypeName(ep.NativeProtocol())
	outBody, upstreamPath, err = loadRequestTransform()(ri.Inbound, providerProto, ri.Path, ep.NativeProtocol(), body)
	if err != nil {
		return nil, "", fmt.Errorf("scheduler: transform inbound=%s providerProto=%s path=%s: %w", ri.Inbound, providerProto, ri.Path, err)
	}
	return outBody, upstreamPath, nil
}
