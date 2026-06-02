package gateway

import (
	"encoding/hex"
	"fmt"

	"github.com/computeflux/tee-provider/tee-chain/contracts/store"
)

// ── Points ───────────────────────────────────────────────

func (g GatewayMutation) Recharge(user store.UniAddr, amount uint64) error {
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
	_, _ = g.rechargeHistory.Insert(g.api.GetTxn(), user, RechargeRecord{
		User:   user,
		Amount: amount,
		Time:   g.api.GetBlockTime(),
	})
	return nil
}

// ── Billing ──────────────────────────────────────────────

// SettleUsage 批量结算（Provider Owner 或 System Account 可调用，每 5 分钟执行一次）
func (g GatewayMutation) SettleUsage(records []Settlement) error {
	for _, r := range records {
		// 防重放检查：每条记录必须有唯一 Nonce，已使用的 Nonce 永久拒绝
		nonceHex := hex.EncodeToString(r.Nonce)
		if nonceHex == "" {
			return fmt.Errorf("settle: empty nonce")
		}
		exists, _ := g.settledNonces.Get(g.api.GetTxn(), nonceHex)
		if exists != nil && *exists {
			return nil
		}

		// 校验 provider 归属（非 system 调用时必须验证）
		prov, err := g.providers.Get(g.api.GetTxn(), r.ProviderID)
		if err != nil || prov == nil {
			return fmt.Errorf("settle: %w", ErrProviderNotFound)
		}

		// 校验模型存在
		m, err := g.models.Get(g.api.GetTxn(), r.ModelID)
		if err != nil || m == nil {
			return fmt.Errorf("settle: %w", ErrModelNotFound)
		}

		// 计算费用（按模型定价，向上取整，每次调用至少 1 point）
		var cost uint64 = 0

		// 非缓存 token 按 InPrice 计费，缓存 token 按 CachedPrice 计费（均按每千向上取整）
		uncached := r.TokenIn - r.CachedToken
		cost = (uncached*m.InPrice + r.CachedToken*m.CachedPrice + r.TokenOut*m.OutPrice) / 1_000_000
		if cost == 0 {
			continue
		}

		// 扣用户余额
		c, err := g.credit_useds.Get(g.api.GetTxn(), r.User)
		if err != nil {
			return fmt.Errorf("settle: qeury error %w", err)
		}
		if c == nil {
			c = new(uint64)
		}
		*c += cost
		g.credit_useds.Set(g.api.GetTxn(), r.User, *c)

		// 加 provider 余额
		pb, err := g.providerCredits.Get(g.api.GetTxn(), r.ProviderID)
		if err != nil {
			return fmt.Errorf("settle: qeury error %w", err)
		}
		if pb == nil {
			pb = new(uint64)
		}
		*pb += cost
		g.providerCredits.Set(g.api.GetTxn(), r.ProviderID, *pb)

		// 记录结算历史（Provider 维度）
		_, _ = g.settlementHistory.Insert(g.api.GetTxn(), r.ProviderID, ProviderSettlementRecord{
			User:        r.User,
			ModelID:     r.ModelID,
			TokenIn:     r.TokenIn,
			TokenOut:    r.TokenOut,
			CachedToken: r.CachedToken,
			Cost:        cost,
			Count:       r.Count,
			Time:        g.api.GetBlockTime(),
		})
		_, _ = g.userSettlementHistory.Insert(g.api.GetTxn(), r.User, UserSettlementRecord{
			ProviderID:  r.ProviderID,
			ModelID:     r.ModelID,
			TokenIn:     r.TokenIn,
			TokenOut:    r.TokenOut,
			CachedToken: r.CachedToken,
			Cost:        cost,
			Count:       r.Count,
			Time:        g.api.GetBlockTime(),
		})

		// 标记 nonce 已使用（防重放永久拒绝）
		if err := g.settledNonces.Set(g.api.GetTxn(), nonceHex, true); err != nil {
			return fmt.Errorf("settle: store nonce failed: %w", err)
		}
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
func (g GatewayQuery) GetCredit(user store.UniAddr) (int64, error) {
	c, err := g.credits.Get(g.api.GetTxn(), user)
	if err != nil || c == nil {
		return 0, nil
	}

	u, err := g.credit_useds.Get(g.api.GetTxn(), user)
	if err != nil {
		return int64(*c), err
	}

	if u == nil {
		return int64(*c), nil
	}

	return int64(*c) - int64(*u), nil
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
func (g GatewayQuery) GetRechargeHistory(user store.UniAddr, offset uint64, limit uint32) ([]RechargeRecord, error) {
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
func (g GatewayQuery) GetSettlementHistory(providerID uint64, offset uint64, limit uint32) ([]ProviderSettlementRecord, error) {
	_, records, err := g.settlementHistory.DescList(g.api.GetTxn(), providerID, (*uint32)(nil), limit)
	if err != nil {
		return nil, err
	}
	if int(offset) >= len(records) {
		return []ProviderSettlementRecord{}, nil
	}
	return records[offset:], nil
}

// GetUserSettlementHistory 查询用户的结算历史（按时间倒序）
func (g GatewayQuery) GetUserSettlementHistory(user store.UniAddr, offset uint64, limit uint32) ([]UserSettlementRecord, error) {
	_, records, err := g.userSettlementHistory.DescList(g.api.GetTxn(), user, (*uint32)(nil), limit)
	if err != nil {
		return nil, err
	}
	if int(offset) >= len(records) {
		return []UserSettlementRecord{}, nil
	}
	return records[offset:], nil
}
