package revive

import (
	"errors"
	"math/big"

	"github.com/centrifuge/go-substrate-rpc-client/v4/types"
	"github.com/computeflux/tee-provider/pkg/chains/revive/subnet"
	"github.com/computeflux/tee-provider/pkg/model"
	chain "github.com/wetee-dao/ink.go"
	"github.com/wetee-dao/ink.go/util"
)

func (c *Contract) GetMintWorker(user types.AccountID) (*model.K8sCluster, error) {
	workerWrap, _, err := c.subnet.QueryMintWorker(user, chain.DefaultParamWithOrigin(types.AccountID(c.signer.AccountID())))
	if err != nil {
		return nil, err
	}

	if workerWrap.IsNone() {
		return nil, errors.New("worker not found")
	}

	tuple, _ := workerWrap.UnWrap()
	worker := tuple.F1
	return &model.K8sCluster{
		Id:            tuple.F0,
		Name:          worker.Name,
		Owner:         worker.Owner,
		Level:         worker.Level,
		RegionId:      worker.RegionId,
		StartBlock:    worker.StartBlock,
		StopBlock:     worker.StopBlock,
		TerminalBlock: worker.TerminalBlock,
		P2pId:         worker.P2pId,
		Ip:            model.Ip(worker.Ip),
		Port:          worker.Port,
		Status:        worker.Status,
	}, nil
}

func (c *Contract) GetWorker(workerId uint64) (*model.K8sCluster, error) {
	workerWrap, _, err := c.subnet.QueryWorker(workerId, chain.DefaultParamWithOrigin(types.AccountID(c.signer.AccountID())))
	if err != nil {
		return nil, err
	}

	if workerWrap.IsNone() {
		return nil, errors.New("worker not found")
	}

	worker, _ := workerWrap.UnWrap()

	return &model.K8sCluster{
		Id:            workerId,
		Name:          worker.Name,
		Owner:         worker.Owner,
		Level:         worker.Level,
		RegionId:      worker.RegionId,
		StartBlock:    worker.StartBlock,
		StopBlock:     worker.StopBlock,
		TerminalBlock: worker.TerminalBlock,
		P2pId:         worker.P2pId,
		Ip:            model.Ip(worker.Ip),
		Port:          worker.Port,
		Status:        worker.Status,
	}, nil
}

func (c *Contract) ResigerCluster(name []byte, p2p_id [32]byte, ip model.Ip, port uint32, level byte, region_id uint32) error {
	return c.subnet.ExecWorkerRegister(name, p2p_id, subnet.Ip(ip), port, level, region_id, chain.ExecParams{
		Signer:    c.signer,
		PayAmount: types.NewU128(*big.NewInt(0)),
	})
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
			NodeID:         uint64(v.F0),
			P2pId:          *model.PubKeyFromByte(v.F1.P2pId[:]),
			ValidatorId:    *model.PubKeyFromByte(v.F1.ValidatorId[:]),
			ValidatorIdBLS: *model.PubKeyFromBLS12381Byte(v.F1.Bls[:]),
		})
	}

	return validators, nil
}

// epoch
func (c *Contract) GetEpoch() (uint32, uint32, uint32, uint32, model.PubKey, error) {
	d, _, err := c.subnet.QueryEpochInfo(chain.DefaultParamWithOrigin(types.AccountID(c.signer.AccountID())))
	if err != nil {
		return 0, 0, 0, 0, model.PubKey{}, err
	}

	return d.Epoch, d.EpochSlot, d.LastEpochBlock, d.Now, *model.PubKeyFromByte(d.TeeChainPub[:]), nil
}

// go to new epoch
func (c *Contract) SetNewEpoch(nodeId uint64) error {
	return c.subnet.ExecSetNextEpoch(nodeId, chain.ExecParams{
		Signer:    c.signer,
		PayAmount: types.NewU128(*big.NewInt(0)),
	})
}

func (c *Contract) TxCallOfSetNextEpoch(nodeId uint64, signer model.PubKey) (*model.ChainCall, error) {
	call, err := c.subnet.CallOfSetNextEpoch(nodeId, chain.DryRunParams{
		Origin:    types.AccountID(signer.AccountID()),
		PayAmount: types.NewU128(*big.NewInt(0)),
	})
	if err != nil {
		return nil, err
	}
	return &model.ChainCall{Substrate: *call}, nil
}

func (c *Contract) GetNextEpochValidatorList() ([]*model.Validator, error) {
	dsecrets, _, err := c.subnet.QueryNextEpochValidators(chain.DefaultParamWithOrigin(types.AccountID(c.signer.AccountID())))
	if err != nil {
		return nil, err
	}

	validators := make([]*model.Validator, 0, len(dsecrets.V))
	for _, v := range dsecrets.V {
		validators = append(validators, &model.Validator{
			NodeID:         v.F0,
			P2pId:          *model.PubKeyFromByte(v.F1.P2pId[:]),
			ValidatorId:    *model.PubKeyFromByte(v.F1.ValidatorId[:]),
			ValidatorIdBLS: *model.PubKeyFromBLS12381Byte(v.F1.Bls[:]),
		})
	}

	return validators, nil
}
