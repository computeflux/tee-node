package gateway

import (
	"encoding/hex"
	"fmt"
	"slices"

	"github.com/computeflux/tee-provider/tee-chain/runtime/store"
)

// ── Billing ──────────────────────────────────────────────

// SettleUsage 批量结算（Provider Owner 或 System Account 可调用，每 5 分钟执行一次）
func (g GatewayMutation) SettleUsageWithRoute(records []SettlementWithRoute) error {
	caller := g.api.GetCaller()
	if caller.T != 0 {
		fmt.Println("SettleUsage must call by node")
		return fmt.Errorf("Recharge: caller is not node")
	}

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

		// 确定定价来源：路由 or 模型
		var inPrice, cachedPrice, outPrice uint64
		isRoute := r.RouteID.IsSome()
		if isRoute {
			routeID, err := r.RouteID.UnWrap()
			if err != nil {
				return fmt.Errorf("settle: %w", err)
			}
			route, err := g.routes.Get(g.api.GetTxn(), routeID)
			if err != nil || route == nil {
				return fmt.Errorf("settle: %w", ErrRouteNotFound)
			}
			if !route.IsActive {
				return fmt.Errorf("settle: route %d is inactive", routeID)
			}
			if !slices.Contains(route.ModelIds, r.ModelID) {
				return fmt.Errorf("settle: model %d not in route %d", r.ModelID, routeID)
			}
			inPrice, cachedPrice, outPrice = route.InPrice, route.CachedPrice, route.OutPrice
		} else {
			inPrice, cachedPrice, outPrice = m.InPrice, m.CachedPrice, m.OutPrice
		}

		// 计算费用（向上取整，每次调用至少 1 point）
		uncached := r.TokenIn - r.CachedToken
		cost := (uncached*inPrice + r.CachedToken*cachedPrice + r.TokenOut*outPrice) / 1_000_000
		if cost == 0 {
			cost = 1
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

	// 聚合本轮各 Provider 及 Model 的成功/失败次数，更新链上信誉与状态（仅非路由记录）
	type rep struct {
		success, fail uint64
	}
	provRep := make(map[uint64]rep)
	modelRep := make(map[uint64]rep)

	for _, r := range records {
		if r.RouteID.IsSome() {
			continue
		}
		// 统计 Provider
		p := provRep[r.ProviderID]
		p.success += uint64(r.Count)
		p.fail += uint64(r.FailCount)
		provRep[r.ProviderID] = p

		// 统计 Model
		mRep := modelRep[r.ModelID]
		mRep.success += uint64(r.Count)
		mRep.fail += uint64(r.FailCount)
		modelRep[r.ModelID] = mRep
	}

	// 更新 Provider 信誉
	for pid, rp := range provRep {
		total := rp.success + rp.fail
		if total == 0 {
			continue
		}
		_ = g.providers.UpdateFn(g.api.GetTxn(), pid, func(p *ProviderInfo) {
			p.TotalRequests += total

			// 滑动窗口 EMA，窗口大小 = 最近的 100 次调用
			const windowSize uint64 = 100
			if p.TotalRequests > windowSize {
				// 满窗口：新旧数据按权重混合
				weight := min(total, windowSize) // 新批次填满整个窗口
				oldWeight := windowSize - weight // 绝不会下溢（weight <= windowSize）
				p.SuccessRate = uint32((uint64(p.SuccessRate)*oldWeight + rp.success*10000) / windowSize)
			} else {
				// 还没满窗口：简单累计平均
				oldWeight := p.TotalRequests - total
				p.SuccessRate = uint32((uint64(p.SuccessRate)*oldWeight + rp.success*10000) / p.TotalRequests)
			}
		})
	}

	// 检查并下线连续失败的模型
	for mid, rp := range modelRep {
		// 如果在这个结算周期没有任何一次成功，且有失败记录，则直接判定模型不可用
		if rp.success == 0 && rp.fail > 0 {
			_ = g.models.UpdateFn(g.api.GetTxn(), mid, func(m *ModelInfo) {
				m.IsActive = false
			})
		}
	}

	return nil
}

// GetSettlementHistory 查询 Provider 结算历史（按时间倒序）
func (g GatewayQuery) GetSettlementHistory(providerID uint64, offset uint64, limit uint32) ([]ProviderSettlementRecord, error) {
	txn := g.api.GetTxn()
	startKey := uint32(offset)
	end, _ := g.settlementHistory.NextID(txn, providerID)
	startKey = end - startKey
	_, records, err := g.settlementHistory.DescList(txn, providerID, &startKey, limit)
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
	txn := g.api.GetTxn()
	startKey := uint32(offset)
	end, _ := g.userSettlementHistory.NextID(txn, user)
	startKey = end - startKey
	_, records, err := g.userSettlementHistory.DescList(txn, user, &startKey, limit)
	if err != nil {
		return nil, err
	}
	return records, nil
}
