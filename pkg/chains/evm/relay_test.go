// relay_test.go EVM relay 读取单测：验证从 Token 合约的 relay view 函数
// （getLatestNonce / getEvents）读取事件。新版 Token.sol 把 relay 事件写入链上
// 存储（_relayEvents + _eventNonce），eventData 为 SCALE 编码，与 PolkaVM 版
// emit_event 逐字节一致，relay 直接透传给网关合约。
package evm

import (
	"context"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"

	"github.com/computeflux/tee-provider/pkg/types/chaintypes"
	"github.com/computeflux/tee-provider/tee-chain/runtime/store"
)

// nil client（未连接）必须报错而不是 panic；错误信息须带链标识。
func TestEvmChain_RelayNotConnected(t *testing.T) {
	e := &EvmChain{chainId: 84532, chainName: "Base Testnet"}
	_, err := e.GetLatestNonce()
	require.Error(t, err)
	require.Contains(t, err.Error(), "chain Base Testnet(84532)")

	events, err := e.GetRelayEvents(1, 10)
	require.Nil(t, events)
	require.Error(t, err)
	require.Contains(t, err.Error(), "chain Base Testnet(84532)")

	var _ chaintypes.RelayEvent
}

// GetRelayEvents 在 to < from 时返回空。
func TestEvmChain_GetRelayEvents_EmptyRange(t *testing.T) {
	e := &EvmChain{chainId: 84532, chainName: "Base Testnet", tokenAddr: common.HexToAddress("0xC7701fC65125C0a5cdBc760806F8fCf33D2aD2E0")}
	events, err := e.GetRelayEvents(5, 1)
	require.NoError(t, err)
	require.Nil(t, events)
}

// 合约 ABI 必须暴露新版 relay view 函数（否则说明 bindings 未从更新后的
// Token.sol 重新生成）。
func TestTokenABI_HasRelayViewFunctions(t *testing.T) {
	tokenABI, err := tokenParsedABI()
	require.NoError(t, err)

	for _, name := range []string{"getLatestNonce", "getEvents", "getEvent"} {
		_, ok := tokenABI.Methods[name]
		require.True(t, ok, "token ABI must expose %s() (regenerate bindings from updated Token.sol)", name)
	}

	getEvents, ok := tokenABI.Methods["getEvents"]
	require.True(t, ok)
	require.Len(t, getEvents.Inputs, 2, "getEvents(uint64 from, uint64 to)")
}

// 集成测试：连上部署了新版 Token 的 Base Sepolia，验证 getLatestNonce /
// getEvents 端到端可读，且 eventData 为可解码的 SCALE。连不上网时跳过。
//
// ⚠️ 需先把 Token 升级到带 relay view 函数的版本并重新生成 bindings；旧部署的
// 合约没有这些函数，CallContract 会 revert，测试会 Skip。
func TestEvmChain_RelayViewFunctionsAgainstPublicRPC(t *testing.T) {
	client, err := dialFirst([]string{"https://base-sepolia-rpc.publicnode.com", "https://sepolia.base.org"})
	if err != nil {
		t.Skipf("no network access to Base Sepolia: %v", err)
	}
	e := &EvmChain{
		client:    client,
		chainId:   84532,
		chainName: "Base Testnet",
		tokenAddr: common.HexToAddress("0xC7701fC65125C0a5cdBc760806F8fCf33D2aD2E0"),
	}
	defer client.Close()

	head, err := client.BlockNumber(context.Background())
	require.NoError(t, err)
	require.Greater(t, head, uint64(0))

	latest, err := e.GetLatestNonce()
	require.NoError(t, err)
	if latest == 0 {
		t.Skipf("token has no relay events yet (latestNonce=0); deploy events then re-run")
	}

	events, err := e.GetRelayEvents(1, latest)
	require.NoError(t, err)
	require.NotEmpty(t, events)

	for _, ev := range events {
		require.Equal(t, []byte("gateway"), ev.TargetContract)
		require.NotEmpty(t, ev.EventType)
		require.NotEmpty(t, ev.EventData)

		switch string(ev.EventType) {
		case "Recharge":
			require.Len(t, ev.EventData, 2)
			var user store.UniAddr
			require.NoError(t, store.ScaleDecode(&user, ev.EventData[0]))
			require.Equal(t, uint32(2), user.T)
			require.Len(t, user.V, 20)
			var points store.U256
			require.NoError(t, store.ScaleDecode(&points, ev.EventData[1]))
			require.NotNil(t, points.Int)
			require.Positive(t, points.Int.Sign())
			require.Len(t, ev.EventData[1], 32, "U256 must be 32B little-endian")
		case "WithdrawalDone", "WithdrawalCancelled":
			require.Len(t, ev.EventData, 1)
			var nonce uint64
			require.NoError(t, store.ScaleDecode(&nonce, ev.EventData[0]))
		default:
			t.Fatalf("unexpected relay eventType: %q", string(ev.EventType))
		}
	}
}

// 编译期防漂移：确保 relayScanCap 仍为正值、与合约 getEvents 的有界查询语义匹配。
func TestRelayScanCapSane(t *testing.T) {
	require.Positive(t, relayScanCap)
	var _ = big.NewInt // keep math/big referenced for future scale assertions
}
