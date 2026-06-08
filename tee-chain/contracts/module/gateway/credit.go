package gateway

import (
	"encoding/hex"
	"fmt"

	"github.com/computeflux/tee-provider/tee-chain/contracts/store"
)

// 420420417 Paseo test evm
var UserTypeToChainID = map[uint32][]uint32{
	2: {420420417},
}

// ── Points ───────────────────────────────────────────────

func (g GatewayMutation) Recharge(user store.UniAddr, amount uint64, nonce []byte) error {
	if !g.api.CallbySystem() {
		return fmt.Errorf("Recharge: caller is not sudo account")
	}

	// 防重放检查：nonce 必须非空且未使用过
	nonceHex := hex.EncodeToString(nonce)
	if nonceHex == "" {
		fmt.Println("Recharge: empty nonce")
		return nil
	}

	exists, _ := g.rechargeNonces.Get(g.api.GetTxn(), nonceHex)
	if exists != nil && *exists {
		fmt.Println("Recharge: duplicate nonce")
		return nil
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

	// 标记 nonce 已使用（防重放永久拒绝）
	if err := g.rechargeNonces.Set(g.api.GetTxn(), nonceHex, true); err != nil {
		return fmt.Errorf("Recharge: store nonce failed: %w", err)
	}
	return nil
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
