package main

import (
	"crypto/sha256"
	"fmt"
	"io"
	"log"

	"go.dedis.ch/kyber/v4"
	"go.dedis.ch/kyber/v4/group/edwards25519"
	"go.dedis.ch/kyber/v4/util/random"
)

// Signer 结构体，包含密钥对和 nonce
type Signer struct {
	PrivKey  kyber.Scalar
	PubKey   kyber.Point
	Nonce    kyber.Scalar
	PubNonce kyber.Point
}

// NewSigner 创建一个新的签名者
func NewSigner(suite kyber.Group) *Signer {
	privateKey := suite.Scalar().Pick(random.New())
	publicKey := suite.Point().Mul(privateKey, nil) // nil 表示基点 G
	nonce := suite.Scalar().Pick(random.New())
	publicNonce := suite.Point().Mul(nonce, nil)
	return &Signer{
		PrivKey: privateKey,
		PubKey:  publicKey,

		Nonce:    nonce,
		PubNonce: publicNonce,
	}
}

// AggPubKeys 聚合公钥
func AggPubKeys(suite kyber.Group, pubs []kyber.Point) kyber.Point {
	aggedPubKey := suite.Point().Null() // 零点
	for _, pub := range pubs {
		aggedPubKey = suite.Point().Add(aggedPubKey, pub)
	}
	return aggedPubKey
}

// AggPubNonceKeys 聚合 Nonce 公钥
func AggPubNonceKeys(suite kyber.Group, pubNonces []kyber.Point) kyber.Point {
	aggedPubNonceKey := suite.Point().Null() // 零点
	for _, n := range pubNonces {
		aggedPubNonceKey = suite.Point().Add(aggedPubNonceKey, n)
	}
	return aggedPubNonceKey
}

// PartialSig 生成部分签名
func PartialSig(suite kyber.Group, signer *Signer, message []byte, aggedPubKey kyber.Point, aggedPubNonceKey kyber.Point) kyber.Scalar {
	// 1. 计算挑战值 e = H(R || PubKey || Message)
	hasher := sha256.New()
	_, err := io.WriteString(hasher, fmt.Sprintf("%v%v%s", aggedPubNonceKey, aggedPubKey, message))
	if err != nil {
		log.Fatal(err)
	}
	e := suite.Scalar().SetBytes(hasher.Sum(nil))

	// 2. 计算 s = nonce + e * sk
	s := suite.Scalar().Mul(e, signer.PrivKey)
	s = suite.Scalar().Add(s, signer.Nonce)

	return s
}

// AggSigs 聚合签名
func AggSigs(suite kyber.Group, partialSigs []kyber.Scalar) kyber.Scalar {
	aggedSig := suite.Scalar().Zero()
	for _, sig := range partialSigs {
		aggedSig = suite.Scalar().Add(aggedSig, sig)
	}
	return aggedSig
}

// VerifySig 验证签名
func VerifySig(suite kyber.Group, aggedPubKey kyber.Point, aggedPubNonceKey kyber.Point, message []byte, aggedSig kyber.Scalar) bool {
	// 1. 计算挑战值 e = H(R || PubKey || Message)
	hasher := sha256.New()
	_, err := io.WriteString(hasher, fmt.Sprintf("%v%v%s", aggedPubNonceKey, aggedPubKey, message))
	if err != nil {
		log.Fatal(err)
	}
	e := suite.Scalar().SetBytes(hasher.Sum(nil))

	// 2. 验证 s * G == R + e * PubKey
	sG := suite.Point().Mul(aggedSig, nil) // nil 表示基点 G
	ePK := suite.Point().Mul(e, aggedPubKey)
	RPlusEPK := suite.Point().Add(aggedPubNonceKey, ePK)

	return sG.Equal(RPlusEPK)
}

func main() {
	// 选择椭圆曲线
	suite := edwards25519.NewBlakeSHA256Ed25519()

	// 消息
	message := []byte("This is a test message for multi-signature.")

	// 创建多个签名者
	numSigners := 300
	signers := make([]*Signer, numSigners)
	for i := range numSigners {
		signers[i] = NewSigner(suite)
	}

	pubs := make([]kyber.Point, numSigners)
	pubNonces := make([]kyber.Point, numSigners)
	for i := range numSigners {
		pubs[i] = signers[i].PubKey
		pubNonces[i] = signers[i].PubNonce
	}

	// 聚合公钥
	aggedPub := AggPubKeys(suite, pubs)
	fmt.Printf("Aggd Pub Key: %v\n", aggedPub)

	// 聚合 Nonce 公钥
	aggedPubNonce := AggPubNonceKeys(suite, pubNonces)
	fmt.Printf("Aggd Nonce Pub Key: %v\n", aggedPubNonce)

	// 生成部分签名
	partialSigs := make([]kyber.Scalar, 0, numSigners)
	for i := range numSigners {
		partialSigs = append(partialSigs, PartialSig(suite, signers[i], message, aggedPub, aggedPubNonce))
	}

	// 聚合签名
	aggedSig := AggSigs(suite, partialSigs)
	fmt.Printf("Aggd Sig: %v\n", aggedSig)

	// 验证签名
	isValid := VerifySig(suite, aggedPub, aggedPubNonce, message, aggedSig)
	fmt.Printf("Sig is valid: %v\n", isValid)
}
