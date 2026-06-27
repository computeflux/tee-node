package revive

import (
	"errors"
	"fmt"
	"math/big"

	"github.com/centrifuge/go-substrate-rpc-client/v4/types"
	"github.com/computeflux/tee-provider/pkg/types/chaintypes"
	"github.com/computeflux/tee-provider/pkg/types/crypto"
	"github.com/computeflux/tee-provider/pkg/util"
	"github.com/computeflux/tee-provider/tee-chain/runtime"
	"github.com/computeflux/tee-provider/tee-chain/runtime/module/batch"
	"github.com/vedhavyas/go-subkey/v2"
	chain "github.com/wetee-dao/ink.go"
	"github.com/wetee-dao/ink.go/pallet/revive"
)

func (e *Contract) PartialSign(signer chain.PartialSignerType, call chaintypes.ChainCall) ([]byte, error) {
	util.LogWithPurple("PartialSign", "signer", subkey.SS58Encode(signer.Public(), 42))
	return e.ChainClient.PartialSign(signer, call.Substrate)
}

// SignAndSubmit signs and submits a chain call.
func (c *Contract) SignAndSubmit(signer chain.SignerType, call chaintypes.ChainCall, untilFinalized bool, nonce uint64) error {
	util.LogWithPurple("Submit SignAndSubmit", "signer", subkey.SS58Encode(signer.Public(), 42))
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

	substrateCalls := make([]types.Call, 0, len(calls))
	for _, c := range calls {
		substrateCalls = append(substrateCalls, c.Substrate)
	}

	batchCall, err := types.NewCall(c.Meta, "Utility."+callMethod, substrateCalls)
	if err != nil {
		return nil, fmt.Errorf("new BatchCall error: %w", err)
	}

	return &chaintypes.ChainCall{Substrate: batchCall}, nil
}

// TEECallToCall converts a TEE attestation into a chain call payload.
func (c *Contract) TEECallToCall(tcall *batch.HubCallIndex, dkgKey []byte) (*chaintypes.ChainCall, error) {
	pub, err := types.NewAccountID(dkgKey)
	if err != nil {
		return nil, err
	}

	var runtimeCall *types.Call

	switch tcall.CallType {
	case 0:
		withdrawApply, err := runtime.QueryWithdrawApply(tcall.CallID)
		if err != nil {
			return nil, err
		}

		if withdrawApply.Contract != nil {
			runtimeCall, err = c.token.CallOfWithdraw(
				types.H160(withdrawApply.User.V),
				types.NewU256(*withdrawApply.Credit.Int),
				withdrawApply.Nonce,
				chain.DryRunParams{
					Origin:    *pub,
					PayAmount: types.NewU128(*big.NewInt(0)),
				},
			)
			if err != nil {
				return nil, err
			}
		} else {
			runtimeCall, err = c.token.CallOfWithdrawErc20(
				types.H160(withdrawApply.Contract),
				types.H160(withdrawApply.User.V),
				types.NewU256(*withdrawApply.Credit.Int),
				withdrawApply.Nonce,
				chain.DryRunParams{
					Origin:    *pub,
					PayAmount: types.NewU128(*big.NewInt(0)),
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
