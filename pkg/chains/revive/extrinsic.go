package revive

import (
	"errors"
	"fmt"

	"github.com/centrifuge/go-substrate-rpc-client/v4/types"
	"github.com/computeflux/tee-provider/pkg/model"
	"github.com/computeflux/tee-provider/pkg/util"
	"github.com/vedhavyas/go-subkey/v2"
	chain "github.com/wetee-dao/ink.go"
	"github.com/wetee-dao/ink.go/pallet/revive"
)

func (e *Contract) PartialSign(signer chain.PartialSignerType, call model.ChainCall) ([]byte, error) {
	return e.ChainClient.PartialSign(signer, call.Substrate)
}

// SignAndSubmit signs and submits a chain call.
func (c *Contract) SignAndSubmit(signer chain.SignerType, call model.ChainCall, untilFinalized bool, nonce uint64) error {
	util.LogWithPurple("Revive SignAndSubmit", "signer", subkey.SS58Encode(signer.Public(), 42))
	return c.ChainClient.SignAndSubmit(signer, call.Substrate, true, 0)
}

var batchMethods = []string{"batch", "batch_all", "force_batch"}

// BatchCall combines multiple calls into a single batch call.
func (c *Contract) BatchCall(callMethod string, calls []model.ChainCall) (*model.ChainCall, error) {
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

	return &model.ChainCall{Substrate: batchCall}, nil
}

// TEECallToCall converts a TEE attestation into a chain call payload.
func (c *Contract) TEECallToCall(tcall *model.TeeCall, dkgKey []byte) (*model.ChainCall, error) {
	callWrap := tcall.Tx
	switch callWrap.(type) {
	case *model.TeeCall_MapAccount:
		runtimeCall := revive.MakeMapAccountCall()
		call, err := (runtimeCall).AsCall()
		if err != nil {
			util.LogWithRed("DKG SendNewEpochPartialSigToSponsor", "AsCall error:"+err.Error())
			return nil, err
		}
		return &model.ChainCall{Substrate: call}, nil
	}
	return nil, errors.New("invalid tee call")
}

// CheckMapAccount checks if an H160 address is mapped to a Substrate account.
func (c *Contract) CheckMapAccount(addr model.PubKey) (bool, error) {
	_, isSome, err := revive.GetOriginalAccountLatest(c.Api().RPC.State, addr.H160())
	if err != nil {
		util.LogWithRed("DKG SendNewEpochPartialSigToSponsor", "GetOriginalAccountLatest error:"+err.Error())
		return false, err
	}
	return isSome, nil
}

// CallOfMapAccount returns the call payload to map an account.
func (c *Contract) CallOfMapAccount(signer model.PubKey) (*model.ChainCall, error) {
	return nil, errors.New("not implemented")
}
