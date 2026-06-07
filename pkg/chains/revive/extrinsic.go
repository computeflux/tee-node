package revive

import (
	"errors"
	"fmt"
	"math/big"

	"github.com/centrifuge/go-substrate-rpc-client/v4/types"
	"github.com/computeflux/tee-provider/pkg/chaintypes"
	"github.com/computeflux/tee-provider/pkg/crypto"
	"github.com/computeflux/tee-provider/pkg/model"
	"github.com/computeflux/tee-provider/pkg/util"
	"github.com/vedhavyas/go-subkey/v2"
	chain "github.com/wetee-dao/ink.go"
	"github.com/wetee-dao/ink.go/pallet/revive"
)

func (e *Contract) PartialSign(signer chain.PartialSignerType, call chaintypes.ChainCall) ([]byte, error) {
	return e.ChainClient.PartialSign(signer, call.Substrate)
}

// SignAndSubmit signs and submits a chain call.
func (c *Contract) SignAndSubmit(signer chain.SignerType, call chaintypes.ChainCall, untilFinalized bool, nonce uint64) error {
	util.LogWithPurple("Revive SignAndSubmit", "signer", subkey.SS58Encode(signer.Public(), 42))
	return c.ChainClient.SignAndSubmit(signer, call.Substrate, false, 0)
}

var batchMethods = []string{"batch", "batch_all", "force_batch"}

// BatchCall combines multiple calls into a single batch call.
func (c *Contract) BatchCall(callMethod string, calls []chaintypes.ChainCall) (*chaintypes.ChainCall, error) {
	isIn := false
	for _, m := range batchMethods {
		if m == callMethod {
			isIn = true
		}
	}
	if !isIn {
		return nil, fmt.Errorf("callMethod %s is not in batchMethods %v", callMethod, batchMethods)
	}

	batchCall, err := types.NewCall(c.Meta, "Utility."+callMethod, calls)
	if err != nil {
		return nil, fmt.Errorf("new BatchCall error: %w", err)
	}

	return &chaintypes.ChainCall{Substrate: batchCall}, nil
}

// TEECallToCall converts a TEE attestation into a chain call payload.
func (c *Contract) TEECallToCall(tcall *model.TeeCall, dkgKey []byte) (*chaintypes.ChainCall, error) {
	pub, err := types.NewAccountID(dkgKey)
	if err != nil {
		return nil, err
	}

	var runtimeCall *types.Call

	switch call := tcall.Tx.(type) {
	case *model.TeeCall_WithdrawApply:
		if call.WithdrawApply.GetContract() == nil {
			runtimeCall, err = c.token.CallOfWithdraw(
				types.H160(call.WithdrawApply.User),
				types.NewU256(*big.NewInt(0).SetUint64(call.WithdrawApply.Credit)),
				call.WithdrawApply.Nonce,
				chain.DryRunParams{
					Origin: *pub,
				},
			)
			if err != nil {
				return nil, err
			}
		} else {
			runtimeCall, err = c.token.CallOfWithdrawErc20(
				types.H160(call.WithdrawApply.Contract),
				types.H160(call.WithdrawApply.User),
				types.NewU256(*big.NewInt(0).SetUint64(call.WithdrawApply.Credit)),
				call.WithdrawApply.Nonce,
				chain.DryRunParams{
					Origin: *pub,
				},
			)
			if err != nil {
				return nil, err
			}
		}
	}

	if runtimeCall != nil {
		return &chaintypes.ChainCall{
			Substrate: *runtimeCall,
		}, nil
	}

	return nil, errors.New("invalid tee call")
}

// CheckMapAccount checks if an H160 address is mapped to a Substrate account.
func (c *Contract) CheckMapAccount(addr crypto.PubKey) (bool, error) {
	_, isSome, err := revive.GetOriginalAccountLatest(c.Api().RPC.State, addr.H160())
	if err != nil {
		util.LogWithRed("DKG SendNewEpochPartialSigToSponsor", "GetOriginalAccountLatest error:"+err.Error())
		return false, err
	}
	return isSome, nil
}

// CallOfMapAccount returns the call payload to map an account.
func (c *Contract) CallOfMapAccount(signer crypto.PubKey) (*chaintypes.ChainCall, error) {
	return nil, errors.New("not implemented")
}
