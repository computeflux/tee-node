// relay_test.go EVM relay 事件解码单测：验证 Token 日志 → gateway 调用的
// SCALE 参数编码（与 gateway_gen dispatch / Rust emit_event 的编码一致）。
package evm

import (
	"context"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/stretchr/testify/require"

	"github.com/computeflux/tee-provider/pkg/types/chaintypes"
	"github.com/computeflux/tee-provider/tee-chain/runtime/store"
)

func TestDecodeRelayLog_Recharged(t *testing.T) {
	tokenABI, err := tokenParsedABI()
	require.NoError(t, err)

	user := common.HexToAddress("0x1234567890AbCdEf1234567890aBcDeF12345678")
	points := big.NewInt(4_000_000_000) // 4e9 points

	// Recharged(address indexed user, uint256 amount, uint256 points, uint256 timestamp)
	// Inputs.Unpack → non-indexed [amount, points, timestamp]; relay uses points.
	data, err := tokenABI.Events["Recharged"].Inputs.Pack(user, big.NewInt(1_000_000), points, big.NewInt(1700000000))
	require.NoError(t, err)

	lg := &ethtypes.Log{
		Address: common.HexToAddress("0x000000000000000000000000000000000000dEaD"),
		Topics: []common.Hash{
			tokenABI.Events["Recharged"].ID,
			common.BytesToHash(user.Bytes()),
		},
		Data: data,
	}

	ev, err := decodeRelayLog(tokenABI, lg)
	require.NoError(t, err)
	require.Equal(t, []byte("gateway"), ev.TargetContract)
	require.Equal(t, []byte("Recharge"), ev.EventType)
	require.Len(t, ev.EventData, 2)

	var userArg store.UniAddr
	require.NoError(t, store.ScaleDecode(&userArg, ev.EventData[0]))
	require.Equal(t, uint32(2), userArg.T)
	require.Equal(t, user.Bytes(), userArg.V)

	var pointsArg store.U256
	require.NoError(t, store.ScaleDecode(&pointsArg, ev.EventData[1]))
	require.NotNil(t, pointsArg.Int)
	require.Equal(t, 0, pointsArg.Int.Cmp(points))

	require.Len(t, ev.EventData[1], 32, "U256 must be 32B little-endian")
}

func TestDecodeRelayLog_WithdrawalDone(t *testing.T) {
	tokenABI, err := tokenParsedABI()
	require.NoError(t, err)

	lg := &ethtypes.Log{
		Topics: []common.Hash{
			tokenABI.Events["WithdrawalDone"].ID,
			common.BigToHash(big.NewInt(42)),
		},
	}

	ev, err := decodeRelayLog(tokenABI, lg)
	require.NoError(t, err)
	require.Equal(t, []byte("gateway"), ev.TargetContract)
	require.Equal(t, []byte("WithdrawalDone"), ev.EventType)
	require.Len(t, ev.EventData, 1)

	var nonce uint64
	require.NoError(t, store.ScaleDecode(&nonce, ev.EventData[0]))
	require.Equal(t, uint64(42), nonce)
}

func TestDecodeRelayLog_WithdrawalCancelled(t *testing.T) {
	tokenABI, err := tokenParsedABI()
	require.NoError(t, err)

	lg := &ethtypes.Log{
		Topics: []common.Hash{
			tokenABI.Events["WithdrawalCancelled"].ID,
			common.BigToHash(big.NewInt(7)),
		},
	}

	ev, err := decodeRelayLog(tokenABI, lg)
	require.NoError(t, err)
	require.Equal(t, []byte("WithdrawalCancelled"), ev.EventType)

	var nonce uint64
	require.NoError(t, store.ScaleDecode(&nonce, ev.EventData[0]))
	require.Equal(t, uint64(7), nonce)
}

func TestDecodeRelayLog_UnrelatedTopic(t *testing.T) {
	tokenABI, err := tokenParsedABI()
	require.NoError(t, err)

	lg := &ethtypes.Log{Topics: []common.Hash{common.HexToHash("0xdeadbeef")}}
	_, err = decodeRelayLog(tokenABI, lg)
	require.Error(t, err)

	lg = &ethtypes.Log{}
	_, err = decodeRelayLog(tokenABI, lg)
	require.Error(t, err)
}

func TestRelayTopic0(t *testing.T) {
	topics := relayTopic0()
	require.Len(t, topics, 3)
	for _, tp := range topics {
		require.NotEqual(t, common.Hash{}, tp)
	}
}

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

// 公共 Base RPC 把 eth_getLogs 限制在 200 个区块（超出返回 HTTP 413）；
// filterRelayLogs 必须自动分片拉取并去重，不能因跨度超限而报错。
// 连不上网时跳过（集成测试）。
func TestFilterRelayLogs_ChunkedAgainstPublicRPC(t *testing.T) {
	client, err := dialFirst([]string{"https://sepolia.base.org"})
	if err != nil {
		t.Skipf("no network access to sepolia.base.org: %v", err)
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

	// 跨度远超 200（部署块起）：旧实现会 413，现在应成功
	logs, err := e.filterRelayLogs(58640908, head)
	require.NoError(t, err)

	// 分片拼接后必须严格升序且无重复（nonce = 序号，顺序/重复都会破坏 nonce 语义）
	seen := map[string]bool{}
	var prevKey uint64
	for i := range logs {
		key := logKey(&logs[i])
		require.False(t, seen[key], "duplicate log at chunk boundary: %s", key)
		seen[key] = true
		require.GreaterOrEqual(t, logs[i].BlockNumber, prevKey, "logs not ascending")
		prevKey = logs[i].BlockNumber
	}
}
