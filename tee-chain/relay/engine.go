// Package relay 主链 → 侧链事件同步 | Main chain → Sidechain event sync
//
// Phase 1: 信任 RPC, 仅 nonce 防重放 | Trust RPC, nonce-only replay protection
// Phase 2: header 同步 + Merkle proof | Header sync + Merkle proof verification
//
// 流程 | Flow:
//
//	Worker 拉取主链事件 → 缓存 | fetch main chain events → cache
//	PrepareProposal 打包成熟事件 | pack mature events into block
//	ProcessProposal 验证 nonce | verify nonce
//	FinalizeTx 执行并写 nonce | execute and write nonce
package relay

import (
	"github.com/computeflux/tee-provider/tee-chain/contracts/store"

	"fmt"

	"github.com/computeflux/tee-provider/pkg/model"
	dbstore "github.com/computeflux/tee-provider/pkg/store"
	"github.com/computeflux/tee-provider/pkg/util"
)

const RELAY_KEY = "RELAY"

// SyncEvent 从主链同步的事件 | Event synced from main chain
type SyncEvent struct {
	ChainID        uint32     // 主链 ID | main chain identifier
	Nonce          uint64     // 事件序号 (per chain, 严格递增) | event sequence (per chain, strictly increasing)
	BlockNum       uint64     // 主链区块号 | main chain block number
	TargetContract []byte     // 目标合约 | target contract name (e.g. "gateway")
	EventType      []byte     // 事件类型 | event type (e.g. "Recharge", "Withdraw")
	EventData      [][]byte   // 合约参数 | contract call args
	Proof          [][32]byte // Merkle proof (Phase 2 启用 | enabled in Phase 2)
	ProofIndex     uint32     // 叶子索引 | leaf index in Merkle tree
}

// Engine 同步引擎 — 每个侧链节点一个实例 | Sync engine — one per sidechain node
type Engine struct {
}

// NewEngine 创建引擎 | Create engine
func NewEngine() *Engine {
	return &Engine{}
}

// CheckRelayEvent 验证 relay 事件 (ProcessProposal 阶段) | Validate relay event (ProcessProposal phase)
//
//	Phase 1: 仅验证 nonce 严格递增 | only verify nonce is strictly increasing
//	Phase 2: 增加 Merkle proof 验证 | add Merkle proof verification
func (e *Engine) CheckRelayEvent(ev SyncEvent) error {
	// 读已提交状态的最新 nonce | read latest nonce from committed state
	last, err := getLastNonceBg(ev.ChainID)
	if err != nil {
		util.LogWithRed(RELAY_KEY, "CheckRelayEvent: getLastNonceBg failed, chain", ev.ChainID, "err:", err)
		return err
	}

	// nonce 必须严格递增 (防重放 + 防乱序) | nonce must be strictly increasing (anti-replay + anti-out-of-order)
	if ev.Nonce != last+1 {
		util.LogWithRed(RELAY_KEY, fmt.Sprintf("CheckRelayEvent: bad nonce, chain=%d expected=%d got=%d", ev.ChainID, last+1, ev.Nonce))
		return fmt.Errorf("relay: expected nonce %d, got %d", last+1, ev.Nonce)
	}

	util.LogOk(RELAY_KEY, fmt.Sprintf("CheckRelayEvent: ok, chain=%d nonce=%d block=%d", ev.ChainID, ev.Nonce, ev.BlockNum))
	return nil
}

// ApplyRelayEvent 执行 relay 事件并更新 nonce (FinalizeTx 阶段) | Execute relay event and update nonce (FinalizeTx phase)
func (e *Engine) ApplyRelayEvent(ev SyncEvent, api store.ContractApi) error {
	txn := api.GetTxn()
	// 二次校验 (区块内可能有多个 relay tx) | double-check (block may contain multiple relay txs)
	last, err := getLastNonce(txn, ev.ChainID)
	if err != nil {
		util.LogWithRed(RELAY_KEY, "ApplyRelayEvent: getLastNonce failed, chain", ev.ChainID, "err:", err)
		return err
	}
	if ev.Nonce != last+1 {
		util.LogWithRed(RELAY_KEY, fmt.Sprintf("ApplyRelayEvent: bad nonce, chain=%d expected=%d got=%d", ev.ChainID, last+1, ev.Nonce))
		return fmt.Errorf("relay: expected nonce %d, got %d", last+1, ev.Nonce)
	}

	// 先写 nonce 再执行, 防止同一事件重复打包 | write nonce first to prevent re-packaging
	setLastNonce(txn, ev.ChainID, ev.Nonce)
	util.LogWithCyan(RELAY_KEY, fmt.Sprintf("ApplyRelayEvent: chain=%d nonce=%d block=%d -> %s.%s", ev.ChainID, ev.Nonce, ev.BlockNum, string(ev.TargetContract), string(ev.EventType)))

	// 分发到目标合约 | dispatch to target contract
	return dispatch(ev, api)
}

// LastNonce 查询已处理的最新 nonce | Query last processed nonce
func (e *Engine) LastNonce(chainID uint32) (uint64, error) {
	// 读已提交 DB (后台 worker / PrepareProposal 无 txn 上下文)
	return getLastNonceBg(chainID)
}

// MakeRelaySysCall 将 SyncEvent 转换为 SysCall_RelaySync (用于创建交易) | Convert SyncEvent to SysCall_RelaySync (for tx creation)
func MakeRelaySysCall(ev SyncEvent) *model.SysCall {
	// [][32]byte → [][]byte (protobuf 只支持 [][]byte)
	proofBytes := make([][]byte, len(ev.Proof))
	for i, p := range ev.Proof {
		proofBytes[i] = p[:]
	}
	return &model.SysCall{
		Payload: &model.SysCall_RelaySync{
			RelaySync: &model.RelaySync{
				ChainId:        ev.ChainID,
				Nonce:          ev.Nonce,
				BlockNum:       ev.BlockNum,
				TargetContract: ev.TargetContract,
				EventType:      ev.EventType,
				EventData:      ev.EventData,
				Proof:          proofBytes,
				ProofIndex:     ev.ProofIndex,
			},
		},
	}
}

// FromRelaySysCall 从 SysCall_RelaySync 提取 SyncEvent (FinalizeTx 解析用) | Extract SyncEvent from SysCall_RelaySync (for FinalizeTx parsing)
func FromRelaySysCall(rs *model.RelaySync) SyncEvent {
	// [][]byte → [][32]byte
	proof := make([][32]byte, len(rs.Proof))
	for i, p := range rs.Proof {
		copy(proof[i][:], p)
	}
	return SyncEvent{
		ChainID:        rs.ChainId,
		Nonce:          rs.Nonce,
		BlockNum:       rs.BlockNum,
		TargetContract: rs.TargetContract,
		EventType:      rs.EventType,
		EventData:      rs.EventData,
		Proof:          proof,
		ProofIndex:     rs.ProofIndex,
	}
}

// BuildSyncEvent 从 ChainApi 返回的原始数据构建 SyncEvent (Worker 用) | Build SyncEvent from raw ChainApi data (for Worker)
func BuildSyncEvent(chainID uint32, nonce, blockNum uint64, target []byte, typ []byte, data [][]byte) SyncEvent {
	return SyncEvent{
		ChainID:        chainID,
		Nonce:          nonce,
		BlockNum:       blockNum,
		TargetContract: target,
		EventType:      typ,
		EventData:      data, // 原始 blob 作为单个参数 | raw blob as single arg
	}
}

// dispatch 调用目标合约 | call target contract
func dispatch(ev SyncEvent, api store.ContractApi) error {
	target := string(ev.TargetContract) + "." + string(ev.EventType)
	if target != "gateway.Recharge" && target != "gateway.Withdraw" {
		util.LogWithGray(RELAY_KEY, fmt.Sprintf("dispatch: unknown target contract %s, skipping", target))
	}

	// return nil
	_, err := api.Call(api.GetSystemAccount(), &model.ContractCall{
		Name:   ev.TargetContract,
		Method: store.MethodToSelectorBytes(string(ev.EventType)),
		Args:   ev.EventData,
	})

	if err != nil {
		util.LogWithRed(RELAY_KEY, fmt.Sprintf("dispatch failed: %s.%s err=%v", string(ev.TargetContract), string(ev.EventType), err))
	} else {
		util.LogOk(RELAY_KEY, fmt.Sprintf("dispatch ok: %s.%s args=%d", string(ev.TargetContract), string(ev.EventType), len(ev.EventData)))
	}
	return err
}

// ── nonce 存储 | nonce storage ──────────────────────────

// getLastNonce 读取 nonce (txn 内, FinalizeTx 用) | Read nonce (within txn, for FinalizeTx)
func getLastNonce(txn store.StoreTx, chainID uint32) (uint64, error) {
	n, err := dbstore.TxnGetCodec[uint64](txn, RELAY_KEY, fmt.Sprintf("nonce_%d", chainID))
	if err != nil {
		return 0, fmt.Errorf("chain %d read error: %w", chainID, err)
	}

	if n == nil {
		return 0, nil // 首次事件, 无历史 nonce | first event, no prior nonce
	}
	return *n, nil
}

// setLastNonce 写入 nonce (txn 内, 原子提交) | Write nonce (within txn, atomic commit)
func setLastNonce(txn store.StoreTx, chainID uint32, v uint64) error {
	return dbstore.TxnSetCodec(txn, RELAY_KEY, fmt.Sprintf("nonce_%d", chainID), v)
}

// getLastNonceBg 读取 nonce (直接读 DB, CheckTx/ProcessProposal 用) | Read nonce (direct DB read, for CheckTx/ProcessProposal)
func getLastNonceBg(chainID uint32) (uint64, error) {
	n, err := dbstore.GetCodec[uint64](RELAY_KEY, fmt.Sprintf("nonce_%d", chainID))
	if err != nil {
		return 0, fmt.Errorf("chain %d read error: %w", chainID, err)
	}
	if n == nil {
		return 0, nil // 首次事件, 无历史 nonce | first event, no prior nonce
	}
	return *n, nil
}
