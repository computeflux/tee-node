package revive

import (
	"errors"
	"math/big"

	"github.com/centrifuge/go-substrate-rpc-client/v4/types"
	"github.com/computeflux/tee-provider/pkg/chains/revive/subnet"
	"github.com/computeflux/tee-provider/pkg/types/chaintypes"
	"github.com/computeflux/tee-provider/pkg/types/crypto"
	chain "github.com/wetee-dao/ink.go"
	"github.com/wetee-dao/ink.go/util"
)

func (c *Contract) GetMintWorker(user types.AccountID) (*chaintypes.K8sCluster, error) {
	workerWrap, _, err := c.subnet.QueryMintWorker(user, chain.DefaultParamWithOrigin(types.AccountID(c.signer.AccountID())))
	if err != nil {
		return nil, err
	}

	if workerWrap.IsNone() {
		return nil, errors.New("worker not found")
	}

	tuple, _ := workerWrap.UnWrap()
	worker := tuple.F1
	return &chaintypes.K8sCluster{
		Id:            tuple.F0,
		Name:          worker.Name,
		Owner:         worker.Owner,
		Level:         worker.Level,
		RegionId:      worker.RegionId,
		StartBlock:    worker.StartBlock,
		StopBlock:     worker.StopBlock,
		TerminalBlock: worker.TerminalBlock,
		P2pId:         worker.P2pId,
		Ip:            chaintypes.Ip(worker.Ip),
		Port:          worker.Port,
		Status:        worker.Status,
	}, nil
}

func (c *Contract) GetWorker(workerId uint64) (*chaintypes.K8sCluster, error) {
	workerWrap, _, err := c.subnet.QueryWorker(workerId, chain.DefaultParamWithOrigin(types.AccountID(c.signer.AccountID())))
	if err != nil {
		return nil, err
	}

	if workerWrap.IsNone() {
		return nil, errors.New("worker not found")
	}

	worker, _ := workerWrap.UnWrap()

	return &chaintypes.K8sCluster{
		Id:            workerId,
		Name:          worker.Name,
		Owner:         worker.Owner,
		Level:         worker.Level,
		RegionId:      worker.RegionId,
		StartBlock:    worker.StartBlock,
		StopBlock:     worker.StopBlock,
		TerminalBlock: worker.TerminalBlock,
		P2pId:         worker.P2pId,
		Ip:            chaintypes.Ip(worker.Ip),
		Port:          worker.Port,
		Status:        worker.Status,
	}, nil
}

func (c *Contract) ResigerCluster(name []byte, p2p_id [32]byte, ip chaintypes.Ip, port uint32, level byte, region_id uint32) error {
	return c.subnet.ExecWorkerRegister(name, p2p_id, subnet.Ip(ip), port, level, region_id, chain.ExecParams{
		Signer:    c.signer,
		PayAmount: types.NewU128(*big.NewInt(0)),
	})
}

// nodes
func (c *Contract) GetBootPeers() ([]chaintypes.P2PAddr, error) {
	result, _, err := c.subnet.QueryBootNodes(chain.DefaultParamWithOrigin(types.AccountID(c.signer.AccountID())))
	if err != nil {
		return nil, err
	}

	nodes := result.V
	boots := make([]chaintypes.P2PAddr, 0, len(nodes))
	for _, n := range nodes {
		boots = append(boots, chaintypes.P2PAddr{
			Id: n.P2pId,
			Ip: chaintypes.Ip{
				Ipv4:   n.Ip.Ipv4,
				Ipv6:   n.Ip.Ipv6,
				Domain: n.Ip.Domain,
			},
			Port: uint16(n.Port),
		})
	}

	return boots, nil
}

func (c *Contract) GetNodes() ([]*chaintypes.Validator, []*crypto.PubKey, error) {
	workers, _, err := c.subnet.QueryWorkers(util.NewNone[uint64](), 5000, chain.DefaultParamWithOrigin(types.AccountID(c.signer.AccountID())))
	if err != nil {
		return nil, nil, err
	}
	dsecrets, _, err := c.subnet.QuerySecrets(chain.DefaultParamWithOrigin(types.AccountID(c.signer.AccountID())))
	if err != nil {
		return nil, nil, err
	}

	nodes := make([]*crypto.PubKey, 0, len(*workers)+len(*dsecrets))
	validators := make([]*chaintypes.Validator, 0, len(*dsecrets))

	for _, v := range *workers {
		nodes = append(nodes, crypto.PubKeyFromByte(v.F1.P2pId[:]))
	}
	for _, v := range *dsecrets {
		if !v.F1.TerminalBlock.IsNone() {
			continue
		}
		nodes = append(nodes, crypto.PubKeyFromByte(v.F1.P2pId[:]))
		validators = append(validators, &chaintypes.Validator{
			NodeID:      v.F0,
			P2pId:       *crypto.PubKeyFromByte(v.F1.P2pId[:]),
			ValidatorId: *crypto.PubKeyFromByte(v.F1.ValidatorId[:]),
		})
	}

	return validators, nodes, nil
}

func (c *Contract) GetValidatorList() ([]*chaintypes.Validator, error) {
	dsecrets, _, err := c.subnet.QueryValidators(chain.DefaultParamWithOrigin(types.AccountID(c.signer.AccountID())))
	if err != nil {
		return nil, err
	}

	validators := make([]*chaintypes.Validator, 0, len(*dsecrets))
	for _, v := range *dsecrets {
		validators = append(validators, &chaintypes.Validator{
			NodeID:         uint64(v.F0),
			P2pId:          *crypto.PubKeyFromByte(v.F1.P2pId[:]),
			ValidatorId:    *crypto.PubKeyFromByte(v.F1.ValidatorId[:]),
			ValidatorIdBLS: *crypto.PubKeyFromBLS12381Byte(v.F1.Bls[:]),
		})
	}

	return validators, nil
}

// epoch
func (c *Contract) GetEpoch() (uint32, uint32, uint32, uint32, crypto.PubKey, error) {
	d, _, err := c.subnet.QueryEpochInfo(chain.DefaultParamWithOrigin(types.AccountID(c.signer.AccountID())))
	if err != nil {
		return 0, 0, 0, 0, crypto.PubKey{}, err
	}

	return d.Epoch, d.EpochSlot, d.LastEpochBlock, d.Now, *crypto.PubKeyFromByte(d.TeeChainPub[:]), nil
}

// code versions
func (c *Contract) GetCodeVersions() ([]*chaintypes.CodeVersion, error) {
	versions, _, err := c.subnet.QueryCodeVersions(chain.DefaultParamWithOrigin(types.AccountID(c.signer.AccountID())))
	if err != nil {
		return nil, err
	}

	out := make([]*chaintypes.CodeVersion, 0, len(*versions))
	for _, v := range *versions {
		out = append(out, &chaintypes.CodeVersion{
			Signer:    v.F1.Signer,
			Signature: v.F1.Signature,
		})
	}
	return out, nil
}

// go to new epoch
func (c *Contract) SetNewEpoch(nodeId uint64) error {
	return c.subnet.ExecSetNextEpoch(nodeId, chain.ExecParams{
		Signer:    c.signer,
		PayAmount: types.NewU128(*big.NewInt(0)),
	})
}

func (c *Contract) TxCallOfSetNextEpoch(nodeId uint64, signer crypto.PubKey) (*chaintypes.ChainCall, error) {
	call, err := c.subnet.CallOfSetNextEpoch(nodeId, chain.DryRunParams{
		Origin:    types.AccountID(signer.AccountID()),
		PayAmount: types.NewU128(*big.NewInt(0)),
	})
	if err != nil {
		return nil, err
	}
	return &chaintypes.ChainCall{Substrate: *call}, nil
}

func (c *Contract) GetNextEpochValidatorList() ([]*chaintypes.Validator, error) {
	dsecrets, _, err := c.subnet.QueryNextEpochValidators(chain.DefaultParamWithOrigin(types.AccountID(c.signer.AccountID())))
	if err != nil {
		return nil, err
	}

	validators := make([]*chaintypes.Validator, 0, len(dsecrets.V))
	for _, v := range dsecrets.V {
		validators = append(validators, &chaintypes.Validator{
			NodeID:         v.F0,
			P2pId:          *crypto.PubKeyFromByte(v.F1.P2pId[:]),
			ValidatorId:    *crypto.PubKeyFromByte(v.F1.ValidatorId[:]),
			ValidatorIdBLS: *crypto.PubKeyFromBLS12381Byte(v.F1.Bls[:]),
		})
	}

	return validators, nil
}
