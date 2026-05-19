package revive

//go:generate go-ink-gen -json ./abi/subnet.json
//go:generate go-ink-gen -json ./abi/token.json
//go:generate go-ink-gen -json ./abi/proxy.json

import (
	"errors"
	"fmt"

	"github.com/computeflux/tee-provider/pkg/chains/revive/subnet"
	"github.com/computeflux/tee-provider/pkg/chains/revive/token"
	chain "github.com/wetee-dao/ink.go"
	"github.com/wetee-dao/ink.go/pallet/revive"
	"github.com/wetee-dao/ink.go/util"

	"github.com/computeflux/tee-provider/pkg/model"
)

var MainChainConfig *model.ChainConfigType

// Contract
type Contract struct {
	*chain.ChainClient
	signer chain.SignerType
	subnet *subnet.Subnet
	token  *token.Token
}

func GetCloudAddress() string {
	return MainChainConfig.CloudAddress
}

func GetSubnetAddress() string {
	return MainChainConfig.SubnetAddress
}

func NewContract(config *model.ChainConfigType, pk *model.PrivKey) (*Contract, error) {
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

	p := pk.ToSigner(model.SuiteEd25519)
	h160 := pk.GetPublicForSuite(model.SuiteEd25519).H160()

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

		util.LogWithYellow("MakeMapAccount", pk.GetPublicForSuite(model.SuiteEd25519).SS58())
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
	fmt.Println("block number", blockNumber)

	return &Contract{
		ChainClient: client,
		signer:      p,
		subnet:      subnet,
		token:       token,
	}, nil
}

func (c *Contract) GetChainUrls() []string {
	return MainChainConfig.Urls
}

func (c *Contract) GetClient() *chain.ChainClient {
	return c.ChainClient
}

func (c *Contract) GetSignerAddress() string {
	return model.PubKeyFromByte(c.signer.Public()).SS58()
}

// ChainType returns the chain type string.
func (c *Contract) ChainType() string {
	return "substrate"
}

// GetCipher returns the signing suite used by this chain.
func (c *Contract) GetCipher() model.SuiteType {
	return model.SuiteEd25519
}
