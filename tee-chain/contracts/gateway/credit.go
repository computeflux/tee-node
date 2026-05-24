package gateway

import (
	"bytes"
	"encoding/json"
	"fmt"

	"github.com/computeflux/tee-provider/pkg/model"
)

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
			// 缓存命中 token 按 10% 计价（向上取整到千）
			effectiveTokens := r.CachedTokens/10 + (r.TokensUsed - r.CachedTokens)
			cost = (effectiveTokens+999)/1000*m.PointsPer1K + m.PromptPricing
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
		usage.CachedTokens += r.CachedTokens
		usage.PointsUsed += cost
		g.usages.Set(g.api.GetTxn(), dayKey, *usage)

		// 记录结算历史
		day := fmt.Sprintf("%d", g.api.GetHeight()/7200)
		_, _ = g.settlementHistory.Insert(g.api.GetTxn(), r.ProviderID, SettlementRecord{
			ProviderID:   r.ProviderID,
			User:         r.User,
			ModelID:      r.ModelID,
			TokensUsed:   r.TokensUsed,
			CachedTokens: r.CachedTokens,
			Cost:         cost,
			Day:          day,
			Block:        g.api.GetHeight(),
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

// ── Query: Single ────────────────────────────────────────
func (g GatewayQuery) GetCredit(user model.UniAddr) (uint64, error) {
	c, err := g.credits.Get(g.api.GetTxn(), user)
	if err != nil || c == nil {
		return 0, nil
	}
	return *c, nil
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
