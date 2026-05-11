package contracts

//go:generate go-ink-gen -json cloud.json
//go:generate go-ink-gen -json subnet.json

import (
	"errors"
	"fmt"
	"math/big"

	"github.com/centrifuge/go-substrate-rpc-client/v4/types"
	chain "github.com/wetee-dao/ink.go"
	"github.com/wetee-dao/ink.go/pallet/revive"
	"github.com/wetee-dao/ink.go/util"
	"github.com/wetee-dao/tee-dsecret/pkg/chains/revive/cloud"
	"github.com/wetee-dao/tee-dsecret/pkg/chains/revive/subnet"

	"github.com/wetee-dao/tee-dsecret/pkg/model"
)

var MainChainConfig *model.ChainConfigType

// Contract
type Contract struct {
	*chain.ChainClient
	signer *chain.Signer
	subnet *subnet.Subnet
	cloud  *cloud.Cloud
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

	cloud, err := cloud.InitCloudContract(client, config.CloudAddress)
	if err != nil {
		util.LogWithPurple("InitCloudContract", config.CloudAddress, err)
		return nil, err
	}

	p := pk.ToSigner()
	h160 := pk.GetPublic().H160()

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

		util.LogWithYellow("MakeMapAccount", p.SS58Address(42))
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
		cloud:       cloud,
	}, nil
}

func (c *Contract) GetChainUrls() []string {
	return MainChainConfig.Urls
}

func (c *Contract) GetClient() *chain.ChainClient {
	return c.ChainClient
}

func (c *Contract) GetSignerAddress() string {
	return c.signer.SS58Address(42)
}

// nodes
func (c *Contract) GetBootPeers() ([]model.P2PAddr, error) {
	result, _, err := c.subnet.QueryBootNodes(chain.DefaultParamWithOrigin(types.AccountID(c.signer.AccountID())))
	if err != nil {
		return nil, err
	}

	nodes := result.V
	boots := make([]model.P2PAddr, 0, len(nodes))
	for _, n := range nodes {
		boots = append(boots, model.P2PAddr{
			Id: n.P2pId,
			Ip: model.Ip{
				Ipv4:   n.Ip.Ipv4,
				Ipv6:   n.Ip.Ipv6,
				Domain: n.Ip.Domain,
			},
			Port: uint16(n.Port),
		})
	}

	return boots, nil
}

func (c *Contract) GetNodes() ([]*model.Validator, []*model.PubKey, error) {
	workers, _, err := c.subnet.QueryWorkers(util.NewNone[uint64](), 5000, chain.DefaultParamWithOrigin(types.AccountID(c.signer.AccountID())))
	if err != nil {
		return nil, nil, err
	}
	dsecrets, _, err := c.subnet.QuerySecrets(chain.DefaultParamWithOrigin(types.AccountID(c.signer.AccountID())))
	if err != nil {
		return nil, nil, err
	}

	nodes := make([]*model.PubKey, 0, len(*workers)+len(*dsecrets))
	validators := make([]*model.Validator, 0, len(*dsecrets))

	for _, v := range *workers {
		nodes = append(nodes, model.PubKeyFromByte(v.F1.P2pId[:]))
	}
	for _, v := range *dsecrets {
		if !v.F1.TerminalBlock.IsNone() {
			continue
		}
		nodes = append(nodes, model.PubKeyFromByte(v.F1.P2pId[:]))
		validators = append(validators, &model.Validator{
			NodeID:      v.F0,
			P2pId:       *model.PubKeyFromByte(v.F1.P2pId[:]),
			ValidatorId: *model.PubKeyFromByte(v.F1.ValidatorId[:]),
		})
	}

	return validators, nodes, nil
}

func (c *Contract) GetValidatorList() ([]*model.Validator, error) {
	dsecrets, _, err := c.subnet.QueryValidators(chain.DefaultParamWithOrigin(types.AccountID(c.signer.AccountID())))
	if err != nil {
		return nil, err
	}

	validators := make([]*model.Validator, 0, len(*dsecrets))
	for _, v := range *dsecrets {
		validators = append(validators, &model.Validator{
			NodeID:      uint64(v.F0),
			P2pId:       *model.PubKeyFromByte(v.F1.P2pId[:]),
			ValidatorId: *model.PubKeyFromByte(v.F1.ValidatorId[:]),
		})
	}

	return validators, nil
}

// epoch
func (c *Contract) GetEpoch() (uint32, uint32, uint32, uint32, types.H160, error) {
	d, _, err := c.subnet.QueryEpochInfo(chain.DefaultParamWithOrigin(types.AccountID(c.signer.AccountID())))
	if err != nil {
		return 0, 0, 0, 0, types.H160{}, err
	}

	return d.Epoch, d.EpochSlot, d.LastEpochBlock, d.Now, d.SideChainPub, nil
}

// go to new epoch
func (c *Contract) SetNewEpoch(nodeId uint64) error {
	return c.subnet.ExecSetNextEpoch(nodeId, chain.ExecParams{
		Signer:    c.signer,
		PayAmount: types.NewU128(*big.NewInt(0)),
	})
}

func (c *Contract) TxCallOfSetNextEpoch(nodeId uint64, signer types.AccountID) (*types.Call, error) {
	return c.subnet.CallOfSetNextEpoch(nodeId, chain.DryRunParams{
		Origin:    signer,
		PayAmount: types.NewU128(*big.NewInt(0)),
	})
}

func (c *Contract) GetNextEpochValidatorList() ([]*model.Validator, error) {
	dsecrets, _, err := c.subnet.QueryNextEpochValidators(chain.DefaultParamWithOrigin(types.AccountID(c.signer.AccountID())))
	if err != nil {
		return nil, err
	}

	validators := make([]*model.Validator, 0, len(dsecrets.V))
	for _, v := range dsecrets.V {
		validators = append(validators, &model.Validator{
			NodeID:      v.F0,
			P2pId:       *model.PubKeyFromByte(v.F1.P2pId[:]),
			ValidatorId: *model.PubKeyFromByte(v.F1.ValidatorId[:]),
		})
	}

	return validators, nil
}

func (c *Contract) GetMintInterval() (uint32, error) {
	v, _, err := c.cloud.QueryMintInterval(chain.DryRunParams{
		Origin:    c.signer.AccountID(),
		PayAmount: types.NewU128(*big.NewInt(0)),
	})
	if err != nil {
		return 100000000, err
	}

	return *v, nil
}

func ConvertTEEType(t cloud.TEEType) model.TEEType {
	return model.TEEType{
		SGX: t.SGX,
		CVM: t.CVM,
	}
}
