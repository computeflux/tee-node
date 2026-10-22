package gateway

import (
	"context"
	"sync"
	"time"

	"github.com/computeflux/tee-provider/pkg/services/notify"
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
// 刷新时检测活跃模型集合变化，对上架/下架的模型推送通知给模型主人。
func getCachedGatewayModels() []cgateway.ModelInfoV2 {
	gatewayModelCache.mu.RLock()
	if gatewayModelCache.models != nil && time.Since(gatewayModelCache.lastRefresh) < gatewayModelCacheTTL {
		out := gatewayModelCache.models
		gatewayModelCache.mu.RUnlock()
		return out
	}
	gatewayModelCache.mu.RUnlock()

	models, nameMap, nameToID := fetchGatewayModelsFromContract()

	// 捕获旧基线（更新缓存前），供下架模型查 owner。
	prevNameMap := gatewayModelCache.nameMap

	// 检测活跃模型集合变化（上架/下架）。
	// 跳过条件：首次刷新（无对比基线）或合约拉取失败（nameMap=nil，避免误报全部下架）。
	var listed, delisted []string
	if prevNameMap != nil && nameMap != nil {
		prevActive := activeModelNames(prevNameMap)
		currActive := activeModelNames(nameMap)
		listed, delisted = diffModelSets(prevActive, currActive)
		if len(listed) > 0 || len(delisted) > 0 {
			GATEWAYLOG().Printf("[NOTIFY] model catalog changed: listed=%v delisted=%v (prev=%d curr=%d)",
				listed, delisted, len(prevActive), len(currActive))
		}
	} else {
		GATEWAYLOG().Printf("[NOTIFY] model catalog diff skipped: prevNameMap=%v currNameMap=%v (models=%d)",
			prevNameMap != nil, nameMap != nil, len(models))
	}

	gatewayModelCache.mu.Lock()
	gatewayModelCache.models = models
	gatewayModelCache.nameMap = nameMap
	gatewayModelCache.nameToID = nameToID
	gatewayModelCache.lastRefresh = time.Now()
	gatewayModelCache.mu.Unlock()

	// 异步推送上架/下架通知给模型主人（不阻塞缓存刷新）
	if len(listed) > 0 || len(delisted) > 0 {
		go pushModelCatalogChanges(prevNameMap, nameMap, listed, delisted)
	}

	return models
}

// activeModelNames 从 nameMap 提取活跃模型名集合。
func activeModelNames(nameMap map[string]cgateway.ModelInfoV2) map[string]bool {
	if nameMap == nil {
		return nil
	}
	out := make(map[string]bool, len(nameMap))
	for name := range nameMap {
		out[name] = true
	}
	return out
}

// diffModelSets 返回 (新增, 移除) 的模型名列表。
func diffModelSets(prev, curr map[string]bool) (added, removed []string) {
	for name := range curr {
		if !prev[name] {
			added = append(added, name)
		}
	}
	for name := range prev {
		if !curr[name] {
			removed = append(removed, name)
		}
	}
	return added, removed
}

// ── 后台定时刷新（无请求时也检测模型目录变更）──────────────────────

var modelRefreshOnce sync.Once

// StartModelCatalogWatcher 启动后台协程，每 10s 刷新模型缓存以检测上架/下架。
// 在 InitOptimizations 中调用，幂等（多次调用只启动一次）。
func StartModelCatalogWatcher() {
	modelRefreshOnce.Do(func() {
		go func() {
			// 启动后立即刷新一次（建立对比基线）
			getCachedGatewayModels()
			ticker := time.NewTicker(10 * time.Second)
			defer ticker.Stop()
			for range ticker.C {
				getCachedGatewayModels()
			}
		}()
		GATEWAYLOG().Printf("[NOTIFY] model catalog watcher started (interval=10s)")
	})
}

// pushModelCatalogChanges 推送模型上架/下架通知给模型主人。
// 上架模型的主人从 currNameMap 取（模型现在活跃），下架模型的主人从 prevNameMap 取（模型之前活跃）。
// owner 为空时跳过该模型（无法确定推送对象）。
func pushModelCatalogChanges(prevNameMap, currNameMap map[string]cgateway.ModelInfoV2, listed, delisted []string) {
	n := notify.NewFromEnv()
	if !n.Enabled() {
		GATEWAYLOG().Printf("[NOTIFY] pushModelCatalogChanges skipped: NOTIFY_ENABLED=false")
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	for _, name := range listed {
		wallet := notify.WalletFromAddr(currNameMap[name].Owner)
		if wallet == "" {
			GATEWAYLOG().Printf("[NOTIFY] model_listed model=%s skipped: empty owner", name)
			continue
		}
		GATEWAYLOG().Printf("[NOTIFY] pushing model_listed model=%s owner=%s activesURL=%s secretSet=%v",
			name, wallet, n.ActivesURL(), n.HasSecret())
		if err := n.Push(ctx, wallet, notify.EventTypeModelListed, map[string]string{"model": name}); err != nil {
			GATEWAYLOG().Printf("[NOTIFY] model_listed model=%s owner=%s failed: %v", name, wallet, err)
		} else {
			GATEWAYLOG().Printf("[NOTIFY] model_listed model=%s owner=%s OK", name, wallet)
		}
	}
	for _, name := range delisted {
		wallet := notify.WalletFromAddr(prevNameMap[name].Owner)
		if wallet == "" {
			GATEWAYLOG().Printf("[NOTIFY] model_delisted model=%s skipped: empty owner", name)
			continue
		}
		GATEWAYLOG().Printf("[NOTIFY] pushing model_delisted model=%s owner=%s activesURL=%s secretSet=%v",
			name, wallet, n.ActivesURL(), n.HasSecret())
		if err := n.Push(ctx, wallet, notify.EventTypeModelDelisted, map[string]string{"model": name}); err != nil {
			GATEWAYLOG().Printf("[NOTIFY] model_delisted model=%s owner=%s failed: %v", name, wallet, err)
		} else {
			GATEWAYLOG().Printf("[NOTIFY] model_delisted model=%s owner=%s OK", name, wallet)
		}
	}
}
