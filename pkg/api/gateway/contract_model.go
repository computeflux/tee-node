package gateway

import (
	"sync"
	"time"

	teechain "github.com/computeflux/tee-provider/tee-chain"
	contracts "github.com/computeflux/tee-provider/tee-chain/runtime"
	cgateway "github.com/computeflux/tee-provider/tee-chain/runtime/module/gateway"
)

// ── Model cache ──────────────────────────────────────────

var gatewayModelCache = struct {
	mu          sync.RWMutex
	models      []cgateway.ModelInfoV2
	nameMap     map[string]cgateway.ModelInfoV2 // Name → active model，O(1) 查询
	nameToID    map[string]uint64               // Name → on-chain model ID
	lastRefresh time.Time
}{}

const gatewayModelCacheTTL = 5 * time.Second

// fetchGatewayModelsFromContract 从合约拉取活跃模型列表，同时构建 nameMap。
func fetchGatewayModelsFromContract() ([]cgateway.ModelInfoV2, map[string]cgateway.ModelInfoV2, map[string]uint64) {
	bridge := getBridge()
	if bridge == nil {
		return nil, nil, nil
	}

	models, err := teechain.WithRuntime(bridge, func(runtime *contracts.Runtime) ([]cgateway.ModelInfoWithID, error) {
		return cgateway.GatewayQuery{Gateway: *cgateway.NewGateway(runtime)}.ListActiveModels(0, 1000)
	})
	if err != nil {
		GATEWAYLOG().Printf("[GATEWAY] contract list models error: %v", err)
		return nil, nil, nil
	}

	out := make([]cgateway.ModelInfoV2, 0, len(models))
	nameMap := make(map[string]cgateway.ModelInfoV2, len(models))
	nameToID := make(map[string]uint64, len(models))
	for _, m := range models {
		if m.Data.IsActive {
			nameMap[m.Data.Name] = m.Data
			nameToID[m.Data.Name] = m.ID
		}
		out = append(out, m.Data)
	}
	return out, nameMap, nameToID
}

// getCachedGatewayModels 返回缓存的合约模型列表。
func getCachedGatewayModels() []cgateway.ModelInfoV2 {
	gatewayModelCache.mu.RLock()
	if gatewayModelCache.models != nil && time.Since(gatewayModelCache.lastRefresh) < gatewayModelCacheTTL {
		out := gatewayModelCache.models
		gatewayModelCache.mu.RUnlock()
		return out
	}
	gatewayModelCache.mu.RUnlock()

	models, nameMap, nameToID := fetchGatewayModelsFromContract()

	gatewayModelCache.mu.Lock()
	gatewayModelCache.models = models
	gatewayModelCache.nameMap = nameMap
	gatewayModelCache.nameToID = nameToID
	gatewayModelCache.lastRefresh = time.Now()
	gatewayModelCache.mu.Unlock()
	return models
}
