package revive

import (
	"encoding/binary"
	"fmt"

	"github.com/centrifuge/go-substrate-rpc-client/v4/types"
	"github.com/computeflux/tee-provider/pkg/types/chaintypes"
	"github.com/computeflux/tee-provider/pkg/util"
	chain "github.com/wetee-dao/ink.go"
	inkutil "github.com/wetee-dao/ink.go/util"
)

// ── ERC20 查询 ────────────────────────────────────────────

// GetErc20List 返回主链已注册的 ERC20 代币列表，并补充每个代币的
// name/symbol/decimals 合约元数据。单个代币元数据查询失败不影响整体列表。
func (c *Contract) GetErc20List() ([]*chaintypes.Erc20Token, error) {
	if c == nil || c.token == nil {
		return nil, fmt.Errorf("token contract not initialized")
	}
	param := chain.DefaultParamWithOrigin(c.signer.AccountID())

	// get_erc20_list 与 get_erc20_list_sol 返回相同的数据（同一注册表），
	// 仅调用约定/解码不同，这里调用一次即可。
	list, _, err := c.token.QueryGetErc20List(param)
	if err != nil {
		return nil, fmt.Errorf("get_erc20_list: %w", err)
	}

	items := make([]*chaintypes.Erc20Token, 0, len(*list))
	for _, t := range *list {
		item := &chaintypes.Erc20Token{
			Address: t.F0.Hex(),
			Active:  t.F1,
			Rate:    t.F2.String(),
			Unit:    t.F3.String(),
		}
		c.fillErc20Meta(item, t.F0, param)
		items = append(items, item)
	}
	return items, nil
}

// fillErc20Meta 查询 ERC20（Solidity）合约的 name/symbol/decimals；失败时保留空值并记录日志。
func (c *Contract) fillErc20Meta(item *chaintypes.Erc20Token, addr types.H160, param chain.DryRunParams) {
	if name, err := queryErc20String(c.ChainClient, addr, "0x06fdde03", param); err == nil {
		item.Name = name
	} else {
		util.LogError("ERC20", "query name", addr.Hex(), err)
	}
	if symbol, err := queryErc20String(c.ChainClient, addr, "0x95d89b41", param); err == nil {
		item.Symbol = symbol
	} else {
		util.LogError("ERC20", "query symbol", addr.Hex(), err)
	}
	if decimals, err := queryErc20Decimals(c.ChainClient, addr, param); err == nil {
		item.Decimals = decimals
	} else {
		util.LogError("ERC20", "query decimals", addr.Hex(), err)
	}
}

// queryErc20String 查询 ERC20（Solidity）合约 string 型只读方法（name/symbol），按 ABI 解码。
func queryErc20String(client *chain.ChainClient, addr types.H160, selector string, param chain.DryRunParams) (string, error) {
	data, err := dryRunErc20Raw(client, addr, param, selector)
	if err != nil {
		return "", err
	}
	return abiDecodeString(data)
}

// queryErc20Decimals 查询 ERC20（Solidity）合约 decimals()（uint8），按 ABI 解码。
func queryErc20Decimals(client *chain.ChainClient, addr types.H160, param chain.DryRunParams) (uint8, error) {
	data, err := dryRunErc20Raw(client, addr, param, "0x313ce567")
	if err != nil {
		return 0, err
	}
	return abiDecodeUint8(data)
}

// dryRunErc20Raw 对任意 ERC20（Solidity）合约执行只读调用，返回原始返回数据（ABI 编码）。
// 与 chain.DryRunInk 相同的调用路径，但保留原始字节供 ABI 解码。
func dryRunErc20Raw(client *chain.ChainClient, address types.H160, param chain.DryRunParams, selector string) ([]byte, error) {
	input, err := (&inkutil.InkContractInput{Selector: selector, Args: []any{}}).Encode()
	if err != nil {
		return nil, fmt.Errorf("encode contract input: %w", err)
	}

	result := inkutil.ContractResult{}
	if err := client.CallRuntimeApi("ReviveApi", "call", []any{
		param.Origin, address, param.PayAmount, param.GasLimit, param.StorageDepositLimit, input,
	}, &result); err != nil {
		return nil, fmt.Errorf("call runtime api: %w", err)
	}

	if result.Result.IsErr {
		return nil, fmt.Errorf("contract call error: %v", result.Result.E)
	}
	if result.Result.V.Flags == 1 {
		return nil, chain.ErrContractReverted
	}
	if len(result.Result.V.Data) == 0 {
		return nil, fmt.Errorf("empty return data")
	}
	return result.Result.V.Data, nil
}

// abiDecodeString 按 Solidity ABI 解码 string 返回值（动态类型：offset + length + data）。
func abiDecodeString(data []byte) (string, error) {
	if len(data) < 64 {
		return "", fmt.Errorf("abi string too short: %d bytes", len(data))
	}
	offset := binary.BigEndian.Uint64(data[24:32])
	if offset+32 > uint64(len(data)) {
		return "", fmt.Errorf("abi string offset out of range: %d", offset)
	}
	length := binary.BigEndian.Uint64(data[offset+24 : offset+32])
	if offset+32+length > uint64(len(data)) {
		return "", fmt.Errorf("abi string length out of range: %d", length)
	}
	return string(data[offset+32 : offset+32+length]), nil
}

// abiDecodeUint8 按 Solidity ABI 解码 uint8 返回值（32 字节 word，值右对齐取最后 1 字节）。
func abiDecodeUint8(data []byte) (uint8, error) {
	if len(data) < 32 {
		return 0, fmt.Errorf("abi uint8 too short: %d bytes", len(data))
	}
	return data[31], nil
}
