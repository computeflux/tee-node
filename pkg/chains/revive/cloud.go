package contracts

import (
	"math/big"

	"github.com/centrifuge/go-substrate-rpc-client/v4/types"
	chain "github.com/wetee-dao/ink.go"
	"github.com/wetee-dao/tee-dsecret/pkg/chains/revive/cloud"
	"github.com/wetee-dao/tee-dsecret/pkg/model"
)

func (c *Contract) GetPodsVersionByWorker(workerId uint64) ([]model.PodVersion, error) {
	data, _, err := c.cloud.QueryWorkerPodsVersion(workerId, chain.DefaultParamWithOrigin(types.AccountID(c.signer.AccountID())))
	if err != nil {
		return nil, err
	}

	list := make([]model.PodVersion, 0, len(*data))
	for _, v := range *data {
		list = append(list, model.PodVersion{
			PodId:    v.F0,
			Version:  v.F1,
			LastMint: v.F2,
			Status:   v.F3,
		})
	}

	return list, nil
}

func (c *Contract) GetPodsByIds(podIds []uint64) ([]model.Pod, error) {
	returnPods, _, err := c.cloud.QueryPodsByIds(podIds, chain.DefaultParamWithOrigin(types.AccountID(c.signer.AccountID())))
	if err != nil {
		return nil, err
	}

	pods := make([]model.Pod, 0, len(*returnPods))
	for _, rpod := range *returnPods {
		contariners := make([]model.Container, 0, len(rpod.F2))
		for i := range rpod.F2 {
			contariners = append(contariners, TransToContainer(rpod.F2[i]))
		}
		pods = append(pods, model.Pod{
			PodId:               rpod.F0,
			Owner:               rpod.F1.Owner,
			Ptype:               model.PodType(rpod.F1.Ptype),
			TeeType:             ConvertTEEType(rpod.F1.TeeType),
			Containers:          contariners,
			Version:             rpod.F3,
			LastMintBlockNumber: rpod.F4,
			Status:              rpod.F5,
		})
	}

	return pods, nil
}

func TransToContainer(rc cloud.Tuple_72) model.Container {
	c := rc.F1.F0
	d := rc.F1.F1
	contariner := model.Container{
		Image:   c.Image,
		Command: model.CopyWithJSON[cloud.Command, model.Command](c.Command),
		Port:    model.CopyWithJSON[[]cloud.Service, []model.Service](c.Port),
		Cpu:     c.Cpu,
		Mem:     c.Mem,
		Disk:    model.CopyWithJSON[[]cloud.ContainerDisk, []model.ContainerDisk](c.Disk),
		Gpu:     c.Gpu,
		Env:     model.CopyWithJSON[[]cloud.PodEnv, []model.Env](c.Env),
	}

	for i := 0; i < len(contariner.Disk); i++ {
		if d[i].IsSome() {
			contariner.Disk[i].Size = d[i].V.SecretSSD.F2
		}
	}

	return contariner
}

func (c *Contract) TxCallOfStartPod(nodeId uint64, pod_key types.AccountID, signer types.AccountID) (*types.Call, error) {
	return c.cloud.CallOfStartPod(nodeId, pod_key, chain.DryRunParams{
		Origin:    signer,
		PayAmount: types.NewU128(*big.NewInt(0)),
	})
}

func (c *Contract) DryStartPod(nodeId uint64, pod_key types.AccountID, signer types.AccountID) error {
	_, _, err := c.cloud.DryRunStartPod(nodeId, pod_key, chain.DefaultParamWithOrigin(signer))
	return err
}

func (c *Contract) TxCallOfMintPod(nodeId uint64, hash types.H256, signer types.AccountID) (*types.Call, error) {
	return c.cloud.CallOfMintPod(nodeId, hash, chain.DryRunParams{
		Origin:    signer,
		PayAmount: types.NewU128(*big.NewInt(0)),
	})
}

func (c *Contract) DryMintPod(nodeId uint64, hash types.H256, signer types.AccountID) error {
	_, _, err := c.cloud.DryRunMintPod(nodeId, hash, chain.DefaultParamWithOrigin(signer))
	return err
}

func (c *Contract) TxCallOfUploadSecret(user types.H160, index uint64, signer types.AccountID) (*types.Call, error) {
	return c.cloud.CallOfMintSecret(user, index, chain.DryRunParams{
		Origin:    signer,
		PayAmount: types.NewU128(*big.NewInt(0)),
	})
}

func (c *Contract) DryUploadSecret(user types.H160, index uint64, signer types.AccountID) error {
	_, _, err := c.cloud.DryRunMintSecret(user, index, chain.DefaultParamWithOrigin(signer))
	return err
}

func (c *Contract) TxCallOfInitDisk(user types.H160, index uint64, hash types.H256, signer types.AccountID) (*types.Call, error) {
	return c.cloud.CallOfUpdateDiskKey(user, index, hash, chain.DryRunParams{
		Origin:    signer,
		PayAmount: types.NewU128(*big.NewInt(0)),
	})
}

func (c *Contract) DryInitDisk(user types.H160, index uint64, hash types.H256, signer types.AccountID) error {
	_, _, err := c.cloud.DryRunUpdateDiskKey(user, index, hash, chain.DefaultParamWithOrigin(signer))
	return err
}
