// Copyright (c) 2022 Sourcenetwork Developers. All rights reserved.
// copy from https://github.com/sourcenetwork/orbis-go

package proxy_reenc

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.dedis.ch/kyber/v4"
	"go.dedis.ch/kyber/v4/share"
	"go.dedis.ch/kyber/v4/suites"
	"go.dedis.ch/kyber/v4/util/random"

	dkgtypes "github.com/computeflux/tee-provider/pkg/dkg/types"
	"github.com/computeflux/tee-provider/pkg/model"
	crypto "github.com/computeflux/tee-provider/pkg/types/crypto"
)

func setupReencTest(t *testing.T) (
	suite suites.Suite,
	priPoly *share.PriPoly,
	pubPoly *share.PubPoly,
	dkgCommit kyber.Point,
	clientPriv kyber.Scalar,
	clientPub kyber.Point,
	clientPubKey *crypto.PubKey,
) {
	suite = suites.MustFind("Ed25519")

	s := suite.Scalar().Pick(suite.RandomStream())
	priPoly = share.NewPriPoly(suite, 3, s, suite.RandomStream())
	pubPoly = priPoly.Commit(nil)
	dkgCommit = pubPoly.Commit()

	clientPriv = suite.Scalar().Pick(suite.RandomStream())
	clientPub = suite.Point().Mul(clientPriv, nil)

	var err error
	clientPubKey, err = crypto.PubKeyFromPoint(clientPub)
	require.NoError(t, err)

	return
}

func makeSecretStore(t *testing.T, encCmt kyber.Point) *model.SecretStore {
	rawEncCmt, err := encCmt.MarshalBinary()
	require.NoError(t, err)
	return &model.SecretStore{RawEncCmt: rawEncCmt}
}

func makeDistKeyShare(t *testing.T, suite suites.Suite, idx uint32, priPoly *share.PriPoly, pubPoly *share.PubPoly) dkgtypes.DistKeyShare {
	return dkgtypes.DistKeyShare{
		CommitsWrap: dkgtypes.KyberPoints{
			Public: []kyber.Point{pubPoly.Eval(idx).V},
		},
		PriShareWrap: dkgtypes.PriShareWithSuite{
			PriShare: &share.PriShare{
				I: idx,
				V: priPoly.Eval(idx).V,
			},
			Suite: "ed25519",
		},
	}
}

func TestInlineReencryptAndVerify(t *testing.T) {
	var (
		n     = 5 //参与者的数量
		th    = 3 // 门限值
		suite = suites.MustFind("Ed25519")

		s       = suite.Scalar().Pick(suite.RandomStream())
		priPoly = share.NewPriPoly(suite, uint32(th), s, suite.RandomStream())
		pubPoly = priPoly.Commit(nil)

		dkgCommit = pubPoly.Commit() // DKG 公钥

		clientPriv = suite.Scalar().Pick(suite.RandomStream())
		clientPub  = suite.Point().Mul(clientPriv, nil)
	)

	var pubShares []*share.PubShare

	// Generate a random secret.
	scrt := make([]byte, 32)
	random.Bytes(scrt, random.New())

	// 1. Encrypt the secret under the DKG public key.
	encCmt, encScrt := EncryptSecret(suite, dkgCommit, scrt)

	for idx := range n {
		dkgSki := priPoly.Eval(uint32(idx)).V
		dkgCmt := pubPoly.Eval(uint32(idx)).V

		// 2. Re-encrypt the key under the reader's public key.
		xncSki, chlgi, proofi, err := reencrypt(suite, dkgSki, clientPub, encCmt)
		require.NoErrorf(t, err, "failed to reencrypt for share %d", idx)

		// 3. Verify the re-encryption from other nodes.
		err = verify(suite, clientPub, encCmt, xncSki, chlgi, proofi, dkgCmt)
		require.NoErrorf(t, err, "failed to verify reencryption for share %d", idx)

		pubShare := &share.PubShare{I: uint32(idx), V: xncSki}
		pubShares = append(pubShares, pubShare)
	}

	// 4 - Recover re-encrypted commmitment using Lagrange interpolation.
	// ski * (xG + rG) => rsG + xsG
	xncCmt, err := share.RecoverCommit(suite, pubShares, uint32(th), uint32(n))
	require.NoErrorf(t, err, "failed to recover commit")

	// 5 - Decode encrypted key with re-encrypted commitment and reader's privatekey.
	scrtHat, err := DecryptSecret(suite, encScrt, dkgCommit, xncCmt, clientPriv)
	require.NoErrorf(t, err, "failed to decode key")
	require.Equal(t, scrt, scrtHat)
}

func TestReencryptAndVerify(t *testing.T) {
	var (
		n     = 5 //参与者的数量
		th    = 3 // 门限值
		suite = suites.MustFind("Ed25519")

		s       = suite.Scalar().Pick(suite.RandomStream())
		priPoly = share.NewPriPoly(suite, uint32(th), s, suite.RandomStream())
		pubPoly = priPoly.Commit(nil)

		dkgCommit = pubPoly.Commit() // DKG 公钥

		clientPriv = suite.Scalar().Pick(suite.RandomStream())
		clientPub  = suite.Point().Mul(clientPriv, nil)
	)

	var pubShares []*share.PubShare

	clientPubKey, _ := crypto.PubKeyFromPoint(clientPub)

	// Generate a random secret.
	scrt := make([]byte, 32)
	random.Bytes(scrt, random.New())

	// 1. Encrypt the secret under the DKG public key.
	encCmt, encScrt := EncryptSecret(suite, dkgCommit, scrt)
	// 将加密的承诺（encCmt）转换为字节切片格式
	rawEncCmt, err := encCmt.MarshalBinary()
	if err != nil {
		panic(err)
	}

	// 将加密的秘密（encScrt）转换为字节切片格式
	rawEncScrt := make([][]byte, len(encScrt))
	for i, encScrti := range encScrt {
		rawEncScrti, err := encScrti.MarshalBinary()
		if err != nil {
			panic(err)
		}
		rawEncScrt[i] = rawEncScrti
	}

	secretStore := &model.SecretStore{
		RawEncCmt: rawEncCmt,
		// RawEncScrt: rawEncScrt,
	}

	for idx := range n {
		dkgSki := priPoly.Eval(uint32(idx)).V
		dkgCmt := pubPoly.Eval(uint32(idx)).V

		distKeyShare := dkgtypes.DistKeyShare{
			CommitsWrap: dkgtypes.KyberPoints{
				Public: []kyber.Point{dkgCmt},
			},
			PriShareWrap: dkgtypes.PriShareWithSuite{
				PriShare: &share.PriShare{
					I: uint32(idx),
					V: dkgSki,
				},
				Suite: "ed25519",
			},
		}

		poly := share.NewPubPoly(suite, nil, distKeyShare.CommitsWrap.Public)

		// 2. Re-encrypt the key under the reader's public key.
		reply, err := Reencrypt(distKeyShare, secretStore, *clientPubKey)
		require.NoErrorf(t, err, "failed to reencrypt for share %d", idx)

		// 3. Verify the re-encryption from other nodes.
		err = Verify(poly, secretStore, *clientPubKey, reply)
		require.NoErrorf(t, err, "failed to verify reencryption for share %d", idx)

		pubShare := &share.PubShare{I: uint32(idx), V: reply.Share.V}
		pubShares = append(pubShares, pubShare)
	}

	// 4 - Recover re-encrypted commmitment using Lagrange interpolation.
	// ski * (xG + rG) => rsG + xsG
	xncCmt, err := share.RecoverCommit(suite, pubShares, uint32(th), uint32(n))
	require.NoErrorf(t, err, "failed to recover commit")

	// 5 - Decode encrypted key with re-encrypted commitment and reader's privatekey.
	scrtHat, err := DecryptSecret(suite, encScrt, dkgCommit, xncCmt, clientPriv)
	require.NoErrorf(t, err, "failed to decode key")
	require.Equal(t, scrt, scrtHat)
}

// ── Recover ────────────────────────────────────────────────

func TestRecover_InsufficientShares(t *testing.T) {
	suite := suites.MustFind("Ed25519")
	// Only 2 shares, but threshold is 3
	shares := []*share.PubShare{
		{I: 0, V: suite.Point().Base()},
		{I: 1, V: suite.Point().Base()},
	}
	result, err := Recover(suite, shares, 3, 5)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "insufficient shares")
	assert.Nil(t, result)
}

func TestRecover_ExactThreshold(t *testing.T) {
	suite, priPoly, pubPoly, dkgCommit, clientPriv, _, clientPubKey := setupReencTest(t)

	scrt := []byte("hello")
	encCmt, encScrt := EncryptSecret(suite, dkgCommit, scrt)
	secretStore := makeSecretStore(t, encCmt)

	// Use exactly 3 shares out of 5 (threshold=3)
	var pubShares []*share.PubShare
	for idx := range 3 {
		dks := makeDistKeyShare(t, suite, uint32(idx), priPoly, pubPoly)
		poly := share.NewPubPoly(suite, nil, dks.CommitsWrap.Public)

		reply, err := Reencrypt(dks, secretStore, *clientPubKey)
		require.NoError(t, err)

		err = Verify(poly, secretStore, *clientPubKey, reply)
		require.NoError(t, err)

		pubShares = append(pubShares, &share.PubShare{I: reply.Share.I, V: reply.Share.V})
	}

	xncCmt, err := Recover(suite, pubShares, 3, 5)
	require.NoError(t, err)
	require.NotNil(t, xncCmt)

	scrtHat, err := DecryptSecret(suite, encScrt, dkgCommit, xncCmt, clientPriv)
	require.NoError(t, err)
	assert.Equal(t, scrt, scrtHat)
}

// ── EncryptSecret / DecryptSecret ──────────────────────────

func TestEncryptDecryptSecret_Empty(t *testing.T) {
	suite := suites.MustFind("Ed25519")
	dkgPk := suite.Point().Mul(suite.Scalar().Pick(suite.RandomStream()), nil)

	encCmt, encScrt := EncryptSecret(suite, dkgPk, []byte{})
	assert.NotNil(t, encCmt)
	assert.Empty(t, encScrt)
}

func TestEncryptDecryptSecret_LongSecret(t *testing.T) {
	suite, priPoly, pubPoly, dkgCommit, clientPriv, _, clientPubKey := setupReencTest(t)

	// Secret longer than a single point can embed (Ed25519 ~32 bytes per point)
	scrt := make([]byte, 128)
	random.Bytes(scrt, random.New())

	encCmt, encScrt := EncryptSecret(suite, dkgCommit, scrt)
	require.NotNil(t, encCmt)
	require.True(t, len(encScrt) > 1, "long secret should be split into multiple points")

	secretStore := makeSecretStore(t, encCmt)

	var pubShares []*share.PubShare
	for idx := range 5 {
		dks := makeDistKeyShare(t, suite, uint32(idx), priPoly, pubPoly)
		poly := share.NewPubPoly(suite, nil, dks.CommitsWrap.Public)

		reply, err := Reencrypt(dks, secretStore, *clientPubKey)
		require.NoError(t, err)

		err = Verify(poly, secretStore, *clientPubKey, reply)
		require.NoError(t, err)

		pubShares = append(pubShares, &share.PubShare{I: reply.Share.I, V: reply.Share.V})
	}

	xncCmt, err := Recover(suite, pubShares, 3, 5)
	require.NoError(t, err)

	scrtHat, err := DecryptSecret(suite, encScrt, dkgCommit, xncCmt, clientPriv)
	require.NoError(t, err)
	assert.Equal(t, scrt, scrtHat)
}

// ── Verify Failure ─────────────────────────────────────────

func TestVerify_TamperedChallenge(t *testing.T) {
	suite, priPoly, pubPoly, dkgCommit, _, _, clientPubKey := setupReencTest(t)

	scrt := []byte("tamper-test")
	encCmt, _ := EncryptSecret(suite, dkgCommit, scrt)
	secretStore := makeSecretStore(t, encCmt)

	dks := makeDistKeyShare(t, suite, 0, priPoly, pubPoly)
	poly := share.NewPubPoly(suite, nil, dks.CommitsWrap.Public)

	reply, err := Reencrypt(dks, secretStore, *clientPubKey)
	require.NoError(t, err)

	// Tamper with the challenge
	reply.Challenge = suite.Scalar().Add(reply.Challenge, suite.Scalar().One())

	err = Verify(poly, secretStore, *clientPubKey, reply)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "verification")
}

func TestVerify_TamperedProof(t *testing.T) {
	suite, priPoly, pubPoly, dkgCommit, _, _, clientPubKey := setupReencTest(t)

	scrt := []byte("tamper-test")
	encCmt, _ := EncryptSecret(suite, dkgCommit, scrt)
	secretStore := makeSecretStore(t, encCmt)

	dks := makeDistKeyShare(t, suite, 0, priPoly, pubPoly)
	poly := share.NewPubPoly(suite, nil, dks.CommitsWrap.Public)

	reply, err := Reencrypt(dks, secretStore, *clientPubKey)
	require.NoError(t, err)

	// Tamper with the proof
	reply.Proof = suite.Scalar().Add(reply.Proof, suite.Scalar().One())

	err = Verify(poly, secretStore, *clientPubKey, reply)
	assert.Error(t, err)
}

func TestVerify_WrongClientPubKey(t *testing.T) {
	suite, priPoly, pubPoly, dkgCommit, _, _, clientPubKey := setupReencTest(t)

	// Wrong client public key
	wrongPriv := suite.Scalar().Pick(suite.RandomStream())
	wrongPub := suite.Point().Mul(wrongPriv, nil)
	wrongPubKey, err := crypto.PubKeyFromPoint(wrongPub)
	require.NoError(t, err)

	scrt := []byte("wrong-client")
	encCmt, _ := EncryptSecret(suite, dkgCommit, scrt)
	secretStore := makeSecretStore(t, encCmt)

	dks := makeDistKeyShare(t, suite, 0, priPoly, pubPoly)
	poly := share.NewPubPoly(suite, nil, dks.CommitsWrap.Public)

	reply, err := Reencrypt(dks, secretStore, *clientPubKey)
	require.NoError(t, err)

	// Verify with wrong client key should fail
	err = Verify(poly, secretStore, *wrongPubKey, reply)
	assert.Error(t, err)
}

// ── Reencrypt Invalid Input ────────────────────────────────

func TestReencrypt_InvalidEncCmt(t *testing.T) {
	suite, priPoly, pubPoly, _, _, _, clientPubKey := setupReencTest(t)

	secretStore := &model.SecretStore{RawEncCmt: []byte("not-a-valid-point")}
	dks := makeDistKeyShare(t, suite, 0, priPoly, pubPoly)

	_, err := Reencrypt(dks, secretStore, *clientPubKey)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unmarshal encCmt")
}

// ── Nil Guards ─────────────────────────────────────────────

func TestReencrypt_NilPriShare(t *testing.T) {
	suite, _, pubPoly, dkgCommit, _, _, clientPubKey := setupReencTest(t)

	scrt := []byte("nil-pri-share")
	encCmt, _ := EncryptSecret(suite, dkgCommit, scrt)
	secretStore := makeSecretStore(t, encCmt)

	// 空 PriShare（网络/存储损坏数据可能触发）
	badShare := dkgtypes.DistKeyShare{
		CommitsWrap: dkgtypes.KyberPoints{
			Public: []kyber.Point{pubPoly.Eval(0).V},
		},
	}
	_, err := Reencrypt(badShare, secretStore, *clientPubKey)
	require.Error(t, err)

	// 共享中的 V 为 nil
	badShare2 := dkgtypes.DistKeyShare{
		CommitsWrap: dkgtypes.KyberPoints{
			Public: []kyber.Point{pubPoly.Eval(0).V},
		},
		PriShareWrap: dkgtypes.PriShareWithSuite{
			PriShare: &share.PriShare{I: 0},
			Suite:    "ed25519",
		},
	}
	_, err = Reencrypt(badShare2, secretStore, *clientPubKey)
	require.Error(t, err)
}

func TestReencrypt_NilSecret(t *testing.T) {
	suite, priPoly, _, _, _, _, clientPubKey := setupReencTest(t)

	dks := dkgtypes.DistKeyShare{
		CommitsWrap: dkgtypes.KyberPoints{
			Public: []kyber.Point{suite.Point().Base()},
		},
		PriShareWrap: dkgtypes.PriShareWithSuite{
			PriShare: &share.PriShare{
				I: 0,
				V: priPoly.Eval(0).V,
			},
			Suite: "ed25519",
		},
	}

	// nil secret
	_, err := Reencrypt(dks, nil, *clientPubKey)
	require.Error(t, err)

	// 空 encCmt
	_, err = Reencrypt(dks, &model.SecretStore{}, *clientPubKey)
	require.Error(t, err)
}

func TestVerify_NilReply(t *testing.T) {
	suite, priPoly, pubPoly, dkgCommit, _, _, clientPubKey := setupReencTest(t)

	scrt := []byte("nil-reply")
	encCmt, _ := EncryptSecret(suite, dkgCommit, scrt)
	secretStore := makeSecretStore(t, encCmt)

	dks := makeDistKeyShare(t, suite, 0, priPoly, pubPoly)
	poly := share.NewPubPoly(suite, nil, dks.CommitsWrap.Public)

	t.Run("nil_reply", func(t *testing.T) {
		err := Verify(poly, secretStore, *clientPubKey, nil)
		require.Error(t, err)
	})

	t.Run("nil_share_point", func(t *testing.T) {
		err := Verify(poly, secretStore, *clientPubKey, &ReencryptReply{
			Challenge: suite.Scalar(),
			Proof:     suite.Scalar(),
		})
		require.Error(t, err)
	})

	t.Run("nil_challenge_or_proof", func(t *testing.T) {
		reply, err := Reencrypt(dks, secretStore, *clientPubKey)
		require.NoError(t, err)
		reply.Challenge = nil
		err = Verify(poly, secretStore, *clientPubKey, reply)
		require.Error(t, err)
	})

	t.Run("nil_poly", func(t *testing.T) {
		reply, err := Reencrypt(dks, secretStore, *clientPubKey)
		require.NoError(t, err)
		err = Verify(nil, secretStore, *clientPubKey, reply)
		require.Error(t, err)
	})

	t.Run("nil_secret", func(t *testing.T) {
		reply, err := Reencrypt(dks, secretStore, *clientPubKey)
		require.NoError(t, err)
		err = Verify(poly, nil, *clientPubKey, reply)
		require.Error(t, err)
	})
}

func TestVerify_NilClientPoint(t *testing.T) {
	suite, priPoly, pubPoly, dkgCommit, _, _, clientPubKey := setupReencTest(t)

	scrt := []byte("nil-client-point")
	encCmt, _ := EncryptSecret(suite, dkgCommit, scrt)
	secretStore := makeSecretStore(t, encCmt)

	dks := makeDistKeyShare(t, suite, 0, priPoly, pubPoly)
	poly := share.NewPubPoly(suite, nil, dks.CommitsWrap.Public)

	// 非法字节长度的公钥 -> Point() 返回 nil
	badPub := crypto.PubKey{}
	badPub.PublicKey = []byte("too-short")

	_, err := Reencrypt(dks, secretStore, badPub)
	require.Error(t, err)

	reply, err := Reencrypt(dks, secretStore, *clientPubKey)
	require.NoError(t, err)
	err = Verify(poly, secretStore, badPub, reply)
	require.Error(t, err)
}

func TestDecryptSecret_NilArgs(t *testing.T) {
	suite := suites.MustFind("Ed25519")
	point := suite.Point().Base()
	scalar := suite.Scalar()

	_, err := DecryptSecret(suite, nil, nil, point, scalar)
	require.Error(t, err)
	_, err = DecryptSecret(suite, nil, point, point, nil)
	require.Error(t, err)
}

func TestHashPoints(t *testing.T) {
	suite := suites.MustFind("Ed25519")
	p1 := suite.Point().Base()
	p2 := suite.Point().Mul(suite.Scalar().One(), nil)

	h1, err := hashPoints(p1)
	require.NoError(t, err)
	assert.Len(t, h1, 32)

	h2, err := hashPoints(p1, p2)
	require.NoError(t, err)
	assert.Len(t, h2, 32)

	// Different inputs should produce different hashes
	assert.NotEqual(t, h1, h2)
}
