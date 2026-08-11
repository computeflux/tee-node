package main

import (
	"testing"

	"go.dedis.ch/kyber/v4"
	"go.dedis.ch/kyber/v4/group/edwards25519"
)

func newTestSigners(t *testing.T, n int) ([]*Signer, []kyber.Point, []kyber.Point) {
	t.Helper()
	suite := edwards25519.NewBlakeSHA256Ed25519()
	signers := make([]*Signer, n)
	pubs := make([]kyber.Point, n)
	pubNonces := make([]kyber.Point, n)
	for i := range signers {
		signers[i] = NewSigner(suite)
		pubs[i] = signers[i].PubKey
		pubNonces[i] = signers[i].PubNonce
	}
	return signers, pubs, pubNonces
}

func aggregateAll(t *testing.T, suite kyber.Group, signers []*Signer, pubs, pubNonces []kyber.Point, message []byte) (kyber.Point, kyber.Point, kyber.Scalar) {
	t.Helper()
	aggedPub := AggPubKeys(suite, pubs)
	aggedPubNonce := AggPubNonceKeys(suite, pubNonces)
	partialSigs := make([]kyber.Scalar, 0, len(signers))
	for i := range signers {
		partialSigs = append(partialSigs, PartialSig(suite, signers[i], message, aggedPub, aggedPubNonce))
	}
	return aggedPub, aggedPubNonce, AggSigs(suite, partialSigs)
}

func TestMultisig_SingleSigner(t *testing.T) {
	suite := edwards25519.NewBlakeSHA256Ed25519()
	signers, pubs, pubNonces := newTestSigners(t, 1)
	message := []byte("hello multisig")

	aggedPub, aggedPubNonce, aggedSig := aggregateAll(t, suite, signers, pubs, pubNonces, message)

	if !VerifySig(suite, aggedPub, aggedPubNonce, message, aggedSig) {
		t.Fatal("single signer aggregate signature must verify")
	}
}

func TestMultisig_ThreeSigners(t *testing.T) {
	suite := edwards25519.NewBlakeSHA256Ed25519()
	signers, pubs, pubNonces := newTestSigners(t, 3)
	message := []byte("This is a test message for multi-signature.")

	aggedPub, aggedPubNonce, aggedSig := aggregateAll(t, suite, signers, pubs, pubNonces, message)

	if !VerifySig(suite, aggedPub, aggedPubNonce, message, aggedSig) {
		t.Fatal("3-signer aggregate signature must verify")
	}

	// 篡改消息后必须验证失败
	if VerifySig(suite, aggedPub, aggedPubNonce, []byte("tampered"), aggedSig) {
		t.Fatal("tampered message must not verify")
	}
}

func TestMultisig_ManySigners(t *testing.T) {
	// 与 main 相同的 300 签名者规模
	suite := edwards25519.NewBlakeSHA256Ed25519()
	signers, pubs, pubNonces := newTestSigners(t, 300)
	message := []byte("This is a test message for multi-signature.")

	aggedPub, aggedPubNonce, aggedSig := aggregateAll(t, suite, signers, pubs, pubNonces, message)

	if !VerifySig(suite, aggedPub, aggedPubNonce, message, aggedSig) {
		t.Fatal("300-signer aggregate signature must verify")
	}
}

func TestMultisig_EmptyAggregation(t *testing.T) {
	suite := edwards25519.NewBlakeSHA256Ed25519()

	aggedPub := AggPubKeys(suite, nil)
	if !aggedPub.Equal(suite.Point().Null()) {
		t.Fatal("empty pub aggregation must be null point")
	}
	aggedNonce := AggPubNonceKeys(suite, nil)
	if !aggedNonce.Equal(suite.Point().Null()) {
		t.Fatal("empty nonce aggregation must be null point")
	}
	aggedSig := AggSigs(suite, nil)
	if !aggedSig.Equal(suite.Scalar().Zero()) {
		t.Fatal("empty sig aggregation must be zero scalar")
	}
}

func TestMultisig_MissingSignerFails(t *testing.T) {
	suite := edwards25519.NewBlakeSHA256Ed25519()
	signers, pubs, pubNonces := newTestSigners(t, 3)
	message := []byte("msg")

	aggedPub, aggedPubNonce, aggedSig := aggregateAll(t, suite, signers[:2], pubs[:2], pubNonces[:2], message)

	// 用 3 个签名者的公钥聚合验证，但只用 2 个签名者签名 → 必须失败
	fullPub := AggPubKeys(suite, pubs)
	if VerifySig(suite, fullPub, aggedPubNonce, message, aggedSig) {
		t.Fatal("incomplete signature set must not verify")
	}
	_ = aggedPub
}

func TestPartialSig_Deterministic(t *testing.T) {
	suite := edwards25519.NewBlakeSHA256Ed25519()
	signers, pubs, pubNonces := newTestSigners(t, 2)
	message := []byte("msg")

	aggedPub := AggPubKeys(suite, pubs)
	aggedPubNonce := AggPubNonceKeys(suite, pubNonces)

	s1 := PartialSig(suite, signers[0], message, aggedPub, aggedPubNonce)
	s2 := PartialSig(suite, signers[0], message, aggedPub, aggedPubNonce)
	if !s1.Equal(s2) {
		t.Fatal("partial signature must be deterministic")
	}

	// 不同消息 → 不同部分签名
	s3 := PartialSig(suite, signers[0], []byte("other"), aggedPub, aggedPubNonce)
	if s1.Equal(s3) {
		t.Fatal("different message must produce different partial signature")
	}
}
