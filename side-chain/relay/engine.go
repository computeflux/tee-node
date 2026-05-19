// Package relay 主链 → 侧链事件同步 (核心逻辑)
//
// 遵循区块链共识流程:
//  1. 后台拉取主链事件 → 构造 ContractCall 交易 → 提交 mempool
//  2. CheckTx: 验证 nonce + Merkle proof → 拒绝无效交易
//  3. FinalizeTx: 执行 dispatch (与其他合约调用一致)
//  4. Commit: nonce 写入状态 (原子)
//
// 多链: 每条主链独立 chainId, 独立 nonce 序列
package relay

import (
	"fmt"

	"github.com/computeflux/tee-provider/pkg/model"
	"golang.org/x/crypto/blake2b"
)

// SyncEvent 从主链同步的事件
type SyncEvent struct {
	ChainID        uint32     // 主链 ID
	Nonce          uint64     // 事件序号 (per chain)
	BlockNum       uint64     // 主链区块号
	TargetContract []byte     // "gateway"
	EventType      []byte     // "Recharge" | "Withdraw" | ...
	EventData      [][]byte   // 合约参数列表
	Proof          [][32]byte // Merkle proof
	ProofIndex     uint32     // 叶子索引
}

// Engine 同步引擎 — 每个侧链节点一个实例
type Engine struct {
	api model.ContractApi
}

func NewEngine(api model.ContractApi) *Engine {
	return &Engine{api: api}
}

// SetRuntime 注入当前区块的 runtime
func (e *Engine) SetRuntime(api model.ContractApi) {
	e.api = api
}

// ── CheckTx 阶段: 验证 ──────────────────────────────────

// CheckRelayEvent 验证 relay 事件 (CheckTx 调用)
//
//	返回 nil 表示验证通过, 可以打包进区块
func (e *Engine) CheckRelayEvent(ev SyncEvent) error {
	txn := e.api.GetTxn()

	// 1. nonce 必须严格递增
	last, err := getLastNonce(txn, ev.ChainID)
	if err != nil {
		return err
	}
	if ev.Nonce != last+1 {
		return fmt.Errorf("relay: expected nonce %d, got %d", last+1, ev.Nonce)
	}

	// 2. Merkle proof 校验
	ok, err := verifyProof(txn, ev)
	if err != nil || !ok {
		return fmt.Errorf("relay: invalid merkle proof")
	}

	return nil
}

// ── FinalizeTx 阶段: 执行 ────────────────────────────────

// ApplyRelayEvent 执行 relay 事件并更新 nonce (FinalizeTx 调用)
func (e *Engine) ApplyRelayEvent(ev SyncEvent) error {
	txn := e.api.GetTxn()

	// 1. nonce 递增校验
	last, err := getLastNonce(txn, ev.ChainID)
	if err != nil {
		return err
	}
	if ev.Nonce != last+1 {
		return fmt.Errorf("relay: expected nonce %d, got %d", last+1, ev.Nonce)
	}

	// 2. 更新 nonce
	setLastNonce(txn, ev.ChainID, ev.Nonce)

	// 3. 分发到合约
	return dispatch(txn, ev, e.api)
}

// ── Chain Management ─────────────────────────────────────

// RegisterChain 注册主链 + 创世区块头
func (e *Engine) RegisterChain(chainID uint32, genesisBlock uint64, genesisHeader [32]byte) error {
	txn := e.api.GetTxn()
	if err := setLastNonce(txn, chainID, 0); err != nil {
		return err
	}
	if err := setHeader(txn, chainID, genesisBlock, genesisHeader); err != nil {
		return err
	}
	return setLatestBlock(txn, chainID, genesisBlock)
}

// SubmitHeader 提交主链区块头 (header chain 必须连续)
func (e *Engine) SubmitHeader(chainID uint32, blockNum uint64, headerHash [32]byte) error {
	txn := e.api.GetTxn()
	latest, err := getLatestBlock(txn, chainID)
	if err != nil {
		return err
	}
	if blockNum != latest+1 {
		return fmt.Errorf("header: expected block %d, got %d", latest+1, blockNum)
	}
	if err := setHeader(txn, chainID, blockNum, headerHash); err != nil {
		return err
	}
	return setLatestBlock(txn, chainID, blockNum)
}

// ── Queries ──────────────────────────────────────────────

func (e *Engine) LastNonce(chainID uint32) (uint64, error) {
	return getLastNonce(e.api.GetTxn(), chainID)
}

func (e *Engine) LatestBlock(chainID uint32) (uint64, error) {
	return getLatestBlock(e.api.GetTxn(), chainID)
}

// ── Helpers (Worker 用) ──────────────────────────────────

// MakeRelaySysCall 创建 relay 的 SysCall_RelaySync
func MakeRelaySysCall(ev SyncEvent) *model.SysCall {
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

// FromRelaySysCall 从 SysCall_RelaySync 提取 SyncEvent
func FromRelaySysCall(rs *model.RelaySync) SyncEvent {
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

// BuildSyncEvent 从 ChainApi 数据构建 SyncEvent
func BuildSyncEvent(chainID uint32, nonce, blockNum uint64, target, typ, data []byte) SyncEvent {
	return SyncEvent{
		ChainID:        chainID,
		Nonce:          nonce,
		BlockNum:       blockNum,
		TargetContract: target,
		EventType:      typ,
		EventData:      [][]byte{data},
	}
}

// ── Storage ──────────────────────────────────────────────

func getLastNonce(txn *model.Txn, chainID uint32) (uint64, error) {
	n, err := model.TxnGetCodec[uint64](txn, "RELAY", fmt.Sprintf("nonce_%d", chainID))
	if err != nil || n == nil {
		return 0, fmt.Errorf("chain %d not registered", chainID)
	}
	return *n, nil
}

func setLastNonce(txn *model.Txn, chainID uint32, v uint64) error {
	return model.TxnSetCodec(txn, "RELAY", fmt.Sprintf("nonce_%d", chainID), v)
}

func getLatestBlock(txn *model.Txn, chainID uint32) (uint64, error) {
	n, err := model.TxnGetCodec[uint64](txn, "RELAY", fmt.Sprintf("latest_%d", chainID))
	if err != nil || n == nil {
		return 0, fmt.Errorf("chain %d not registered", chainID)
	}
	return *n, nil
}

func setLatestBlock(txn *model.Txn, chainID uint32, v uint64) error {
	return model.TxnSetCodec(txn, "RELAY", fmt.Sprintf("latest_%d", chainID), v)
}

func setHeader(txn *model.Txn, chainID uint32, blockNum uint64, hash [32]byte) error {
	return model.TxnSetCodec(txn, "RELAY", fmt.Sprintf("hdr_%d_%d", chainID, blockNum), hash)
}

func getHeader(txn *model.Txn, chainID uint32, blockNum uint64) ([32]byte, error) {
	h, err := model.TxnGetCodec[[32]byte](txn, "RELAY", fmt.Sprintf("hdr_%d_%d", chainID, blockNum))
	if err != nil || h == nil {
		return [32]byte{}, fmt.Errorf("header %d/%d not found", chainID, blockNum)
	}
	return *h, nil
}

// ── Merkle Proof ─────────────────────────────────────────

func verifyProof(txn *model.Txn, ev SyncEvent) (bool, error) {
	hdr, err := getHeader(txn, ev.ChainID, ev.BlockNum)
	if err != nil {
		return false, err
	}
	return verifyMerkle(ev, hdr), nil
}

func verifyMerkle(ev SyncEvent, hdr [32]byte) bool {
	var buf []byte
	buf = append(buf, byte(ev.ChainID>>24), byte(ev.ChainID>>16), byte(ev.ChainID>>8), byte(ev.ChainID))
	buf = append(buf, byte(ev.Nonce>>56), byte(ev.Nonce>>48), byte(ev.Nonce>>40), byte(ev.Nonce>>32),
		byte(ev.Nonce>>24), byte(ev.Nonce>>16), byte(ev.Nonce>>8), byte(ev.Nonce))
	buf = append(buf, ev.EventType...)
	for _, d := range ev.EventData {
		buf = append(buf, d...)
	}
	leaf := blake2b.Sum256(buf)
	idx := ev.ProofIndex
	for _, sib := range ev.Proof {
		if idx%2 == 0 {
			leaf = hashPair(leaf, sib)
		} else {
			leaf = hashPair(sib, leaf)
		}
		idx /= 2
	}
	return leaf == hdr
}

func hashPair(a, b [32]byte) [32]byte {
	return blake2b.Sum256(append(a[:], b[:]...))
}

// ── Dispatch ─────────────────────────────────────────────

func dispatch(txn *model.Txn, ev SyncEvent, api model.ContractApi) error {
	call := model.ContractCall{
		Name:   ev.TargetContract,
		Method: ev.EventType,
		Args:   ev.EventData,
	}
	_, err := api.Call(model.UniAddr{}, call)
	return err
}
