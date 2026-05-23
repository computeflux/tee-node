package gateway

import (
	"fmt"

	"github.com/wetee-dao/ink.go/util"
)

// ── Provider / Channel ───────────────────────────────────

func (g GatewayMutation) CreateProvider(
	name, logoURL, description string,
	region uint8, deposit uint64,
) error {
	owner := g.api.GetCaller()
	txn := g.api.GetTxn()

	// 一人只能注册一个 provider
	if existing, _ := g.providerByOwner.Get(txn, owner); existing != nil {
		return fmt.Errorf("provider already exists for this account (id=%d)", *existing)
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

func (g GatewayMutation) UpdateProvider(
	providerID uint64,
	name, logoURL, description string,
	modelIds []uint64, region uint8, feeRate uint32,
	priority uint32, uptimeSLA uint32,
) error {
	return g.providers.UpdateFn(g.api.GetTxn(), providerID, func(ch *ProviderInfo) {
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

// GetProviderByOwner 查询当前用户的 provider
func (g GatewayQuery) GetProviderByOwner() (util.Option[uint64], error) {
	owner := g.api.GetCaller()
	id, err := g.providerByOwner.Get(g.api.GetTxn(), owner)
	if err != nil || id == nil {
		return util.NewNone[uint64](), nil
	}
	return util.NewSome(*id), nil
}

func (g GatewayQuery) GetProviderCount() (uint64, error) {
	return g.providers.Len(g.api.GetTxn())
}

func (g GatewayQuery) ListProviders(offset uint64, limit uint32) ([]ProviderInfo, error) {
	_, vals, err := g.providers.List(g.api.GetTxn(), offset, limit)
	return vals, err
}

func (g GatewayQuery) ListActiveProviders(offset uint64, limit uint32) ([]ProviderInfo, error) {
	return g.listProvidersFiltered(offset, limit, func(ch *ProviderInfo) bool { return ch.IsActive })
}

func (g GatewayQuery) listProvidersFiltered(
	offset uint64, limit uint32, fn func(*ProviderInfo) bool,
) ([]ProviderInfo, error) {
	txn := g.api.GetTxn()
	total, _ := g.providers.Len(txn)
	out := make([]ProviderInfo, 0, limit)
	var skipped, collected uint64
	for i := uint64(0); i < total && collected < uint64(limit); i++ {
		ch, err := g.providers.Get(txn, i)
		if err != nil || ch == nil {
			continue
		}
		if fn != nil && !fn(ch) {
			continue
		}
		if skipped < offset {
			skipped++
			continue
		}
		out = append(out, *ch)
		collected++
	}
	return out, nil
}

// GetProviderByName 返回指定名称的所有 Provider（使用 Name exact 索引）
func (g GatewayQuery) GetProviderByName(name string) ([]ProviderInfo, error) {
	keys, err := g.providers.Query(g.api.GetTxn(), map[string]any{"Name": name})
	if err != nil {
		return nil, err
	}
	out := make([]ProviderInfo, 0, len(keys))
	for _, k := range keys {
		p, err := g.providers.Get(g.api.GetTxn(), k)
		if err != nil || p == nil {
			continue
		}
		out = append(out, *p)
	}
	return out, nil
}
