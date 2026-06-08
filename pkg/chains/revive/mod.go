package revive

//go:generate go-ink-gen -json ./abi/subnet.json
//go:generate go-ink-gen -json ./abi/token.json
//go:generate go-ink-gen -json ./abi/proxy.json

import (
	"errors"

	"github.com/computeflux/tee-provider/pkg/chains/revive/subnet"
	"github.com/computeflux/tee-provider/pkg/chains/revive/token"
	"github.com/computeflux/tee-provider/pkg/chaintypes"
	"github.com/computeflux/tee-provider/pkg/crypto"
	chain "github.com/wetee-dao/ink.go"
	"github.com/wetee-dao/ink.go/pallet/revive"
	"github.com/wetee-dao/ink.go/util"
)

// Contract
type Contract struct {
	*chain.ChainClient
	config *chaintypes.ChainConfigType
	signer chain.SignerType
	subnet *subnet.Subnet
	token  *token.Token
}

func NewContract(config *chaintypes.ChainConfigType, pk *crypto.PrivKey) (*Contract, error) {
	client, err := chain.InitClient(config.Urls, false)
	if err != nil {
		return nil, err
	}

	subnet, err := subnet.InitSubnetContract(client, config.SubnetAddress)
	if err != nil {
		util.LogWithPurple("InitSubnetContract", err)
		return nil, err
	}

	token, err := token.InitTokenContract(client, config.TokenAddress)
	if err != nil {
		util.LogWithPurple("InitTokenContract", err)
		return nil, err
	}

	p := pk.ToSigner(crypto.SuiteEd25519)
	h160 := pk.GetPublicForSuite(crypto.SuiteEd25519).H160()

	// check account is mapaccount in revive
	_, isSome, err := revive.GetOriginalAccountLatest(client.Api().RPC.State, h160)
	if err != nil {
		util.LogWithPurple("GetOriginalAccountLatest", err)
		return nil, err
	}
	if !isSome {
		runtimeCall := revive.MakeMapAccountCall()
		call, err := (runtimeCall).AsCall()
		if err != nil {
			util.LogWithPurple("MakeMapAccountCall", err)
			return nil, err
		}

		util.LogWithYellow("MakeMapAccount", pk.GetPublicForSuite(crypto.SuiteEd25519).SS58())
		err = client.SignAndSubmit(p, call, true, 0)
		if err != nil {
			return nil, err
		}
	}

	// test get block number
	blockNumber, err := client.GetBlockNumber()
	if err != nil {
		return nil, errors.New("CheckBlock error: " + err.Error())
	}
	util.LogWithYellow("Main Chain", "block number", blockNumber)

	return &Contract{
		config:      config,
		ChainClient: client,
		signer:      p,
		subnet:      subnet,
		token:       token,
	}, nil
}

func (c *Contract) GetChainUrls() []string {
	return c.config.Urls
}

func (c *Contract) GetClient() *chain.ChainClient {
	return c.ChainClient
}

func (c *Contract) GetSignerAddress() string {
	return crypto.PubKeyFromByte(c.signer.Public()).SS58()
}

// ChainType returns the chain type string.
func (c *Contract) ChainType() string {
	return "substrate"
}

func (c *Contract) ChainId() uint32 {
	return c.config.ChainID
}

// GetCipher returns the signing suite used by this chain.
func (c *Contract) GetCipher() crypto.SuiteType {
	return crypto.SuiteEd25519
}
