package dkg

import (
	"crypto/ed25519"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/computeflux/tee-provider/pkg/chains"
	dkgtypes "github.com/computeflux/tee-provider/pkg/dkg/types"
	"github.com/computeflux/tee-provider/pkg/network/local"
	"github.com/computeflux/tee-provider/pkg/types"
	"github.com/computeflux/tee-provider/pkg/types/chaintypes"
	crypto "github.com/computeflux/tee-provider/pkg/types/crypto"
	"github.com/computeflux/tee-provider/pkg/util"
	"github.com/stretchr/testify/require"

	chain "github.com/wetee-dao/ink.go"
)

func TestDSS(t *testing.T) {
	os.RemoveAll("./token_chain_data")

	db, err := types.NewDB()
	if err != nil {
		require.NoErrorf(t, err, "failed types.InitDB")
		os.Exit(1)
	}
	defer db.Close()

	nodes := []*crypto.PubKey{}
	validators := []*chaintypes.Validator{}
	for _, s := range peerSecret {
		nodeSecret, _ := crypto.PrivateKeyFromHex(s)
		nodes = append(nodes, nodeSecret.GetPublicForSuite(crypto.SuiteEd25519))
		validators = append(validators, &chaintypes.Validator{
			ValidatorId:    *nodeSecret.GetPublicForSuite(crypto.SuiteEd25519),
			P2pId:          *nodeSecret.GetPublicForSuite(crypto.SuiteEd25519),
			ValidatorIdBLS: *nodeSecret.GetPublicForSuite(crypto.SuiteBLS12381),
		})
	}

	peers := make([]*local.Peer, 0, len(nodes))
	for _, s := range peerSecret {
		nodeSecret, _ := crypto.PrivateKeyFromHex(s)

		peer, err := local.NewNetwork(nodeSecret, []string{}, nodes, uint32(0), uint32(0))
		require.NoErrorf(t, err, "failed peer.NewNetwork")

		peers = append(peers, peer)
	}

	dkgs := make([]*DKG, 0, len(nodes))
	for i, s := range peerSecret {
		nodeSecret, _ := crypto.PrivateKeyFromHex(s)

		dkg, err := NewDKG(nodeSecret, peers[i], Logger{
			NodeTag: "NODE " + fmt.Sprint(i),
		})
		require.NoErrorf(t, err, "failed NewDKG")
		go dkg.Start()

		dkgs = append(dkgs, dkg)
	}
	// 先于 db.Close() 执行（defer LIFO）：停止 DKG 并等待在途消息
	defer stopDKGs(dkgs)

	done := make(chan struct{})
	consensusOk := false
	err = dkgs[0].TryEpochConsensus(dkgtypes.ConsensusMsg{
		Validators: validators,
		Epoch:      1,
	}, func(signer *DssSigner, nodeId uint64) {
		util.LogWithBlue("CONSENSUS SUCCESS", nodeId)
		consensusOk = true
		// for _, dkg := range dkgs {
		// 	dkg.ToNewEpoch()
		// }
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

	// 等所有节点完成共识迁移后再签名，否则部分节点 DkgKeyShares 仍为 nil
	waitDKGReady(t, dkgs)

	msg, _ := hex.DecodeString("64070000000000528c1e000f000000d6eec26135305a8ad257a20d003357284c8aa03d0bdb2b357ab0a22371e11ef2d6eec26135305a8ad257a20d003357284c8aa03d0bdb2b357ab0a22371e11ef200")
	signers := []DssSigner{}
	sigs := [][]byte{}

	// Partial Sign
	for _, d := range dkgs {
		signer := DssSigner{
			dkg: d,
		}
		sig, _ := signer.PartialSign(msg)
		signers = append(signers, signer)

		json.Unmarshal(sig, &dkgtypes.PartialSigWrap{}) // sanity check

		sigs = append(sigs, sig)
	}
	signers[0].SetSigs(sigs)

	// sign
	sigbt, err := signers[0].Sign(msg)
	if err != nil {
		t.Fatal(err)
	}

	fmt.Println(dkgs[0].GetDkgPubKey(crypto.SuiteEd25519).SS58())
	isok := ed25519.Verify(dkgs[0].GetDkgPubKey(crypto.SuiteEd25519).Ed25519PublicKey(), msg, sigbt)
	fmt.Println("ed25519.Verify", isok)
}

func TestDssSubmitTx(t *testing.T) {
	os.RemoveAll("./token_chain_data")

	db, err := types.NewDB()
	if err != nil {
		require.NoErrorf(t, err, "failed types.InitDB")
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

	nodePriv, err := crypto.PrivateKeyFromHex("0xc068c8db6ead1dd57777daeb3ec8bb84342bf9d7e08a812aee130d684283b5a8a39c7474d228cb55224f1932652e05a949b4d9ba6328413485289159e519bd99")
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

	nodes := []*crypto.PubKey{}
	validators := []*chaintypes.Validator{}
	for _, s := range peerSecret {
		nodeSecret, _ := crypto.PrivateKeyFromHex(s)
		nodes = append(nodes, nodeSecret.GetPublicForSuite(crypto.SuiteEd25519))
		validators = append(validators, &chaintypes.Validator{
			ValidatorId:    *nodeSecret.GetPublicForSuite(crypto.SuiteEd25519),
			P2pId:          *nodeSecret.GetPublicForSuite(crypto.SuiteEd25519),
			ValidatorIdBLS: *nodeSecret.GetPublicForSuite(crypto.SuiteBLS12381),
		})
	}

	peers := make([]*local.Peer, 0, len(nodes))
	for _, s := range peerSecret {
		nodeSecret, _ := crypto.PrivateKeyFromHex(s)

		peer, err := local.NewNetwork(nodeSecret, []string{}, nodes, uint32(0), uint32(0))
		require.NoErrorf(t, err, "failed peer.NewNetwork")

		peers = append(peers, peer)
	}

	dkgs := make([]*DKG, 0, len(nodes))
	for i, s := range peerSecret {
		nodeSecret, _ := crypto.PrivateKeyFromHex(s)

		dkg, err := NewDKG(nodeSecret, peers[i], Logger{
			NodeTag: "NODE " + fmt.Sprint(i),
		})
		require.NoErrorf(t, err, "failed NewDKG")
		go dkg.Start()

		dkgs = append(dkgs, dkg)
	}
	// 先于 db.Close() 执行（defer LIFO）：停止 DKG 并等待在途消息
	defer stopDKGs(dkgs)

	err = dkgs[0].TryEpochConsensus(dkgtypes.ConsensusMsg{
		Validators: validators,
		Epoch:      1,
	}, func(signer *DssSigner, nodeId uint64) {
		util.LogWithBlue("CONSENSUS SUCCESS", nodeId)
		call, err := chains.MainChain.TxCallOfSetNextEpoch(nodeId, *signer.Pubkey())
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
