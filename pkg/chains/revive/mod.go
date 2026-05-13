package contracts

//go:generate go-ink-gen -json cloud.json
//go:generate go-ink-gen -json subnet.json

import (
	"errors"
	"fmt"
	"math/big"

	"github.com/centrifuge/go-substrate-rpc-client/v4/types"
	"github.com/computeflux/tee-provider/pkg/chains/revive/cloud"
	"github.com/computeflux/tee-provider/pkg/chains/revive/subnet"
	"github.com/computeflux/tee-provider/pkg/model"
	chain "github.com/wetee-dao/ink.go"
	"github.com/wetee-dao/ink.go/pallet/revive"
	"github.com/wetee-dao/ink.go/util"
)

var MainChainConfig *model.ChainConfigType

// Contract implements MainChainApi for Substrate (revive) chains.
type Contract struct {
	*chain.ChainClient
	signer *chain.Signer
	subnet *subnet.Subnet
	cloud  *cloud.Cloud
}

// ── ChainApi / MainChainApi identity ───────────────────────

func (c *Contract) ChainType() string { return "substrate" }

func (c *Contract) GetCipher() model.SuiteType { return model.SuiteEd25519 }

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

	signer := pk.ToSigner(model.SuiteEd25519)
	// signer is chain.SignerType; for Substrate we need *chain.Signer.
	// The ed25519 path returns *chain.Signer directly.
	subSigner, ok := signer.(*chain.Signer)
	if !ok {
		return nil, errors.New("substrate requires ed25519 signer")
	}

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

		util.LogWithYellow("MakeMapAccount", subSigner.SS58Address(42))
		err = client.SignAndSubmit(subSigner, call, true, 0)
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
		signer:      subSigner,
		subnet:      subnet,
		cloud:       cloud,
	}, nil
}

// ── ChainApi ──────────────────────────────────────────────

func (c *Contract) GetChainUrls() []string {
	return MainChainConfig.Urls
}

func (c *Contract) GetClient() *chain.ChainClient {
	return c.ChainClient
}

func (c *Contract) GetSignerAddress() string {
	return c.signer.SS58Address(42)
}

// ── Nodes / Validators ────────────────────────────────────

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

// ── Epoch ─────────────────────────────────────────────────

func (c *Contract) GetEpoch() (uint32, uint32, uint32, uint32, model.ChainH160, error) {
	d, _, err := c.subnet.QueryEpochInfo(chain.DefaultParamWithOrigin(types.AccountID(c.signer.AccountID())))
	if err != nil {
		return 0, 0, 0, 0, model.ChainH160{}, err
	}

	var h160 model.ChainH160
	copy(h160[:], d.SideChainPub[:])
	return d.Epoch, d.EpochSlot, d.LastEpochBlock, d.Now, h160, nil
}

func (c *Contract) SetNewEpoch(nodeId uint64) error {
	return c.subnet.ExecSetNextEpoch(nodeId, chain.ExecParams{
		Signer:    c.signer,
		PayAmount: types.NewU128(*big.NewInt(0)),
	})
}

func (c *Contract) TxCallOfSetNextEpoch(nodeId uint64, signer model.ChainAccountID) (*model.ChainCall, error) {
	call, err := c.subnet.CallOfSetNextEpoch(nodeId, chain.DryRunParams{
		Origin:    types.AccountID(signer),
		PayAmount: types.NewU128(*big.NewInt(0)),
	})
	if err != nil {
		return nil, err
	}
	return &model.ChainCall{Data: call.Args, Method: "setNextEpoch"}, nil
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

// ── Extrinsic helpers ─────────────────────────────────────

func (c *Contract) BatchCall(method string, calls []model.ChainCall) (*model.ChainCall, error) {
	var data []byte
	for _, call := range calls {
		data = append(data, call.Data...)
	}
	return &model.ChainCall{Data: data, Method: method}, nil
}

func (c *Contract) CheckMapAccount(addr model.ChainH160) (bool, error) {
	h160 := types.NewH160(addr[:])
	_, isSome, err := revive.GetOriginalAccountLatest(c.Api().RPC.State, h160)
	return isSome, err
}

func (c *Contract) CallOfMapAccount(signer model.ChainAccountID) (*model.ChainCall, error) {
	runtimeCall := revive.MakeMapAccountCall()
	call, err := (runtimeCall).AsCall()
	if err != nil {
		return nil, err
	}
	return &model.ChainCall{Data: call.Args, Method: "mapAccount"}, nil
}

func (c *Contract) PartialSign(signer chain.PartialSignerType, call model.ChainCall) ([]byte, error) {
	return signer.PartialSign(call.Data)
}

func (c *Contract) SignAndSubmit(signer chain.SignerType, call model.ChainCall, untilFinalized bool, nonce uint64) error {
	// Build a Substrate Call from the data and submit via chain client.
	subCall := types.Call{Args: call.Data}
	return c.ChainClient.SignAndSubmit(c.signer, subCall, untilFinalized, nonce)
}

// ── Points (stubs for Substrate) ──────────────────────────

func (c *Contract) TEECallToCall(tcall *model.TeeCall, dkgKey []byte) (*model.ChainCall, error) {
	var accountID types.AccountID
	copy(accountID[:], dkgKey)
	return c.teeCallToSubstrateCall(tcall, accountID)
}

// teeCallToSubstrateCall wraps the internal Substrate implementation.
func (c *Contract) teeCallToSubstrateCall(tcall *model.TeeCall, dkgKey types.AccountID) (*model.ChainCall, error) {
	call, err := teeCallToSubstrateCall(c, tcall, dkgKey)
	if err != nil {
		return nil, err
	}
	return &model.ChainCall{Data: call.Args, Method: "tee_call"}, nil
}

func (c *Contract) SyncRecharge(user model.UniAddr, amount uint64, txHash []byte) error { return nil }
func (c *Contract) SyncWithdraw(user model.UniAddr, amount uint64, txHash []byte) error { return nil }
func (c *Contract) RecordSpend(user model.UniAddr, amount uint64) error                 { return nil }
func (c *Contract) GetTotalRecharged(user model.UniAddr) (uint64, error)                { return 0, nil }
func (c *Contract) GetTotalSpent(user model.UniAddr) (uint64, error)                    { return 0, nil }
func (c *Contract) SyncRechargeEvents(fromBlock uint64) ([]model.RechargeEvent, error) {
	return nil, nil
}
func (c *Contract) SyncWithdrawEvents(fromBlock uint64) ([]model.WithdrawEvent, error) {
	return nil, nil
}
func (c *Contract) SyncProviderRegister(provider model.UniAddr, name string, endpoint string) error {
	return nil
}
func (c *Contract) SyncModelRegister(provider model.UniAddr, modelName string, pointsPer1K uint64) error {
	return nil
}

// ── Misc ──────────────────────────────────────────────────

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
