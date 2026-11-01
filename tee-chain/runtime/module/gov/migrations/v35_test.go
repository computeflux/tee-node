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

// TestV35_RerunsV34 验证 V35 会重跑 V34 的编码重写（充值 + 用户结算），
// 用于 34 升级时迁移在 WASM 路径失败后的恢复。
func TestV35_RerunsV34(t *testing.T) {
	rt := testutil.NewMockRuntime()
	rt.System = true
	txn := rt.GetTxn()

	user := store.UniAddr{T: 1, V: []byte{0xaa}}

	// 旧充值记录（V33 编码，无 BlockHeight）
	legacyRecharge := legacyRechargeHistoryV33()
	_, err := legacyRecharge.Insert(txn, user, gateway.RechargeRecordV33{
		User: user, Amount: 42, Time: 7, ChainID: 420420417, Nonce: 3,
	})
	require.NoError(t, err)

	// 旧用户结算记录（无 BlockHeight）
	legacySettlement := legacyUserSettlementHistoryV33()
	_, err = legacySettlement.Insert(txn, user, gateway.UserSettlementRecordV33{
		ProviderID: 7, ModelID: 1, Cost: 200, Count: 1, Time: 7,
	})
	require.NoError(t, err)

	gm := govmod.GovMutation{Gov: *govmod.NewGov(rt)}
	require.NoError(t, gm.Migration(35))

	q := gateway.GatewayQuery{Gateway: *gateway.NewGateway(rt)}

	recharges, err := q.GetRechargeHistory(user, 0, 10)
	require.NoError(t, err)
	require.Len(t, recharges, 1)
	assert.Equal(t, uint64(42), recharges[0].Amount)
	assert.Equal(t, uint32(420420417), recharges[0].ChainID)
	assert.Equal(t, uint64(0), recharges[0].BlockHeight) // 旧记录高度未知

	settlements, err := q.GetUserSettlementHistory(user, 0, 10)
	require.NoError(t, err)
	require.Len(t, settlements, 1)
	assert.Equal(t, uint64(200), settlements[0].Cost)
	assert.Equal(t, uint64(0), settlements[0].BlockHeight)
}

// noListStoreTx 统计 ListByPrefix 调用次数：V35 必须逐条点查，
// 不得一次性扫描整个前缀（WASM 路径共享缓冲区只有 64KB，会越界）。
type noListStoreTx struct {
	*testutil.MockStoreTx
	listCalls int
}

func (t *noListStoreTx) ListByPrefix(prefix []byte) ([][]byte, [][]byte, error) {
	t.listCalls++
	return t.MockStoreTx.ListByPrefix(prefix)
}

// TestV35_LargeHistoryNoBulkScan 大量存量记录（各 3000 条）下 V35 仍能完成迁移，
// 且全程不调用 ListByPrefix（原生路径无 64KB 限制，但 WASM 路径必须依赖该约束）。
func TestV35_LargeHistoryNoBulkScan(t *testing.T) {
	const n = 3000

	txn := &noListStoreTx{MockStoreTx: testutil.NewMockStoreTx()}
	rt := testutil.NewMockRuntime()
	rt.System = true
	rt.Txn = txn

	user := store.UniAddr{T: 1, V: []byte{0xaa}}

	legacyRecharge := legacyRechargeHistoryV33()
	for i := 0; i < n; i++ {
		_, err := legacyRecharge.Insert(txn, user, gateway.RechargeRecordV33{
			User: user, Amount: uint64(i + 1), Time: int64(i + 1),
			ChainID: 420420417, Nonce: uint64(i + 1),
		})
		require.NoError(t, err)
	}

	legacySettlement := legacyUserSettlementHistoryV33()
	for i := 0; i < n; i++ {
		_, err := legacySettlement.Insert(txn, user, gateway.UserSettlementRecordV33{
			ProviderID: 7, ModelID: 1, Cost: uint64(i + 1), Count: 1, Time: int64(i + 1),
		})
		require.NoError(t, err)
	}

	gm := govmod.GovMutation{Gov: *govmod.NewGov(rt)}
	require.NoError(t, gm.Migration(35))

	// 关键断言：V35 不得使用 ListByPrefix（否则 WASM 路径会再次越界）
	assert.Zero(t, txn.listCalls, "V35 must not use ListByPrefix")

	q := gateway.GatewayQuery{Gateway: *gateway.NewGateway(rt)}

	recharges, err := q.GetRechargeHistory(user, 0, n)
	require.NoError(t, err)
	require.Len(t, recharges, n)
	assert.Equal(t, uint64(n), recharges[0].Amount)
	assert.Equal(t, uint64(0), recharges[0].BlockHeight)

	settlements, err := q.GetUserSettlementHistory(user, 0, n)
	require.NoError(t, err)
	require.Len(t, settlements, n)
	assert.Equal(t, uint64(n), settlements[0].Cost)
	assert.Equal(t, uint64(0), settlements[0].BlockHeight)
}

// TestV35_Idempotent 重复执行 V35 不得抹掉新记录已写入的 BlockHeight。
func TestV35_Idempotent(t *testing.T) {
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
	require.NoError(t, gm.Migration(35))

	// 迁移后写入一笔带区块高度的新充值
	rt.Height = 12345
	require.NoError(t, gateway.GatewayMutation{Gateway: *gateway.NewGateway(rt)}.
		Recharge(user, 9, 420420417, 4))

	// 再次执行 V35：旧记录跳过，新记录保留高度
	require.NoError(t, gm.Migration(35))

	q := gateway.GatewayQuery{Gateway: *gateway.NewGateway(rt)}
	records, err := q.GetRechargeHistory(user, 0, 10)
	require.NoError(t, err)
	require.Len(t, records, 2)

	assert.Equal(t, uint64(9), records[0].Amount)
	assert.Equal(t, uint64(12345), records[0].BlockHeight)
	assert.Equal(t, uint64(7), records[1].Amount)
	assert.Equal(t, uint64(0), records[1].BlockHeight)
}
