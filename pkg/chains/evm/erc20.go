// erc20.go EVM 链已注册 ERC20 代币列表（供 GraphQL / 前端展示）。
package evm

import (
	"context"
	"fmt"
	"strings"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"

	"github.com/computeflux/tee-provider/pkg/types/chaintypes"
)

// erc20MetaABI reads name/symbol/decimals from an ERC20 contract.
var erc20MetaABI = func() *abi.ABI {
	parsed, err := abi.JSON(strings.NewReader(`[
		{"type":"function","name":"name","stateMutability":"view","inputs":[],"outputs":[{"type":"string"}]},
		{"type":"function","name":"symbol","stateMutability":"view","inputs":[],"outputs":[{"type":"string"}]},
		{"type":"function","name":"decimals","stateMutability":"view","inputs":[],"outputs":[{"type":"uint8"}]}
	]`))
	if err != nil {
		panic("evm: bad erc20 meta ABI: " + err.Error())
	}
	return &parsed
}()

// GetErc20List returns the ERC20 tokens registered on the Token contract,
// enriched with name/symbol/decimals from each token contract. Per-token
// metadata failures leave the fields empty without failing the whole list.
func (e *EvmChain) GetErc20List() ([]*chaintypes.Erc20Token, error) {
	e.mu.RLock()
	client := e.client
	token := e.token
	e.mu.RUnlock()

	if token == nil {
		return nil, fmt.Errorf("evm: token contract not bound")
	}

	out, err := token.GetERC20List(&bind.CallOpts{Context: context.Background()})
	if err != nil {
		return nil, fmt.Errorf("evm: getERC20List: %w", err)
	}

	tokens := make([]*chaintypes.Erc20Token, 0, len(out.Tokens))
	for i, addr := range out.Tokens {
		item := &chaintypes.Erc20Token{Address: addr.Hex()}
		if i < len(out.Actives) {
			item.Active = out.Actives[i]
		}
		if i < len(out.Rates) && out.Rates[i] != nil {
			item.Rate = out.Rates[i].String()
		}
		if i < len(out.Units) && out.Units[i] != nil {
			item.Unit = out.Units[i].String()
		}

		ctx := context.Background()
		if name, ok := callERC20String(ctx, client, addr, "name"); ok {
			item.Name = name
		}
		if symbol, ok := callERC20String(ctx, client, addr, "symbol"); ok {
			item.Symbol = symbol
		}
		if decimals, ok := callERC20Uint8(ctx, client, addr, "decimals"); ok {
			item.Decimals = decimals
		}
		tokens = append(tokens, item)
	}
	return tokens, nil
}

func callERC20String(ctx context.Context, client *ethclient.Client, addr common.Address, method string) (string, bool) {
	out, err := client.CallContract(ctx, ethereum.CallMsg{To: &addr, Data: erc20MetaABI.Methods[method].ID}, nil)
	if err != nil {
		return "", false
	}
	vals, err := erc20MetaABI.Methods[method].Outputs.Unpack(out)
	if err != nil || len(vals) == 0 {
		return "", false
	}
	s, ok := vals[0].(string)
	return s, ok
}

func callERC20Uint8(ctx context.Context, client *ethclient.Client, addr common.Address, method string) (uint8, bool) {
	out, err := client.CallContract(ctx, ethereum.CallMsg{To: &addr, Data: erc20MetaABI.Methods[method].ID}, nil)
	if err != nil {
		return 0, false
	}
	vals, err := erc20MetaABI.Methods[method].Outputs.Unpack(out)
	if err != nil || len(vals) == 0 {
		return 0, false
	}
	v, ok := vals[0].(uint8)
	return v, ok
}
