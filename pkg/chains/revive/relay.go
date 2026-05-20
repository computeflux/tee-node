package revive

import (
	"github.com/computeflux/tee-provider/pkg/model"
	chain "github.com/wetee-dao/ink.go"
)

// GetLatestNonce 查询主链合约最新事件 nonce | Query latest event nonce on main chain contract
func (c *Contract) GetLatestNonce() (uint64, error) {
	param := chain.DefaultParamWithOrigin(c.signer.AccountID())
	v, _, err := c.token.QueryGetLatestNonce(param)
	if err != nil || v == nil {
		return 0, err
	}
	return uint64(*v), nil
}

// GetRelayEvents 批量查询 events (from..=to) | Batch query events from..=to
func (c *Contract) GetRelayEvents(from, to uint64) ([]model.RelayEvent, error) {
	param := chain.DefaultParamWithOrigin(c.signer.AccountID())
	records, _, err := c.token.QueryGetEvents(from, to, param)
	if err != nil {
		return nil, err
	}
	if records == nil || len(*records) == 0 {
		return nil, nil
	}
	events := make([]model.RelayEvent, 0, len(*records))
	for _, rec := range *records {
		events = append(events, model.RelayEvent{
			TargetContract: rec.TargetContract,
			EventType:      rec.EventType,
			EventData:      rec.EventData,
		})
	}
	return events, nil
}
