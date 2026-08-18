package migrations

import (
	"encoding/json"
	"fmt"

	"github.com/computeflux/tee-provider/tee-chain/runtime/module/gateway"
	"github.com/computeflux/tee-provider/tee-chain/runtime/store"
)

// V20 将旧版 models(ModelInfo) 数据原地迁移到 modelsV2(ModelInfoV2) 编码。
//
// 迁移背景: modelsV2 与旧 models 共用同一存储前缀(gateway/models_、models_next_id),
// 因此这是"同键原地重写编码": 读出现有 ModelInfo(仍以旧编码存于 models_ 前缀)→
// 转换为 ModelInfoV2 → 经 modelsV2 写回原 key。二者索引字段(Name/ProviderID 等 idx tag)
// 完全一致, 无需重建索引。
//
// 字段映射: 仅迁移完全同名的字段; ModelInfoV2 新增字段(Thinking/Meta/ModelType)
// 本次保留零值, 后续再按需填充。
func V20(api store.ContractApi) error {
	txn := api.GetTxn()

	// 读取旧 models 数据: models 字段已下线, 用同前缀的 StoreMapping 按 ModelInfo 解码
	old := &store.StoreMapping[uint64, gateway.ModelInfo]{
		Namespace: "gateway",
		KeyPrefix: "models_",
	}
	keys, vals, err := old.List(txn)
	if err != nil {
		return err
	}

	fmt.Printf("[V20] models total: %d\n", len(vals))

	// 注意: 不能用 modelsV2.Update —— 它会先按 ModelInfoV2 解码当前位置(旧数据仍是
	// ModelInfo 编码, 会解码失败)。modelsV2.Insert 会重新分配 id, 也不可用。
	// 因此这里直接通过同前缀 StoreMapping 编码写入 ModelInfoV2。
	v2raw := &store.StoreMapping[uint64, gateway.ModelInfoV2]{
		Namespace: "gateway",
		KeyPrefix: "models_",
	}

	migrated := 0
	for i, key := range keys {
		// ModelType 必须是路由类型：由旧 ProviderType(协议类型) 映射
		modelType := gateway.RouterTypeOfModel(vals[i].ProviderType)

		// 仅迁移相同字段; V2 新增字段保持零值
		v2 := gateway.ModelInfoV2{
			Name:            vals[i].Name,
			Owner:           vals[i].Owner,
			ProviderID:      vals[i].ProviderID,
			InputModalities: vals[i].InputModalities,
			ContextLength:   vals[i].ContextLength,
			InPrice:         vals[i].InPrice,
			CachedPrice:     vals[i].CachedPrice,
			OutPrice:        vals[i].OutPrice,
			RouteID:         vals[i].RouteID,
			APIKeys:         vals[i].APIKeys,
			MaxConcurrency:  vals[i].MaxConcurrency,
			Balance:         vals[i].Balance,
			ReputationScore: vals[i].ReputationScore,
			TotalAudits:     vals[i].TotalAudits,
			PassedAudits:    vals[i].PassedAudits,
			IsActive:        vals[i].IsActive,
			CreatedAt:       vals[i].CreatedAt,

			// ModelInfoV2 新增: Thinking/Meta/ModelType 保持零值
			ModelType: modelType,
			Thinking:  true,
			URIS:      getUrls(vals[i].ProviderType, vals[i].URI),
			Meta:      "",
		}

		if err := v2raw.Set(txn, key, v2); err != nil {
			return fmt.Errorf("set model %d: %w", key, err)
		}
		migrated++
	}

	fmt.Printf("[V20] migrated: %d / %d\n", migrated, len(vals))
	return nil
}

func getUrls(providerType uint16, url string) string {
	urls := map[uint16]string{
		providerType: url,
	}

	bt, _ := json.Marshal(urls)
	return string(bt)
}
