package revive

import (
	"math/big"

	"github.com/centrifuge/go-substrate-rpc-client/v4/types"
	"github.com/centrifuge/go-substrate-rpc-client/v4/types/codec"
	chain "github.com/wetee-dao/ink.go"
	"github.com/wetee-dao/ink.go/util"
)

func (c *Contract) GetLatestNonce() (uint64, error) {
	param := chain.DefaultParamWithOrigin(c.signer.AccountID())
	v, _, err := chain.DryRunInk[types.U64](
		c.token, param.Origin, param.PayAmount,
		param.GasLimit, param.StorageDepositLimit,
		util.InkContractInput{Selector: "0xbbb975bf"},
	)
	if err != nil || v == nil {
		return 0, err
	}
	return uint64(*v), nil
}

func (c *Contract) GetRelayEvent(nonce uint64) (target, typ, data []byte, err error) {
	param := chain.DefaultParamWithOrigin(c.signer.AccountID())
	raw, _, err := chain.DryRunInk[types.Bytes](
		c.token, param.Origin, param.PayAmount,
		param.GasLimit, param.StorageDepositLimit,
		util.InkContractInput{
			Selector: "0xcd2308da",
			Args:     []any{types.U64(nonce)},
		},
	)
	if err != nil || raw == nil || len(*raw) == 0 || (*raw)[0] == 0 {
		return nil, nil, nil, err
	}
	rec, err := decodeEventRecord((*raw)[1:])
	if err != nil {
		return nil, nil, nil, err
	}
	return rec.TargetContract, rec.EventType, rec.EventData, nil
}

func decodeEventRecord(b []byte) (*eventRecord, error) {
	var rec eventRecord
	if err := codec.Decode(b, &rec); err != nil {
		return nil, err
	}
	return &rec, nil
}

type eventRecord struct {
	TargetContract types.Bytes
	EventType      types.Bytes
	EventData      types.Bytes
}

var _ = big.NewInt
