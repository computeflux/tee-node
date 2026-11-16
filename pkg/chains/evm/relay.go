// relay.go EVM 链 → 侧链事件同步 | EVM chain → sidechain event sync
//
// Base 版 Token.sol 把 relay 事件写入链上存储（_relayEvents + _eventNonce），
// 语义与 PolkaVM（revive）版 Token 完全一致，并暴露三个 view 函数供 relay 读取：
//
//	getLatestNonce()            → 当前最新事件 nonce（从 1 起单调递增；0 = 无事件）
//	getEvents(from, to)         → EventRecord[]（to == 0 表示查到最新；跳过空 nonce）
//	getEvent(nonce)             → 单个 EventRecord（不存在时 targetContract 为空）
//
// EventRecord{ targetContract="gateway", eventType="Recharge"/"WithdrawalDone"/
// "WithdrawalCancelled", eventData=SCALE }，eventData 字节与 PolkaVM 版 emit_event
// 逐字节一致，因此 relay 直接透传给网关合约，无需本地解码：
//
//	Recharge           → gateway.Recharge(user UniAddr{2}, points u256)
//	WithdrawalDone     → gateway.WithdrawalDone(nonce u64)
//	WithdrawalCancelled→ gateway.WithdrawalCancelled(nonce u64)
//
// relay engine 用 (chainID, nonce) 防重放；worker 按区间拉取，engine 强制 nonce
// 严格连续。nonce 由合约给出（_eventNonce），与链上日志顺序编号等价（合约写入点
// 与 Recharged/WithdrawalDone/WithdrawalCancelled 日志一一对应）。
package evm

import (
	"context"
	"fmt"
	"math/big"
	"strings"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"

	"github.com/computeflux/tee-provider/pkg/chains/evm/bindings"
	"github.com/computeflux/tee-provider/pkg/types/chaintypes"
	"github.com/computeflux/tee-provider/pkg/util"
)

// relayScanCap bounds one GetRelayEvents call so a large nonce gap cannot
// produce an unbounded getEvents query.
const relayScanCap = 500

// relayProbeNonces is how many nonces beyond the caller's upper bound to probe.
// Public Base RPCs evaluate eth_call (and therefore getLatestNonce()/getEvents())
// against the sealing pending block, which can lag the newest mined block by
// ~20s: a recharge mined in the newest block is invisible to BOTH view calls
// until a later block is sealed, so a range of [last+1, latest] systematically
// misses the newest events (observed as "always exactly one behind", and as
// rechargeHistory showing only pre-upgrade records). Probing a few extra nonces
// closes the gap: the contract's getEvents skips non-existent nonces, so the
// extra probes are no-ops when there is nothing new, and the worker maps the
// returned records to contiguous nonces starting at `from` (the engine enforces
// strict nonce continuity, so gaps are surfaced as errors).
const relayProbeNonces = 8

// relayCallRetries bounds the reconnect-and-retry attempts for one contract
// view call after a transport failure. Public Base RPCs sit behind Cloudflare
// and periodically kill long-lived HTTP/2 connections ("stream error ...;
// PROTOCOL_ERROR; received from peer", "unexpected EOF"). go-ethereum's
// ethclient keeps reusing the broken transport, so without reconnecting the
// failure repeats forever on every poll; reconnecting rebuilds the client.
const relayCallRetries = 2

// relayRetryBackoff is the wait between retry attempts.
var relayRetryBackoff = 500 * time.Millisecond

// relayEventNames are the Token relay event types written by the contract.
var relayEventNames = []string{"Recharge", "WithdrawalDone", "WithdrawalCancelled"}

// relayErr prefixes a relay error with the chain label and id so logs from
// multi-chain deployments identify the failing relay chain.
func (e *EvmChain) relayErr(format string, args ...any) error {
	return fmt.Errorf("relay[chain %s(%d)]: "+format, append([]any{e.chainName, e.chainId}, args...)...)
}

// GetLatestNonce returns the contract's current relay event nonce (the 1-based
// nonce of the newest relay event; 0 when none). Public RPCs evaluate the call
// against their sealing view, which can lag the newest block; events in the lag
// window are picked up by the extra-nonce probe in GetRelayEvents.
func (e *EvmChain) GetLatestNonce() (uint64, error) {
	e.mu.RLock()
	client := e.client
	e.mu.RUnlock()
	if client == nil {
		return 0, e.relayErr("client not connected")
	}

	nonce, err := e.callGetLatestNonce()
	if err != nil {
		return 0, err
	}
	return nonce, nil
}

// GetRelayEvents returns relay events whose nonces fall in [from, to]
// (1-based, ascending). The contract's getEvents skips non-existent nonces, so
// the returned slice may be shorter than to-from+1; undecodable records are
// logged and skipped (the engine's nonce check surfaces any permanent gap).
//
// Latest-view lag: public RPC nodes evaluate getLatestNonce()/getEvents() against
// a view that can lag the newest mined block (sealing/pending on public Base
// RPCs, last committed on strict nodes; on Base's sub-second blocks that is
// easily ~20 blocks). The worker's range [lastProcessed+1, latest] would then
// never include events inside the lag window, so we always probe a few nonces
// beyond latest (see relayProbeNonces): the contract skips non-existent nonces,
// so the extra probes are no-ops when there is nothing new and pick up the
// newest events as soon as the RPC's view catches up.
func (e *EvmChain) GetRelayEvents(from, to uint64) ([]chaintypes.RelayEvent, error) {
	if to < from {
		return nil, nil
	}
	if e.client == nil {
		return nil, e.relayErr("client not connected")
	}
	to = to + relayProbeNonces // probe past latest: RPC pending view can lag the chain head
	if to-from+1 > relayScanCap {
		to = from + relayScanCap - 1
	}

	records, err := e.callGetEvents(from, to)
	if err != nil {
		return nil, err
	}

	events := make([]chaintypes.RelayEvent, 0, len(records))
	for i := range records {
		rec := &records[i]
		if len(rec.TargetContract) == 0 {
			continue // empty record (nonce not present)
		}
		if string(rec.TargetContract) != "gateway" {
			util.LogWithRed("RELAY", "unexpected target contract, chain", e.chainId, "target", string(rec.TargetContract), "type", string(rec.EventType))
			continue
		}
		events = append(events, chaintypes.RelayEvent{
			TargetContract: rec.TargetContract,
			EventType:      rec.EventType,
			EventData:      rec.EventData,
		})
	}
	return events, nil
}

// ── Contract view calls ───────────────────────────────

// callGetLatestNonce calls Token.getLatestNonce() with reconnect-and-retry.
//
// The block number is nil (RPC default view). Events newer than that view are
// picked up via the extra-nonce probe in GetRelayEvents; "pending" is not used
// explicitly because public Base RPCs reject it for eth_call with -32602.
func (e *EvmChain) callGetLatestNonce() (uint64, error) {
	return e.callGetLatestNonceAt(nil)
}

func (e *EvmChain) callGetLatestNonceAt(block *big.Int) (uint64, error) {
	tokenABI, err := tokenParsedABI()
	if err != nil {
		return 0, err
	}
	method, ok := tokenABI.Methods["getLatestNonce"]
	if !ok {
		return 0, e.relayErr("token ABI has no getLatestNonce() (regenerate bindings from updated Token.sol)")
	}

	var lastErr error
	for attempt := 0; attempt <= relayCallRetries; attempt++ {
		e.mu.RLock()
		client := e.client
		e.mu.RUnlock()
		if client == nil {
			lastErr = e.relayErr("client not connected")
		} else {
			out, err := client.CallContract(context.Background(), ethereum.CallMsg{
				To:   &e.tokenAddr,
				Data: method.ID,
			}, block)
			if err == nil {
				vals, err := method.Outputs.Unpack(out)
				if err != nil {
					return 0, e.relayErr("unpack getLatestNonce: %w", err)
				}
				if len(vals) == 0 {
					return 0, e.relayErr("getLatestNonce returned no output")
				}
				if n, ok := vals[0].(uint64); ok {
					return n, nil
				}
				return 0, e.relayErr("getLatestNonce output is not uint64")
			}
			lastErr = err
		}
		if attempt < relayCallRetries {
			util.LogWithYellow("RELAY", "getLatestNonce failed, chain", e.chainId, "attempt", attempt, "reconnecting:", lastErr)
			if rerr := e.reconnect(); rerr != nil {
				return 0, e.relayErr("getLatestNonce: %v (reconnect: %v)", lastErr, rerr)
			}
			time.Sleep(relayRetryBackoff)
		}
	}
	return 0, e.relayErr("getLatestNonce: %v", lastErr)
}

// callGetEvents calls Token.getEvents(from, to) with reconnect-and-retry and
// returns the decoded EventRecord list (ascending nonce, gaps skipped).
func (e *EvmChain) callGetEvents(from, to uint64) ([]eventRecord, error) {
	tokenABI, err := tokenParsedABI()
	if err != nil {
		return nil, err
	}
	method, ok := tokenABI.Methods["getEvents"]
	if !ok {
		return nil, e.relayErr("token ABI has no getEvents(uint256,uint256) (regenerate bindings from updated Token.sol)")
	}
	data, err := method.Inputs.Pack(from, to)
	if err != nil {
		return nil, e.relayErr("pack getEvents args: %w", err)
	}

	var lastErr error
	for attempt := 0; attempt <= relayCallRetries; attempt++ {
		e.mu.RLock()
		client := e.client
		e.mu.RUnlock()
		if client == nil {
			lastErr = e.relayErr("client not connected")
		} else {
			out, err := client.CallContract(context.Background(), ethereum.CallMsg{
				To:   &e.tokenAddr,
				Data: append(method.ID, data...),
			}, nil)
			if err == nil {
				vals, err := method.Outputs.Unpack(out)
				if err != nil {
					return nil, e.relayErr("unpack getEvents: %w", err)
				}
				if len(vals) == 0 {
					return nil, nil
				}
				if recs, ok := vals[0].([]struct {
					TargetContract []byte   `json:"targetContract"`
					EventType      []byte   `json:"eventType"`
					EventData      [][]byte `json:"eventData"`
				}); ok {
					records := make([]eventRecord, 0, len(recs))
					for _, r := range recs {
						records = append(records, eventRecord{
							TargetContract: r.TargetContract,
							EventType:      r.EventType,
							EventData:      r.EventData,
						})
					}
					return records, nil
				}
				return nil, e.relayErr("getEvents output shape unexpected")
			}
			lastErr = err
		}
		if attempt < relayCallRetries {
			util.LogWithYellow("RELAY", "getEvents failed, chain", e.chainId, "range", fmt.Sprintf("[%d,%d]", from, to), "attempt", attempt, "reconnecting:", lastErr)
			if rerr := e.reconnect(); rerr != nil {
				return nil, e.relayErr("getEvents [%d,%d]: %v (reconnect: %v)", from, to, lastErr, rerr)
			}
			time.Sleep(relayRetryBackoff)
		}
	}
	return nil, e.relayErr("getEvents [%d,%d]: %v", from, to, lastErr)
}

// eventRecord mirrors the contract's EventRecord struct.
type eventRecord struct {
	TargetContract []byte
	EventType      []byte
	EventData      [][]byte
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
