// Package contractquery 提供对 tee-chain gateway 合约模块的查询封装，
// 供 pkg/indexer、pkg/api/gateway 等上层包复用。
package contractquery

import (
	"github.com/computeflux/tee-provider/pkg/types/chaintypes"
	teechain "github.com/computeflux/tee-provider/tee-chain"
	contracts "github.com/computeflux/tee-provider/tee-chain/runtime"
	cgateway "github.com/computeflux/tee-provider/tee-chain/runtime/module/gateway"
)

// runGatewayQuery 构造 GatewayQuery 并执行 fn，统一处理合约 runtime 的创建与回滚。
func runGatewayQuery[T any](chain *teechain.TeeChain, fn func(cgateway.GatewayQuery) (T, error)) (T, error) {
	return teechain.WithRuntime(chain, func(runtime *contracts.Runtime) (T, error) {
		return fn(cgateway.GatewayQuery{Gateway: *cgateway.NewGateway(runtime)})
	})
}

// runGatewaySecret 构造 GatewaySecret 并执行 fn，返回未脱敏数据（仅 TEE 内部使用）。
func runGatewaySecret[T any](chain *teechain.TeeChain, fn func(cgateway.GatewaySecret) (T, error)) (T, error) {
	return teechain.WithRuntime(chain, func(runtime *contracts.Runtime) (T, error) {
		return fn(cgateway.GatewaySecret{Gateway: *cgateway.NewGateway(runtime)})
	})
}

// GatewayListActiveModelsWithinProvider 按上下文长度和价格区间查询链上活跃模型列表。
// 返回的 APIKey 已被脱敏，仅供公开查询 / GraphQL 使用。
func GatewayListActiveModelsWithinProvider(
	chain *teechain.TeeChain,
	minContext, maxContext, minPrice, maxPrice uint64,
	offset uint64, limit uint32,
) ([]cgateway.ModelInfoWithProvider, error) {
	return runGatewayQuery(chain, func(q cgateway.GatewayQuery) ([]cgateway.ModelInfoWithProvider, error) {
		return q.ListActiveModelsWithinProvider(minContext, maxContext, minPrice, maxPrice, offset, limit)
	})
}

// GatewaySecretListActiveModelsWithinProvider 查询链上活跃模型列表并返回未脱敏的完整 APIKey。
// 仅 scheduler 等内部组件使用，不得暴露给外部 API。
func GatewaySecretListActiveModelsWithinProvider(chain *teechain.TeeChain) ([]cgateway.ModelInfoWithProvider, error) {
	// 先通过公开查询拿到活跃模型列表与 Provider 信息（已脱敏）。
	models, err := runGatewayQuery(chain, func(q cgateway.GatewayQuery) ([]cgateway.ModelInfoWithProvider, error) {
		return q.ListActiveModelsWithinProvider(0, 0, 0, 0, 0, 100000)
	})
	if err != nil {
		return nil, err
	}

	out := make([]cgateway.ModelInfoWithProvider, 0, len(models))
	for _, m := range models {
		secret, err := runGatewaySecret(chain, func(s cgateway.GatewaySecret) (*cgateway.ModelInfoV2, error) {
			return s.GetModel(m.ID)
		})
		if err != nil || secret == nil {
			continue
		}
		out = append(out, cgateway.ModelInfoWithProvider{
			ID:       m.ID,
			Model:    *secret,
			Provider: m.Provider,
		})
	}
	return out, nil
}

// GatewayGetCredit 查询用户剩余积分（credits - credit_useds）。
func GatewayGetCredit(chain *teechain.TeeChain, user chaintypes.UniAddr) (int64, error) {
	return runGatewayQuery(chain, func(q cgateway.GatewayQuery) (int64, error) {
		return q.GetCredit(user)
	})
}

// GatewayListAllRoutes 查询链上全部路由列表。
func GatewayListAllRoutes(chain *teechain.TeeChain, offset uint64, limit uint32) ([]cgateway.ModelRouteWithID, error) {
	return runGatewayQuery(chain, func(q cgateway.GatewayQuery) ([]cgateway.ModelRouteWithID, error) {
		return q.ListRoutes(offset, limit)
	})
}
