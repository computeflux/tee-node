package gateway

import (
	"encoding/hex"
	"fmt"
	"sync"
	"time"

	"github.com/computeflux/tee-provider/pkg/model"
	teechain "github.com/computeflux/tee-provider/tee-chain"
	cgateway "github.com/computeflux/tee-provider/tee-chain/contracts/gateway"
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
	info, err := bridge.GatewayLookupAPIKey(hash)
	if err != nil {
		return nil
	}
	if info == nil {
		GATEWAYLOG().Printf("[GATEWAY] contract lookup returned nil for hexHash=%s", hexHash)
		return nil
	}
	return &model.GatewayTokenInfo{
		Owner:     fmt.Sprintf("%x", info.Owner.V),
		Key:       key,
		RateLimit: info.RateLimit,
		IsActive:  info.IsActive,
	}
}

// lookupAPIKeyViaContract 是内部使用的别名。
func lookupAPIKeyViaContract(key string) *model.GatewayTokenInfo {
	return LookupAPIKeyViaContract(key)
}

// providerTypeName 根据 ProviderType 返回名称。
func providerTypeName(providerType uint16) string {
	switch providerType {
	case 1:
		return "openai"
	case 2:
		return "anthropic"
	case 3:
		return "gemini"
	default:
		return "openai"
	}
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
	lastRefresh time.Time
}{}

const gatewayModelCacheTTL = 5 * time.Second

// fetchGatewayModelsFromContract 从合约拉取活跃模型列表。
func fetchGatewayModelsFromContract() []cgateway.ModelInfo {
	bridge := getBridge()
	if bridge == nil {
		return nil
	}
	models, err := bridge.GatewayListActiveModels(0, 1000)
	if err != nil {
		GATEWAYLOG().Printf("[GATEWAY] contract list models error: %v", err)
		return nil
	}
	out := make([]cgateway.ModelInfo, len(models))
	copy(out, models)
	return out
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

	models := fetchGatewayModelsFromContract()

	gatewayModelCache.mu.Lock()
	gatewayModelCache.models = models
	gatewayModelCache.lastRefresh = time.Now()
	gatewayModelCache.mu.Unlock()
	return models
}

// findGatewayModelByName 按模型名称从合约查询（必须活跃）。
func findGatewayModelByName(name string) *cgateway.ModelInfo {
	for _, m := range getCachedGatewayModels() {
		if m.Name == name && m.IsActive {
			cp := m
			return &cp
		}
	}
	return nil
}

// buildEndpointsFromModel 根据合约 ModelInfo 构建 GatewayEndpoint 列表。
// 使用 TEE KMS 解密链上存储的加密 API Key（明文永不上链）。
func buildEndpointsFromModel(m *cgateway.ModelInfo) []*model.GatewayEndpoint {
	url := m.URI
	if url == "" {
		url = providerDefaultURL(m.ProviderType)
	}
	if len(m.APIKeyRefs) == 0 {
		return nil
	}

	// 获取全局密钥（内存缓存，~0 开销）
	gk, err := model.GetGlobalKey()
	if err != nil {
		GATEWAYLOG().Printf("[GATEWAY] GetGlobalKey failed: %v", err)
		return nil
	}

	bridge := getBridge()

	eps := make([]*model.GatewayEndpoint, 0, len(m.APIKeyRefs))
	for i, ref := range m.APIKeyRefs {
		var apiKey string
		if bridge != nil {
			encKey, err := bridge.GatewayGetEncryptedAPIKey(ref)
			if err != nil || encKey == nil {
				GATEWAYLOG().Printf("[GATEWAY] get encrypted key ref=%s err=%v", ref, err)
				continue
			}
			kmsKey := &model.KMSKey{
				Ref:        encKey.Ref,
				Ciphertext: encKey.Ciphertext,
				Nonce:      encKey.Nonce,
			}
			apiKey, err = model.DecryptWithGK(gk, kmsKey)
			if err != nil {
				GATEWAYLOG().Printf("[GATEWAY] decrypt ref=%s err=%v", ref, err)
				continue
			}
		}
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
