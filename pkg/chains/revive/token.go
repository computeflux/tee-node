package revive

import (
	"errors"

	"github.com/computeflux/tee-provider/pkg/model"
)

// SyncRecharge tracks a recharge event on chain.
func (c *Contract) SyncRecharge(user model.UniAddr, amount uint64, txHash []byte) error {
	return errors.New("not implemented")
}

// SyncWithdraw tracks a withdraw event on chain.
func (c *Contract) SyncWithdraw(user model.UniAddr, amount uint64, txHash []byte) error {
	return errors.New("not implemented")
}

// GetTotalRecharged returns the total recharged amount for a user.
func (c *Contract) GetTotalRecharged(user model.UniAddr) (uint64, error) {
	return 0, errors.New("not implemented")
}

// SyncRechargeEvents syncs recharge events from chain logs.
func (c *Contract) SyncRechargeEvents(fromBlock uint64) ([]model.RechargeEvent, error) {
	return nil, errors.New("not implemented")
}

// SyncWithdrawEvents syncs withdraw events from chain logs.
func (c *Contract) SyncWithdrawEvents(fromBlock uint64) ([]model.WithdrawEvent, error) {
	return nil, errors.New("not implemented")
}
