package revive

import (
	"errors"

	"github.com/computeflux/tee-provider/pkg/model"
	chain "github.com/wetee-dao/ink.go"
)

func (e *Contract) PartialSign(signer chain.PartialSignerType, call model.ChainCall) ([]byte, error) {
	return e.ChainClient.PartialSign(signer, call.Substrate)
}

// SignAndSubmit signs and submits a chain call.
func (c *Contract) SignAndSubmit(signer chain.SignerType, call model.ChainCall, untilFinalized bool, nonce uint64) error {
	return c.ChainClient.SignAndSubmit(signer, call.Substrate, false, 0)
}

// BatchCall combines multiple calls into a single batch call.
func (c *Contract) BatchCall(method string, calls []model.ChainCall) (*model.ChainCall, error) {
	return nil, errors.New("not implemented")
}

// CheckMapAccount checks if an H160 address is mapped to a Substrate account.
func (c *Contract) CheckMapAccount(addr model.ChainH160) (bool, error) {
	return false, errors.New("not implemented")
}

// CallOfMapAccount returns the call payload to map an account.
func (c *Contract) CallOfMapAccount(signer model.ChainAccountID) (*model.ChainCall, error) {
	return nil, errors.New("not implemented")
}
