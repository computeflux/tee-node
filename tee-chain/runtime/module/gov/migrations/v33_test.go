package migrations_test

import (
	"testing"

	"github.com/computeflux/tee-provider/tee-chain/runtime/module/gateway"
	govmod "github.com/computeflux/tee-provider/tee-chain/runtime/module/gov"
	"github.com/computeflux/tee-provider/tee-chain/runtime/store"
	"github.com/computeflux/tee-provider/tee-chain/runtime/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// legacyRechargeHistory 用旧版 RechargeRecordV1 编码写入与 gateway.rechargeHistory
// 完全相同的存储前缀，以模拟 V33 之前的链上数据。
func legacyRechargeHistory() *store.StoreList2D[store.UniAddr, uint32, gateway.RechargeRecordV1] {
	return store.NewStoreList2D[store.UniAddr, uint32, gateway.RechargeRecordV1](
		"gateway",
		"recharge_history_k1_to_id_",
		"recharge_history_k1_length_",
		"recharge_history_k2_next_",
		"recharge_history_store_",
	)
}

// TestV33_MigrateRechargeHistory 将旧版 RechargeRecord 编码原地迁移为含 ChainID/Nonce 的新编码。
func TestV33_MigrateRechargeHistory(t *testing.T) {
	rt := testutil.NewMockRuntime()
	rt.System = true
	txn := rt.GetTxn()

	user := store.UniAddr{T: 1, V: []byte{0xaa}}
	other := store.UniAddr{T: 1, V: []byte{0xbb}}

	legacy := legacyRechargeHistory()
	require.NoError(t, func() error {
		for i := 0; i < 3; i++ {
			_, err := legacy.Insert(txn, user, gateway.RechargeRecordV1{
				User: user, Amount: uint64(100 * (i + 1)), Time: int64(i + 1),
			})
			if err != nil {
				return err
			}
		}
		_, err := legacy.Insert(txn, other, gateway.RechargeRecordV1{
			User: other, Amount: 500, Time: 9,
		})
		return err
	}())

	// 运行迁移
	gm := govmod.GovMutation{Gov: *govmod.NewGov(rt)}
	require.NoError(t, gm.Migration(33))

	// 新编码可被公开查询 API 正常解码，且字段保持一致
	q := gateway.GatewayQuery{Gateway: *gateway.NewGateway(rt)}
	records, err := q.GetRechargeHistory(user, 0, 10)
	require.NoError(t, err)
	require.Len(t, records, 3)

	assert.Equal(t, uint64(300), records[0].Amount)
	assert.Equal(t, int64(3), records[0].Time)
	assert.Equal(t, uint64(100), records[2].Amount)
	for _, r := range records {
		assert.Equal(t, user, r.User)
		// 旧记录无法反推来源链，链信息保持 0
		assert.Equal(t, uint32(0), r.ChainID)
		assert.Equal(t, uint64(0), r.Nonce)
	}

	otherRecords, err := q.GetRechargeHistory(other, 0, 10)
	require.NoError(t, err)
	require.Len(t, otherRecords, 1)
	assert.Equal(t, uint64(500), otherRecords[0].Amount)
}

// TestV33_Idempotent 迁移重复执行不得抹掉已写入的 ChainID/Nonce。
func TestV33_Idempotent(t *testing.T) {
	rt := testutil.NewMockRuntime()
	rt.System = true
	txn := rt.GetTxn()

	user := store.UniAddr{T: 1, V: []byte{0xaa}}
	legacy := legacyRechargeHistory()
	_, err := legacy.Insert(txn, user, gateway.RechargeRecordV1{User: user, Amount: 7, Time: 1})
	require.NoError(t, err)

	gm := govmod.GovMutation{Gov: *govmod.NewGov(rt)}
	require.NoError(t, gm.Migration(33))

	// 迁移后写入一笔跨链充值（新编码，带链信息）
	rt.System = true
	require.NoError(t, gateway.GatewayMutation{Gateway: *gateway.NewGateway(rt)}.
		Recharge(user, 42, 420420417, 8))

	// 再次执行迁移：旧记录不再变化，新记录必须保留链信息
	require.NoError(t, gm.Migration(33))

	q := gateway.GatewayQuery{Gateway: *gateway.NewGateway(rt)}
	records, err := q.GetRechargeHistory(user, 0, 10)
	require.NoError(t, err)
	require.Len(t, records, 2)

	assert.Equal(t, uint64(42), records[0].Amount)
	assert.Equal(t, uint32(420420417), records[0].ChainID)
	assert.Equal(t, uint64(8), records[0].Nonce)

	assert.Equal(t, uint64(7), records[1].Amount)
	assert.Equal(t, uint32(0), records[1].ChainID)
	assert.Equal(t, uint64(0), records[1].Nonce)
}

// TestV33_EmptyStore 空存储迁移不报错。
func TestV33_EmptyStore(t *testing.T) {
	rt := testutil.NewMockRuntime()
	rt.System = true
	gm := govmod.GovMutation{Gov: *govmod.NewGov(rt)}
	require.NoError(t, gm.Migration(33))
}
