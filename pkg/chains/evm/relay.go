// relay.go EVM 链 → 侧链事件同步 | EVM chain → sidechain event sync
//
// Base 版 Token.sol 的积分事件语义与 PolkaVM 版一致：
//
//	Recharged(user, amount, points, timestamp) → gateway.Recharge(user, points)
//	WithdrawalDone(nonce)                      → gateway.WithdrawalDone(nonce)
//	WithdrawalCancelled(nonce)                 → gateway.WithdrawalCancelled(nonce)
//
// 事件 nonce = EVM 日志序号：从 Token 部署块（relayStartBlock，可按
// RELAY_FROM_BLOCK_<chainID> 覆盖）起，按区块顺序对每类 relay 事件连续编号
// （1 起，链上单调递增）。relay engine 用 (chainID, nonce) 防重放；worker
// 按区间拉取，engine 强制 nonce 严格连续。
package evm

import (
	"context"
	"fmt"
	"math/big"
	"os"
	"strings"
	"sync"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"

	"github.com/computeflux/tee-provider/pkg/chains/evm/bindings"
	"github.com/computeflux/tee-provider/pkg/types/chaintypes"
	"github.com/computeflux/tee-provider/pkg/util"
	"github.com/computeflux/tee-provider/tee-chain/runtime/store"
)

// relayedFromBlock caches the per-chainID block to start scanning logs from.
var relayedFromBlock = map[uint32]uint64{}

// relayScanCap bounds one GetRelayEvents call so a large nonce gap cannot
// produce an unbounded eth_getLogs query.
const relayScanCap = 500

// relayChunkBlocks bounds ONE eth_getLogs range. Public Base RPCs reject
// ranges wider than 200 blocks (HTTP 413 "eth_getLogs is limited to a 200
// range"); larger spans are split into chunks and concatenated. Nonce
// numbering is the position in the concatenated ascending log list, so
// chunking does not change nonces.
const relayChunkBlocks = uint64(200)

// relayEventNames are the Token events relayed to the sidechain. Each
// relay-relevant log counts as one nonce.
var relayEventNames = []string{"Recharged", "WithdrawalDone", "WithdrawalCancelled"}

// relayErr prefixes a relay error with the chain label and id so logs from
// multi-chain deployments identify the failing relay chain.
func (e *EvmChain) relayErr(format string, args ...any) error {
	return fmt.Errorf("relay[chain %s(%d)]: "+format, append([]any{e.chainName, e.chainId}, args...)...)
}

// GetLatestNonce returns the number of relay-relevant Token logs emitted so
// far (the 1-based nonce of the newest relay event; 0 when none).
func (e *EvmChain) GetLatestNonce() (uint64, error) {
	e.mu.RLock()
	client := e.client
	e.mu.RUnlock()
	if client == nil {
		return 0, e.relayErr("client not connected")
	}

	head, err := client.BlockNumber(context.Background())
	if err != nil {
		return 0, e.relayErr("block number: %w", err)
	}

	from, err := e.relayStartBlock()
	if err != nil {
		return 0, err
	}
	if from > head {
		return 0, nil
	}

	count, err := e.countRelayLogs(from, head)
	if err != nil {
		return 0, err
	}
	return count, nil
}

// GetRelayEvents returns relay events whose nonces fall in [from, to]
// (1-based, ascending). Nonces beyond the chain's current log count are
// simply absent; undecodable logs are logged and skipped (the engine's nonce
// check surfaces any permanent gap).
func (e *EvmChain) GetRelayEvents(from, to uint64) ([]chaintypes.RelayEvent, error) {
	if to < from {
		return nil, nil
	}
	if e.client == nil {
		return nil, e.relayErr("client not connected")
	}
	if to-from+1 > relayScanCap {
		to = from + relayScanCap - 1
	}

	startBlock, err := e.relayStartBlock()
	if err != nil {
		return nil, err
	}

	e.mu.RLock()
	client := e.client
	e.mu.RUnlock()

	head, err := client.BlockNumber(context.Background())
	if err != nil {
		return nil, e.relayErr("block number: %w", err)
	}

	logs, err := e.filterRelayLogs(startBlock, head)
	if err != nil {
		return nil, err
	}

	tokenABI, err := tokenParsedABI()
	if err != nil {
		return nil, err
	}

	events := make([]chaintypes.RelayEvent, 0, to-from+1)
	for i := range logs {
		nonce := uint64(i) + 1 // 1-based global relay nonce
		if nonce < from {
			continue
		}
		if nonce > to {
			break
		}
		ev, err := decodeRelayLog(tokenABI, &logs[i])
		if err != nil {
			err = e.relayErr("%w", err)
			util.LogWithRed("RELAY", "decode log failed, chain", e.chainId, "block", logs[i].BlockNumber, "tx", logs[i].TxHash.Hex(), "index", logs[i].Index, "err:", err)
			continue
		}
		events = append(events, ev)
	}
	return events, nil
}

// relayStartBlock resolves the first block to scan (deployment block of the
// Token contract), cached per chain. Override with RELAY_FROM_BLOCK_<chainID>.
func (e *EvmChain) relayStartBlock() (uint64, error) {
	e.mu.RLock()
	from, ok := relayedFromBlock[e.chainId]
	e.mu.RUnlock()
	if ok {
		return from, nil
	}
	if e.client == nil {
		return 0, e.relayErr("client not connected")
	}

	head, err := e.client.BlockNumber(context.Background())
	if err != nil {
		return 0, e.relayErr("block number: %w", err)
	}

	start := uint64(0)
	if v := os.Getenv(fmt.Sprintf("RELAY_FROM_BLOCK_%d", e.chainId)); v != "" {
		if _, err := fmt.Sscanf(v, "%d", &start); err != nil {
			return 0, e.relayErr("bad RELAY_FROM_BLOCK_%d: %s", e.chainId, v)
		}
	} else {
		var deploy uint64
		found := false
		for lo, hi := uint64(0), head; lo <= hi; {
			mid := (lo + hi) / 2
			code, err := e.client.CodeAt(context.Background(), e.tokenAddr, new(big.Int).SetUint64(mid))
			if err != nil {
				return 0, e.relayErr("code at %d: %w", mid, err)
			}
			if len(code) > 0 {
				deploy = mid
				found = true
				if mid == 0 {
					break
				}
				hi = mid - 1
			} else {
				lo = mid + 1
			}
		}
		if !found {
			return 0, e.relayErr("token %s has no code on this chain", e.tokenAddr.Hex())
		}
		start = deploy
	}

	e.mu.Lock()
	relayedFromBlock[e.chainId] = start
	e.mu.Unlock()
	util.LogWithCyan("RELAY", "start block resolved, chain", e.chainId, "block", start)
	return start, nil
}

// filterRelayLogs returns all relay-relevant Token logs in [from, to],
// in ascending block order. The range is split into ≤ relayChunkBlocks-block
// chunks so public RPCs with a getLogs range limit (Base public: 200) do not
// reject the query; overlapping chunk boundaries are deduplicated.
func (e *EvmChain) filterRelayLogs(from, to uint64) ([]types.Log, error) {
	if e.client == nil {
		return nil, e.relayErr("client not connected")
	}
	topic0 := relayTopic0()

	var out []types.Log
	var lastKey string // last appended log's unique key, for chunk-boundary dedup
	for lo := from; ; {
		hi := lo + relayChunkBlocks - 1
		if hi > to || hi < lo { // overflow guard
			hi = to
		}
		logs, err := e.client.FilterLogs(context.Background(), ethereum.FilterQuery{
			FromBlock: new(big.Int).SetUint64(lo),
			ToBlock:   new(big.Int).SetUint64(hi),
			Addresses: []common.Address{e.tokenAddr},
			Topics:    [][]common.Hash{topic0},
		})
		if err != nil {
			return nil, e.relayErr("filter logs [%d,%d]: %w", lo, hi, err)
		}
		for i := range logs {
			key := logKey(&logs[i])
			if key == lastKey {
				continue // boundary duplicate (inclusive FromBlock/ToBlock)
			}
			out = append(out, logs[i])
			lastKey = key
		}
		if hi >= to {
			break
		}
		lo = hi + 1
	}
	return out, nil
}

// logKey is the unique identity of a log (chain-reorg-safe ordering key).
func logKey(lg *types.Log) string {
	return fmt.Sprintf("%s/%d", lg.TxHash.Hex(), lg.Index)
}

// countRelayLogs counts relay-relevant logs in [from, to].
func (e *EvmChain) countRelayLogs(from, to uint64) (uint64, error) {
	logs, err := e.filterRelayLogs(from, to)
	if err != nil {
		return 0, err
	}
	return uint64(len(logs)), nil
}

// relayTopic0 computes the topic0 hashes of the relayed events from the
// binding ABI metadata.
func relayTopic0() []common.Hash {
	tokenABI, _ := tokenParsedABI()
	topics := make([]common.Hash, 0, len(relayEventNames))
	for _, name := range relayEventNames {
		topics = append(topics, tokenABI.Events[name].ID)
	}
	return topics
}

// tokenParsedABI parses (and caches) the Token binding ABI JSON.
func tokenParsedABI() (*abi.ABI, error) {
	tokenABICached.Do(func() {
		parsed, err := abi.JSON(strings.NewReader(bindings.TokenMetaData.ABI))
		if err != nil {
			tokenABIErr = fmt.Errorf("relay: parse token ABI: %w", err)
			return
		}
		tokenABIVal = parsed
	})
	if tokenABIErr != nil {
		return nil, tokenABIErr
	}
	return &tokenABIVal, nil
}

var (
	tokenABICached sync.Once
	tokenABIVal    abi.ABI
	tokenABIErr    error
)

// decodeRelayLog converts one EVM log into a sidechain RelayEvent with the
// SCALE-encoded arguments the gateway contract expects:
//
//	Recharged           → gateway.Recharge(user UniAddr{2}, points u64)
//	WithdrawalDone      → gateway.WithdrawalDone(nonce u64)
//	WithdrawalCancelled → gateway.WithdrawalCancelled(nonce u64)
func decodeRelayLog(tokenABI *abi.ABI, lg *types.Log) (chaintypes.RelayEvent, error) {
	if len(lg.Topics) == 0 {
		return chaintypes.RelayEvent{}, fmt.Errorf("relay: log without topics")
	}

	switch lg.Topics[0] {
	case tokenABI.Events["Recharged"].ID:
		user := common.BytesToAddress(lg.Topics[1].Bytes())
		out, err := tokenABI.Events["Recharged"].Inputs.Unpack(lg.Data)
		if err != nil {
			return chaintypes.RelayEvent{}, fmt.Errorf("relay: unpack Recharged: %w", err)
		}
		// Inputs.Unpack includes the indexed prefix for events: [user, amount, points, timestamp]
		points, ok := out[2].(*big.Int)
		if !ok || points == nil {
			return chaintypes.RelayEvent{}, fmt.Errorf("relay: Recharged points missing")
		}
		uni := chaintypes.UniAddr{T: 2, V: user.Bytes()}
		userBt, err := store.ScaleEncode(uni)
		if err != nil {
			return chaintypes.RelayEvent{}, fmt.Errorf("relay: encode user: %w", err)
		}
		pointsBt, err := store.ScaleEncode(toU256(points))
		if err != nil {
			return chaintypes.RelayEvent{}, fmt.Errorf("relay: encode points: %w", err)
		}
		return chaintypes.RelayEvent{
			TargetContract: []byte("gateway"),
			EventType:      []byte("Recharge"),
			EventData:      [][]byte{userBt, pointsBt},
		}, nil

	case tokenABI.Events["WithdrawalDone"].ID:
		return decodeRelayNonce("WithdrawalDone", lg)

	case tokenABI.Events["WithdrawalCancelled"].ID:
		return decodeRelayNonce("WithdrawalCancelled", lg)
	}

	return chaintypes.RelayEvent{}, fmt.Errorf("relay: not a relay event (topic0=%s)", lg.Topics[0].Hex())
}

// decodeRelayNonce decodes a uint64-indexed event (WithdrawalDone/Cancelled)
// into a gateway mutation event with the SCALE-encoded nonce. The nonce is
// topic1 (uint64 in a 32-byte big-endian word).
func decodeRelayNonce(eventName string, lg *types.Log) (chaintypes.RelayEvent, error) {
	if len(lg.Topics) < 2 {
		return chaintypes.RelayEvent{}, fmt.Errorf("relay: %s without indexed nonce", eventName)
	}
	nonce := new(big.Int).SetBytes(lg.Topics[1].Bytes()).Uint64()
	nonceBt, err := store.ScaleEncode(nonce)
	if err != nil {
		return chaintypes.RelayEvent{}, fmt.Errorf("relay: encode nonce: %w", err)
	}
	return chaintypes.RelayEvent{
		TargetContract: []byte("gateway"),
		EventType:      []byte(eventName),
		EventData:      [][]byte{nonceBt},
	}, nil
}

// toU256 wraps a *big.Int as a store.U256 (SCALE: 32B little-endian).
func toU256(v *big.Int) store.U256 {
	return store.NewU256(new(big.Int).Set(v))
}
