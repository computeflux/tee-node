package dkg

import (
	"fmt"
	"os"
	"runtime"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	dkgtypes "github.com/computeflux/tee-provider/pkg/dkg/types"
	"github.com/computeflux/tee-provider/pkg/network/local"
	"github.com/computeflux/tee-provider/pkg/types"
	"github.com/computeflux/tee-provider/pkg/types/chaintypes"
	crypto "github.com/computeflux/tee-provider/pkg/types/crypto"
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

// stopDKGs 停止所有 DKG 并等待本地 peer 的在途消息处理完成，
// 必须在 db.Close() 之前执行（通过 defer 逆序保证），
// 否则后台 handler goroutine 会继续向已关闭的 pebble DB 写入而 panic。
// 注意顺序：必须先全部 Stop()（关闭消息链并等各自处理循环退出），
// 再统一 Wait()（等待派发 goroutine），否则 Wait 可能在其它节点的
// 处理循环仍在派发新消息时提前返回。
func stopDKGs(dkgs []*DKG) {
	for _, d := range dkgs {
		d.Stop()
	}
	for _, d := range dkgs {
		if lp, ok := d.Peer.(*local.Peer); ok {
			lp.Wait()
		}
	}
}

// waitDKGReady 等待所有 DKG 完成共识迁移（DkgKeyShares 就绪），
// 确保上一轮 epoch 的消息全部处理完、网络达到静止，再进入下一轮。
func waitDKGReady(t *testing.T, dkgs []*DKG) {
	t.Helper()
	deadline := time.Now().Add(15 * time.Second)
	for {
		ready := true
		for _, d := range dkgs {
			if d.GetDkgKeyShare(crypto.SuiteEd25519) == nil {
				ready = false
				break
			}
		}
		if ready {
			return
		}
		if time.Now().After(deadline) {
			t.Fatal("timeout waiting for all DKGs to complete consensus")
		}
		time.Sleep(50 * time.Millisecond)
	}
}

func runEpoch(t *testing.T, dkgs []*DKG, validators []*chaintypes.Validator, epoch uint32, tag string) (*crypto.PubKey, *crypto.PubKey) {
	t.Helper()

	done := make(chan struct{})
	success := false
	err := dkgs[0].TryEpochConsensus(dkgtypes.ConsensusMsg{
		Validators: validators,
		Epoch:      epoch,
	}, func(signer *DssSigner, nodeId uint64) {
		util.LogWithBlue("CONSENSUS SUCCESS "+tag, nodeId)
		success = true
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

	// 等所有节点完成迁移后再返回，避免下一轮 epoch 开始时
	// 仍有上一轮的消息在途（会导致响应/交易污染新一轮会话）。
	if success {
		waitDKGReady(t, dkgs)
	}

	edPub := dkgs[0].GetDkgPubKey(crypto.SuiteEd25519)
	blsPub := dkgs[0].GetDkgPubKey(crypto.SuiteBLS12381)
	return edPub, blsPub
}

func TestNetwork(t *testing.T) {
	os.RemoveAll("./token_chain_data")

	db, err := types.NewDB()
	if err != nil {
		require.NoErrorf(t, err, "failed store.InitDB")
		os.Exit(1)
	}
	defer db.Close()

	skipPartialSign = true

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

	dkgs := make([]*DKG, 0, len(nodes))
	for i, s := range peerSecret {
		nodeSecret, _ := crypto.PrivateKeyFromHex(s)
		peer, err := local.NewNetwork(nodeSecret, []string{}, nodes, uint32(0), uint32(0))
		require.NoErrorf(t, err, "failed peer.NewNetwork")
		dkg, err := NewDKG(nodeSecret, peer, Logger{NodeTag: "NODE " + fmt.Sprint(i)})
		require.NoErrorf(t, err, "failed NewDKG")
		go dkg.Start()
		dkgs = append(dkgs, dkg)
	}
	// 先于 db.Close() 执行（defer LIFO）：停止 DKG 并等待在途消息。
	// 注意：必须用闭包捕获 dkgs，因为 E2 阶段还会 append 新节点，
	// 直接传切片会固化为 3 个元素的旧 header，导致新增节点从未被 Stop。
	defer func() { stopDKGs(dkgs) }()

	// Epoch 1
	edPub1, blsPub1 := runEpoch(t, dkgs, validators, 1, "E1")
	require.NotNil(t, edPub1, "ed25519 DKG pubkey should not be nil")
	require.NotNil(t, blsPub1, "bls12381 DKG pubkey should not be nil")
	fmt.Println("E1 OK | ed25519 =", edPub1.SS58(), "| bls12381 =", fmt.Sprintf("0x%x", blsPub1.Byte()))

	// Epoch 2 — add 2 new peers
	for _, s := range newPeerSecret {
		nodeSecret, _ := crypto.PrivateKeyFromHex(s)
		nodes = append(nodes, nodeSecret.GetPublicForSuite(crypto.SuiteEd25519))
		validators = append(validators, &chaintypes.Validator{
			ValidatorId:    *nodeSecret.GetPublicForSuite(crypto.SuiteEd25519),
			P2pId:          *nodeSecret.GetPublicForSuite(crypto.SuiteEd25519),
			ValidatorIdBLS: *nodeSecret.GetPublicForSuite(crypto.SuiteBLS12381),
		})
	}

	for i, s := range newPeerSecret {
		nodeSecret, _ := crypto.PrivateKeyFromHex(s)
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

	edPub2, blsPub2 := runEpoch(t, dkgs, validators, 2, "E2")
	require.NotNil(t, edPub2, "ed25519 DKG pubkey should not be nil")
	require.NotNil(t, blsPub2, "bls12381 DKG pubkey should not be nil")
	fmt.Println("E2 OK | ed25519 =", edPub2.SS58(), "| bls12381 =", fmt.Sprintf("0x%x", blsPub2.Byte()))
	require.Equal(t, edPub1.SS58(), edPub2.SS58(), "ed25519 pubkey must be consistent")
	require.Equal(t, blsPub1.Byte(), blsPub2.Byte(), "bls12381 pubkey must be consistent")

	// Epoch 3
	runtime.Gosched()
	time.Sleep(200 * time.Millisecond)
	edPub3, blsPub3 := runEpoch(t, dkgs, validators, 3, "E3")
	require.NotNil(t, edPub3, "ed25519 DKG pubkey should not be nil")
	require.NotNil(t, blsPub3, "bls12381 DKG pubkey should not be nil")
	fmt.Println("E3 OK | ed25519 =", edPub3.SS58(), "| bls12381 =", fmt.Sprintf("0x%x", blsPub3.Byte()))
	require.Equal(t, edPub1.SS58(), edPub3.SS58(), "ed25519 pubkey must be consistent")
	require.Equal(t, blsPub1.Byte(), blsPub3.Byte(), "bls12381 pubkey must be consistent")
}
