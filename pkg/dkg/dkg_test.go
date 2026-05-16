package dkg

import (
	"fmt"
	"os"
	"runtime"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/computeflux/tee-provider/pkg/model"
	"github.com/computeflux/tee-provider/pkg/network/local"
	"github.com/computeflux/tee-provider/pkg/util"
)

var peerSecret = []string{
	"7512939e37970b04c2b9a6060b16654473cf0721b71f8e56126ee314cbd0a7e9fe9125a81688d932ea792e9722c777f7696117363f86a107ab9d3681a8c922c8",
	"dc72ceaf44e1e382a2ee4bbf47d6eabcca460740d5c28dd7bc097db90700594636922ff5c13eded54d1ba710bf043eed221112eb726da532149e1e3619fb9336",
	"ebecd85320c3a05a1c170738c321a58d6560be9a1fbb323028f24aa874dece755a10bfc8865e3015c7496a3831d0e5e3abaadf964116d5ea5d8ada52135286a1",
}

var newPeerSecret = []string{
	"2d1f5379cc0475c169e7d5ee5e53437c0b6fe60dc4ea822c61d7db9dfe5e33b8931f5771269844909f1e73bd51e9da82eb8a7b2def0313417003f978d9010eb4",
	"66dfdf585852c8aa4b111c15b69d4b40ad6db7cfabd7ae0b25384c950f5018cb2b4c0a71a9d97d6eb9bfcb29acc0d71df41015c5c5cc697d9471728a2a508a8a",
}

func runEpoch(t *testing.T, dkgs []*DKG, validators []*model.Validator, epoch uint32, tag string) (*model.PubKey, *model.PubKey) {
	t.Helper()

	done := make(chan struct{})
	err := dkgs[0].TryEpochConsensus(model.ConsensusMsg{
		Validators: validators,
		Epoch:      epoch,
	}, func(signer *DssSigner, nodeId uint64) {
		util.LogWithBlue("CONSENSUS SUCCESS "+tag, nodeId)
		for _, d := range dkgs {
			d.ToNewEpoch()
		}
		close(done)
	}, func(e error) {
		util.LogWithBlue("CONSENSUS Error "+tag, e.Error())
		close(done)
	})
	if err != nil {
		t.Fatal(err)
	}

	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("DKG consensus timeout for " + tag)
	}

	edPub := dkgs[0].GetDkgPubKey(model.SuiteEd25519)
	secpPub := dkgs[0].GetDkgPubKey(model.SuiteSecp256k1)
	return edPub, secpPub
}

func TestNetwork(t *testing.T) {
	os.RemoveAll("./token_chain_data")

	db, err := model.NewDB()
	if err != nil {
		require.NoErrorf(t, err, "failed store.InitDB")
		os.Exit(1)
	}
	defer db.Close()

	skipPartialSign = true

	nodes := []*model.PubKey{}
	validators := []*model.Validator{}
	for _, s := range peerSecret {
		nodeSecret, _ := model.PrivateKeyFromHex(s)
		nodes = append(nodes, nodeSecret.GetPublicForSuite(model.SuiteEd25519))
		validators = append(validators, &model.Validator{
			ValidatorId:     *nodeSecret.GetPublicForSuite(model.SuiteEd25519),
			P2pId:           *nodeSecret.GetPublicForSuite(model.SuiteEd25519),
			ValidatorIdSecp: *nodeSecret.GetPublicForSuite(model.SuiteSecp256k1),
		})
	}

	dkgs := make([]*DKG, 0, len(nodes))
	for i, s := range peerSecret {
		nodeSecret, _ := model.PrivateKeyFromHex(s)
		peer, err := local.NewNetwork(nodeSecret, []string{}, nodes, uint32(0), uint32(0))
		require.NoErrorf(t, err, "failed peer.NewNetwork")
		dkg, err := NewDKG(nodeSecret, peer, Logger{NodeTag: "NODE " + fmt.Sprint(i)})
		require.NoErrorf(t, err, "failed NewDKG")
		go dkg.Start()
		dkgs = append(dkgs, dkg)
	}

	// Epoch 1
	edPub1, secpPub1 := runEpoch(t, dkgs, validators, 1, "E1")
	require.NotNil(t, edPub1, "ed25519 DKG pubkey should not be nil")
	require.NotNil(t, secpPub1, "secp256k1 DKG pubkey should not be nil")
	fmt.Println("E1 OK | ed25519 =", edPub1.SS58(), "| secp256k1 =", fmt.Sprintf("0x%x", secpPub1.Secp256k1CompressPubkey()))

	// Epoch 2 — add 2 new peers
	for _, s := range newPeerSecret {
		nodeSecret, _ := model.PrivateKeyFromHex(s)
		nodes = append(nodes, nodeSecret.GetPublicForSuite(model.SuiteEd25519))
		validators = append(validators, &model.Validator{
			ValidatorId:     *nodeSecret.GetPublicForSuite(model.SuiteEd25519),
			P2pId:           *nodeSecret.GetPublicForSuite(model.SuiteEd25519),
			ValidatorIdSecp: *nodeSecret.GetPublicForSuite(model.SuiteSecp256k1),
		})
	}

	for i, s := range newPeerSecret {
		nodeSecret, _ := model.PrivateKeyFromHex(s)
		peer, err := local.NewNetwork(nodeSecret, []string{}, nodes, uint32(0), uint32(0))
		require.NoErrorf(t, err, "failed peer.NewNetwork")
		dkg, err := NewDKG(nodeSecret, peer, Logger{NodeTag: "NODE " + fmt.Sprint(3+i)})
		require.NoErrorf(t, err, "failed NewDKG")
		go dkg.Start()
		dkgs = append(dkgs, dkg)
	}
	for _, d := range dkgs {
		if lp, ok := d.Peer.(*local.Peer); ok {
			lp.SetNodes(nodes)
		}
	}
	// Ensure new goroutines are scheduled and ready
	runtime.Gosched()
	time.Sleep(500 * time.Millisecond)

	edPub2, secpPub2 := runEpoch(t, dkgs, validators, 2, "E2")
	require.NotNil(t, edPub2, "ed25519 DKG pubkey should not be nil")
	require.NotNil(t, secpPub2, "secp256k1 DKG pubkey should not be nil")
	fmt.Println("E2 OK | ed25519 =", edPub2.SS58(), "| secp256k1 =", fmt.Sprintf("0x%x", secpPub2.Secp256k1CompressPubkey()))
	require.Equal(t, edPub1.SS58(), edPub2.SS58(), "ed25519 pubkey must be consistent")
	require.Equal(t, secpPub1.Secp256k1CompressPubkey(), secpPub2.Secp256k1CompressPubkey(), "secp256k1 pubkey must be consistent")

	// Epoch 3
	runtime.Gosched()
	time.Sleep(200 * time.Millisecond)
	edPub3, secpPub3 := runEpoch(t, dkgs, validators, 3, "E3")
	require.NotNil(t, edPub3, "ed25519 DKG pubkey should not be nil")
	require.NotNil(t, secpPub3, "secp256k1 DKG pubkey should not be nil")
	fmt.Println("E3 OK | ed25519 =", edPub3.SS58(), "| secp256k1 =", fmt.Sprintf("0x%x", secpPub3.Secp256k1CompressPubkey()))
	require.Equal(t, edPub1.SS58(), edPub3.SS58(), "ed25519 pubkey must be consistent")
	require.Equal(t, secpPub1.Secp256k1CompressPubkey(), secpPub3.Secp256k1CompressPubkey(), "secp256k1 pubkey must be consistent")
}
