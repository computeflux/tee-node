package gateway

import (
	"bytes"
	"errors"
	"fmt"
	"slices"

	"github.com/computeflux/tee-provider/tee-chain/runtime/store"
)

// ── Route ──────────────────────────────────────────────────

// CreateRoute 创建模型路由（仅 gov/sudo 可调用，创世时用于官方路由）。
// 返回 Result[uint64, error]：Ok(routeID) 为新创建的路由 ID。
// contractgen 在 dispatch 边界会 UnWrap 成 error，外部调用方拿不到 routeID；
// 需要 routeID 的内部调用方（如 CreateOfficialRoutes）可以直接拿 Ok 值。
func (g GatewayMutation) CreateRoute(
	name string,
	routeType uint16,
	inPrice, cachedPrice, outPrice uint64,
	isOfficial bool,
) store.Result[uint64, error] {
	if !g.api.CallbySystem() {
		return store.NewErr[uint64, error](errors.New("only system can call this method"))
	}

	txn := g.api.GetTxn()

	// 路由名唯一性
	if name != "" {
		dup, _ := g.routes.Query(txn, store.QueryCond{Field: "Name", Val: name})
		if len(dup) > 0 {
			return store.NewErr[uint64, error](fmt.Errorf("route name %q already exists", name))
		}
	}

	id, err := g.routes.Insert(txn, ModelRoute{
		Name:        name,
		RouteType:   routeType,
		InPrice:     inPrice,
		CachedPrice: cachedPrice,
		OutPrice:    outPrice,
		ModelIds:    []uint64{},
		IsActive:    true,
		IsOfficial:  isOfficial,
		CreatedAt:   uint64(g.api.GetBlockTime()),
	})
	if err != nil {
		return store.NewErr[uint64, error](err)
	}
	return store.NewOk[uint64, error](id)
}

// UpdateRoute 更新路由配置（仅路由 owner 或 gov/sudo）。
func (g GatewayMutation) UpdateRoute(
	routeID uint64,
	name string,
	inPrice, cachedPrice, outPrice uint64,
	isActive *bool,
) error {
	if !g.api.CallbySystem() {
		return errors.New("only system can call this method")
	}

	_, err := g.routes.Get(g.api.GetTxn(), routeID)
	if err != nil {
		return err
	}

	return g.routes.UpdateFn(g.api.GetTxn(), routeID, func(r *ModelRoute) {
		if name != "" {
			r.Name = name
		}
		if inPrice != 0 {
			r.InPrice = inPrice
		}
		if cachedPrice != 0 {
			r.CachedPrice = cachedPrice
		}
		if outPrice != 0 {
			r.OutPrice = outPrice
		}
		if isActive != nil {
			r.IsActive = *isActive
		}
	})
}

// DeleteRoute 删除路由（官方路由不可删除）。
func (g GatewayMutation) DeleteRoute(routeID uint64) error {
	if !g.api.CallbySystem() {
		return errors.New("only system can call this method")
	}

	route, err := g.routes.Get(g.api.GetTxn(), routeID)
	if err != nil || route == nil {
		return err
	}
	if route.IsOfficial {
		return errors.New("cannot delete official route")
	}
	if len(route.ModelIds) > 0 {
		return errors.New("route still has models, remove them first")
	}
	return g.routes.Delete(g.api.GetTxn(), routeID)
}

// JoinRoute 模型加入路由：加入后价格由路由统一决定，不可单独设置。
func (g GatewayMutation) JoinRoute(modelID, routeID uint64) error {
	txn := g.api.GetTxn()

	// 校验路由存在且活跃
	route, err := g.routes.Get(txn, routeID)
	if err != nil || route == nil {
		return fmt.Errorf("route %d not found", routeID)
	}
	if !route.IsActive {
		return fmt.Errorf("route %d is inactive", routeID)
	}

	// 校验模型存在且发起者是模型拥有者
	model, err := g.models.Get(txn, modelID)
	if err != nil || model == nil {
		return fmt.Errorf("model %d not found", modelID)
	}
	caller := g.api.GetCaller()
	if caller.T != model.Owner.T || !bytes.Equal(caller.V, model.Owner.V) {
		return errors.New("only model owner can join route")
	}

	// // 校验路由类型与模型 ProviderType 一致
	// if model.ProviderType != route.RouteType {
	// 	return fmt.Errorf("model providerType %d does not match route type %d", model.ProviderType, route.RouteType)
	// }

	// 更新模型的 RouteID，价格由路由统一
	if err := g.models.UpdateFn(txn, modelID, func(m *ModelInfo) {
		m.RouteID = routeID
	}); err != nil {
		return err
	}

	// 将模型 ID 加入路由的 ModelIds
	return g.routes.UpdateFn(txn, routeID, func(r *ModelRoute) {
		if slices.Contains(r.ModelIds, modelID) {
			return
		}
		r.ModelIds = append(r.ModelIds, modelID)
	})
}

// LeaveRoute 模型退出路由：恢复独立定价。
func (g GatewayMutation) LeaveRoute(modelID uint64) error {
	txn := g.api.GetTxn()

	model, err := g.models.Get(txn, modelID)
	if err != nil || model == nil {
		return fmt.Errorf("model %d not found", modelID)
	}

	caller := g.api.GetCaller()
	if caller.T != model.Owner.T || !bytes.Equal(caller.V, model.Owner.V) {
		return errors.New("only model owner can leave route")
	}

	routeID := model.RouteID
	if routeID == 0 {
		return errors.New("model is not in any route")
	}

	// 清除模型的 RouteID
	if err := g.models.UpdateFn(txn, modelID, func(m *ModelInfo) {
		m.RouteID = 0
	}); err != nil {
		return err
	}

	// 从路由的 ModelIds 中移除
	return g.routes.UpdateFn(txn, routeID, func(r *ModelRoute) {
		for i, id := range r.ModelIds {
			if id == modelID {
				r.ModelIds = append(r.ModelIds[:i], r.ModelIds[i+1:]...)
				return
			}
		}
	})
}

// ── Query ──────────────────────────────────────────────────

func (g GatewayQuery) GetRoute(routeID uint64) (*ModelRoute, error) {
	return g.routes.Get(g.api.GetTxn(), routeID)
}

func (g GatewayQuery) GetRouteByName(name string) (*ModelRouteWithID, error) {
	keys, err := g.routes.Query(g.api.GetTxn(), store.QueryCond{Field: "Name", Val: name})
	if err != nil {
		return nil, err
	}
	if len(keys) == 0 {
		return nil, errors.New("route not found: " + name)
	}
	r, err := g.routes.Get(g.api.GetTxn(), keys[0])
	if err != nil || r == nil {
		return nil, errors.New("route not found: " + name)
	}
	return &ModelRouteWithID{ID: keys[0], Data: *r}, nil
}

func (g GatewayQuery) ListRoutes(offset uint64, limit uint32) ([]ModelRouteWithID, error) {
	keys, vals, err := g.routes.List(g.api.GetTxn(), offset, limit)
	if err != nil {
		return nil, err
	}
	out := make([]ModelRouteWithID, len(vals))
	for i := range vals {
		out[i] = ModelRouteWithID{ID: keys[i], Data: vals[i]}
	}
	return out, nil
}

func (g GatewayQuery) ListActiveRoutesByType(routeType uint16, offset uint64, limit uint32) ([]ModelRouteWithID, error) {
	ids, err := g.routes.Query(g.api.GetTxn(), store.QueryAND{
		store.QueryCond{Field: "RouteType", Val: routeType},
		store.QueryCond{Field: "IsActive", Val: uint64(1)},
	})
	if err != nil {
		return nil, err
	}
	if uint64(len(ids)) <= offset {
		return nil, nil
	}
	end := offset + uint64(limit)
	if end > uint64(len(ids)) {
		end = uint64(len(ids))
	}

	txn := g.api.GetTxn()
	out := make([]ModelRouteWithID, 0, end-offset)
	for _, id := range ids[offset:end] {
		r, err := g.routes.Get(txn, id)
		if err != nil || r == nil {
			continue
		}
		out = append(out, ModelRouteWithID{ID: id, Data: *r})
	}
	return out, nil
}

func getRouterTypeOfModel(providerType uint16) uint16 {
	if providerType < 9999 {
		return MiningPoolRoute
	}
	if providerType == 9999 {
		return EdgeRoute
	}
	return TEERoute
}
