package dkg

import (
	"crypto/ed25519"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/computeflux/tee-provider/pkg/chains"
	"github.com/computeflux/tee-provider/pkg/model"
	"github.com/computeflux/tee-provider/pkg/network/local"
	"github.com/computeflux/tee-provider/pkg/util"
	"github.com/stretchr/testify/require"

	chain "github.com/wetee-dao/ink.go"
)

func TestDSS(t *testing.T) {
	os.RemoveAll("./token_chain_data")

	db, err := model.NewDB()
	if err != nil {
		require.NoErrorf(t, err, "failed store.InitDB")
		os.Exit(1)
	}
	defer db.Close()

	nodes := []*model.PubKey{}
	validators := []*model.Validator{}
	for _, s := range peerSecret {
		nodeSecret, _ := model.PrivateKeyFromHex(s)
		nodes = append(nodes, nodeSecret.GetPublicForSuite(model.SuiteEd25519))
		validators = append(validators, &model.Validator{
			ValidatorId:    *nodeSecret.GetPublicForSuite(model.SuiteEd25519),
			P2pId:          *nodeSecret.GetPublicForSuite(model.SuiteEd25519),
			ValidatorIdBLS: *nodeSecret.GetPublicForSuite(model.SuiteBLS12381),
		})
	}

	peers := make([]*local.Peer, 0, len(nodes))
	for _, s := range peerSecret {
		nodeSecret, _ := model.PrivateKeyFromHex(s)

		peer, err := local.NewNetwork(nodeSecret, []string{}, nodes, uint32(0), uint32(0))
		require.NoErrorf(t, err, "failed peer.NewNetwork")

		peers = append(peers, peer)
	}

	dkgs := make([]*DKG, 0, len(nodes))
	for i, s := range peerSecret {
		nodeSecret, _ := model.PrivateKeyFromHex(s)

		dkg, err := NewDKG(nodeSecret, peers[i], Logger{
			NodeTag: "NODE " + fmt.Sprint(i),
		})
		require.NoErrorf(t, err, "failed NewDKG")
		go dkg.Start()

		dkgs = append(dkgs, dkg)
	}

	done := make(chan struct{})
	consensusOk := false
	err = dkgs[0].TryEpochConsensus(model.ConsensusMsg{
		Validators: validators,
		Epoch:      1,
	}, func(signer *DssSigner, nodeId uint64) {
		util.LogWithBlue("CONSENSUS SUCCESS", nodeId)
		consensusOk = true
		for _, dkg := range dkgs {
			dkg.ToNewEpoch()
		}
		close(done)
	}, func(e error) {
		util.LogWithBlue("CONSENSUS Error", e.Error())
		close(done)
	})
	if err != nil {
		t.Fatal(err)
	}

	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("DKG consensus timeout")
	}
	if !consensusOk {
		t.Fatal("DKG consensus failed")
	}

	msg := []byte("hello word")
	signers := []DssSigner{}
	sigs := [][]byte{}

	// Partial Sign
	for _, d := range dkgs {
		signer := DssSigner{
			dkg: d,
		}
		sig, _ := signer.PartialSign(msg)
		signers = append(signers, signer)
		sigs = append(sigs, sig)
	}
	signers[0].SetSigs(sigs)

	// sign
	sigbt, err := signers[0].Sign(msg)
	if err != nil {
		t.Fatal(err)
	}

	isok := ed25519.Verify(dkgs[0].GetDkgPubKey(model.SuiteEd25519).Ed25519PublicKey(), msg, sigbt)
	fmt.Println("ed25519.Verify", isok)
}

func TestDssSubmitTx(t *testing.T) {
	os.RemoveAll("./token_chain_data")

	db, err := model.NewDB()
	if err != nil {
		require.NoErrorf(t, err, "failed store.InitDB")
		os.Exit(1)
	}
	defer db.Close()

	// init amount
	alice, err := chain.Sr25519PairFromSecret("//Alice", 42)
	if err != nil {
		util.LogWithPurple("Sr25519PairFromSecret", err)
		t.Fatal(err)
	}
	fmt.Println(alice.Address)

	nodePriv, err := model.PrivateKeyFromHex("0xc068c8db6ead1dd57777daeb3ec8bb84342bf9d7e08a812aee130d684283b5a8a39c7474d228cb55224f1932652e05a949b4d9ba6328413485289159e519bd99")
	if err != nil {
		fmt.Println(err)
		t.Fatal(err)
	}

	// Link to polkadot
	_, err = chains.ConnectMainChain("local", nodePriv)
	if err != nil {
		fmt.Println("Connect to chain error:", err)
		t.Skip("skipping: blockchain RPC not available")
	}

	nodes := []*model.PubKey{}
	validators := []*model.Validator{}
	for _, s := range peerSecret {
		nodeSecret, _ := model.PrivateKeyFromHex(s)
		nodes = append(nodes, nodeSecret.GetPublicForSuite(model.SuiteEd25519))
		validators = append(validators, &model.Validator{
			ValidatorId:    *nodeSecret.GetPublicForSuite(model.SuiteEd25519),
			P2pId:          *nodeSecret.GetPublicForSuite(model.SuiteEd25519),
			ValidatorIdBLS: *nodeSecret.GetPublicForSuite(model.SuiteBLS12381),
		})
	}

	peers := make([]*local.Peer, 0, len(nodes))
	for _, s := range peerSecret {
		nodeSecret, _ := model.PrivateKeyFromHex(s)

		peer, err := local.NewNetwork(nodeSecret, []string{}, nodes, uint32(0), uint32(0))
		require.NoErrorf(t, err, "failed peer.NewNetwork")

		peers = append(peers, peer)
	}

	dkgs := make([]*DKG, 0, len(nodes))
	for i, s := range peerSecret {
		nodeSecret, _ := model.PrivateKeyFromHex(s)

		dkg, err := NewDKG(nodeSecret, peers[i], Logger{
			NodeTag: "NODE " + fmt.Sprint(i),
		})
		require.NoErrorf(t, err, "failed NewDKG")
		go dkg.Start()

		dkgs = append(dkgs, dkg)
	}

	err = dkgs[0].TryEpochConsensus(model.ConsensusMsg{
		Validators: validators,
		Epoch:      1,
	}, func(signer *DssSigner, nodeId uint64) {
		util.LogWithBlue("CONSENSUS SUCCESS", nodeId)
		call, err := chains.MainChain.TxCallOfSetNextEpoch(nodeId, signer.ChainAccountID())
		if err != nil {
			panic(err)
		}

		err = chains.MainChain.SignAndSubmit(signer, *call, false, 0)
		fmt.Println(err)

		for _, dkg := range dkgs {
			dkg.ToNewEpoch()
		}
	}, func(e error) {
		util.LogWithBlue("CONSENSUS Error", e.Error())
	})
	if err != nil {
		t.Fatal(err)
	}

	time.Sleep(time.Second * 1)
}
