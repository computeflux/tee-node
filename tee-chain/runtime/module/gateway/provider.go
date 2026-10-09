package gateway

import (
	"errors"
	"fmt"
	"strings"

	"github.com/computeflux/tee-provider/tee-chain/runtime/store"
)

// isReservedProviderName 检查 provider 名是否与 route 类型前缀冲突。
func isReservedProviderName(name string) bool {
	switch strings.ToLower(name) {
	case "edge", "miningpool", "tee":
		return true
	}
	return false
}

// ── Provider / Provider ───────────────────────────────────

// MinProviderDeposit provider 最低质押金：deposit 必须大于该值才可注册。
const MinProviderDeposit = 100_000_000

func (g GatewayMutation) CreateProvider(
	name, logoURL, description string,
	region uint8, deposit uint64,
) error {
	owner := g.api.GetCaller()
	txn := g.api.GetTxn()

	if name == "" {
		return errors.New("provider name is required")
	}

	if isReservedProviderName(name) {
		return errors.New("provider name conflicts with route prefix")
	}

	// 最低质押校验：deposit 必须大于 MinProviderDeposit
	if deposit <= MinProviderDeposit {
		return fmt.Errorf("deposit must be greater than %d, got %d", MinProviderDeposit, deposit)
	}

	// 一人只能注册一个 provider
	if existing, _ := g.providerByOwner.Get(txn, owner); existing != nil {
		return fmt.Errorf("provider already exists for this account (id=%d)", *existing)
	}

	// provider name 唯一性检查
	dup, _ := g.providers.Query(txn, store.QueryCond{Field: "Name", Val: name})
	if len(dup) > 0 {
		return errors.New("provider name already exists")
	}

	// priority 由 deposit 自动计算
	var priority uint32
	switch {
	case deposit >= 10000:
		priority = 90
	case deposit >= 1000:
		priority = 60
	default:
		priority = 30
	}

	// 真实质押：从调用者可用余额扣除 deposit（关闭店铺时才返还）
	if err := g.stakeDeposit(owner, deposit); err != nil {
		return err
	}

	id, err := g.providers.Insert(txn, ProviderInfo{
		Owner:       owner,
		Name:        name,
		LogoURL:     logoURL,
		Description: description,
		Region:      region,
		Deposit:     deposit,
		Priority:    priority,
		IsActive:    true,
		CreatedAt:   uint64(g.api.GetHeight()),
	})
	if err != nil {
		return err
	}

	// 记录 owner → providerID 映射
	if err := g.providerByOwner.Set(txn, owner, id); err != nil {
		return err
	}
	return nil
}

// ── 质押（deposit 从用户可用余额锁定，关闭店铺时返还）──────────

// stakeDeposit 从 owner 可用余额扣除 amount 并锁定为质押：
// 与 TransferCredit 一致，仅从 credits（总余额）扣除，不动 credit_useds（已用额度）。
func (g Gateway) stakeDeposit(owner store.UniAddr, amount uint64) error {
	txn := g.api.GetTxn()
	bal, _ := g.credits.Get(txn, owner)
	if bal == nil {
		bal = new(uint64)
	}
	used, _ := g.credit_useds.Get(txn, owner)
	if used == nil {
		used = new(uint64)
	}
	if *bal-*used < amount {
		return fmt.Errorf("stake deposit: %w", ErrLowCredit)
	}
	*bal -= amount
	if err := g.credits.Set(txn, owner, *bal); err != nil {
		return fmt.Errorf("stake deposit: debit failed: %w", err)
	}
	return nil
}

// releaseDeposit stakeDeposit 的逆操作：将 amount 质押返还给 owner 的可用余额。
func (g Gateway) releaseDeposit(owner store.UniAddr, amount uint64) error {
	txn := g.api.GetTxn()
	bal, _ := g.credits.Get(txn, owner)
	if bal == nil {
		bal = new(uint64)
	}
	*bal += amount
	if err := g.credits.Set(txn, owner, *bal); err != nil {
		return fmt.Errorf("release deposit: credit failed: %w", err)
	}
	return nil
}

// CloseProvider 关闭店铺（仅 owner 可调用）：下架 provider 及其名下模型，
// 将质押金返还到 owner 可用余额，并清除 owner→providerID 映射；
// 同一账户重新开店需重新注册并重新质押。
func (g GatewayMutation) CloseProvider(providerID uint64) error {
	owner := g.api.GetCaller()
	txn := g.api.GetTxn()

	prov, err := g.providers.Get(txn, providerID)
	if err != nil {
		return err
	}
	if prov == nil {
		return ErrProviderNotFound
	}
	if owner.String() != prov.Owner.String() {
		return ErrNotOwner
	}
	if !prov.IsActive {
		return errors.New("provider is already closed")
	}

	// 下架 provider 及其名下模型
	if err := g.providers.UpdateFn(txn, providerID, func(p *ProviderInfo) {
		p.IsActive = false
	}); err != nil {
		return err
	}
	modelIDs, err := g.modelsV2.Query(txn, store.QueryCond{Field: "ProviderID", Val: uint64(providerID)})
	if err != nil {
		return err
	}
	for _, modelID := range modelIDs {
		if err := g.modelsV2.UpdateFn(txn, modelID, func(m *ModelInfoV2) {
			m.IsActive = false
		}); err != nil {
			return err
		}
	}

	// 返还质押金到 owner 可用余额
	if err := g.releaseDeposit(prov.Owner, prov.Deposit); err != nil {
		return err
	}

	// 清除 owner→providerID 映射：重新开店需重新质押
	return g.providerByOwner.Delete(txn, owner)
}

// AddDeposit 追加质押（仅 owner 可调用）：用户需要提供更多信用时，
// 从可用余额扣除更多余额并累加到 provider 的 deposit。
func (g GatewayMutation) AddDeposit(providerID uint64, amount uint64) error {
	if amount == 0 {
		return ErrZeroAmount
	}
	owner := g.api.GetCaller()
	txn := g.api.GetTxn()

	prov, err := g.providers.Get(txn, providerID)
	if err != nil {
		return err
	}
	if prov == nil {
		return ErrProviderNotFound
	}
	if owner.String() != prov.Owner.String() {
		return ErrNotOwner
	}
	if !prov.IsActive {
		return errors.New("provider is closed, cannot add deposit")
	}

	if err := g.stakeDeposit(owner, amount); err != nil {
		return err
	}
	return g.providers.UpdateFn(txn, providerID, func(p *ProviderInfo) {
		p.Deposit += amount
	})
}

func (g GatewayMutation) UpdateProvider(
	providerID uint64,
	name, logoURL, description string,
	modelIds []uint64, region uint8, feeRate uint32,
	priority uint32, uptimeSLA uint32,
) error {
	txn := g.api.GetTxn()

	// 若修改 name，需检查唯一性
	if name != "" {
		if isReservedProviderName(name) {
			return errors.New("provider name conflicts with route prefix")
		}
		cur, _ := g.providers.Get(txn, providerID)
		if cur != nil && cur.Name != name {
			dup, _ := g.providers.Query(txn, store.QueryCond{Field: "Name", Val: name})
			if len(dup) > 0 {
				return errors.New("provider name already exists")
			}
		}
	}

	return g.providers.UpdateFn(txn, providerID, func(ch *ProviderInfo) {
		if name != "" {
			ch.Name = name
		}
		if logoURL != "" {
			ch.LogoURL = logoURL
		}
		if description != "" {
			ch.Description = description
		}
		if modelIds != nil {
			ch.ModelIds = modelIds
		}
		if region != 0 {
			ch.Region = region
		}
		if feeRate != 0 {
			ch.FeeRate = feeRate
		}
		if priority != 0 {
			ch.Priority = priority
		}
		if uptimeSLA != 0 {
			ch.UptimeSLA = uptimeSLA
		}
	})
}

func (g GatewayQuery) GetProvider(providerID uint64) (*ProviderInfo, error) {
	return g.providers.Get(g.api.GetTxn(), providerID)
}

// ReportProviderCall 链上更新 Provider 信誉：请求数、成功率、平均延迟（仅系统可调用）。
// 成功率用指数移动平均（最近 100 次权重主导），避免历史数据失真。
func (g GatewayMutation) ReportProviderCall(providerID uint64, success bool, latencyMs uint32) error {
	caller := g.api.GetCaller()
	if caller.T != 0 {
		return fmt.Errorf("ReportProviderCall: must be called by system")
	}
	return g.providers.UpdateFn(g.api.GetTxn(), providerID, func(p *ProviderInfo) {
		p.TotalRequests++

		const alpha = uint64(100) // EMA 平滑窗口
		oldRate := uint64(p.SuccessRate)
		var inc uint64
		if success {
			inc = 10000
		}
		if p.TotalRequests == 1 {
			p.SuccessRate = uint32(inc)
			p.AvgLatency = latencyMs
		} else {
			p.SuccessRate = uint32((oldRate*(alpha-1) + inc) / alpha)
			p.AvgLatency = uint32((uint64(p.AvgLatency)*(alpha-1) + uint64(latencyMs)) / alpha)
		}
	})
}

// GetProviderByOwner 查询当前用户的 provider
func (g GatewayQuery) GetProviderByOwner() (store.Option[uint64], error) {
	owner := g.api.GetCaller()
	id, err := g.providerByOwner.Get(g.api.GetTxn(), owner)
	if err != nil || id == nil {
		return store.NewNone[uint64](), nil
	}
	return store.NewSome(*id), nil
}

func (g GatewayQuery) GetProviderCount() (uint64, error) {
	return g.providers.Len(g.api.GetTxn())
}

func (g GatewayQuery) listProvidersByIDs(ids []uint64, offset uint64, limit uint32) ([]ProviderInfoWithID, error) {
	if uint64(len(ids)) <= offset {
		return nil, nil
	}
	end := offset + uint64(limit)
	if end > uint64(len(ids)) {
		end = uint64(len(ids))
	}
	txn := g.api.GetTxn()
	out := make([]ProviderInfoWithID, 0, end-offset)
	for _, id := range ids[offset:end] {
		p, err := g.providers.Get(txn, id)
		if err != nil || p == nil {
			continue
		}
		out = append(out, ProviderInfoWithID{
			ID:   id,
			Data: *p,
		})
	}
	return out, nil
}

func (g GatewayQuery) ListProviders(offset uint64, limit uint32) ([]ProviderInfo, error) {
	_, vals, err := g.providers.List(g.api.GetTxn(), offset, limit)
	return vals, err
}

func (g GatewayQuery) ListActiveProviders(offset uint64, limit uint32) ([]ProviderInfoWithID, error) {
	ids, err := g.providers.Query(g.api.GetTxn(), store.QueryCond{Field: "IsActive", Val: uint64(1)})
	if err != nil {
		return nil, err
	}

	return g.listProvidersByIDs(ids, offset, limit)
}

// GetProviderByName 返回指定名称的所有 Provider（使用 Name exact 索引）
func (g GatewayQuery) GetProviderByName(name string) (*ProviderInfoWithID, error) {
	keys, err := g.providers.Query(g.api.GetTxn(), store.QueryCond{Field: "Name", Val: name})
	if err != nil {
		return nil, err
	}

	if len(keys) == 0 {
		return nil, errors.New("provider not found with name: " + name)
	}

	p, err := g.providers.Get(g.api.GetTxn(), keys[0])
	if err != nil || p == nil {
		return nil, errors.New("provider not found with name: " + name)
	}

	return &ProviderInfoWithID{
		ID:   keys[0],
		Data: *p,
	}, nil
}
