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

// legacyRechargeHistoryV33 用 V33 编码（无 BlockHeight）写入与 gateway.rechargeHistory
// 完全相同的存储前缀，以模拟 V34 之前的链上数据。
func legacyRechargeHistoryV33() *store.StoreList2D[store.UniAddr, uint32, gateway.RechargeRecordV33] {
	return store.NewStoreList2D[store.UniAddr, uint32, gateway.RechargeRecordV33](
		"gateway",
		"recharge_history_k1_to_id_",
		"recharge_history_k1_length_",
		"recharge_history_k2_next_",
		"recharge_history_store_",
	)
}

// legacyUserSettlementHistoryV33 用旧版 UserSettlementRecordV33 编码（无 BlockHeight）
// 写入与 gateway.userSettlementHistory 完全相同的存储前缀，以模拟 V34 之前的链上数据。
func legacyUserSettlementHistoryV33() *store.StoreList2D[store.UniAddr, uint32, gateway.UserSettlementRecordV33] {
	return store.NewStoreList2D[store.UniAddr, uint32, gateway.UserSettlementRecordV33](
		"gateway",
		"user_settlement_history_k1_to_id_",
		"user_settlement_history_k1_length_",
		"user_settlement_history_k2_next_",
		"user_settlement_history_store_",
	)
}

// TestV34_MigrateRechargeHistory 将 V33 编码的充值记录原地迁移为含 BlockHeight 的新编码。
func TestV34_MigrateRechargeHistory(t *testing.T) {
	rt := testutil.NewMockRuntime()
	rt.System = true
	txn := rt.GetTxn()

	user := store.UniAddr{T: 1, V: []byte{0xaa}}
	other := store.UniAddr{T: 1, V: []byte{0xbb}}

	legacy := legacyRechargeHistoryV33()
	require.NoError(t, func() error {
		for i := 0; i < 3; i++ {
			_, err := legacy.Insert(txn, user, gateway.RechargeRecordV33{
				User: user, Amount: uint64(100 * (i + 1)), Time: int64(i + 1),
				ChainID: 420420417, Nonce: uint64(i + 1),
			})
			if err != nil {
				return err
			}
		}
		_, err := legacy.Insert(txn, other, gateway.RechargeRecordV33{
			User: other, Amount: 500, Time: 9,
		})
		return err
	}())

	// 运行迁移
	gm := govmod.GovMutation{Gov: *govmod.NewGov(rt)}
	require.NoError(t, gm.Migration(34))

	// 新编码可被公开查询 API 正常解码，且字段保持一致
	q := gateway.GatewayQuery{Gateway: *gateway.NewGateway(rt)}
	records, err := q.GetRechargeHistory(user, 0, 10)
	require.NoError(t, err)
	require.Len(t, records, 3)

	assert.Equal(t, uint64(300), records[0].Amount)
	assert.Equal(t, int64(3), records[0].Time)
	assert.Equal(t, uint32(420420417), records[0].ChainID)
	assert.Equal(t, uint64(3), records[0].Nonce)
	assert.Equal(t, uint64(100), records[2].Amount)
	for _, r := range records {
		assert.Equal(t, user, r.User)
		// 旧记录无法反推写入高度，BlockHeight 保持 0
		assert.Equal(t, uint64(0), r.BlockHeight)
	}

	otherRecords, err := q.GetRechargeHistory(other, 0, 10)
	require.NoError(t, err)
	require.Len(t, otherRecords, 1)
	assert.Equal(t, uint64(500), otherRecords[0].Amount)
	assert.Equal(t, uint64(0), otherRecords[0].BlockHeight)
}

// TestV34_MigrateUserSettlementHistory 将旧编码的用户结算记录原地迁移为含 BlockHeight 的新编码。
func TestV34_MigrateUserSettlementHistory(t *testing.T) {
	rt := testutil.NewMockRuntime()
	rt.System = true
	txn := rt.GetTxn()

	user := store.UniAddr{T: 1, V: []byte{0xaa}}

	legacy := legacyUserSettlementHistoryV33()
	require.NoError(t, func() error {
		for i := 0; i < 2; i++ {
			_, err := legacy.Insert(txn, user, gateway.UserSettlementRecordV33{
				ProviderID: 7, ModelID: 1,
				TokenIn: 100, TokenOut: 50, CachedToken: 10,
				Cost: 200, Count: 1, Time: int64(i + 1),
			})
			if err != nil {
				return err
			}
		}
		return nil
	}())

	gm := govmod.GovMutation{Gov: *govmod.NewGov(rt)}
	require.NoError(t, gm.Migration(34))

	q := gateway.GatewayQuery{Gateway: *gateway.NewGateway(rt)}
	records, err := q.GetUserSettlementHistory(user, 0, 10)
	require.NoError(t, err)
	require.Len(t, records, 2)

	assert.Equal(t, uint64(7), records[0].ProviderID)
	assert.Equal(t, uint64(200), records[0].Cost)
	for _, r := range records {
		// 旧记录无法反推写入高度，BlockHeight 保持 0
		assert.Equal(t, uint64(0), r.BlockHeight)
	}
}

// TestV34_Idempotent 迁移重复执行不得抹掉已写入的 BlockHeight。
func TestV34_Idempotent(t *testing.T) {
	rt := testutil.NewMockRuntime()
	rt.System = true
	txn := rt.GetTxn()

	user := store.UniAddr{T: 1, V: []byte{0xaa}}
	legacy := legacyRechargeHistoryV33()
	_, err := legacy.Insert(txn, user, gateway.RechargeRecordV33{
		User: user, Amount: 7, Time: 1, ChainID: 420420417, Nonce: 1,
	})
	require.NoError(t, err)

	gm := govmod.GovMutation{Gov: *govmod.NewGov(rt)}
	require.NoError(t, gm.Migration(34))

	// 迁移后写入一笔跨链充值（新编码，带区块高度）
	rt.Height = 999
	require.NoError(t, gateway.GatewayMutation{Gateway: *gateway.NewGateway(rt)}.
		Recharge(user, 42, 420420417, 2))

	// 再次执行迁移：旧记录不再变化，新记录必须保留区块高度
	require.NoError(t, gm.Migration(34))

	q := gateway.GatewayQuery{Gateway: *gateway.NewGateway(rt)}
	records, err := q.GetRechargeHistory(user, 0, 10)
	require.NoError(t, err)
	require.Len(t, records, 2)

	assert.Equal(t, uint64(42), records[0].Amount)
	assert.Equal(t, uint64(999), records[0].BlockHeight)

	assert.Equal(t, uint64(7), records[1].Amount)
	assert.Equal(t, uint64(0), records[1].BlockHeight)
}

// TestV34_EmptyStore 空存储迁移不报错。
func TestV34_EmptyStore(t *testing.T) {
	rt := testutil.NewMockRuntime()
	rt.System = true
	gm := govmod.GovMutation{Gov: *govmod.NewGov(rt)}
	require.NoError(t, gm.Migration(34))
}
