package gateway

import (
	"encoding/hex"
	"fmt"
	"sync"

	"github.com/computeflux/tee-provider/pkg/types"
	teechain "github.com/computeflux/tee-provider/tee-chain"
	contracts "github.com/computeflux/tee-provider/tee-chain/runtime"
	cgateway "github.com/computeflux/tee-provider/tee-chain/runtime/module/gateway"
	"golang.org/x/crypto/blake2b"
)

// contractBridge 是 TeeChain 的合约查询桥接
var contractBridge struct {
	chain *teechain.TeeChain
	mu    sync.RWMutex
}

// SetContractBridge 设置合约查询桥（由 graph/mod.go 在启动时注入）。
func SetContractBridge(chain *teechain.TeeChain) {
	contractBridge.mu.Lock()
	contractBridge.chain = chain
	contractBridge.mu.Unlock()
}

func getBridge() *teechain.TeeChain {
	contractBridge.mu.RLock()
	defer contractBridge.mu.RUnlock()
	return contractBridge.chain
}

// hashAPIKey 对明文 API Key 做 blake2b-256 哈希。
func hashAPIKey(key string) []byte {
	h := blake2b.Sum256([]byte(key))
	return h[:]
}

// LookupAPIKeyViaContract 通过合约查询 API Key 信息（exported for graph/auth_helpers.go）。
func LookupAPIKeyViaContract(key string) *types.GatewayTokenInfo {
	bridge := getBridge()
	hash := hashAPIKey(key)
	hexHash := hex.EncodeToString(hash)

	if bridge == nil {
		return nil
	}

	info, err := teechain.WithRuntime(bridge, func(runtime *contracts.Runtime) (*cgateway.APIKeyInfo, error) {
		return cgateway.GatewayQuery{Gateway: *cgateway.NewGateway(runtime)}.LookupAPIKey(hash)
	})
	if err != nil {
		return nil
	}

	if info == nil {
		GATEWAYLOG().Printf("[GATEWAY] contract lookup returned nil for hexHash=%s", hexHash)
		return nil
	}

	return &types.GatewayTokenInfo{
		Owner:     fmt.Sprintf("%x", info.Owner.V),
		Addr:      info.Owner,
		Key:       key,
		RateLimit: info.RateLimit,
		IsActive:  info.IsActive,
	}
}

// providerDefaultURL 返回各供应商的默认官方地址。
func providerDefaultURL(providerType uint16) string {
	switch providerType {
	case cgateway.ProviderOpenAI:
		return "https://api.openai.com"
	case cgateway.ProviderClaude:
		return "https://api.anthropic.com"
	case cgateway.ProviderGemini:
		return "https://generativelanguage.googleapis.com"
	case cgateway.ProviderPersonal:
		return "local://yamux" // 本地模型通过反向隧道转发，不需要真实 URL
	default:
		return ""
	}
}

// buildEndpointsFromModel 根据合约 ModelInfo 构建 GatewayEndpoint 列表。
func buildEndpointsFromModel(m *cgateway.ModelInfo) []*types.GatewayEndpoint {
	url := m.URI
	if url == "" {
		url = providerDefaultURL(m.ProviderType)
	}

	// 本地模型使用 Yamux 反向隧道，不需要 APIKeys 也不依赖 URI
	if m.ProviderType == cgateway.ProviderPersonal {
		return []*types.GatewayEndpoint{{
			ID:         m.ProviderID,
			ProviderID: m.ProviderID,
			URL:        url,
			APIKey:     "",
			Weight:     1,
			IsActive:   true,
		}}
	}

	if len(m.APIKeys) == 0 {
		return nil
	}

	eps := make([]*types.GatewayEndpoint, 0, len(m.APIKeys))
	for i, apiKey := range m.APIKeys {
		if apiKey == "" {
			continue
		}
		eps = append(eps, &types.GatewayEndpoint{
			ID:         uint64(i) + 1,
			ProviderID: m.ProviderID,
			URL:        url,
			APIKey:     apiKey,
			Weight:     1,
			IsActive:   true,
		})
	}
	return eps
}

// buildEndpointsFromRoute 从路由中收集所有模型的 endpoint，用于路由模式的均匀分配。
func buildEndpointsFromRoute(routeID uint64) []*types.GatewayEndpoint {
	bridge := getBridge()
	if bridge == nil {
		return nil
	}

	route, err := teechain.WithRuntime(bridge, func(runtime *contracts.Runtime) (*cgateway.ModelRoute, error) {
		return cgateway.GatewayQuery{Gateway: *cgateway.NewGateway(runtime)}.GetRoute(routeID)
	})
	if err != nil || route == nil || !route.IsActive {
		return nil
	}

	// 收集路由中所有模型的 endpoint
	var allEndpoints []*types.GatewayEndpoint
	models, err := teechain.WithRuntime(bridge, func(runtime *contracts.Runtime) ([]*cgateway.ModelInfo, error) {
		gateway := cgateway.NewGateway(runtime)
		result := make([]*cgateway.ModelInfo, 0, len(route.ModelIds))
		for _, modelID := range route.ModelIds {
			model, err := cgateway.GatewaySecret{Gateway: *gateway}.GetModel(modelID)
			if err != nil || model == nil || !model.IsActive {
				if err != nil {
					GATEWAYLOG().Printf("[ROUTE] buildEndpointsFromRoute: fetch model %d failed: %v", modelID, err)
				}
				continue
			}
			result = append(result, model)
		}
		return result, nil
	})
	if err != nil {
		return nil
	}
	for _, model := range models {
		eps := buildEndpointsFromModel(model)
		allEndpoints = append(allEndpoints, eps...)
	}
	return allEndpoints
}

// FindProviderByModelAPIKey 根据模型名和 API Key 查找对应的本地模型供应商 ID。
// 流程：
//  1. 用 apiKey 找到对应的用户 token，得到用户地址 Addr。
//  2. 以该用户地址为 caller 创建合约 runtime，调用 GetProviderByOwner 获取 ProviderID。
//  3. 检查该 ProviderID 下是否存在名为 modelName 的活跃本地模型。
//
// local-client 传的就是用户登录后的 session token（也是调用网关的 Authorization Bearer key）。
func FindProviderByModelAPIKey(modelName, apiKey string) (uint64, bool) {
	bridge := getBridge()
	if bridge == nil {
		GATEWAYLOG().Printf("[LOCAL-TUNNEL] FindProviderByModelAPIKey: no contract bridge")
		return 0, false
	}

	// 1. 查 token（先缓存，再合约）
	token := FindTokenByKey(apiKey)
	if token == nil {
		token = LookupAPIKeyViaContract(apiKey)
	}
	if token == nil {
		GATEWAYLOG().Printf("[LOCAL-TUNNEL] apiKey=%s token not found", truncateKey(apiKey))
		return 0, false
	}
	if !token.IsActive || token.IsExpired() {
		GATEWAYLOG().Printf("[LOCAL-TUNNEL] apiKey=%s token inactive or expired", truncateKey(apiKey))
		return 0, false
	}

	// 2. 以 token.Addr 为 caller 查询 ProviderID
	runtime, err := bridge.ContractRuntimeWithCaller(token.Addr)
	if err != nil {
		GATEWAYLOG().Printf("[LOCAL-TUNNEL] contract runtime error: %v", err)
		return 0, false
	}
	defer runtime.GetTxn().Rollback()

	query := cgateway.GatewayQuery{Gateway: *cgateway.NewGateway(runtime)}
	opt, err := query.GetProviderByOwner()
	if err != nil {
		GATEWAYLOG().Printf("[LOCAL-TUNNEL] GetProviderByOwner error: %v", err)
		return 0, false
	}
	if opt.IsNone() {
		GATEWAYLOG().Printf("[LOCAL-TUNNEL] apiKey=%s has no provider", truncateKey(apiKey))
		return 0, false
	}
	providerID, err := opt.UnWrap()
	if err != nil {
		GATEWAYLOG().Printf("[LOCAL-TUNNEL] unwrap providerID error: %v", err)
		return 0, false
	}

	// 3. 检查该 provider 下是否有对应活跃本地模型
	model, err := query.GetModelByName(providerID, modelName)
	if err != nil {
		GATEWAYLOG().Printf("[LOCAL-TUNNEL] GetModelByName provider=%d model=%s error: %v", providerID, modelName, err)
		return 0, false
	}
	if model == nil || !model.Data.IsActive {
		GATEWAYLOG().Printf("[LOCAL-TUNNEL] model=%s not found or inactive under provider=%d", modelName, providerID)
		return 0, false
	}
	if model.Data.ProviderType != cgateway.ProviderPersonal {
		GATEWAYLOG().Printf("[LOCAL-TUNNEL] model=%s provider=%d providerType=%d is not local", modelName, providerID, model.Data.ProviderType)
		return 0, false
	}

	GATEWAYLOG().Printf("[LOCAL-TUNNEL] model=%s matched provider=%d", modelName, providerID)
	return providerID, true
}

func truncateKey(key string) string {
	if len(key) <= 8 {
		return key
	}
	return key[:8] + "..."
}
