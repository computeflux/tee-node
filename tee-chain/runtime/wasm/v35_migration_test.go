package wasm

import (
	"encoding/binary"
	"testing"

	"github.com/computeflux/tee-provider/tee-chain/runtime/module/gateway"
	"github.com/computeflux/tee-provider/tee-chain/runtime/store"
	"github.com/stretchr/testify/require"
)

// systemRuntime 让 gov.migration 通过 CallbySystem 校验。
type systemRuntime struct {
	*testRuntime
}

func (r *systemRuntime) CallbySystem() bool { return true }

// TestV35_MigrationLargeHistoryViaWasm 回归：V35 迁移在 WASM 路径下逐条点查，
// 大量存量记录（3000 条）也不会再因 storage_scan_prefix 的 64KB 共享缓冲区
// 越界而报 `wasm execute: wasm error: unreachable`（V34 全量 ListByPrefix 会）。
func TestV35_MigrationLargeHistoryViaWasm(t *testing.T) {
	const n = 3000

	txn := newMockStoreTx()
	api := &systemRuntime{&testRuntime{
		txn:    txn,
		height: 1,
		sudo:   store.UniAddr{T: 1, V: []byte("sudo")},
	}}

	legacy := store.NewStoreList2D[store.UniAddr, uint32, gateway.RechargeRecordV33](
		"gateway",
		"recharge_history_k1_to_id_",
		"recharge_history_k1_length_",
		"recharge_history_k2_next_",
		"recharge_history_store_",
	)
	user := store.UniAddr{T: 1, V: []byte("user")}
	for i := 0; i < n; i++ {
		_, err := legacy.Insert(txn, user, gateway.RechargeRecordV33{
			User: user, Amount: uint64(i + 1), Time: int64(i + 1),
			ChainID: 420420417, Nonce: uint64(i + 1),
		})
		require.NoError(t, err)
	}

	e := NewEngine()
	loadCachedWasm(t, e)

	ver := make([]byte, 4)
	binary.LittleEndian.PutUint32(ver, 35)
	require.NoError(t, e.ExecuteMutation(0, &store.InContractCall{
		Name:   []byte("gov"),
		Method: []byte{0xa6, 0x2c, 0x84, 0xf8}, // gov.migration
		Args:   [][]byte{ver},
	}, api))

	// 迁移后记录必须是新编码（旧编码会 EOF reading BlockHeight）
	raw, err := txn.Get(legacy.StorageKey(0, 0))
	require.NoError(t, err)
	require.NotEmpty(t, raw)

	var rec gateway.RechargeRecord
	nn, err := rec.Decode(raw)
	require.NoError(t, err)
	require.Equal(t, len(raw), nn)
	require.Equal(t, uint64(1), rec.Amount)
	require.Equal(t, uint64(0), rec.BlockHeight)
}
