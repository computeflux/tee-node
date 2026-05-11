package dkg

import (
	"fmt"

	"github.com/centrifuge/go-substrate-rpc-client/v4/types"
	"github.com/wetee-dao/tee-dsecret/pkg/model"
)

type DKGStore struct {
	// Peer 是 P2P 网络主机
	Nodes []*model.Validator
	// Threshold 是密钥重建所需的最小份额数量
	Threshold int
	// DKG epoch
	Epoch uint32

	// DistPubKey globle public key
	DkgPubKey *model.PubKey
	// DistKeyShare is the node private share
	DkgKeyShare  *model.DistKeyShare
	SideKeyPub   types.AccountID
	SideKeyShare []byte

	// next epoch data
	NewNodes        []*model.Validator
	NewEpoch        uint32
	NewDkgPubKey    *model.PubKey // dkg key
	NewDkgKeyShare  *model.DistKeyShare
	NewSideKeyPub   types.AccountID // side key
	NewSideKeyShare []byte

	status uint8
}

// Restore dkg state
func (dkg *DKG) reState() error {
	from, err := model.GetJson[DKGStore]("DKG", dkg.Signer.GetPublic().SS58())
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
	to.DkgPubKey = from.DkgPubKey
	to.DkgKeyShare = from.DkgKeyShare

	to.NewNodes = from.NewNodes
	to.NewEpoch = from.NewEpoch
	to.NewDkgPubKey = from.NewDkgPubKey
	to.NewDkgKeyShare = from.NewDkgKeyShare

	return nil
}

func (dkg *DKG) saveState() error {
	to := DKGStore{}
	from := dkg

	to.Threshold = from.Threshold
	to.Epoch = from.Epoch
	to.status = from.status
	to.Nodes = from.Nodes
	to.DkgPubKey = from.DkgPubKey
	to.DkgKeyShare = from.DkgKeyShare

	to.NewNodes = from.NewNodes
	to.NewEpoch = from.NewEpoch
	to.NewDkgPubKey = from.NewDkgPubKey
	to.NewDkgKeyShare = from.NewDkgKeyShare

	return model.SetJson("DKG", dkg.Signer.GetPublic().SS58(), &to)
}
