package indexer

import (
	"context"
	"fmt"

	localtunnel "github.com/computeflux/tee-provider/pkg/services/local_tunnel"
	cgateway "github.com/computeflux/tee-provider/tee-chain/runtime/module/gateway"
)

// ListModels 返回当前可调用的模型列表（与 GraphQL Models 查询同一数据路径）。
// 个人算力（9999）必须存在可用本地隧道连接才展示；第三方模型必须配置 APIKey。
func (idx *Indexer) ListModels(ctx context.Context, req GatewayModelSearchRequest) ([]cgateway.ModelInfoWithProvider, error) {
	modelIdxRaw, err := idx.Get(GatewayModelIndexName)
	if err != nil {
		return nil, err
	}
	modelIdx, ok := modelIdxRaw.(*GatewayModelIndex)
	if !ok {
		return nil, fmt.Errorf("invalid gateway model index type")
	}
	models, err := modelIdx.Search(ctx, req)
	if err != nil {
		return nil, err
	}

	pool := localtunnel.GetGlobalPool()
	out := make([]cgateway.ModelInfoWithProvider, 0, len(models))
	for _, item := range models {
		m := item.Model
		// 个人算力（ProviderType 9999）必须存在可用本地隧道连接才展示
		if m.ModelType == cgateway.EdgeRoute && !pool.HasEntry(m.ProviderID, m.Name) {
			continue
		}

		// 跳过没有服务的模型
		if m.ModelType != cgateway.EdgeRoute && len(m.APIKeys) == 0 {
			continue
		}

		out = append(out, item)
	}
	return out, nil
}
