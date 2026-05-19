// Package relay 主链 → 侧链事件同步
//
// Phase 1: 信任 RPC, 仅 nonce 防重放
// Phase 2: header 同步 + Merkle proof
package relay

import (
	"fmt"

	"github.com/computeflux/tee-provider/pkg/model"
)

type SyncEvent struct {
	ChainID        uint32
	Nonce          uint64
	BlockNum       uint64
	TargetContract []byte
	EventType      []byte
	EventData      [][]byte
	Proof          [][32]byte
	ProofIndex     uint32
}

type Engine struct {
	api model.ContractApi
}

func NewEngine(api model.ContractApi) *Engine {
	return &Engine{api: api}
}

func (e *Engine) SetRuntime(api model.ContractApi) { e.api = api }

// CheckRelayEvent Phase 1: 仅验 nonce
func (e *Engine) CheckRelayEvent(ev SyncEvent) error {
	last, err := getLastNonceBg(ev.ChainID)
	if err != nil {
		return err
	}
	if ev.Nonce != last+1 {
		return fmt.Errorf("relay: expected nonce %d, got %d", last+1, ev.Nonce)
	}
	return nil
}

// ApplyRelayEvent 执行并写 nonce
func (e *Engine) ApplyRelayEvent(ev SyncEvent) error {
	txn := e.api.GetTxn()
	last, err := getLastNonce(txn, ev.ChainID)
	if err != nil {
		return err
	}
	if ev.Nonce != last+1 {
		return fmt.Errorf("relay: expected nonce %d, got %d", last+1, ev.Nonce)
	}
	setLastNonce(txn, ev.ChainID, ev.Nonce)
	return dispatch(ev, e.api)
}

func (e *Engine) LastNonce(chainID uint32) (uint64, error) {
	return getLastNonce(e.api.GetTxn(), chainID)
}

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

func dispatch(ev SyncEvent, api model.ContractApi) error {
	_, err := api.Call(model.UniAddr{}, model.ContractCall{
		Name:   ev.TargetContract,
		Method: ev.EventType,
		Args:   ev.EventData,
	})
	return err
}

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

func getLastNonceBg(chainID uint32) (uint64, error) {
	n, err := model.GetCodec[uint64]("RELAY", fmt.Sprintf("nonce_%d", chainID))
	if err != nil || n == nil {
		return 0, fmt.Errorf("chain %d not registered", chainID)
	}
	return *n, nil
}
