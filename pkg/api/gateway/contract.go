package gateway

import (
	"encoding/hex"
	"fmt"
	"sync"
	"time"

	"github.com/computeflux/tee-provider/pkg/chaintypes"
	"github.com/computeflux/tee-provider/pkg/store"
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
func LookupAPIKeyViaContract(key string) *store.GatewayTokenInfo {
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

	return &store.GatewayTokenInfo{
		Owner:     fmt.Sprintf("%x", info.Owner.V),
		Addr:      info.Owner,
		Key:       key,
		RateLimit: info.RateLimit,
		IsActive:  info.IsActive,
	}
}

// lookupAPIKeyViaContract 是内部使用的别名。
func lookupAPIKeyViaContract(key string) *store.GatewayTokenInfo {
	return LookupAPIKeyViaContract(key)
}

// providerDefaultURL 返回各供应商的默认官方地址。
func providerDefaultURL(providerType uint16) string {
	switch providerType {
	case 1: // openai
		return "https://api.openai.com"
	case 2: // anthropic
		return "https://api.anthropic.com"
	case 3: // googleapis
		return "https://generativelanguage.googleapis.com"
	case 9, 9999: // local
		return "local://yamux" // 本地模型通过反向隧道转发，不需要真实 URL
	default:
		return ""
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

// FindProviderByModelAPIKey 根据模型名和 API Key 查找对应的本地模型供应商 ID。
// 流程：
//   1. 用 apiKey 找到对应的用户 token，得到用户地址 Addr。
//   2. 以该用户地址为 caller 创建合约 runtime，调用 GetProviderByOwner 获取 ProviderID。
//   3. 检查该 ProviderID 下是否存在名为 modelName 的活跃本地模型。
// local-client 传的就是用户登录后的 session token（也是调用网关的 Authorization Bearer key）。
func FindProviderByModelAPIKey(modelName, apiKey string) (uint64, bool) {
	bridge := getBridge()
	if bridge == nil {
		GATEWAYLOG().Printf("[LOCAL-TUNNEL] FindProviderByModelAPIKey: no contract bridge")
		return 0, false
	}

	// 1. 查 token（先缓存，再合约）
	token := store.FindGatewayTokenByKey(apiKey)
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
	if model.Data.ProviderType != cgateway.ProviderLocal && model.Data.ProviderType != 9999 {
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
func buildEndpointsFromModel(m *cgateway.ModelInfo) []*store.GatewayEndpoint {
	url := m.URI
	if url == "" {
		url = providerDefaultURL(m.ProviderType)
	}

	// 本地模型使用 Yamux 反向隧道，不需要 APIKeys 也不依赖 URI
	if m.ProviderType == 9999 || m.ProviderType == 9 {
		return []*store.GatewayEndpoint{{
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

	eps := make([]*store.GatewayEndpoint, 0, len(m.APIKeys))
	for i, apiKey := range m.APIKeys {
		if apiKey == "" {
			continue
		}
		eps = append(eps, &store.GatewayEndpoint{
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
func GatewayGetCredit(chain *teechain.TeeChain, user chaintypes.UniAddr) (int64, error) {
	runtime, err := contractRuntime(chain)
	if err != nil {
		return 0, err
	}
	defer runtime.GetTxn().Rollback()

	return cgateway.GatewayQuery{Gateway: *cgateway.NewGateway(runtime)}.GetCredit(user)
}

// ── Testable function pointers ───────────────────────────

var (
	findProviderNameFn       func(string) *cgateway.ProviderInfoWithID                   = findProviderName
	findGatewayModelByNameFn func(uint64, string) *cgateway.ModelInfoWithID              = findGatewayModelByName
	getCachedGatewayModelsFn func() []cgateway.ModelInfo                                 = getCachedGatewayModels
	getBridgeFn              func() *teechain.TeeChain                                   = getBridge
	gatewayGetCreditFn       func(*teechain.TeeChain, chaintypes.UniAddr) (int64, error) = GatewayGetCredit
)

// GatewayGetRechargeHistory 查询用户充值历史。
func GatewayGetRechargeHistory(chain *teechain.TeeChain, user chaintypes.UniAddr, offset uint64, limit uint32) ([]cgateway.RechargeRecord, error) {
	runtime, err := contractRuntime(chain)
	if err != nil {
		return nil, err
	}
	defer runtime.GetTxn().Rollback()

	return cgateway.GatewayQuery{Gateway: *cgateway.NewGateway(runtime)}.GetRechargeHistory(user, offset, limit)
}
