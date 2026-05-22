package gateway

import (
	"bytes"
	"encoding/hex"
	"errors"
	"fmt"

	"github.com/computeflux/tee-provider/pkg/model"
)

var (
	ErrLowCredit        = errors.New("low Credit")
	ErrModelNotFound    = errors.New("model not found")
	ErrProviderNotFound = errors.New("provider not found")
	ErrAPIKeyNotFound   = errors.New("api key not found")
	ErrNotOwner         = errors.New("not owner")
)

// Gateway API 网关合约
type Gateway struct {
	api model.ContractApi

	credits         *model.StoreMapping[model.UniAddr, uint64] `store:"keyPfx:credits"`
	models          *model.IndexedList[uint64, ModelInfo]      `store:"keyPfx:v0_models"`
	providers       *model.IndexedList[uint64, ProviderInfo]   `store:"keyPfx:v0_providers"`
	providerByOwner *model.StoreMapping[model.UniAddr, uint64] `store:"keyPfx:v0_provider_by_owner"` // owner → providerID（一人一 provider）
	providerCredits *model.StoreMapping[uint64, uint64]        // providerID → accumulated credits
	apiKeys         *model.StoreMapping[string, APIKeyInfo]    // hex(keyHash) → APIKeyInfo
	usages          *model.StoreMapping[string, DailyUsage]
	encryptedKeys   *model.StoreMapping[string, EncryptedAPIKey] // ref → EncryptedAPIKey
}

type GatewayQuery struct{ Gateway }
type GatewayMutation struct{ Gateway }

// ── Init ─────────────────────────────────────────────────

func (g GatewayMutation) Init() error {
	return nil
}

// ── Points ───────────────────────────────────────────────

func (g GatewayMutation) Recharge(user model.UniAddr, amount uint64) error {
	if !g.api.CallbySystem() {
		return fmt.Errorf("Recharge: caller is not sudo account")
	}

	bal, _ := g.credits.Get(g.api.GetTxn(), user)
	if bal == nil {
		bal = new(uint64)
	}
	*bal += amount
	return g.credits.Set(g.api.GetTxn(), user, *bal)
}

// ── Billing ──────────────────────────────────────────────

// SettleUsage 批量结算（仅模型所属 Provider 可调用，每 5 分钟执行一次）
// TODO: 防重放 — 增加 batchNonce/merkle root + TEE 签名校验，防止 Provider 重复提交同一批结算
func (g GatewayMutation) SettleUsage(records []Settlement) error {
	caller := g.api.GetCaller()

	for _, r := range records {
		// 校验 provider 归属
		prov, err := g.providers.Get(g.api.GetTxn(), r.ProviderID)
		if err != nil || prov == nil {
			return fmt.Errorf("settle: %w", ErrProviderNotFound)
		}
		if !bytes.Equal(prov.Owner.V, caller.V) || prov.Owner.T != caller.T {
			return fmt.Errorf("settle: provider %d not owned by caller", r.ProviderID)
		}

		// 校验模型存在
		m, err := g.models.Get(g.api.GetTxn(), r.ModelID)
		if err != nil || m == nil {
			return fmt.Errorf("settle: %w", ErrModelNotFound)
		}

		cost := r.Cost
		if cost == 0 {
			cost = (r.TokensUsed+999)/1000*m.PointsPer1K + m.PromptPricing
		}
		if cost < 1 {
			cost = 1
		}

		// 扣用户余额
		bal, _ := g.credits.Get(g.api.GetTxn(), r.User)
		if bal == nil || *bal < cost {
			return fmt.Errorf("settle user %x: %w", r.User.V, ErrLowCredit)
		}
		*bal -= cost
		g.credits.Set(g.api.GetTxn(), r.User, *bal)

		// 加 provider 余额
		pb, _ := g.providerCredits.Get(g.api.GetTxn(), r.ProviderID)
		if pb == nil {
			pb = new(uint64)
		}
		*pb += cost
		g.providerCredits.Set(g.api.GetTxn(), r.ProviderID, *pb)

		// 记录每日用量
		dayKey := fmt.Sprintf("%x_%d_%d_%s", r.User.V, r.ProviderID, r.ModelID, "today")
		usage, _ := g.usages.Get(g.api.GetTxn(), dayKey)
		if usage == nil {
			usage = &DailyUsage{User: r.User, ProviderID: r.ProviderID, ModelID: r.ModelID}
		}
		usage.Count++
		usage.TokensUsed += r.TokensUsed
		usage.PointsUsed += cost
		g.usages.Set(g.api.GetTxn(), dayKey, *usage)
	}
	return nil
}

func (g GatewayMutation) Withdraw(providerID uint64, amount uint64) error {
	if !g.api.CallbySystem() {
		return fmt.Errorf("Withdraw: caller is not sudo account")
	}

	pb, err := g.providerCredits.Get(g.api.GetTxn(), providerID)
	if err != nil || pb == nil {
		return ErrProviderNotFound
	}

	if *pb < amount {
		return ErrLowCredit
	}
	*pb -= amount
	return g.providerCredits.Set(g.api.GetTxn(), providerID, *pb)
}

// ── API Key ──────────────────────────────────────────────

// RegisterAPIKey 注册 API Key 哈希（原始 key 绝不上链）
func (g GatewayMutation) RegisterAPIKey(keyHash []byte, rateLimit uint32) error {
	if len(keyHash) != 32 {
		return fmt.Errorf("keyHash must be 32 bytes")
	}
	owner := g.api.GetCaller()
	hexKey := hex.EncodeToString(keyHash)

	if existing, _ := g.apiKeys.Get(g.api.GetTxn(), hexKey); existing != nil {
		return fmt.Errorf("api key already registered")
	}

	return g.apiKeys.Set(g.api.GetTxn(), hexKey, APIKeyInfo{
		Owner:     owner,
		KeyHash:   keyHash,
		RateLimit: rateLimit,
		IsActive:  true,
		CreatedAt: uint64(g.api.GetHeight()),
	})
}

// RevokeAPIKey 吊销 API Key
func (g GatewayMutation) RevokeAPIKey(keyHash []byte) error {
	if len(keyHash) != 32 {
		return fmt.Errorf("keyHash must be 32 bytes")
	}
	owner := g.api.GetCaller()
	hexKey := hex.EncodeToString(keyHash)

	info, err := g.apiKeys.Get(g.api.GetTxn(), hexKey)
	if err != nil || info == nil {
		return ErrAPIKeyNotFound
	}
	if !bytes.Equal(info.Owner.V, owner.V) || info.Owner.T != owner.T {
		return ErrNotOwner
	}
	info.IsActive = false
	return g.apiKeys.Set(g.api.GetTxn(), hexKey, *info)
}

// ── Query: Single ────────────────────────────────────────

func (g GatewayQuery) GetCredit(user model.UniAddr) (uint64, error) {
	c, err := g.credits.Get(g.api.GetTxn(), user)
	if err != nil || c == nil {
		return 0, nil
	}
	return *c, nil
}

// LookupAPIKey TEE 提供方根据 hash 查找 API Key 对应的用户
func (g GatewayQuery) LookupAPIKey(keyHash []byte) (*APIKeyInfo, error) {
	if len(keyHash) != 32 {
		return nil, ErrAPIKeyNotFound
	}
	hexKey := hex.EncodeToString(keyHash)
	return g.apiKeys.Get(g.api.GetTxn(), hexKey)
}

func (g GatewayQuery) GetProviderCredit(providerID uint64) (uint64, error) {
	bal, err := g.providerCredits.Get(g.api.GetTxn(), providerID)
	if err != nil || bal == nil {
		return 0, nil
	}
	return *bal, nil
}
