package migrations_test

import (
	"encoding/json"
	"testing"

	"github.com/computeflux/tee-provider/tee-chain/runtime/module/gateway"
	govmod "github.com/computeflux/tee-provider/tee-chain/runtime/module/gov"
	"github.com/computeflux/tee-provider/tee-chain/runtime/store"
	"github.com/computeflux/tee-provider/tee-chain/runtime/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestV20_MigrateModels 将旧版 models(ModelInfo 编码)原地迁移为 modelsV2(ModelInfoV2 编码)。
func TestV20_MigrateModels(t *testing.T) {
	rt := testutil.NewMockRuntime()
	rt.System = true
	txn := rt.GetTxn()

	// 模拟旧链上数据: models 字段已下线, 用同前缀 StoreMapping 按 ModelInfo 原始编码写入
	old := &store.StoreMapping[uint64, gateway.ModelInfo]{
		Namespace: "gateway",
		KeyPrefix: "models_",
	}
	d1 := gateway.ModelInfo{
		Name:            "gpt-4o",
		Owner:           store.UniAddr{T: 1, V: []byte{0xaa}},
		ProviderID:      7,
		InputModalities: 3,
		ContextLength:   128000,
		InPrice:         1000,
		CachedPrice:     500,
		OutPrice:        2000,
		RouteID:         0,
		ProviderType:    1,
		ModelID:         "legacy-model-id",
		URI:             "https://example.com/v1",
		APIKeys:         []string{"key-a"},
		MaxConcurrency:  8,
		Balance:         999,
		ReputationScore: 9000,
		TotalAudits:     10,
		PassedAudits:    9,
		IsActive:        true,
		CreatedAt:       12345,
	}
	require.NoError(t, old.Set(txn, 0, d1))

	// 设置 next_id = 1（旧 StoreList 的计数）
	nextID := &store.StoreValue[uint64]{Namespace: "gateway", Key: "models_next_id"}
	require.NoError(t, nextID.Set(txn, 1))

	// 运行迁移
	gm := govmod.GovMutation{Gov: *govmod.NewGov(rt)}
	require.NoError(t, gm.Migration(20))

	// 验证: 数据现在能被 modelsV2 按 ModelInfoV2 解码（通过公开查询 API）
	base := gateway.NewGateway(rt)
	q := gateway.GatewayQuery{Gateway: *base}
	v2, err := q.GetModel(0)
	require.NoError(t, err)
	require.NotNil(t, v2)

	// 相同字段已迁移
	assert.Equal(t, "gpt-4o", v2.Name)
	assert.Equal(t, d1.Owner, v2.Owner)
	assert.Equal(t, uint64(7), v2.ProviderID)
	assert.Equal(t, uint8(3), v2.InputModalities)
	assert.Equal(t, uint64(128000), v2.ContextLength)
	assert.Equal(t, uint64(1000), v2.InPrice)
	assert.Equal(t, uint64(500), v2.CachedPrice)
	assert.Equal(t, uint64(2000), v2.OutPrice)
	assert.Equal(t, uint64(0), v2.RouteID)
	// URI → URIS: 迁移时打包成多协议 map[uint16]string 的 JSON(旧单协议 → map[providerType]url)
	assert.Equal(t, `{"1":"https://example.com/v1"}`, v2.URIS)
	assert.Equal(t, []string{"key-a"}, v2.APIKeys)
	assert.Equal(t, uint32(8), v2.MaxConcurrency)
	assert.Equal(t, uint64(999), v2.Balance)
	assert.Equal(t, uint32(9000), v2.ReputationScore)
	assert.Equal(t, uint64(10), v2.TotalAudits)
	assert.Equal(t, uint64(9), v2.PassedAudits)
	assert.Equal(t, true, v2.IsActive)
	assert.Equal(t, uint64(12345), v2.CreatedAt)

	// 迁移填充的 V2 字段: 多协议 map 可解析、默认 thinking、按 ProviderType 推断路由类型
	var urlMap map[uint16]string
	require.NoError(t, json.Unmarshal([]byte(v2.URIS), &urlMap))
	assert.Equal(t, "https://example.com/v1", urlMap[gateway.OpenAI])
	assert.True(t, v2.Thinking)
	assert.Equal(t, "", v2.Meta)
	// ProviderType=1(<10) → 推断为 MiningPoolRoute=2
	assert.Equal(t, uint16(2), v2.ModelType)

	// 模型计数与查询保持一致
	count, err := q.GetModelCount()
	require.NoError(t, err)
	assert.Equal(t, uint64(1), count)
}

// TestV20_EmptyStore 空存储迁移不报错。
func TestV20_EmptyStore(t *testing.T) {
	rt := testutil.NewMockRuntime()
	rt.System = true
	gm := govmod.GovMutation{Gov: *govmod.NewGov(rt)}
	require.NoError(t, gm.Migration(20))
}
