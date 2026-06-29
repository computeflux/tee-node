package gateway

import (
	"context"
	"time"

	"github.com/computeflux/tee-provider/pkg/api/gateway/protocol"
	relay "github.com/computeflux/tee-provider/pkg/services/gateway_relay"
	"github.com/computeflux/tee-provider/pkg/services/scheduler"
	teechain "github.com/computeflux/tee-provider/tee-chain"
	cgateway "github.com/computeflux/tee-provider/tee-chain/runtime/module/gateway"
)

// InitScheduler 创建并启动 LLM 路由调度器，把它接到 gateway 的请求链路上。
// 应在合约桥(SetContractBridge)注入之后调用。
//
// 职责：
//   - 注册固定的协议 adaptor（openai/anthropic/gemini/personal）
//   - 注入请求协议转换 hook（选完端点后按端点协议转换请求体 + 解析上游路径）
//   - 注入链上桥 getter 给 reloader
//   - 设为全局单例供 HandleRelay 使用
//   - 启动熔断评估 + 路由表热更新
func InitScheduler(ctx context.Context, chain *teechain.TeeChain) error {
	gw, err := scheduler.NewGatewayScheduler()
	if err != nil {
		return err
	}

	// 协议 adaptor 命名必须与 reloader.providerTypeName 输出一致。
	gw.RegisterProviders(map[string]scheduler.ProviderClient{
		"openai":    relay.GetProviderClient(cgateway.ProviderOpenAI),
		"anthropic": relay.GetProviderClient(cgateway.ProviderClaude),
		"gemini":    relay.GetProviderClient(cgateway.ProviderGemini),
		"personal":  relay.GetProviderClient(cgateway.ProviderPersonal),
	})

	scheduler.SetRequestTransform(schedulerRequestTransform)
	scheduler.SetBridgeGetter(func() *teechain.TeeChain { return chain })
	scheduler.SetGlobal(gw)

	gw.Start(ctx)
	go scheduler.NewReloader(gw, 30*time.Second).Run(ctx)
	return nil
}

// schedulerRequestTransform 把入口协议请求体转换为目标 provider 协议请求体，并解析上游路径。
// providerProtoName 形参未用——用 providerType 推导原生协议更稳妥
// （ProviderPersonal 的协议名是 "personal"，但其原生协议是 OpenAI 兼容）。
func schedulerRequestTransform(inbound, _ string, reqPath string, providerType uint16, body []byte) ([]byte, string, error) {
	in := protocol.Protocol(inbound)
	prov := protocol.ProviderNativeProtocol(providerType)

	upstreamPath, err := protocol.ResolveUpstreamPath(in, prov, reqPath)
	if err != nil {
		return nil, "", err
	}
	if in == prov {
		return body, upstreamPath, nil
	}
	converted, err := convertRequest(body, in, prov, "", providerType)
	if err != nil {
		return nil, "", err
	}
	return converted, upstreamPath, nil
}
