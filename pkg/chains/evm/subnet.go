package evm

import (
	"context"
	"fmt"

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
	vals, err := e.GetValidatorList()
	if err != nil {
		return nil, nil, err
	}
	var pubs []*model.PubKey
	for _, v := range vals {
		pubs = append(pubs, &v.ValidatorId)
	}
	return vals, pubs, nil
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
