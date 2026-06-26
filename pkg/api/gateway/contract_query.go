package gateway

import (
	"github.com/computeflux/tee-provider/pkg/types/chaintypes"
	teechain "github.com/computeflux/tee-provider/tee-chain"
	"github.com/computeflux/tee-provider/tee-chain/contracts"
	cgateway "github.com/computeflux/tee-provider/tee-chain/contracts/module/gateway"
)

// runGatewayQuery 构造 GatewayQuery 并执行 fn，统一处理合约 runtime 的创建与回滚。
func runGatewayQuery[T any](chain *teechain.TeeChain, fn func(cgateway.GatewayQuery) (T, error)) (T, error) {
	return teechain.WithRuntime(chain, func(runtime *contracts.Runtime) (T, error) {
		return fn(cgateway.GatewayQuery{Gateway: *cgateway.NewGateway(runtime)})
	})
}

// GatewayListActiveModelsWithinProvider 按上下文长度和价格区间查询链上活跃模型列表。
func GatewayListActiveModelsWithinProvider(
	chain *teechain.TeeChain,
	minContext, maxContext, minPrice, maxPrice uint64,
	offset uint64, limit uint32,
) ([]cgateway.ModelInfoWithProvider, error) {
	return runGatewayQuery(chain, func(q cgateway.GatewayQuery) ([]cgateway.ModelInfoWithProvider, error) {
		return q.ListActiveModelsWithinProvider(minContext, maxContext, minPrice, maxPrice, offset, limit)
	})
}

// GatewayGetCredit 查询用户剩余积分（credits - credit_useds）。
func GatewayGetCredit(chain *teechain.TeeChain, user chaintypes.UniAddr) (int64, error) {
	return runGatewayQuery(chain, func(q cgateway.GatewayQuery) (int64, error) {
		return q.GetCredit(user)
	})
}
