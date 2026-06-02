package gateway

import (
	"encoding/hex"
	"fmt"
	"sync"
	"time"

	"github.com/computeflux/tee-provider/pkg/model"
	teechain "github.com/computeflux/tee-provider/tee-chain"
	"github.com/computeflux/tee-provider/tee-chain/contracts"
	cgateway "github.com/computeflux/tee-provider/tee-chain/contracts/module/gateway"
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
func LookupAPIKeyViaContract(key string) *model.GatewayTokenInfo {
	bridge := getBridge()
	hash := hashAPIKey(key)
	hexHash := hex.EncodeToString(hash)

	if bridge == nil {
		return nil
	}

	info, err := GatewayLookupAPIKey(bridge, hash)
	if err != nil {
		return nil
	}

	if info == nil {
		GATEWAYLOG().Printf("[GATEWAY] contract lookup returned nil for hexHash=%s", hexHash)
		return nil
	}

	return &model.GatewayTokenInfo{
		Owner:     fmt.Sprintf("%x", info.Owner.V),
		Addr:      info.Owner,
		Key:       key,
		RateLimit: info.RateLimit,
		IsActive:  info.IsActive,
	}
}

// lookupAPIKeyViaContract 是内部使用的别名。
func lookupAPIKeyViaContract(key string) *model.GatewayTokenInfo {
	return LookupAPIKeyViaContract(key)
}

// providerDefaultURL 返回各供应商的默认官方地址。
func providerDefaultURL(providerType uint16) string {
	switch providerType {
	case 1:
		return "https://api.openai.com"
	case 2:
		return "https://api.anthropic.com"
	case 3:
		return "https://generativelanguage.googleapis.com"
	default:
		return "https://api.openai.com"
	}
}

// ── Model cache ──────────────────────────────────────────
var gatewayModelCache = struct {
	mu          sync.RWMutex
	models      []cgateway.ModelInfo
	nameMap     map[string]cgateway.ModelInfo // Name → active model，O(1) 查询
	lastRefresh time.Time
}{}

const gatewayModelCacheTTL = 5 * time.Second

// fetchGatewayModelsFromContract 从合约拉取活跃模型列表，同时构建 nameMap。
func fetchGatewayModelsFromContract() ([]cgateway.ModelInfo, map[string]cgateway.ModelInfo) {
	bridge := getBridge()
	if bridge == nil {
		return nil, nil
	}

	models, err := GatewayListActiveModels(bridge, 0, 1000)
	if err != nil {
		GATEWAYLOG().Printf("[GATEWAY] contract list models error: %v", err)
		return nil, nil
	}

	out := make([]cgateway.ModelInfo, len(models))
	copy(out, models)

	nameMap := make(map[string]cgateway.ModelInfo, len(models))
	for _, m := range out {
		if m.IsActive {
			nameMap[m.Name] = m
		}
	}
	return out, nameMap
}

// getCachedGatewayModels 返回缓存的合约模型列表。
func getCachedGatewayModels() []cgateway.ModelInfo {
	gatewayModelCache.mu.RLock()
	if gatewayModelCache.models != nil && time.Since(gatewayModelCache.lastRefresh) < gatewayModelCacheTTL {
		out := gatewayModelCache.models
		gatewayModelCache.mu.RUnlock()
		return out
	}
	gatewayModelCache.mu.RUnlock()

	models, nameMap := fetchGatewayModelsFromContract()

	gatewayModelCache.mu.Lock()
	gatewayModelCache.models = models
	gatewayModelCache.nameMap = nameMap
	gatewayModelCache.lastRefresh = time.Now()
	gatewayModelCache.mu.Unlock()
	return models
}

// findGatewayModelByName 按模型名称从合约查询（必须活跃），TEE 内部使用，返回完整 API Key。
func findGatewayModelByName(pid uint64, name string) *cgateway.ModelInfoWithID {
	bridge := getBridge()
	if bridge == nil {
		return nil
	}
	model, err := GatewaySecretGetModelByName(bridge, pid, name)
	if err != nil {
		GATEWAYLOG().Printf("[GATEWAY] SecretGetModelByName name=%s err=%v", name, err)
		return nil
	}

	return model
}

func findProviderName(name string) *cgateway.ProviderInfoWithID {
	bridge := getBridge()
	if bridge == nil {
		return nil
	}
	provider, err := GatewayGetProviderByName(bridge, name)
	if err != nil {
		GATEWAYLOG().Printf("[GATEWAY] GetProviderByName name=%s err=%v", name, err)
		return nil
	}
	return provider
}

// buildEndpointsFromModel 根据合约 ModelInfo 构建 GatewayEndpoint 列表
func buildEndpointsFromModel(m *cgateway.ModelInfo) []*model.GatewayEndpoint {
	url := m.URI
	if url == "" {
		url = providerDefaultURL(m.ProviderType)
	}

	if len(m.APIKeys) == 0 {
		return nil
	}

	eps := make([]*model.GatewayEndpoint, 0, len(m.APIKeys))
	for i, apiKey := range m.APIKeys {
		if apiKey == "" {
			continue
		}
		eps = append(eps, &model.GatewayEndpoint{
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

// ── Gateway 合约查询（从 tee-chain/contract.go 拆分出来）──────

// contractRuntime 构造合约运行时上下文（本包内部使用）。
func contractRuntime(chain *teechain.TeeChain) (*contracts.Runtime, error) {
	return chain.ContractRuntime()
}

// GatewayLookupAPIKey 根据 keyHash 查询链上 API Key 信息。
func GatewayLookupAPIKey(chain *teechain.TeeChain, keyHash []byte) (*cgateway.APIKeyInfo, error) {
	runtime, err := contractRuntime(chain)
	if err != nil {
		return nil, err
	}
	defer runtime.GetTxn().Rollback()

	return cgateway.GatewayQuery{Gateway: *cgateway.NewGateway(runtime)}.LookupAPIKey(keyHash)
}

// GatewayListActiveModels 查询链上活跃模型列表。
func GatewayListActiveModels(chain *teechain.TeeChain, offset uint64, limit uint32) ([]cgateway.ModelInfo, error) {
	runtime, err := contractRuntime(chain)
	if err != nil {
		return nil, err
	}
	defer runtime.GetTxn().Rollback()

	return cgateway.GatewayQuery{Gateway: *cgateway.NewGateway(runtime)}.ListActiveModels(offset, limit)
}

// GatewayListActiveProviders 查询链上活跃 Provider 列表。
func GatewayListActiveProviders(chain *teechain.TeeChain, offset uint64, limit uint32) ([]cgateway.ProviderInfo, error) {
	runtime, err := contractRuntime(chain)
	if err != nil {
		return nil, err
	}
	defer runtime.GetTxn().Rollback()

	return cgateway.GatewayQuery{Gateway: *cgateway.NewGateway(runtime)}.ListActiveProviders(offset, limit)
}

// GatewayGetProvider 查询单个 Provider。
func GatewayGetProvider(chain *teechain.TeeChain, id uint64) (*cgateway.ProviderInfo, error) {
	runtime, err := contractRuntime(chain)
	if err != nil {
		return nil, err
	}
	defer runtime.GetTxn().Rollback()

	return cgateway.GatewayQuery{Gateway: *cgateway.NewGateway(runtime)}.GetProvider(id)
}

// GatewayGetModel 查询单个 Model。
func GatewayGetModel(chain *teechain.TeeChain, id uint64) (*cgateway.ModelInfo, error) {
	runtime, err := contractRuntime(chain)
	if err != nil {
		return nil, err
	}
	defer runtime.GetTxn().Rollback()

	return cgateway.GatewayQuery{Gateway: *cgateway.NewGateway(runtime)}.GetModel(id)
}

// GatewayGetModelByName 按名称查询活跃模型（走 string exact 索引，O(1) 查询）。
func GatewayGetModelByName(chain *teechain.TeeChain, pid uint64, name string) (*cgateway.ModelInfoWithID, error) {
	runtime, err := contractRuntime(chain)
	if err != nil {
		return nil, err
	}
	defer runtime.GetTxn().Rollback()

	return cgateway.GatewayQuery{Gateway: *cgateway.NewGateway(runtime)}.GetModelByName(pid, name)
}

// GatewaySecretGetModel 查询单个 Model（TEE 内部，不脱敏）。
func GatewaySecretGetModel(chain *teechain.TeeChain, id uint64) (*cgateway.ModelInfo, error) {
	runtime, err := contractRuntime(chain)
	if err != nil {
		return nil, err
	}
	defer runtime.GetTxn().Rollback()

	return cgateway.GatewaySecret{Gateway: *cgateway.NewGateway(runtime)}.GetModel(id)
}

// GatewaySecretGetModelByName 按名称查询模型（TEE 内部，不脱敏）。
func GatewaySecretGetModelByName(chain *teechain.TeeChain, pid uint64, name string) (*cgateway.ModelInfoWithID, error) {
	runtime, err := contractRuntime(chain)
	if err != nil {
		return nil, err
	}
	defer runtime.GetTxn().Rollback()

	return cgateway.GatewaySecret{Gateway: *cgateway.NewGateway(runtime)}.GetModelByName(pid, name)
}

// GatewaySecretListActiveModels 查询活跃模型列表（TEE 内部，不脱敏）。
func GatewaySecretListActiveModels(chain *teechain.TeeChain, offset uint64, limit uint32) ([]cgateway.ModelInfo, error) {
	runtime, err := contractRuntime(chain)
	if err != nil {
		return nil, err
	}
	defer runtime.GetTxn().Rollback()

	return cgateway.GatewaySecret{Gateway: *cgateway.NewGateway(runtime)}.ListActiveModels(offset, limit)
}

// GatewayGetProviderByName 按名称查询 Provider（走 string exact 索引，O(1) 查询）。
func GatewayGetProviderByName(chain *teechain.TeeChain, name string) (*cgateway.ProviderInfoWithID, error) {
	runtime, err := contractRuntime(chain)
	if err != nil {
		return nil, err
	}
	defer runtime.GetTxn().Rollback()

	return cgateway.GatewayQuery{Gateway: *cgateway.NewGateway(runtime)}.GetProviderByName(name)
}

// GatewayGetCredit 查询用户剩余积分（credits - credit_useds）。
func GatewayGetCredit(chain *teechain.TeeChain, user model.UniAddr) (int64, error) {
	runtime, err := contractRuntime(chain)
	if err != nil {
		return 0, err
	}
	defer runtime.GetTxn().Rollback()

	return cgateway.GatewayQuery{Gateway: *cgateway.NewGateway(runtime)}.GetCredit(user)
}

// GatewayGetRechargeHistory 查询用户充值历史。
func GatewayGetRechargeHistory(chain *teechain.TeeChain, user model.UniAddr, offset uint64, limit uint32) ([]cgateway.RechargeRecord, error) {
	runtime, err := contractRuntime(chain)
	if err != nil {
		return nil, err
	}
	defer runtime.GetTxn().Rollback()

	return cgateway.GatewayQuery{Gateway: *cgateway.NewGateway(runtime)}.GetRechargeHistory(user, offset, limit)
}
