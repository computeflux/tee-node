package dkg

import (
	"crypto/ed25519"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/wetee-dao/tee-dsecret/pkg/model"
	"github.com/wetee-dao/tee-dsecret/pkg/network/local"
	"github.com/wetee-dao/tee-dsecret/pkg/util"
)

func TestRandomShareGeneration(t *testing.T) {
	os.RemoveAll("./chain_data")

	db, err := model.NewDB()
	require.NoError(t, err)
	defer db.Close()

	// Skip partial sign to avoid chain dependencies.
	skipPartialSign = true

	nodes := []*model.PubKey{}
	validators := []*model.Validator{}
	for _, s := range peerSecret {
		nodeSecret, _ := model.PrivateKeyFromHex(s)
		nodes = append(nodes, nodeSecret.GetPublic())
		validators = append(validators, &model.Validator{
			ValidatorId: *nodeSecret.GetPublic(),
			P2pId:       *nodeSecret.GetPublic(),
		})
	}

	peers := make([]*local.Peer, 0, len(nodes))
	for _, s := range peerSecret {
		nodeSecret, _ := model.PrivateKeyFromHex(s)
		peer, err := local.NewNetwork(nodeSecret, []string{}, nodes, uint32(0), uint32(0))
		require.NoError(t, err)
		peers = append(peers, peer)
	}

	dkgs := make([]*DKG, 0, len(nodes))
	for i, s := range peerSecret {
		nodeSecret, _ := model.PrivateKeyFromHex(s)
		dkg, err := NewDKG(nodeSecret, peers[i], Logger{
			NodeTag: "NODE " + fmt.Sprint(i),
		})
		require.NoError(t, err)
		go dkg.Start()
		dkgs = append(dkgs, dkg)
	}

	// Run main DKG consensus.
	err = dkgs[0].TryEpochConsensus(model.ConsensusMsg{
		Validators: validators,
		Epoch:      1,
	}, func(signer *DssSigner, nodeId uint64) {
		util.LogWithBlue("CONSENSUS SUCCESS", nodeId)
		for _, dkg := range dkgs {
			dkg.ToNewEpoch()
		}
	}, func(error) {
		util.LogWithBlue("CONSENSUS Error", err)
	})
	require.NoError(t, err)

	// Wait for main DKG to complete.
	time.Sleep(time.Second * 1)

	// Trigger random share generation explicitly on node 0 only.
	// The P2P broadcast ensures all nodes receive the deals/responses.
	for _, d := range dkgs {
		d.RandomPool.SetShares(nil) // ensure pool is empty so generation triggers
	}
	start := time.Now()
	dkgs[0].StartRandomShareGeneration(10)

	// Wait for random DKG to complete.
	// With 20-way parallelism, 10 shares should complete in ~2-4s.
	for {
		allDone := true
		for _, d := range dkgs {
			if d.RandomPool.Len() < 10 {
				allDone = false
				break
			}
		}
		if allDone {
			break
		}
		if time.Since(start) > time.Second*30 {
			t.Fatal("timeout waiting for random share generation")
		}
		time.Sleep(100 * time.Millisecond)
	}
	elapsed := time.Since(start)
	fmt.Printf("Generated 10 random shares in %v (parallel)\n", elapsed)

	// Verify each DKG has at least 10 random shares.
	for i, d := range dkgs {
		require.GreaterOrEqual(t, d.RandomPool.Len(), 10, "node %d should have random shares", i)
	}

	// Verify all nodes have the same commitments for randomIndex 1.
	var expectedPubKey string
	var expectedCommits []string
	for i, d := range dkgs {
		share, err := d.RandomPool.Acquire(1)
		require.NoError(t, err, "node %d should have randomIndex 1", i)
		require.NotNil(t, share, "node %d random share should not be nil", i)
		// Verify random share differs from long share.
		require.NotEqual(t, d.DkgKeyShare.PriShare().V.String(), share.PriShare().V.String(),
			"node %d: random share should differ from long share", i)

		pubKeyStr := share.CommitsWrap.Public[0].String()
		if i == 0 {
			expectedPubKey = pubKeyStr
			expectedCommits = make([]string, len(share.CommitsWrap.Public))
			for j, p := range share.CommitsWrap.Public {
				expectedCommits[j] = p.String()
			}
		} else {
			require.Equal(t, expectedPubKey, pubKeyStr, "node %d: pubkey should match node 0", i)
			require.Equal(t, len(expectedCommits), len(share.CommitsWrap.Public), "node %d: commit count mismatch", i)
			for j, p := range share.CommitsWrap.Public {
				require.Equal(t, expectedCommits[j], p.String(), "node %d: commit %d mismatch", i, j)
			}
		}
		fmt.Printf("Node %d random share: I=%d pubkey=%s\n", i, share.PriShare().I, pubKeyStr)
	}

	// Test signing with a random share (txIndex=1 -> randomIndex=1).
	msg := []byte("test message with random share")
	signers := make([]*DssSigner, 0, len(dkgs))
	partialSigs := make([][]byte, 0, len(dkgs))

	for _, d := range dkgs {
		signer := NewDssSigner(d, 1)
		require.NotNil(t, signer.randomShare, "signer should have acquired a random share")
		sig, err := signer.PartialSign(msg)
		require.NoError(t, err)
		signers = append(signers, signer)
		partialSigs = append(partialSigs, sig)
	}

	// Aggregate signature.
	signers[0].SetSigs(partialSigs)
	aggregated, err := signers[0].Sign(msg)
	require.NoError(t, err)

	// Verify with Ed25519.
	isok := ed25519.Verify(dkgs[0].GetDkgPubKey().Ed25519PublicKey(), msg, aggregated)
	require.True(t, isok, "aggregated signature should verify")

	fmt.Println("RandomSharePool tests passed!")
}
