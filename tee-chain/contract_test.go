package teechain

import (
	"encoding/binary"
	"encoding/hex"
	"testing"

	"github.com/computeflux/tee-provider/tee-chain/runtime/module"
	"github.com/computeflux/tee-provider/tee-chain/runtime/store"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestRuntimeUpgradeSelector 回归：selector 必须与 abis/gov.json 一致，
// 否则升级交易不会被识别为 runtime_upgrade，无法走原生落库路径。
func TestRuntimeUpgradeSelector(t *testing.T) {
	assert.Equal(t, "a4771d9e", hex.EncodeToString(runtimeUpgradeSelector))
}

func TestRuntimeUpgradeTargetVersion(t *testing.T) {
	payload := make([]byte, 8)
	binary.LittleEndian.PutUint32(payload, 36)

	call := &store.InContractCall{
		Name:   []byte("gov"),
		Method: runtimeUpgradeSelector,
		Args:   [][]byte{payload},
	}
	v, ok := runtimeUpgradeTargetVersion(call)
	require.True(t, ok)
	assert.Equal(t, uint32(36), v)

	// 非 gov 合约
	_, ok = runtimeUpgradeTargetVersion(&store.InContractCall{
		Name:   []byte("gateway"),
		Method: runtimeUpgradeSelector,
		Args:   [][]byte{payload},
	})
	assert.False(t, ok)

	// 非 runtime_upgrade 方法
	_, ok = runtimeUpgradeTargetVersion(&store.InContractCall{
		Name:   []byte("gov"),
		Method: store.MethodToSelectorBytes("transfer"),
		Args:   [][]byte{payload},
	})
	assert.False(t, ok)

	// 参数缺失/过短
	_, ok = runtimeUpgradeTargetVersion(&store.InContractCall{
		Name:   []byte("gov"),
		Method: runtimeUpgradeSelector,
	})
	assert.False(t, ok)
	_, ok = runtimeUpgradeTargetVersion(&store.InContractCall{
		Name:   []byte("gov"),
		Method: runtimeUpgradeSelector,
		Args:   [][]byte{{0x01, 0x02}},
	})
	assert.False(t, ok)
}

// TestNativeVersionForCall 目标版本等于本二进制 NativeVersion 时走原生路径，
// 避免大体积 WASM 载荷经 256KB 共享缓冲区被截断。
func TestNativeVersionForCall(t *testing.T) {
	payload := make([]byte, 8)
	binary.LittleEndian.PutUint32(payload, module.NativeVersion)
	upgradeCall := &store.InContractCall{
		Name:   []byte("gov"),
		Method: runtimeUpgradeSelector,
		Args:   [][]byte{payload},
	}
	assert.Equal(t, module.NativeVersion, nativeVersionForCall(5, upgradeCall))

	// 目标版本与 NativeVersion 不同 → 保持链上版本（走 WASM）
	binary.LittleEndian.PutUint32(payload, module.NativeVersion+1)
	assert.Equal(t, uint32(5), nativeVersionForCall(5, upgradeCall))

	// 普通合约调用 → 保持链上版本
	assert.Equal(t, uint32(5), nativeVersionForCall(5, &store.InContractCall{
		Name:   []byte("gov"),
		Method: store.MethodToSelectorBytes("transfer"),
	}))
}
