package gateway

import (
	"encoding/hex"
	"fmt"

	"github.com/computeflux/tee-provider/tee-chain/runtime/store"
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

// TransferCredit 用户将自己的 Credit 转账给另一个用户。
// 调用者即为转出方；转账金额不能超过当前可用余额。
func (g GatewayMutation) TransferCredit(to store.UniAddr, amount uint64) error {
	if amount == 0 {
		return nil
	}

	from := g.api.GetCaller()

	// 检查并扣减转出方余额（credits 存储总余额，credit_useds 为已用额度）
	fromBal, _ := g.credits.Get(g.api.GetTxn(), from)
	if fromBal == nil {
		fromBal = new(uint64)
	}
	fromUsed, _ := g.credit_useds.Get(g.api.GetTxn(), from)
	if fromUsed == nil {
		fromUsed = new(uint64)
	}
	available := *fromBal - *fromUsed
	if amount > available {
		return fmt.Errorf("TransferCredit: %w", ErrLowCredit)
	}

	*fromBal -= amount
	if err := g.credits.Set(g.api.GetTxn(), from, *fromBal); err != nil {
		return fmt.Errorf("TransferCredit: debit sender failed: %w", err)
	}

	// 增加接收方余额
	toBal, _ := g.credits.Get(g.api.GetTxn(), to)
	if toBal == nil {
		toBal = new(uint64)
	}
	*toBal += amount
	if err := g.credits.Set(g.api.GetTxn(), to, *toBal); err != nil {
		return fmt.Errorf("TransferCredit: credit receiver failed: %w", err)
	}

	// 记录转账历史
	now := g.api.GetBlockTime()
	_, _ = g.transferHistory.Insert(g.api.GetTxn(), from, TransferRecord{
		From:   from,
		To:     to,
		Amount: amount,
		Time:   now,
	})
	_, _ = g.transferHistory.Insert(g.api.GetTxn(), to, TransferRecord{
		From:   from,
		To:     to,
		Amount: amount,
		Time:   now,
	})

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

// GetRechargeHistory 查询用户充值历史（按时间倒序, 从最新起跳过 offset 条, 最多返回 limit 条）
func (g GatewayQuery) GetRechargeHistory(user store.UniAddr, offset uint64, limit uint32) ([]RechargeRecord, error) {
	txn := g.api.GetTxn()
	total, err := g.rechargeHistory.NextID(txn, user)
	if err != nil {
		return nil, err
	}
	startKey, ok := historyStartKey(total, offset)
	if !ok {
		return nil, nil
	}
	_, records, err := g.rechargeHistory.DescList(txn, user, &startKey, limit)
	if err != nil {
		return nil, err
	}
	return records, nil
}

// GetTransferHistory 查询用户转账历史（按时间倒序, 从最新起跳过 offset 条, 最多返回 limit 条）
func (g GatewayQuery) GetTransferHistory(user store.UniAddr, offset uint64, limit uint32) ([]TransferRecord, error) {
	txn := g.api.GetTxn()
	total, err := g.transferHistory.NextID(txn, user)
	if err != nil {
		return nil, err
	}
	startKey, ok := historyStartKey(total, offset)
	if !ok {
		return nil, nil
	}
	_, records, err := g.transferHistory.DescList(txn, user, &startKey, limit)
	if err != nil {
		return nil, err
	}
	return records, nil
}
