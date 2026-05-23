package gateway

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/computeflux/tee-provider/pkg/model"
	"golang.org/x/crypto/blake2b"
)

// uniAddrKey 将 UniAddr 转为确定性 uint64，用于 exact 索引。
func uniAddrKey(a model.UniAddr) uint64 {
	h, _ := blake2b.New256(nil)
	h.Write(a.V)
	var tBytes [4]byte
	binary.BigEndian.PutUint32(tBytes[:], a.T)
	h.Write(tBytes[:])
	return binary.BigEndian.Uint64(h.Sum(nil)[:8])
}

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

	credits           *model.StoreMapping[model.UniAddr, uint64] `store:"keyPfx:credits"`
	models            *model.IndexedList[uint64, ModelInfo]      `store:"keyPfx:v0_models"`
	providers         *model.IndexedList[uint64, ProviderInfo]   `store:"keyPfx:v0_providers"`
	providerByOwner   *model.StoreMapping[model.UniAddr, uint64] `store:"keyPfx:v0_provider_by_owner"` // owner → providerID（一人一 provider）
	providerCredits   *model.StoreMapping[uint64, uint64]        // providerID → accumulated credits
	apiKeys           *model.IndexedStore[string, APIKeyInfo]    // hex(keyHash) → APIKeyInfo，带 OwnerKey exact 索引
	usages            *model.StoreMapping[string, DailyUsage]
	encryptedKeys     *model.StoreMapping[string, EncryptedAPIKey]              // ref → EncryptedAPIKey
	rechargeHistory   *model.StoreList2D[model.UniAddr, uint32, RechargeRecord] // user → []RechargeRecord
	settlementHistory *model.StoreList2D[uint64, uint32, SettlementRecord]      // providerID → []SettlementRecord
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
	if err := g.credits.Set(g.api.GetTxn(), user, *bal); err != nil {
		return err
	}

	// 记录充值历史
	day := fmt.Sprintf("%d", g.api.GetHeight()/7200) // 约每日一个区块高度周期
	_, _ = g.rechargeHistory.Insert(g.api.GetTxn(), user, RechargeRecord{
		User:   user,
		Amount: amount,
		Day:    day,
		Block:  g.api.GetHeight(),
	})
	return nil
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

		// 记录结算历史
		day := fmt.Sprintf("%d", g.api.GetHeight()/7200)
		_, _ = g.settlementHistory.Insert(g.api.GetTxn(), r.ProviderID, SettlementRecord{
			ProviderID: r.ProviderID,
			User:       r.User,
			ModelID:    r.ModelID,
			TokensUsed: r.TokensUsed,
			Cost:       cost,
			Day:        day,
			Block:      g.api.GetHeight(),
		})
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

	if existing, _ := g.apiKeys.Primary.Get(g.api.GetTxn(), hexKey); existing != nil {
		return fmt.Errorf("api key already registered")
	}

	return g.apiKeys.Put(g.api.GetTxn(), hexKey, APIKeyInfo{
		OwnerKey:  uniAddrKey(owner),
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

	info, err := g.apiKeys.Primary.Get(g.api.GetTxn(), hexKey)
	if err != nil || info == nil {
		return ErrAPIKeyNotFound
	}
	if !bytes.Equal(info.Owner.V, owner.V) || info.Owner.T != owner.T {
		return ErrNotOwner
	}
	return g.apiKeys.Update(g.api.GetTxn(), hexKey, func(cur *APIKeyInfo) {
		cur.IsActive = false
	})
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
	return g.apiKeys.Primary.Get(g.api.GetTxn(), hexKey)
}

// GetAPIKeysByOwner 返回某个用户注册的所有 API Key（使用 OwnerKey exact 索引，O(1) 查询）
func (g GatewayQuery) GetAPIKeysByOwner(owner model.UniAddr) ([]APIKeyInfo, error) {
	keys, err := g.apiKeys.Query(g.api.GetTxn(), map[string]any{"OwnerKey": uniAddrKey(owner)})
	if err != nil {
		return nil, err
	}
	out := make([]APIKeyInfo, 0, len(keys))
	for _, k := range keys {
		info, err := g.apiKeys.Primary.Get(g.api.GetTxn(), k)
		if err != nil || info == nil {
			continue
		}
		out = append(out, *info)
	}
	return out, nil
}

func (g GatewayQuery) GetProviderCredit(providerID uint64) (uint64, error) {
	bal, err := g.providerCredits.Get(g.api.GetTxn(), providerID)
	if err != nil || bal == nil {
		return 0, nil
	}
	return *bal, nil
}

// ── Query: History ───────────────────────────────────────

// GetRechargeHistory 查询用户充值历史（按时间倒序）
func (g GatewayQuery) GetRechargeHistory(user model.UniAddr, offset uint64, limit uint32) ([]RechargeRecord, error) {
	_, records, err := g.rechargeHistory.DescList(g.api.GetTxn(), user, (*uint32)(nil), limit)
	if err != nil {
		return nil, err
	}
	// DescList 从最新开始返回，若 offset > 0 需要跳过前面条目
	if int(offset) >= len(records) {
		return []RechargeRecord{}, nil
	}
	return records[offset:], nil
}

// GetSettlementHistory 查询 Provider 结算历史（按时间倒序）
func (g GatewayQuery) GetSettlementHistory(providerID uint64, offset uint64, limit uint32) ([]SettlementRecord, error) {
	_, records, err := g.settlementHistory.DescList(g.api.GetTxn(), providerID, (*uint32)(nil), limit)
	if err != nil {
		return nil, err
	}
	if int(offset) >= len(records) {
		return []SettlementRecord{}, nil
	}
	return records[offset:], nil
}

// GetUsageHistory 查询用户在某 Provider+Model 上的用量历史（按日聚合）
// providerID=0 且 modelID=0 时查询该用户所有 Provider+Model 的用量。
func (g GatewayQuery) GetUsageHistory(user model.UniAddr, providerID uint64, modelID uint64, startDay string, endDay string) ([]DailyUsage, error) {
	var prefix string
	if providerID == 0 && modelID == 0 {
		prefix = fmt.Sprintf("gateway:usage_%x_", user.V)
	} else {
		prefix = fmt.Sprintf("gateway:usage_%x_%d_%d_", user.V, providerID, modelID)
	}
	_, vals, err := g.api.GetTxn().ListByPrefix([]byte(prefix))
	if err != nil {
		return nil, err
	}
	var out []DailyUsage
	for _, v := range vals {
		var u DailyUsage
		if err := json.Unmarshal(v, &u); err != nil {
			continue
		}
		if u.Day >= startDay && u.Day <= endDay {
			out = append(out, u)
		}
	}
	return out, nil
}

// GetAllUsageByUser 查询用户的全部用量历史（不区分 Provider/Model，按日聚合）
func (g GatewayQuery) GetAllUsageByUser(user model.UniAddr, startDay string, endDay string) ([]DailyUsage, error) {
	return g.GetUsageHistory(user, 0, 0, startDay, endDay)
}
