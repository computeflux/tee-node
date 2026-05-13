package evm

import (
	"context"
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/core/types"

	"github.com/computeflux/tee-provider/pkg/chains/evm/bindings"
	"github.com/computeflux/tee-provider/pkg/model"
	chain "github.com/wetee-dao/ink.go"
)

// ── Extrinsic helpers ──────────────────────────────────────

func (e *EvmChain) TEECallToCall(tcall *model.TeeCall, dkgKey []byte) (*model.ChainCall, error) {
	return &model.ChainCall{Method: "tee_attest"}, nil
}

func (e *EvmChain) TxCallOfSetNextEpoch(nodeId uint64, signer model.ChainAccountID) (*model.ChainCall, error) {
	abi, err := bindings.SubnetMetaData.GetAbi()
	if err != nil {
		return nil, fmt.Errorf("get ABI: %w", err)
	}
	data, err := abi.Pack("setNextEpoch", nodeId)
	if err != nil {
		return nil, fmt.Errorf("pack setNextEpoch: %w", err)
	}
	return &model.ChainCall{Data: data, Method: "setNextEpoch"}, nil
}

func (e *EvmChain) BatchCall(method string, calls []model.ChainCall) (*model.ChainCall, error) {
	var data []byte
	for _, c := range calls {
		data = append(data, c.Data...)
	}
	return &model.ChainCall{Data: data, Method: method}, nil
}

func (e *EvmChain) CallOfMapAccount(signer model.ChainAccountID) (*model.ChainCall, error) {
	abi, err := bindings.SubnetMetaData.GetAbi()
	if err != nil {
		return nil, fmt.Errorf("get ABI: %w", err)
	}
	data, err := abi.Pack("mapAccount", signer)
	if err != nil {
		return nil, fmt.Errorf("pack mapAccount: %w", err)
	}
	return &model.ChainCall{Data: data, Method: "mapAccount"}, nil
}

// ── Signing ────────────────────────────────────────────────

func (e *EvmChain) PartialSign(signer chain.PartialSignerType, call model.ChainCall) ([]byte, error) {
	return signer.PartialSign(call.Data)
}

func (e *EvmChain) SignAndSubmit(signer chain.SignerType, call model.ChainCall, untilFinalized bool, nonce uint64) error {
	tx := types.NewTransaction(nonce, e.subnetAddr, big.NewInt(0), 300000, big.NewInt(50_000_000_000), call.Data)
	signedTx, err := e.auth.Signer(e.auth.From, tx)
	if err != nil {
		return fmt.Errorf("sign: %w", err)
	}
	e.mu.RLock()
	err = e.client.SendTransaction(context.Background(), signedTx)
	e.mu.RUnlock()
	if err != nil {
		if rerr := e.reconnect(); rerr == nil {
			e.mu.RLock()
			err = e.client.SendTransaction(context.Background(), signedTx)
			e.mu.RUnlock()
		}
	}
	if err != nil {
		return fmt.Errorf("send tx: %w", err)
	}
	if untilFinalized {
		_, err = bind.WaitMined(context.Background(), e.client, signedTx)
		return err
	}
	return nil
}
