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
