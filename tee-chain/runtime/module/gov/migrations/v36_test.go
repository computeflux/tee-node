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

// TestV36_RerunsV35 验证 V36 会重跑 V35 的点查迁移（充值 + 用户结算）。
// 用于 35 升级时 WASM 落库被截断、迁移未执行的恢复。
func TestV36_RerunsV35(t *testing.T) {
	rt := testutil.NewMockRuntime()
	rt.System = true
	txn := rt.GetTxn()

	user := store.UniAddr{T: 1, V: []byte{0xaa}}

	legacyRecharge := legacyRechargeHistoryV33()
	_, err := legacyRecharge.Insert(txn, user, gateway.RechargeRecordV33{
		User: user, Amount: 42, Time: 7, ChainID: 420420417, Nonce: 3,
	})
	require.NoError(t, err)

	legacySettlement := legacyUserSettlementHistoryV33()
	_, err = legacySettlement.Insert(txn, user, gateway.UserSettlementRecordV33{
		ProviderID: 7, ModelID: 1, Cost: 200, Count: 1, Time: 7,
	})
	require.NoError(t, err)

	gm := govmod.GovMutation{Gov: *govmod.NewGov(rt)}
	require.NoError(t, gm.Migration(36))

	q := gateway.GatewayQuery{Gateway: *gateway.NewGateway(rt)}

	recharges, err := q.GetRechargeHistory(user, 0, 10)
	require.NoError(t, err)
	require.Len(t, recharges, 1)
	assert.Equal(t, uint64(42), recharges[0].Amount)
	assert.Equal(t, uint32(420420417), recharges[0].ChainID)
	assert.Equal(t, uint64(0), recharges[0].BlockHeight)

	settlements, err := q.GetUserSettlementHistory(user, 0, 10)
	require.NoError(t, err)
	require.Len(t, settlements, 1)
	assert.Equal(t, uint64(200), settlements[0].Cost)
	assert.Equal(t, uint64(0), settlements[0].BlockHeight)

	// 幂等：再次执行不报错、不改变结果
	require.NoError(t, gm.Migration(36))
	recharges, err = q.GetRechargeHistory(user, 0, 10)
	require.NoError(t, err)
	require.Len(t, recharges, 1)
}
