package graph

import (
	"testing"

	"github.com/computeflux/tee-provider/pkg/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ── crossChainInfoFromRelaySync ───────────────────────────
//
// 注意：relay 管道（tee-chain/tx_from_relay.go 的 BuildSyncEvent）传入的
// BlockNum 是**侧链**共识高度，并非主链区块号；chaintypes.RelayEvent 也未携带
// 主链区块号。因此在「上游带出真实主链区块号」的改造完成前，cross_chain 只输出
// 语义可靠的 chain_id / nonce / event_type，main_block 恒为 0、main_explorer_url
// 恒为空字符串（前端判 0 / 判空隐藏），避免给出误导性跳转。
// 下面的断言固化这一「不输出伪造数据」的约定。

func TestCrossChainInfoFromRelaySync_KnownChain(t *testing.T) {
	rs := &model.RelaySync{
		ChainId:        420420417, // Paseo Asset Hub
		Nonce:          7,
		BlockNum:       123456, // 侧链高度，**不是**主链区块号
		TargetContract: []byte("gateway"),
		EventType:      []byte("Recharge"),
	}

	info := crossChainInfoFromRelaySync(rs)
	require.NotNil(t, info)
	// 语义可靠的字段
	assert.Equal(t, 420420417, info.ChainID)
	assert.Equal(t, 7, info.Nonce)
	require.NotNil(t, info.EventType)
	assert.Equal(t, "Recharge", *info.EventType)
	// 主链区块号尚未被捕获：不输出伪造数据
	assert.Equal(t, 0, info.MainBlock)
	assert.Equal(t, "", info.MainExplorerURL)
}

func TestCrossChainInfoFromRelaySync_UnknownChain(t *testing.T) {
	rs := &model.RelaySync{
		ChainId:  999,
		Nonce:    1,
		BlockNum: 42,
	}

	info := crossChainInfoFromRelaySync(rs)
	require.NotNil(t, info)
	assert.Equal(t, 999, info.ChainID)
	assert.Equal(t, 1, info.Nonce)
	assert.Nil(t, info.EventType)
	// 未知主链同样不输出伪造的主链区块号 / 链接
	assert.Equal(t, 0, info.MainBlock)
	assert.Equal(t, "", info.MainExplorerURL)
}

func TestCrossChainInfoFromRelaySync_EmptyEventType(t *testing.T) {
	rs := &model.RelaySync{
		ChainId:  420420417,
		Nonce:    3,
		BlockNum: 100,
		// EventType 为空
	}

	info := crossChainInfoFromRelaySync(rs)
	require.NotNil(t, info)
	assert.Nil(t, info.EventType)
	assert.Equal(t, 0, info.MainBlock)
	assert.Equal(t, "", info.MainExplorerURL)
}
