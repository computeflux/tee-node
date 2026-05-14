package evm

import (
	"context"
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"

	"github.com/computeflux/tee-provider/pkg/chains/evm/bindings"
	"github.com/computeflux/tee-provider/pkg/model"
	inkutil "github.com/wetee-dao/ink.go/util"
)

// ── Subnet: Epoch ────────────────────────────────────────

func (e *EvmChain) GetEpoch() (uint32, uint32, uint32, uint32, model.ChainH160, error) {
	e.mu.RLock()
	info, err := e.subnet.EpochInfo(&bind.CallOpts{})
	e.mu.RUnlock()
	if err != nil {
		if rerr := e.reconnect(); rerr == nil {
			e.mu.RLock()
			info, err = e.subnet.EpochInfo(&bind.CallOpts{})
			e.mu.RUnlock()
		}
	}
	if err != nil {
		return 0, 0, 0, 0, model.ChainH160{}, err
	}
	var h160 model.ChainH160
	copy(h160[:], info.SideChainPub.Bytes())
	return info.Epoch, info.EpochSlot, uint32(info.LastEpochBlock.Uint64()), info.Now, h160, nil
}

func (e *EvmChain) SetNewEpoch(nodeId uint64) error {
	e.mu.RLock()
	tx, err := e.subnet.SetNextEpoch(e.auth, nodeId)
	e.mu.RUnlock()
	if err != nil {
		if rerr := e.reconnect(); rerr == nil {
			e.mu.RLock()
			tx, err = e.subnet.SetNextEpoch(e.auth, nodeId)
			e.mu.RUnlock()
		}
	}
	if err != nil {
		return fmt.Errorf("setNextEpoch: %w", err)
	}
	_, err = bind.WaitMined(context.Background(), e.client, tx)
	return err
}

// ── Subnet: Validators ────────────────────────────────────

func (e *EvmChain) GetBootPeers() ([]model.P2PAddr, error) {
	e.mu.RLock()
	nodes, err := e.subnet.GetBootNodes(&bind.CallOpts{})
	e.mu.RUnlock()
	if err != nil {
		if rerr := e.reconnect(); rerr == nil {
			e.mu.RLock()
			nodes, err = e.subnet.GetBootNodes(&bind.CallOpts{})
			e.mu.RUnlock()
		}
	}
	if err != nil {
		return nil, err
	}
	var peers []model.P2PAddr
	for _, n := range nodes {
		var id [32]byte
		copy(id[:], n.P2pId[:])
		ipv4 := uint32(n.Ip.A)<<24 | uint32(n.Ip.B)<<16 | uint32(n.Ip.C)<<8 | uint32(n.Ip.D)
		peers = append(peers, model.P2PAddr{
			Ip:   model.Ip{Ipv4: inkutil.NewSome(ipv4)},
			Port: uint16(n.Port),
			Id:   id,
		})
	}
	return peers, nil
}

func (e *EvmChain) GetValidatorList() ([]*model.Validator, error) {
	e.mu.RLock()
	ids, nodes, _, err := e.subnet.GetValidators(&bind.CallOpts{})
	e.mu.RUnlock()
	if err != nil {
		if rerr := e.reconnect(); rerr == nil {
			e.mu.RLock()
			ids, nodes, _, err = e.subnet.GetValidators(&bind.CallOpts{})
			e.mu.RUnlock()
		}
	}
	if err != nil {
		return nil, err
	}
	return buildValidators(ids, nodes), nil
}

func (e *EvmChain) GetNextEpochValidatorList() ([]*model.Validator, error) {
	e.mu.RLock()
	ids, nodes, _, err := e.subnet.NextEpochValidators(&bind.CallOpts{})
	e.mu.RUnlock()
	if err != nil {
		if rerr := e.reconnect(); rerr == nil {
			e.mu.RLock()
			ids, nodes, _, err = e.subnet.NextEpochValidators(&bind.CallOpts{})
			e.mu.RUnlock()
		}
	}
	if err != nil {
		return nil, err
	}
	return buildValidators(ids, nodes), nil
}

func buildValidators(ids []uint64, nodes []bindings.SecretNode) []*model.Validator {
	validators := make([]*model.Validator, 0, len(ids))
	for i, id := range ids {
		var vId, pId [32]byte
		if i < len(nodes) {
			copy(vId[:], nodes[i].ValidatorId[:])
			copy(pId[:], nodes[i].P2pId[:])
		}
		validators = append(validators, &model.Validator{
			NodeID:      id,
			ValidatorId: *model.PubKeyFromByte(vId[:]),
			P2pId:       *model.PubKeyFromByte(pId[:]),
		})
	}
	return validators
}

func (e *EvmChain) GetNodes() ([]*model.Validator, []*model.PubKey, error) {
	// 获取所有 workers（与旧版 ink! 逻辑一致）
	workers, _, err := e.subnet.GetWorkers(&bind.CallOpts{}, 0, 5000)
	if err != nil {
		return nil, nil, fmt.Errorf("GetWorkers: %w", err)
	}

	// 获取所有 secrets（与旧版 ink! QuerySecrets 逻辑一致）
	// 注意：getSecrets(start, size) 是从 start 往 0 降序遍历，
	// start 必须 >= total 才能拿到全部。传一个大数让合约自动 clamp 到 total-1。
	bigStart := big.NewInt(1 << 60) // 足够大，合约会 clamp 到 nextSecretId-1
	bigSize := big.NewInt(5000)
	secretsResult, err := e.subnet.GetSecrets(&bind.CallOpts{}, bigStart, bigSize)
	if err != nil {
		return nil, nil, fmt.Errorf("GetSecrets: %w", err)
	}

	nodes := make([]*model.PubKey, 0, len(workers)+len(secretsResult.Nodes))
	validators := make([]*model.Validator, 0, len(secretsResult.Nodes))

	// Workers → P2P nodes
	for _, w := range workers {
		var p2pId [32]byte
		copy(p2pId[:], w.P2pId[:])
		nodes = append(nodes, model.PubKeyFromByte(p2pId[:]))
	}

	// Secrets → P2P nodes + validators（排除已终止的）
	for _, s := range secretsResult.Nodes {
		if s.TerminalBlock.Sign() > 0 {
			continue
		}
		var vId, pId [32]byte
		copy(vId[:], s.ValidatorId[:])
		copy(pId[:], s.P2pId[:])
		nodes = append(nodes, model.PubKeyFromByte(pId[:]))
		validators = append(validators, &model.Validator{
			NodeID:      s.Id,
			P2pId:       *model.PubKeyFromByte(pId[:]),
			ValidatorId: *model.PubKeyFromByte(vId[:]),
		})
	}

	return validators, nodes, nil
}

// ── Subnet: MapAccount ────────────────────────────────────

func (e *EvmChain) CheckMapAccount(addr model.ChainH160) (bool, error) {
	var b32 [32]byte
	copy(b32[12:], addr[:])
	e.mu.RLock()
	result, err := e.subnet.MintOfWorker(&bind.CallOpts{}, b32)
	e.mu.RUnlock()
	if err != nil {
		if rerr := e.reconnect(); rerr == nil {
			e.mu.RLock()
			result, err = e.subnet.MintOfWorker(&bind.CallOpts{}, b32)
			e.mu.RUnlock()
		}
	}
	if err != nil {
		return false, nil
	}
	return result > 0, nil
}
