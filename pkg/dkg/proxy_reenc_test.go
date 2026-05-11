package dkg

// import (
// 	"crypto/rand"
// 	"fmt"
// 	"os"
// 	"testing"
// 	"time"

// 	"github.com/stretchr/testify/require"
// 	proxy_reenc "github.com/wetee-dao/tee-dsecret/pkg/dkg/proxy-reenc"
// 	"github.com/wetee-dao/tee-dsecret/pkg/model"
// 	"github.com/wetee-dao/tee-dsecret/pkg/network/local"
// 	"github.com/wetee-dao/tee-dsecret/pkg/util"
// 	"go.dedis.ch/kyber/v4/share"
// 	"go.dedis.ch/kyber/v4/util/random"
// )

// func TestReencrypt(t *testing.T) {
// 	os.RemoveAll("./chain_data")

// 	db, err := model.NewDB()
// 	if err != nil {
// 		require.NoErrorf(t, err, "failed store.InitDB")
// 		os.Exit(1)
// 	}
// 	defer db.Close()

// 	// Skip partial sign
// 	skipPartialSign = true

// 	nodes := []*model.PubKey{}
// 	validators := []*model.Validator{}
// 	for _, s := range peerSecret {
// 		nodeSecret, _ := model.PrivateKeyFromHex(s)
// 		nodes = append(nodes, nodeSecret.GetPublic())
// 		validators = append(validators, &model.Validator{
// 			ValidatorId: *nodeSecret.GetPublic(),
// 			P2pId:       *nodeSecret.GetPublic(),
// 		})
// 	}

// 	peers := make([]*local.Peer, 0, len(nodes))
// 	for _, s := range peerSecret {
// 		nodeSecret, _ := model.PrivateKeyFromHex(s)

// 		peer, err := local.NewNetwork(nodeSecret, []string{}, nodes, uint32(0), uint32(0))
// 		require.NoErrorf(t, err, "failed peer.NewNetwork")

// 		peers = append(peers, peer)
// 	}

// 	dkgs := make([]*DKG, 0, len(nodes))
// 	for i, s := range peerSecret {
// 		nodeSecret, _ := model.PrivateKeyFromHex(s)

// 		dkg, err := NewDKG(nodeSecret, peers[i], Logger{
// 			NodeTag: "NODE " + fmt.Sprint(i),
// 		})
// 		require.NoErrorf(t, err, "failed NewDKG")
// 		go dkg.Start()

// 		dkgs = append(dkgs, dkg)
// 	}

// 	err = dkgs[0].TryEpochConsensus(model.ConsensusMsg{
// 		Validators: validators,
// 		Epoch:      1,
// 	}, func(signer *DssSigner, nodeId uint64) {
// 		util.LogWithBlue("CONSENSUS SUCCESS", nodeId)
// 		for _, dkg := range dkgs {
// 			dkg.ToNewEpoch()
// 		}
// 	}, func(error) {
// 		util.LogWithBlue("CONSENSUS Error", err.Error())
// 	})
// 	if err != nil {
// 		t.Fatal(err)
// 	}

// 	time.Sleep(time.Second * 1)

// 	for _, d := range dkgs {
// 		util.LogWithYellow("V0 |||", d.DkgKeyShare.PriShare().String())
// 	}

// 	err = Reencrypt(dkgs)
// 	if err != nil {
// 		util.LogWithBlue("Reencrypt", err)
// 		t.Fatal(err)
// 	}
// }

// func Reencrypt(dkgs []*DKG) error {
// 	scrt := make([]byte, 32)
// 	random.Bytes(scrt, random.New())
// 	fmt.Println(scrt)

// 	var suite = dkgs[0].Suite
// 	currDKG := dkgs[0]

// 	// Encrypt the secret under the DKG public key.
// 	encCmt, encScrt := proxy_reenc.EncryptSecret(suite, currDKG.DkgPubKey.Point(), scrt)
// 	rawEncCmt, err := encCmt.MarshalBinary()
// 	if err != nil {
// 		return err
// 	}

// 	// 将加密的秘密（encScrt）转换为字节切片格式
// 	rawEncScrt := make([][]byte, len(encScrt))
// 	for i, encScrti := range encScrt {
// 		rawEncScrti, err := encScrti.MarshalBinary()
// 		if err != nil {
// 			return err
// 		}
// 		rawEncScrt[i] = rawEncScrti
// 	}

// 	secretData := model.Secret{
// 		EncCmt:  rawEncCmt,
// 		EncScrt: rawEncScrt,
// 	}

// 	rdrSk, rdrPk, err := model.GenerateEd25519KeyPair(rand.Reader)
// 	if err != nil {
// 		fmt.Println(err)
// 		return err
// 	}

// 	replys := make([]proxy_reenc.ReencryptReply, 0, len(dkgs))
// 	for _, d := range dkgs {
// 		share := d.Share()

// 		// 解析加密的承诺
// 		encCmt := suite.Point().Base()
// 		err = encCmt.UnmarshalBinary(rawEncCmt)
// 		if err != nil {
// 			return fmt.Errorf("unmarshal encrypted commitment: %s", err)
// 		}

// 		reply, err := proxy_reenc.Reencrypt(share, &secretData, *rdrPk)
// 		if err != nil {
// 			return fmt.Errorf("reencrypt: %w", err)
// 		}

// 		replys = append(replys, reply)
// 	}

// 	shares := make([]*share.PubShare, 0, len(dkgs))
// 	for _, reply := range replys {
// 		distKeyShare := currDKG.Share()
// 		poly := share.NewPubPoly(suite, nil, distKeyShare.Commitments())
// 		err = proxy_reenc.Verify(*rdrPk, poly, encCmt, reply)
// 		if err != nil {
// 			return fmt.Errorf("verify reencrypt reply: %s", err)
// 		}

// 		shares = append(shares, &reply.Share)
// 	}

// 	// 从收集的响应中恢复重加密承诺
// 	xncCmt, err := proxy_reenc.Recover(suite, shares, currDKG.Threshold, len(dkgs))
// 	if err != nil {
// 		return fmt.Errorf("recover reencrypt reply: %s", err)
// 	}

// 	scrtHat, err := proxy_reenc.DecryptSecret(suite, encScrt, currDKG.DkgPubKey.Point(), xncCmt, rdrSk.Scalar())
// 	if err != nil {
// 		return fmt.Errorf("decrypt secret: %s", err)
// 	}
// 	fmt.Println(scrtHat)

// 	return nil
// }
