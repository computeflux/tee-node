package wasm

import (
	"bytes"
	"encoding/binary"
	"testing"

	govmod "github.com/computeflux/tee-provider/tee-chain/runtime/module/gov"
	"github.com/computeflux/tee-provider/tee-chain/runtime/store"
	"github.com/stretchr/testify/require"
)

// TestRuntimeUpgradeLargeWasmViaWasm 回归：共享缓冲区扩大后，包含 ~1MB WASM
// 的整个 PendingRuntimeUpgrade 能经 WASM 路径完整写入并读回。
// 旧 256KB 缓冲区会静默截断 value，链上 WASM gunzip 报 unexpected EOF。
func TestRuntimeUpgradeLargeWasmViaWasm(t *testing.T) {
	const wasmSize = 1 << 20 // 1MB

	txn := newMockStoreTx()
	api := &systemRuntime{&testRuntime{
		txn:    txn,
		height: 100,
		sudo:   store.UniAddr{T: 1, V: []byte("sudo")},
	}}

	wasmBytes := make([]byte, wasmSize)
	for i := range wasmBytes {
		wasmBytes[i] = byte(i * 31)
	}

	upgrade := govmod.PendingRuntimeUpgrade{
		Version:     1, // 链上 runtime_version 初始为 0，目标必须为 current+1
		CodeHash:    bytes.Repeat([]byte{0xAB}, 32),
		WasmBytes:   wasmBytes,
		Description: "large wasm regression test",
	}
	payload, err := store.ScaleEncode(upgrade)
	require.NoError(t, err)
	require.Greater(t, len(payload), wasmSize)

	e := NewEngine()
	loadCachedWasm(t, e)

	require.NoError(t, e.ExecuteMutation(0, &store.InContractCall{
		Name:   []byte("gov"),
		Method: []byte{0xa4, 0x77, 0x1d, 0x9e}, // runtime_upgrade
		Args:   [][]byte{payload},
	}, api))

	// 写入必须完整：长度和内容都要一致（旧缓冲会截断）
	raw, err := txn.Get([]byte("gov_runtime_meta_1"))
	require.NoError(t, err)
	require.Equal(t, len(payload), len(raw))

	var got govmod.PendingRuntimeUpgrade
	require.NoError(t, store.ScaleDecode(&got, raw))
	require.Equal(t, upgrade.Version, got.Version)
	require.Equal(t, upgrade.CodeHash, got.CodeHash)
	require.Equal(t, upgrade.Description, got.Description)
	require.Equal(t, uint64(100), got.ActivationHeight)
	require.Equal(t, wasmBytes, got.WasmBytes)

	// 读回路径：runtime_meta 查询需把 1MB value 从宿主传到 guest
	versionArg := make([]byte, 4)
	binary.LittleEndian.PutUint32(versionArg, 1)
	out, err := e.ExecuteQuery(0, &store.InContractCall{
		Name:   []byte("gov"),
		Method: []byte{0x6f, 0x29, 0xbd, 0xa3}, // runtime_meta
		Args:   [][]byte{versionArg},
	}, api)
	require.NoError(t, err)
	require.Greater(t, len(out), wasmSize)
}
