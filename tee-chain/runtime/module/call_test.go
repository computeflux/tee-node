package module

import (
	"testing"

	"github.com/computeflux/tee-provider/tee-chain/runtime/store"
	"github.com/computeflux/tee-provider/tee-chain/runtime/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestQuery_UnknownContract(t *testing.T) {
	rt := testutil.NewMockRuntime()
	data, err := Query(&store.InContractCall{Name: []byte("unknown"), Method: store.MethodToSelectorBytes("m")}, rt)
	require.NoError(t, err)
	assert.Nil(t, data)
}

func TestMutation_UnknownContract(t *testing.T) {
	rt := testutil.NewMockRuntime()
	err := Mutation(&store.InContractCall{Name: []byte("unknown"), Method: store.MethodToSelectorBytes("m")}, rt)
	require.NoError(t, err)
}

// 已知合约 + 未知方法选择器 → 返回错误（而不是静默成功）
func TestQuery_KnownContract_UnknownMethod(t *testing.T) {
	rt := testutil.NewMockRuntime()
	for _, name := range []string{"gov", "auth", "gateway", "batch"} {
		t.Run(name, func(t *testing.T) {
			_, err := Query(&store.InContractCall{Name: []byte(name), Method: store.MethodToSelectorBytes("nonexistent_method")}, rt)
			require.Error(t, err)
		})
	}
}

func TestMutation_KnownContract_UnknownMethod(t *testing.T) {
	rt := testutil.NewMockRuntime()
	for _, name := range []string{"gov", "auth", "gateway", "batch"} {
		t.Run(name, func(t *testing.T) {
			err := Mutation(&store.InContractCall{Name: []byte(name), Method: store.MethodToSelectorBytes("nonexistent_method")}, rt)
			require.Error(t, err)
		})
	}
}

func TestNativeVersion(t *testing.T) {
	// NativeVersion 必须与当前二进制的原生版本一致（WASM 路径选择依据）
	assert.Equal(t, uint32(20), NativeVersion)
}
