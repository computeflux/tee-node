package main

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"testing"

	"github.com/computeflux/tee-provider/pkg/types/crypto"
	oed "github.com/oasisprotocol/curve25519-voi/primitives/ed25519"
)

func TestKeyGen_GenerateEd25519KeyPair(t *testing.T) {
	priv, pub, err := crypto.GenerateEd25519KeyPair(rand.Reader)
	if err != nil {
		t.Fatalf("GenerateEd25519KeyPair: %v", err)
	}
	if priv == nil || pub == nil {
		t.Fatal("nil key returned")
	}
	if len(priv.PrivateKey) != ed25519.PrivateKeySize {
		t.Fatalf("private key size = %d, want %d", len(priv.PrivateKey), ed25519.PrivateKeySize)
	}
	if len(pub.PublicKey) != ed25519.PublicKeySize {
		t.Fatalf("public key size = %d, want %d", len(pub.PublicKey), ed25519.PublicKeySize)
	}

	// 公钥必须与私钥派生一致
	derived := priv.PrivateKey.Public().(ed25519.PublicKey)
	if !bytes.Equal(derived, pub.PublicKey) {
		t.Fatal("public key does not match private key")
	}

	// 私钥签名必须能被公钥验证
	msg := []byte("hello world")
	sig := ed25519.Sign(priv.PrivateKey, msg)
	if !ed25519.Verify(pub.PublicKey, msg, sig) {
		t.Fatal("signature verification failed")
	}
}

func TestKeyGen_StdToOed25519_SignaturesMatch(t *testing.T) {
	// 两个 ed25519 实现（stdlib 与 curve25519-voi）都是确定性 RFC8032 签名。
	// 若 StdToOed25519 转换正确，同一私钥对同一消息的签名必须逐字节一致。
	_, privGo, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}

	privOasis, err := crypto.StdToOed25519(privGo)
	if err != nil {
		t.Fatalf("StdToOed25519: %v", err)
	}

	msg := []byte("hello world")
	sigStd := ed25519.Sign(privGo, msg)
	sigOed := oed.Sign(privOasis, msg)

	if !bytes.Equal(sigStd, sigOed) {
		t.Fatalf("signatures differ:\n std:  %x\n oed:  %x", sigStd, sigOed)
	}

	// 两个签名都能用 oed 公钥验证
	pubOasis := privOasis.Public().(oed.PublicKey)
	if !oed.Verify(pubOasis, msg, sigOed) {
		t.Fatal("oed verify failed")
	}
}

func TestKeyGen_StdToOed25519_InvalidLength(t *testing.T) {
	// 长度不是 64 字节必须报错而不是 panic
	if _, err := crypto.StdToOed25519(ed25519.PrivateKey{0x01}); err == nil {
		t.Fatal("expected error for short private key")
	}
	if _, err := crypto.StdToOed25519(nil); err == nil {
		t.Fatal("expected error for nil private key")
	}
}
