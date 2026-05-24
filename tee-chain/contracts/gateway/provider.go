package gateway

import (
	"errors"
	"fmt"

	"github.com/computeflux/tee-provider/pkg/model"
	"github.com/wetee-dao/ink.go/util"
)

// ── Provider / Provider ───────────────────────────────────

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

	// provider name 唯一性检查
	if name != "" {
		dup, _ := g.providers.Query(txn, model.QueryCond{Field: "Name", Val: name})
		if len(dup) > 0 {
			return errors.New("provider name already exists")
		}
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
	txn := g.api.GetTxn()

	// 若修改 name，需检查唯一性
	if name != "" {
		cur, _ := g.providers.Get(txn, providerID)
		if cur != nil && cur.Name != name {
			dup, _ := g.providers.Query(txn, model.QueryCond{Field: "Name", Val: name})
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

func (g GatewayQuery) listProvidersByIDs(ids []uint64, offset uint64, limit uint32) ([]ProviderInfo, error) {
	if uint64(len(ids)) <= offset {
		return nil, nil
	}
	end := offset + uint64(limit)
	if end > uint64(len(ids)) {
		end = uint64(len(ids))
	}
	txn := g.api.GetTxn()
	out := make([]ProviderInfo, 0, end-offset)
	for _, id := range ids[offset:end] {
		p, err := g.providers.Get(txn, id)
		if err != nil || p == nil {
			continue
		}
		out = append(out, *p)
	}
	return out, nil
}

func (g GatewayQuery) ListProviders(offset uint64, limit uint32) ([]ProviderInfo, error) {
	_, vals, err := g.providers.List(g.api.GetTxn(), offset, limit)
	return vals, err
}

func (g GatewayQuery) ListActiveProviders(offset uint64, limit uint32) ([]ProviderInfo, error) {
	ids, err := g.providers.Query(g.api.GetTxn(), model.QueryCond{Field: "IsActive", Val: uint64(1)})
	if err != nil {
		return nil, err
	}
	return g.listProvidersByIDs(ids, offset, limit)
}

// GetProviderByName 返回指定名称的所有 Provider（使用 Name exact 索引）
func (g GatewayQuery) GetProviderByName(name string) (*DataWithID[ProviderInfo], error) {
	keys, err := g.providers.Query(g.api.GetTxn(), model.QueryCond{Field: "Name", Val: name})
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

	return &DataWithID[ProviderInfo]{
		ID:   keys[0],
		Data: *p,
	}, nil
}
