package evm

import (
	"fmt"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"

	"github.com/computeflux/tee-provider/pkg/model"
)

// ── Token contract methods ────────────────────────────────

func (e *EvmChain) SyncRecharge(user model.UniAddr, amount uint64, txHash []byte) error {
	return e.syncPoints(user, int64(amount))
}

func (e *EvmChain) SyncWithdraw(user model.UniAddr, amount uint64, txHash []byte) error {
	return e.syncPoints(user, -int64(amount))
}

func (e *EvmChain) syncPoints(user model.UniAddr, delta int64) error {
	key := fmt.Sprintf("token:%x", user.V)
	existing, _ := model.GetJson[model.PointsAccount]("POINTS", key)
	if existing == nil {
		existing = &model.PointsAccount{User: user}
	}
	if delta > 0 {
		existing.TotalRecharged += uint64(delta)
	} else {
		d := uint64(-delta)
		if existing.Balance() < d {
			return fmt.Errorf("insufficient balance")
		}
		existing.TotalRecharged -= d
	}
	return model.SetJson("POINTS", key, existing)
}

func (e *EvmChain) GetTotalRecharged(user model.UniAddr) (uint64, error) {
	addr := common.BytesToAddress(user.V)
	e.mu.RLock()
	bal, err := e.token.Balances(&bind.CallOpts{}, addr)
	e.mu.RUnlock()
	if err != nil {
		if rerr := e.reconnect(); rerr == nil {
			e.mu.RLock()
			bal, err = e.token.Balances(&bind.CallOpts{}, addr)
			e.mu.RUnlock()
		}
	}
	if err != nil {
		return 0, fmt.Errorf("token.Balances: %w", err)
	}
	return bal.Uint64(), nil
}

func (e *EvmChain) SyncRechargeEvents(fromBlock uint64) ([]model.RechargeEvent, error) {
	e.mu.RLock()
	iter, err := e.token.FilterRecharged(&bind.FilterOpts{Start: fromBlock}, nil)
	e.mu.RUnlock()
	if err != nil {
		if rerr := e.reconnect(); rerr == nil {
			e.mu.RLock()
			iter, err = e.token.FilterRecharged(&bind.FilterOpts{Start: fromBlock}, nil)
			e.mu.RUnlock()
		}
	}
	if err != nil || iter == nil {
		return nil, err
	}
	defer iter.Close()

	var events []model.RechargeEvent
	for iter.Next() {
		evt := iter.Event
		events = append(events, model.RechargeEvent{
			User:   model.UniAddr{T: 2, V: evt.User.Bytes()},
			Amount: evt.PointsAmount.Uint64(),
			TxHash: evt.Raw.TxHash.Bytes(),
			Block:  evt.Raw.BlockNumber,
			Chain:  e.chainName,
		})
	}
	return events, nil
}

func (e *EvmChain) SyncWithdrawEvents(fromBlock uint64) ([]model.WithdrawEvent, error) {
	e.mu.RLock()
	iter, err := e.token.FilterWithdrawn(&bind.FilterOpts{Start: fromBlock}, nil)
	e.mu.RUnlock()
	if err != nil {
		if rerr := e.reconnect(); rerr == nil {
			e.mu.RLock()
			iter, err = e.token.FilterWithdrawn(&bind.FilterOpts{Start: fromBlock}, nil)
			e.mu.RUnlock()
		}
	}
	if err != nil || iter == nil {
		return nil, err
	}
	defer iter.Close()

	var events []model.WithdrawEvent
	for iter.Next() {
		evt := iter.Event
		events = append(events, model.WithdrawEvent{
			User:   model.UniAddr{T: 2, V: evt.User.Bytes()},
			Amount: evt.PointsAmount.Uint64(),
			TxHash: evt.Raw.TxHash.Bytes(),
			Block:  evt.Raw.BlockNumber,
			Chain:  e.chainName,
		})
	}
	return events, nil
}
