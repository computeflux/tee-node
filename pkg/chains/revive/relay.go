package revive

import (
	"math/big"

	"github.com/centrifuge/go-substrate-rpc-client/v4/types"
	"github.com/centrifuge/go-substrate-rpc-client/v4/types/codec"
	chain "github.com/wetee-dao/ink.go"
	"github.com/wetee-dao/ink.go/util"
)

// ── Relay 同步 ───────────────────────────────────────────

// GetLatestNonce 从主链 Token 合约获取最新事件 nonce
func (c *Contract) GetLatestNonce() (uint64, error) {
	param := chain.DefaultParamWithOrigin(c.signer.AccountID())
	v, _, err := chain.DryRunInk[types.U64](
		c.token, param.Origin, param.PayAmount,
		param.GasLimit, param.StorageDepositLimit,
		util.InkContractInput{Selector: "0xbbb975bf"},
	)
	if err != nil {
		return 0, err
	}
	if v == nil {
		return 0, nil
	}
	return uint64(*v), nil
}

// GetRelayEvent 获取指定 nonce 的 relay 事件
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
	if err != nil {
		return nil, nil, nil, err
	}
	if raw == nil || len(*raw) == 0 || (*raw)[0] == 0 {
		return nil, nil, nil, nil
	}
	// raw = Option::Some → 跳过头字节 0x01
	rec, err := decodeEventRecord((*raw)[1:])
	if err != nil {
		return nil, nil, nil, err
	}
	return rec.TargetContract, rec.EventType, rec.EventData, nil
}

// GetChainID 获取主链注册的 chainId
func (c *Contract) GetChainID() uint32 {
	param := chain.DefaultParamWithOrigin(c.signer.AccountID())
	v, _, err := chain.DryRunInk[types.U32](
		c.token, param.Origin, param.PayAmount,
		param.GasLimit, param.StorageDepositLimit,
		util.InkContractInput{Selector: "0x05ac82a3"},
	)
	if err != nil {
		return 1
	}
	if v == nil {
		return 1
	}
	return uint32(*v)
}

// ── SCALE 解码 ───────────────────────────────────────────

type eventRecord struct {
	TargetContract types.Bytes
	EventType      types.Bytes
	EventData      types.Bytes
}

func decodeEventRecord(b []byte) (*eventRecord, error) {
	var rec eventRecord
	if err := codec.Decode(b, &rec); err != nil {
		return nil, err
	}
	return &rec, nil
}

var _ = big.NewInt // keep import for future use
