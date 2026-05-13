package dkg

import (
	"fmt"

	"github.com/centrifuge/go-substrate-rpc-client/v4/types"
	"github.com/computeflux/tee-provider/pkg/model"
)

type DKGStore struct {
	// Peer 是 P2P 网络主机
	Nodes []*model.Validator
	// Threshold 是密钥重建所需的最小份额数量
	Threshold int
	// DKG epoch
	Epoch uint32

	// DistPubKey globle public key
	DkgPubKeys map[model.SuiteType]*model.PubKey
	// DistKeyShare is the node private share
	DkgKeyShares map[model.SuiteType]*model.DistKeyShare
	SideKeyPub   types.AccountID
	SideKeyShare []byte

	// next epoch data
	NewNodes        []*model.Validator
	NewEpoch        uint32
	NewDkgPubKeys   map[model.SuiteType]*model.PubKey // dkg key
	NewDkgKeyShares map[model.SuiteType]*model.DistKeyShare
	NewSideKeyPub   types.AccountID // side key
	NewSideKeyShare []byte

	status uint8
}

// Restore dkg state
func (dkg *DKG) reState() error {
	from, err := model.GetJson[DKGStore]("DKG", dkg.Signer.GetPublicForSuite(model.SuiteEd25519).SS58())
	if err != nil {
		return fmt.Errorf("get dkg: %w", err)
	}

	if from == nil {
		return nil
	}

	to := dkg

	to.Threshold = from.Threshold
	to.Epoch = from.Epoch
	to.status = from.status
	to.Nodes = from.Nodes
	to.DkgPubKeys = from.DkgPubKeys
	to.DkgKeyShares = from.DkgKeyShares

	to.NewNodes = from.NewNodes
	to.NewEpoch = from.NewEpoch
	to.NewDkgPubKeys = from.NewDkgPubKeys
	to.NewDkgKeyShares = from.NewDkgKeyShares

	return nil
}

func (dkg *DKG) saveState() error {
	to := DKGStore{}
	from := dkg

	to.Threshold = from.Threshold
	to.Epoch = from.Epoch
	to.status = from.status
	to.Nodes = from.Nodes
	to.DkgPubKeys = from.DkgPubKeys
	to.DkgKeyShares = from.DkgKeyShares

	to.NewNodes = from.NewNodes
	to.NewEpoch = from.NewEpoch
	to.NewDkgPubKeys = from.NewDkgPubKeys
	to.NewDkgKeyShares = from.NewDkgKeyShares

	return model.SetJson("DKG", dkg.Signer.GetPublicForSuite(model.SuiteEd25519).SS58(), &to)
}
